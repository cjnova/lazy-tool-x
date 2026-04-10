package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

func TestBenchmark_Ambiguous_MarkdownReport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	correct := capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown report content.", "office word_from_markdown markdown report summary document create")
	correct.LastSeenAt = time.Now()
	distractor := capRec("docs1", "docs__read_document_fully", "read_document_fully", "Reads a document end to end.", "docs read document fully content")
	for _, rec := range []models.CapabilityRecord{correct, distractor} {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "markdown report", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != correct.CanonicalName {
		t.Fatalf("expected %s first, got %+v", correct.CanonicalName, out.Results)
	}
}

func TestBenchmark_Ambiguous_AzurePricing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	correct := capRec("az1", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure pricing by service and region.", "azure pricing query prices cached service sku region")
	correct.LastSeenAt = time.Now()
	distractor := capRec("az2", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier")
	for _, rec := range []models.CapabilityRecord{correct, distractor} {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "azure pricing", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != correct.CanonicalName {
		t.Fatalf("expected %s first, got %+v", correct.CanonicalName, out.Results)
	}
}

func TestBenchmark_Ambiguous_ReadDocsFully(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	correct := capRec("docs1", "docs__read_document_fully", "read_document_fully", "Reads documents fully and returns the full text.", "docs read document fully entire content text")
	correct.LastSeenAt = time.Now()
	distractor := capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input.", "office word_from_markdown markdown summary report")
	for _, rec := range []models.CapabilityRecord{correct, distractor} {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "read docs fully", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != correct.CanonicalName {
		t.Fatalf("expected %s first, got %+v", correct.CanonicalName, out.Results)
	}
}

func TestBenchmark_TieBreak_FavoritedToolRanksHigher(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	base := capRec("a", "src__tool_a", "tool_a", "Does shared work.", "src tool shared work token")
	base.LastSeenAt = time.Now()
	fav := capRec("b", "src__tool_b", "tool_b", "Does shared work.", "src tool shared work token")
	fav.LastSeenAt = time.Now()
	for _, rec := range []models.CapabilityRecord{base, fav} {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "shared", Limit: 5, FavoriteIDs: map[string]struct{}{fav.ID: {}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != fav.CanonicalName {
		t.Fatalf("expected %s first, got %+v", fav.CanonicalName, out.Results)
	}
}

func TestBenchmark_TieBreak_InvokedToolRanksHigher(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	base := capRec("a", "src__tool_a", "tool_a", "Does shared work.", "src tool shared work token")
	base.LastSeenAt = time.Now()
	invoked := capRec("b", "src__tool_b", "tool_b", "Does shared work.", "src tool shared work token")
	invoked.LastSeenAt = time.Now()
	for _, rec := range []models.CapabilityRecord{base, invoked} {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "shared", Limit: 5, InvocationStats: map[string]models.InvocationStat{invoked.CanonicalName: {InvokeCount: 5}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != invoked.CanonicalName {
		t.Fatalf("expected %s first, got %+v", invoked.CanonicalName, out.Results)
	}
}
