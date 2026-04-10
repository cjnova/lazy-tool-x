package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/metrics"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

func TestConversationalRecall(t *testing.T) {
	var mode string
	prev := metrics.SearchCandidateGeneration
	metrics.SearchCandidateGeneration = func(m string) { mode = m }
	defer func() { metrics.SearchCandidateGeneration = prev }()

	p := filepath.Join(t.TempDir(), "recall.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID:                "office-1",
		Kind:              models.CapabilityKindTool,
		SourceID:          "office",
		SourceType:        "server",
		CanonicalName:     "office__word_from_markdown",
		OriginalName:      "word_from_markdown",
		GeneratedSummary:  "Creates a new Word document populated from Markdown summary, conversation summary, notes, or report content.",
		SearchText:        "office word_from_markdown creates a new word document populated from markdown summary conversation summary notes report content",
		VersionHash:       "1",
		LastSeenAt:        time.Now(),
		InputSchemaJSON:   "{}",
		MetadataJSON:      "{}",
	}
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)
	ranked, err := svc.Search(ctx, models.SearchQuery{
		Text:  "create a new Word document populated with a summary of this conversation",
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if mode != models.SearchCandidatePathSubstringFullCatalogFTSZeroRows {
		t.Fatalf("metrics path: got %q want %q", mode, models.SearchCandidatePathSubstringFullCatalogFTSZeroRows)
	}
	if ranked.CandidatePath != models.SearchCandidatePathSubstringFullCatalogFTSZeroRows {
		t.Fatalf("CandidatePath: got %q want %q", ranked.CandidatePath, models.SearchCandidatePathSubstringFullCatalogFTSZeroRows)
	}
	if len(ranked.Results) == 0 || ranked.Results[0].ProxyToolName != "office__word_from_markdown" {
		t.Fatalf("expected office__word_from_markdown top hit, got %#v", ranked.Results)
	}

	// After Task 14 (FTS softening), this test should use a faster FTS path instead of full-catalog fallback.
	switch ranked.CandidatePath {
	case models.SearchCandidatePathSubstringFullCatalogFTSZeroRows:
		// Current behavior: FTS AND conjunction returns zero rows, then full-catalog substring recovers the tool.
	case models.SearchCandidatePathSubstringSkippedFTSHit:
		// Post-fix behavior: the tool should surface via the faster FTS path.
	default:
		t.Fatalf("CandidatePath: got %q want %q or %q", ranked.CandidatePath, models.SearchCandidatePathSubstringFullCatalogFTSZeroRows, models.SearchCandidatePathSubstringSkippedFTSHit)
	}
}
