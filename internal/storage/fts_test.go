package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazy-tool/pkg/models"
)

func TestFTS_syncOnUpsertAndSearch(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fts.db")
	s, err := OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	rec := models.CapabilityRecord{
		ID:               "c1",
		Kind:             models.CapabilityKindTool,
		SourceID:         "src-alpha",
		SourceType:       "gateway",
		CanonicalName:    "src_alpha__widget",
		OriginalName:     "widget",
		GeneratedSummary: "Does something with zeta particles.",
		SearchText:       "src-alpha widget zeta particles",
		Tags:             []string{"zeta", "input"},
		VersionHash:      "v1",
		LastSeenAt:       time.Now(),
		InputSchemaJSON:  "{}",
		MetadataJSON:     "{}",
	}
	if err := s.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}

	match := BuildFTSMatchQuery("zeta particles")
	if match == "" {
		t.Fatal("expected fts match query")
	}
	ids, err := s.SearchFTSCandidateIDs(ctx, match, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != rec.ID {
		t.Fatalf("fts ids: %+v", ids)
	}

	ids, err = s.SearchFTSCandidateIDs(ctx, match, []string{"src-alpha"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("want 1 with source filter, got %+v", ids)
	}

	ids, err = s.SearchFTSCandidateIDs(ctx, match, []string{"other"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("want 0 for wrong source, got %+v", ids)
	}
}

func TestFTS_deleteStaleRemovesFTSRow(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fts2.db")
	s, err := OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	r1 := models.CapabilityRecord{
		ID: "a", Kind: models.CapabilityKindTool, SourceID: "s1", SourceType: "g",
		CanonicalName: "s1__keep", OriginalName: "keep", GeneratedSummary: "k",
		SearchText: "s1 keep k", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	r2 := models.CapabilityRecord{
		ID: "b", Kind: models.CapabilityKindTool, SourceID: "s1", SourceType: "g",
		CanonicalName: "s1__gone", OriginalName: "gone", GeneratedSummary: "g",
		SearchText: "s1 gone g", VersionHash: "2", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := s.UpsertCapability(ctx, r1); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCapability(ctx, r2); err != nil {
		t.Fatal(err)
	}

	n, err := s.DeleteStale(ctx, "s1", map[string]struct{}{"a": {}})
	if err != nil || n != 1 {
		t.Fatalf("DeleteStale: n=%d err=%v", n, err)
	}

	var cnt int
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM capabilities_fts WHERE id=?`, "b")
	if err := row.Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("fts row for deleted id should be gone, count=%d", cnt)
	}
}

func TestListIDsByOriginalNameFold(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fts3.db")
	s, err := OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	r1 := models.CapabilityRecord{
		ID: "x1", Kind: models.CapabilityKindTool, SourceID: "s1", SourceType: "g",
		CanonicalName: "s1__dup", OriginalName: "echo", GeneratedSummary: "",
		SearchText: "echo", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	r2 := models.CapabilityRecord{
		ID: "x2", Kind: models.CapabilityKindTool, SourceID: "s2", SourceType: "g",
		CanonicalName: "s2__dup", OriginalName: "echo", GeneratedSummary: "",
		SearchText: "echo", VersionHash: "2", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := s.UpsertCapability(ctx, r1); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCapability(ctx, r2); err != nil {
		t.Fatal(err)
	}
	ids, err := s.ListIDsByOriginalNameFold(ctx, "echo", nil)
	if err != nil || len(ids) != 2 {
		t.Fatalf("got %v err=%v", ids, err)
	}
	ids, err = s.ListIDsByOriginalNameFold(ctx, "echo", []string{"s2"})
	if err != nil || len(ids) != 1 || ids[0] != "x2" {
		t.Fatalf("got %v err=%v", ids, err)
	}
}

func TestGetCapabilitiesByIDs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fts4.db")
	s, err := OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	rec := models.CapabilityRecord{
		ID: "z", Kind: models.CapabilityKindTool, SourceID: "s", SourceType: "g",
		CanonicalName: "s__t", OriginalName: "t", GeneratedSummary: "g",
		SearchText: "t", VersionHash: "1", LastSeenAt: time.Now(),
		InputSchemaJSON: "{}", MetadataJSON: "{}",
	}
	if err := s.UpsertCapability(ctx, rec); err != nil {
		t.Fatal(err)
	}
	m, err := s.GetCapabilitiesByIDs(ctx, []string{"z", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m["z"].CanonicalName != "s__t" {
		t.Fatalf("%+v", m)
	}
}

func TestFTS_singleCharQueryReturnsEmptyMatch(t *testing.T) {
	// Single-char queries produce empty FTS MATCH strings by design.
	// The search pipeline falls back to substring scan for these.
	match := BuildFTSMatchQuery("a")
	if match != "" {
		t.Fatalf("single-char query should produce empty match, got %q", match)
	}
	// Two-char tokens should work normally.
	match = BuildFTSMatchQuery("ab")
	if match == "" {
		t.Fatal("two-char query should produce non-empty match")
	}
	// Mixed: only 2+ char tokens survive.
	match = BuildFTSMatchQuery("a bc d ef")
	if match != `"bc" AND "ef"` {
		t.Fatalf("want only 2+ char tokens, got %q", match)
	}
}

func TestBuildFTSMatchQuery_shortQuery_strictAND(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "one token", query: "alpha", want: `"alpha"`},
		{name: "two tokens", query: "alpha beta", want: `"alpha" AND "beta"`},
		{name: "three tokens", query: "alpha beta gamma", want: `"alpha" AND "beta" AND "gamma"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildFTSMatchQuery(tt.query); got != tt.want {
				t.Fatalf("BuildFTSMatchQuery(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestBuildFTSMatchQuery_longQuery_softened(t *testing.T) {
	query := "create a new Word document populated with a summary of this conversation"
	got := BuildFTSMatchQuery(query)
	want := `("document" AND "populated" AND "conversation") OR "create" OR "new" OR "word" OR "with" OR "summary" OR "of" OR "this"`
	if got != want {
		t.Fatalf("BuildFTSMatchQuery(%q) = %q, want %q", query, got, want)
	}
	if got == `"create" AND "new" AND "word" AND "document" AND "populated" AND "with" AND "summary" AND "of" AND "this" AND "conversation"` {
		t.Fatalf("expected softened query, got strict AND %q", got)
	}
}
