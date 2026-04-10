package search

import (
	"context"
	"path/filepath"
	"testing"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

// indexOf returns the position of proxyToolName in results, or -1 if absent.
func indexOf(results []models.SearchResult, proxyToolName string) int {
	for i, r := range results {
		if r.ProxyToolName == proxyToolName {
			return i
		}
	}
	return -1
}

// scoreOf returns the score of proxyToolName in results, or 0 if absent.
func scoreOf(results []models.SearchResult, proxyToolName string) float64 {
	for _, r := range results {
		if r.ProxyToolName == proxyToolName {
			return r.Score
		}
	}
	return 0
}

// newTestService creates a fresh SQLite store, upserts the given records, and
// returns a search Service configured with default weights and noop embeddings.
func newTestService(t *testing.T, records []models.CapabilityRecord) *Service {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	for _, rec := range records {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	return NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
}

// ---------- Test 1: Exact canonical beats substring ----------

func TestRelativeOutcome_ExactBeatsSubstring(t *testing.T) {
	exact := capRec("w1", "office__word_from_markdown", "word_from_markdown",
		"Creates a new Word document from Markdown input.",
		"office word_from_markdown creates new word document markdown")
	substr := capRec("w2", "office__word_template_placeholders", "word_template_placeholders",
		"Inspects Word templates. Mentions word_from_markdown in passing.",
		"office word_template_placeholders inspect template word_from_markdown mention")

	svc := newTestService(t, []models.CapabilityRecord{exact, substr})
	out, err := svc.Search(context.Background(), models.SearchQuery{
		Text:  "office__word_from_markdown",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	rankExact := indexOf(out.Results, exact.CanonicalName)
	rankSubstr := indexOf(out.Results, substr.CanonicalName)
	if rankExact < 0 {
		t.Fatalf("exact match not found in results: %+v", out.Results)
	}
	if rankSubstr < 0 {
		t.Fatalf("substring match not found in results: %+v", out.Results)
	}
	if rankExact >= rankSubstr {
		t.Fatalf("expected %s (rank %d) above %s (rank %d), results: %+v",
			exact.CanonicalName, rankExact, substr.CanonicalName, rankSubstr, out.Results)
	}
}

// ---------- Test 2: Exact query dominates; paraphrase still finds target ----------

func TestRelativeOutcome_ExactBeatsParaphrase(t *testing.T) {
	target := capRec("w1", "office__word_from_markdown", "word_from_markdown",
		"Creates a new Word document populated from Markdown summary, conversation summary, notes, or report content.",
		"office word_from_markdown creates new word document populated summary conversation markdown report notes")
	distractor := capRec("ppt1", "office__read_powerpoint_speaker_notes", "read_powerpoint_speaker_notes",
		"Reads speaker notes from a PowerPoint deck.",
		"office powerpoint speaker notes read deck slides")

	svc := newTestService(t, []models.CapabilityRecord{target, distractor})
	ctx := context.Background()

	// Search 1: exact canonical name query → target must be rank 1
	out1, err := svc.Search(ctx, models.SearchQuery{Text: "office__word_from_markdown", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(out1.Results) == 0 || out1.Results[0].ProxyToolName != target.CanonicalName {
		t.Fatalf("exact query: expected %s first, got %+v", target.CanonicalName, out1.Results)
	}

	// Search 2: paraphrase query → target must still be rank 1
	out2, err := svc.Search(ctx, models.SearchQuery{
		Text:  "create a new Word document populated with a summary of this conversation",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out2.Results) == 0 || out2.Results[0].ProxyToolName != target.CanonicalName {
		t.Fatalf("paraphrase query: expected %s first, got %+v", target.CanonicalName, out2.Results)
	}
}

// ---------- Test 3: Parameter terms beat unrelated ----------

func TestRelativeOutcome_ParameterTermBeatsUnrelated(t *testing.T) {
	matched := capRec("w1", "office__word_from_markdown", "word_from_markdown",
		"Creates a new Word document from Markdown input.",
		"office word_from_markdown output_path markdown word document")
	unrelated := capRec("w2", "office__read_word_template", "read_word_template",
		"Reads a Word template file.",
		"office read_word_template template file")

	svc := newTestService(t, []models.CapabilityRecord{matched, unrelated})
	out, err := svc.Search(context.Background(), models.SearchQuery{
		Text:  "output_path markdown word",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	rankMatched := indexOf(out.Results, matched.CanonicalName)
	rankUnrelated := indexOf(out.Results, unrelated.CanonicalName)
	if rankMatched < 0 {
		t.Fatalf("parameter-matched tool not found: %+v", out.Results)
	}
	if rankMatched >= rankUnrelated && rankUnrelated >= 0 {
		t.Fatalf("expected %s (rank %d) above %s (rank %d), results: %+v",
			matched.CanonicalName, rankMatched, unrelated.CanonicalName, rankUnrelated, out.Results)
	}
}

// ---------- Test 4: Conversational query finds correct tool ----------

func TestRelativeOutcome_ConversationalQueryFindsCorrectTool(t *testing.T) {
	target := capRec("o1", "office__append_staffing_row_to_office_table", "append_staffing_row_to_office_table",
		"Appends a staffing row to an office table.",
		"office staffing row table append insert office")
	distractor := capRec("w1", "office__word_from_markdown", "word_from_markdown",
		"Creates a new Word document from Markdown input.",
		"office word_from_markdown markdown summary report")

	svc := newTestService(t, []models.CapabilityRecord{target, distractor})
	out, err := svc.Search(context.Background(), models.SearchQuery{
		Text:  "append staffing row to office table",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	rankTarget := indexOf(out.Results, target.CanonicalName)
	rankDistractor := indexOf(out.Results, distractor.CanonicalName)
	if rankTarget < 0 {
		t.Fatalf("target tool not found: %+v", out.Results)
	}
	if rankDistractor >= 0 && rankTarget >= rankDistractor {
		t.Fatalf("expected %s (rank %d) above %s (rank %d), results: %+v",
			target.CanonicalName, rankTarget, distractor.CanonicalName, rankDistractor, out.Results)
	}
}

// ---------- Test 5: Ambiguous query favors stronger signal ----------

func TestRelativeOutcome_AmbiguousQueryFavorsSpecificTool(t *testing.T) {
	stronger := capRec("az1", "azure__azure_query_prices", "azure_query_prices",
		"Queries cached Azure pricing by service and region.",
		"azure pricing query prices cached service sku region")
	weaker := capRec("az2", "azure__firewall_premium_price", "firewall_premium_price",
		"Returns Azure firewall premium pricing.",
		"azure firewall premium price security tier")

	svc := newTestService(t, []models.CapabilityRecord{stronger, weaker})
	out, err := svc.Search(context.Background(), models.SearchQuery{
		Text:  "azure pricing",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	rankStronger := indexOf(out.Results, stronger.CanonicalName)
	rankWeaker := indexOf(out.Results, weaker.CanonicalName)
	if rankStronger < 0 {
		t.Fatalf("stronger tool not found: %+v", out.Results)
	}
	if rankWeaker < 0 {
		t.Fatalf("weaker tool not found: %+v", out.Results)
	}
	if rankStronger >= rankWeaker {
		t.Fatalf("expected %s (rank %d) above %s (rank %d), results: %+v",
			stronger.CanonicalName, rankStronger, weaker.CanonicalName, rankWeaker, out.Results)
	}
}

// ---------- Test 6: Cross-class — exact query score >= paraphrase query score ----------

func TestRelativeOutcome_CrossClass_ExactBeatsSameToolParaphrase(t *testing.T) {
	target := capRec("az1", "azure__azure_query_prices", "azure_query_prices",
		"Queries cached Azure service pricing by sku, region, and quantity.",
		"azure azure_query_prices service sku region quantity price cached")
	distractor := capRec("az2", "azure__firewall_premium_price", "firewall_premium_price",
		"Returns Azure firewall premium pricing.",
		"azure firewall premium price security tier")

	svc := newTestService(t, []models.CapabilityRecord{target, distractor})
	ctx := context.Background()

	// Exact query — uses canonical name directly
	out1, err := svc.Search(ctx, models.SearchQuery{Text: "azure__azure_query_prices", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	scoreExact := scoreOf(out1.Results, target.CanonicalName)
	if scoreExact == 0 {
		t.Fatalf("target tool not found in exact query results: %+v", out1.Results)
	}

	// Paraphrase query — natural language description
	out2, err := svc.Search(ctx, models.SearchQuery{Text: "calculate azure vm monthly cost from cached prices", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	scoreParaphrase := scoreOf(out2.Results, target.CanonicalName)

	if scoreExact < scoreParaphrase {
		t.Fatalf("exact query score (%v) should be >= paraphrase query score (%v) for %s",
			scoreExact, scoreParaphrase, target.CanonicalName)
	}
}
