# Search Improvement: Learnings

## 2026-04-10 Session Init

### Codebase Structure

- `internal/search/keyword.go` — `keywordScore()` (lines 31-46), `tokenize()` (9-29), `fallbackConjunctionTokens()` (48-85)
- `internal/search/scoring.go` — `scoreLexical()` (11-72), `normalizeCosine()` (91-100)
- `internal/search/service.go` — `scoreCandidate()`, `buildCandidates()`, `searchEmptyQuery()`
- `internal/search/explain_scores_test.go` — Pattern for breakdown tests with ExplainScores:true
- `internal/search/e2e_test.go` — Pattern: t.TempDir() SQLite, UpsertCapability, NewService, Search
- `internal/search/candidate_path_test.go` — Matrix test pattern with metrics hook

### Key Code Facts

- `keywordScore()` uses `strings.Count()` unbounded repetition: `count × (1 + 0.1×len(token))`
- `normalizeCosine()` maps `(v+1)/2` so cosine=0 → 0.5 × VectorMultiplier(6) = 3 pts
- FTS uses strict AND conjunction via `BuildFTSMatchQuery()` in `internal/storage/fts.go:78-89`
- Score weights: ExactCanonical=10, ExactName=8, Substring=2, VectorMultiplier=6
- `keyword_test.go` already exists but only has a minimal single test (TestKeywordScore)
- `candidate_path_test.go` already has the conversational query test case

### Test Patterns

- All tests use `package search` (internal package tests)
- Use `embeddings.Noop{}` for lexical-only tests
- Use `metrics.SearchCandidateGeneration` hook for candidate path testing
- `NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)` for basic service creation
- `DefaultScoreWeights()` when you need actual default weights

### No Local Go Toolchain

- All `go test`, `go vet`, golangci-lint run via GitHub CI
- CI triggers on `push` with a draft PR open targeting main
- Verification flow: push branch → gh run watch → gh run view --exit-status
- Must create draft PR at start of each PR phase to enable CI

### Git Context

- Working on main branch (HEAD: 949dfd1)
- Need to create branch `search-improvement/pr1-lexical-cap` for Wave 1.1

# Regression query baseline notes

- Built `TestRegressionQuerySet` with 15 subtests across exact routing, parameter/schema, paraphrase, conversational, and ambiguous-intent classes.
- Used `t.TempDir()` SQLite per subtest and behavior-based assertions on top ordering only.

## [2026-04-10] Task: 4

- keywordScore() now uses binary presence cap: score += (1 + 0.1×len(token)) when count > 0
- normalization: divide by len(tokens)
- TestInflationKeywordRepetition: t.Skip removed, now passes in CI
- TestKeywordScore_repetitionInflation: updated to assert capped behavior

## [2026-04-10] Task: 5

- `TestSearch_explainScores_keywordCapBounded` compares single-token vs 12x repeated `create` searchText and checks the explain breakdown stays bounded.
- The test keeps the existing `sum(score_breakdown) ≈ score` assertion pattern and adds a relative boundedness check instead of exact score matching.
- Reused the existing `ExplainScores:true` setup with temp SQLite and `UpsertCapability` records, matching the file's current style.
