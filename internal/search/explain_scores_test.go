package search

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

func TestSearch_explainScores_populatesBreakdown(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID: "1", Kind: models.CapabilityKindTool, SourceID: "gw", SourceType: "gateway",
		CanonicalName: "gw__echo", OriginalName: "echo",
		GeneratedSummary: "echo tool", SearchText: "gw echo tool",
		VersionHash: "v", LastSeenAt: time.Now(),
	}
	if err := st.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{
		ExactCanonical: 10,
		ExactName:      8,
		Substring:      2,
	}, false)
	ranked, err := svc.Search(ctx, models.SearchQuery{Text: "gw__echo", Limit: 5, ExplainScores: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked.Results) != 1 {
		t.Fatalf("results: %d", len(ranked.Results))
	}
	bd := ranked.Results[0].ScoreBreakdown
	if bd == nil {
		t.Fatal("no breakdown")
	}
	// Ranker normalizes scores; breakdown is scaled to the same ratio as normalized Score.
	sum := 0.0
	for _, v := range bd {
		sum += v
	}
	if math.Abs(sum-ranked.Results[0].Score) > 0.02 {
		t.Fatalf("breakdown sum %v vs score %v: %#v", sum, ranked.Results[0].Score, bd)
	}
}

func TestSearch_explainScores_keywordCapBounded(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	base := models.CapabilityRecord{
		Kind:             models.CapabilityKindTool,
		SourceID:         "gw",
		SourceType:       "gateway",
		GeneratedSummary:  "utility tool",
		VersionHash:      "v",
		LastSeenAt:       time.Now(),
	}
	records := []models.CapabilityRecord{
		{
			ID: "1", CanonicalName: "gw__single", OriginalName: "single",
			SearchText: "create", GeneratedSummary: base.GeneratedSummary,
			Kind: base.Kind, SourceID: base.SourceID, SourceType: base.SourceType,
			VersionHash: base.VersionHash, LastSeenAt: base.LastSeenAt,
		},
		{
			ID: "2", CanonicalName: "gw__repeat", OriginalName: "repeat",
			SearchText: strings.TrimSpace(strings.Repeat("create ", 12)), GeneratedSummary: base.GeneratedSummary,
			Kind: base.Kind, SourceID: base.SourceID, SourceType: base.SourceType,
			VersionHash: base.VersionHash, LastSeenAt: base.LastSeenAt,
		},
	}
	for _, rec := range records {
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(st, nil, embeddings.Noop{}, ScoreWeights{
		ExactCanonical: 10,
		ExactName:      8,
		Substring:      2,
	}, false)
	ranked, err := svc.Search(ctx, models.SearchQuery{Text: "create", Limit: 5, ExplainScores: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked.Results) != 2 {
		t.Fatalf("results: %d", len(ranked.Results))
	}
	byID := map[string]models.SearchResult{}
	for _, r := range ranked.Results {
		byID[r.CapabilityID] = r
	}
	single, ok := byID["1"]
	if !ok {
		t.Fatal("missing single-occurrence result")
	}
	repeated, ok := byID["2"]
	if !ok {
		t.Fatal("missing repeated-occurrence result")
	}
	getKeyword := func(bd map[string]float64) float64 {
		if v := bd["keyword"]; v != 0 {
			return v
		}
		return bd["lexical"]
	}
	for name, r := range map[string]models.SearchResult{"single": single, "repeated": repeated} {
		if r.ScoreBreakdown == nil {
			t.Fatalf("%s: no breakdown", name)
		}
		sum := 0.0
		for _, v := range r.ScoreBreakdown {
			sum += v
		}
		if math.Abs(sum-r.Score) > 0.02 {
			t.Fatalf("%s: breakdown sum %v vs score %v: %#v", name, sum, r.Score, r.ScoreBreakdown)
		}
	}
	if math.Abs(getKeyword(repeated.ScoreBreakdown)-getKeyword(single.ScoreBreakdown)) > 0.02 {
		t.Fatalf("keyword breakdown should be bounded: single=%#v repeated=%#v", single.ScoreBreakdown, repeated.ScoreBreakdown)
	}
}
