package search

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

func TestService_Search_hybridLexical(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID:                  "1",
		Kind:                models.CapabilityKindTool,
		SourceID:            "github-gateway",
		SourceType:          "gateway",
		CanonicalName:       "github_gateway__create_issue",
		OriginalName:        "create_issue",
		OriginalDescription: "Create an issue in a repo",
		GeneratedSummary:    "Creates GitHub issues with title and body.",
		Tags:                []string{"title", "body", "repo"},
		InputSchemaJSON:     `{"properties":{"repo":{"type":"string"},"title":{"type":"string"}}}`,
		VersionHash:         "h1",
		LastSeenAt:          time.Now(),
	}
	rec.SearchText = "github-gateway create_issue repo title body issue"
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "create github issue", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != rec.CanonicalName {
		t.Fatalf("got %v", out.Results[0])
	}
	if out.Results[0].Kind != models.CapabilityKindTool {
		t.Fatalf("kind: got %q want tool", out.Results[0].Kind)
	}
}

func TestRegressionAnchorTop3(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantTop string
		records []models.CapabilityRecord
	}{
		{
			name:    "exact_routing_office_word_from_markdown",
			query:   "office__word_from_markdown",
			wantTop: "office__word_from_markdown",
			records: []models.CapabilityRecord{
				capRec("word-exact", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document populated from Markdown summary content.", "office word_from_markdown create document markdown summary"),
				capRec("word-weak", "office__word_template_placeholders", "word_template_placeholders", "Inspects Word templates and placeholders.", "office word_template_placeholders inspect template placeholders word"),
				capRec("ppt-weak", "office__read_powerpoint_speaker_notes", "read_powerpoint_speaker_notes", "Reads speaker notes from PowerPoint.", "office powerpoint speaker notes read deck slides"),
			},
		},
		{
			name:    "paraphrase_word_from_markdown",
			query:   "create a new Word document populated with a summary of this conversation",
			wantTop: "office__word_from_markdown",
			records: []models.CapabilityRecord{
				capRec("word-paraphrase", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document populated from Markdown summary, conversation summary, notes, or report content.", "office word_from_markdown creates new word document populated summary conversation markdown report notes"),
				capRec("docs-paraphrase", "docs__read_document_fully", "read_document_fully", "Reads a document end to end.", "docs read document fully content"),
			},
		},
		{
			name:    "conversational_firewall_premium_price",
			query:   "firewall premium price",
			wantTop: "azure__firewall_premium_price",
			records: []models.CapabilityRecord{
				capRec("az-firewall", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
				capRec("az-prices", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure pricing by service sku and region.", "azure query prices service sku region quantity cached pricing"),
			},
		},
		{
			name:    "parameter_service_sku_region_quantity",
			query:   "service sku region quantity",
			wantTop: "azure__azure_query_prices",
			records: []models.CapabilityRecord{
				capRec("az-prices", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure service pricing by sku, region, and quantity.", "azure azure_query_prices service sku region quantity price cached"),
				capRec("az-firewall", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
			},
		},
		{
			name:    "ambiguous_speaker_notes_powerpoint",
			query:   "speaker notes powerpoint",
			wantTop: "office__read_powerpoint_speaker_notes",
			records: []models.CapabilityRecord{
				capRec("ppt-speak", "office__read_powerpoint_speaker_notes", "read_powerpoint_speaker_notes", "Reads speaker notes from a PowerPoint deck.", "office powerpoint speaker notes read deck slides"),
				capRec("word-summary", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input.", "office word_from_markdown markdown summary report"),
				capRec("docs-read", "docs__read_document_fully", "read_document_fully", "Reads a document end to end.", "docs read document fully content"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "s.db")
			st, err := storage.OpenSQLite(p)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = st.Close() }()

			ctx := context.Background()
			for _, rec := range tc.records {
				if err := st.UpsertCapability(ctx, rec); err != nil {
					t.Fatal(err)
				}
			}

			svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
			out, err := svc.Search(ctx, models.SearchQuery{Text: tc.query, Limit: 3})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Results) < 1 {
				t.Fatal("no results")
			}
			if out.Results[0].ProxyToolName != tc.wantTop {
				t.Fatalf("expected %s first, got %+v", tc.wantTop, out.Results)
			}

			if tc.name == "exact_routing_office_word_from_markdown" {
				if len(out.Results) < 2 {
					t.Fatalf("expected a weaker match behind exact hit, got %+v", out.Results)
				}
				if out.Results[1].ProxyToolName == tc.wantTop {
					t.Fatalf("expected weaker match below exact hit, got %+v", out.Results)
				}
			}
		})
	}
}

func TestService_Search_exactCanonicalBeatsWeakerMatch(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	weak := models.CapabilityRecord{
		ID: "w", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "s__other", OriginalName: "other",
		GeneratedSummary: "Mentions github_gateway__create_issue in passing.",
		SearchText:       "s other github_gateway__create_issue mention", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	exact := models.CapabilityRecord{
		ID: "e", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "github_gateway__create_issue", OriginalName: "create_issue",
		GeneratedSummary: "Creates issues.",
		SearchText:       "github_gateway create_issue", VersionHash: "2", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := st.UpsertCapability(ctx, weak); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, exact); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "github_gateway__create_issue", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) < 1 || out.Results[0].ProxyToolName != exact.CanonicalName {
		t.Fatalf("want exact canonical first, got %+v", out.Results)
	}
}

func TestService_Search_exactOriginalName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID: "1", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "s__my_tool", OriginalName: "unique_orig_name",
		GeneratedSummary: "Summary.", SearchText: "s unique_orig_name summary", VersionHash: "1",
		LastSeenAt: time.Now(), InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "unique_orig_name", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 1 || out.Results[0].CapabilityID != rec.ID {
		t.Fatalf("got %+v", out.Results)
	}
	if len(out.Results[0].WhyMatched) == 0 {
		t.Fatal("expected why_matched")
	}
}

func TestService_Search_ftsTagOrSource(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID: "t1", Kind: models.CapabilityKindTool, SourceID: "special-source", SourceType: "gateway",
		CanonicalName: "special_source__noop", OriginalName: "noop",
		GeneratedSummary: "Unrelated summary text.",
		Tags:             []string{"quark"},
		SearchText:       "special-source noop unrelated quark",
		VersionHash:      "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "quark", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 || out.Results[0].ProxyToolName != rec.CanonicalName {
		t.Fatalf("got %+v", out.Results)
	}
	out2, err := svc.Search(ctx, models.SearchQuery{Text: "special-source", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out2.Results) == 0 {
		t.Fatal("expected hit by source id in fts")
	}
}

func TestService_Search_userSummaryBoost(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	base := models.CapabilityRecord{
		Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		OriginalName: "t", GeneratedSummary: "same",
		SearchText: "s t shared-token", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	a := base
	a.ID = "a"
	a.CanonicalName = "s__a"
	b := base
	b.ID = "b"
	b.CanonicalName = "s__b"
	b.UserSummary = "operator pinned"
	if err := st.UpsertCapability(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, b); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "shared-token", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) < 2 {
		t.Fatalf("need 2 hits, got %d", len(out.Results))
	}
	if out.Results[0].CapabilityID != b.ID {
		t.Fatalf("user-edited summary should rank first, got %+v", out.Results)
	}
	found := false
	for _, w := range out.Results[0].WhyMatched {
		if w == "user:edited-summary" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected user:edited-summary in why_matched: %#v", out.Results[0].WhyMatched)
	}
}

func TestService_Search_userSummaryContentMatchesLexical(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()

	// Two tools with identical generated summaries. Only "b" has a user summary
	// containing the search term "email". The lexical scorer should rank "b"
	// higher because its effective summary matches the query.
	a := models.CapabilityRecord{
		ID: "a", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "s__a", OriginalName: "a_tool",
		GeneratedSummary: "generic helper utility",
		SearchText:       "s a_tool generic helper utility email", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	b := models.CapabilityRecord{
		ID: "b", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "s__b", OriginalName: "b_tool",
		GeneratedSummary: "generic helper utility",
		UserSummary:      "sends email notifications to users",
		SearchText:       "s b_tool generic helper utility sends email notifications", VersionHash: "2", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := st.UpsertCapability(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, b); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "email", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	// "b" should rank first because its effective summary (user summary) contains "email"
	if out.Results[0].CapabilityID != b.ID {
		t.Fatalf("user summary content should boost relevance; want b first, got %+v", out.Results)
	}
	// Verify the summary match signal is present
	found := false
	for _, w := range out.Results[0].WhyMatched {
		if strings.Contains(w, "summary:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected summary: signal in why_matched: %v", out.Results[0].WhyMatched)
	}
}

func TestService_Search_noopEmbeddingsNoPanic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID: "n1", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "gateway",
		CanonicalName: "s__x", OriginalName: "x", GeneratedSummary: "hello world",
		SearchText: "s x hello world", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	_, err = svc.Search(ctx, models.SearchQuery{Text: "hello world", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
}
