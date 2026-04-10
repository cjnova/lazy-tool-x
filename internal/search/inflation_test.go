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

// TestInflationKeywordRepetition demonstrates keyword score inflation.
// A tool with a repetitive keyword-heavy summary incorrectly outranks a concise,
// more relevant tool. This test is skipped until keywordScore() is capped in Task 4.
func TestInflationKeywordRepetition(t *testing.T) {
	t.Skip("demonstrates keyword inflation — will pass after keywordScore() is capped in Task 4")

	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()

	// Tool A: keyword-heavy summary with highly repetitive content.
	// The word "create" appears many times — this inflates its keyword score for
	// queries containing "create", even though the tool isn't particularly relevant.
	heavy := models.CapabilityRecord{
		ID:               "heavy",
		Kind:             models.CapabilityKindTool,
		SourceID:         "src",
		SourceType:       "server",
		CanonicalName:    "src__keyword_heavy_tool",
		OriginalName:     "keyword_heavy_tool",
		GeneratedSummary: "create create create create create document document document document document word word word word word report report report report report",
		SearchText:       "src keyword_heavy_tool create document word report",
		VersionHash:      "1",
		LastSeenAt:       time.Now(),
		InputSchemaJSON:  "{}",
		MetadataJSON:     "{}",
	}

	// Tool B: concise, relevant tool for creating a Word document.
	// Clean summary with the query term appearing once — should rank higher.
	concise := models.CapabilityRecord{
		ID:               "concise",
		Kind:             models.CapabilityKindTool,
		SourceID:         "office",
		SourceType:       "server",
		CanonicalName:    "office__word_create",
		OriginalName:     "word_create",
		GeneratedSummary: "Creates a Word document from the provided content.",
		SearchText:       "office word_create creates word document content",
		VersionHash:      "1",
		LastSeenAt:       time.Now(),
		InputSchemaJSON:  "{}",
		MetadataJSON:     "{}",
	}

	if err := st.UpsertCapability(ctx, heavy); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, concise); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: "create word document", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(out.Results))
	}

	// After the fix: the concise, relevant tool should rank at or above the
	// keyword-heavy tool. The repetitive summary should NOT inflate the score.
	if out.Results[0].CapabilityID != concise.ID {
		t.Fatalf("expected concise tool to rank first (got %s first); keyword inflation still present",
			out.Results[0].CapabilityID)
	}
}
