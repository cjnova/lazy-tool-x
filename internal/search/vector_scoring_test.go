package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/internal/vector"
	"lazy-tool/pkg/models"
)

func TestVectorInflation(t *testing.T) {
	ctx := context.Background()
	st, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "vector-inflation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	idx, err := vector.NewInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	queryVec := []float32{0, 1, 0, 0}
	toolALexical := models.CapabilityRecord{
		ID:                  "a",
		Kind:                models.CapabilityKindTool,
		SourceID:            "src-a",
		SourceType:          "gateway",
		CanonicalName:       "src-a__go_fetch_helper",
		OriginalName:        "go_fetch_helper",
		OriginalDescription: "Lexically relevant helper.",
		GeneratedSummary:    "go fetch helper for search ranking",
		SearchText:          "go fetch helper go fetch helper go helper fetch helper",
		InputSchemaJSON:     "{}",
		MetadataJSON:        "{}",
		VersionHash:         "v1",
		LastSeenAt:          time.Now().UTC(),
	}
	toolBVector := models.CapabilityRecord{
		ID:                  "b",
		Kind:                models.CapabilityKindTool,
		SourceID:            "src-b",
		SourceType:          "gateway",
		CanonicalName:       "src-b__neutral_vector",
		OriginalName:        "neutral_vector",
		OriginalDescription: "Semantically neutral, only vector-scored.",
		GeneratedSummary:    "unrelated helper service",
		SearchText:          "unrelated helper service for other tasks",
		InputSchemaJSON:     "{}",
		MetadataJSON:        "{}",
		VersionHash:         "v2",
		LastSeenAt:          time.Now().UTC(),
		EmbeddingModel:      "test",
		EmbeddingVector:     []float32{1, 0, 0, 0},
	}

	if err := st.UpsertCapability(ctx, toolALexical); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, toolBVector); err != nil {
		t.Fatal(err)
	}
	if err := idx.RebuildFromRecords(ctx, []models.CapabilityRecord{toolBVector}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, idx, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{
		Text:         "go fetch",
		Limit:        5,
		HasEmbedding: true,
		Embedding:    queryVec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) < 2 {
		t.Fatalf("expected both candidates, got %+v", out.Results)
	}
	if out.Results[0].ProxyToolName != toolALexical.CanonicalName {
		t.Fatalf("expected lexical winner first, got %+v", out.Results)
	}
	if out.Results[1].ProxyToolName != toolBVector.CanonicalName {
		t.Fatalf("expected neutral vector candidate second, got %+v", out.Results)
	}
}

func TestLexicalOnlyInvariance(t *testing.T) {
	ctx := context.Background()
	st, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "lexical-only-invariance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	idx, err := vector.NewInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	queryVec := []float32{1, 0, 0, 0}
	toolALexical := models.CapabilityRecord{
		ID:                  "a",
		Kind:                models.CapabilityKindTool,
		SourceID:            "src-a",
		SourceType:          "gateway",
		CanonicalName:       "src-a__go_fetch_helper",
		OriginalName:        "go_fetch_helper",
		OriginalDescription: "Lexically relevant helper.",
		GeneratedSummary:    "go fetch helper for search ranking",
		SearchText:          "go fetch helper go fetch helper go helper fetch helper",
		InputSchemaJSON:     "{}",
		MetadataJSON:        "{}",
		VersionHash:         "v1",
		LastSeenAt:          time.Now().UTC(),
	}
	toolBVector := models.CapabilityRecord{
		ID:                  "b",
		Kind:                models.CapabilityKindTool,
		SourceID:            "src-b",
		SourceType:          "gateway",
		CanonicalName:       "src-b__neutral_vector",
		OriginalName:        "neutral_vector",
		OriginalDescription: "Semantically neutral, only vector-scored.",
		GeneratedSummary:    "unrelated helper service",
		SearchText:          "unrelated helper service for other tasks",
		InputSchemaJSON:     "{}",
		MetadataJSON:        "{}",
		VersionHash:         "v2",
		LastSeenAt:          time.Now().UTC(),
		EmbeddingModel:      "test",
		EmbeddingVector:     []float32{1, 0, 0, 0},
	}

	if err := st.UpsertCapability(ctx, toolALexical); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapability(ctx, toolBVector); err != nil {
		t.Fatal(err)
	}
	if err := idx.RebuildFromRecords(ctx, []models.CapabilityRecord{toolBVector}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, idx, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{
		Text:          "go fetch",
		Limit:         5,
		HasEmbedding:  true,
		Embedding:     queryVec,
		LexicalOnly:   true,
		ExplainScores: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatalf("expected at least one result, got %+v", out.Results)
	}
	if out.Results[0].ProxyToolName != toolALexical.CanonicalName {
		t.Fatalf("expected lexical result first, got %+v", out.Results)
	}
	if _, ok := out.Results[0].ScoreBreakdown["vector"]; ok {
		t.Fatalf("expected no vector score contribution, got %+v", out.Results[0].ScoreBreakdown)
	}
}
