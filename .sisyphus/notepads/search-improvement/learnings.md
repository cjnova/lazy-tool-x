# XK|# Search Improvement: Learnings

# KM|

# RV|## 2026-04-10 Session Init

# RW|

# PK|### Codebase Structure

# SY|

# NH|- `internal/search/keyword.go` — `keywordScore()` (lines 31-46), `tokenize()` (9-29), `fallbackConjunctionTokens()` (48-85)

# NM|- `internal/search/scoring.go` — `scoreLexical()` (11-72), `normalizeCosine()` (91-100)

# PQ|- `internal/search/service.go` — `scoreCandidate()`, `buildCandidates()`, `searchEmptyQuery()`

# BT|- `internal/search/explain_scores_test.go` — Pattern for breakdown tests with ExplainScores:true

# MX|- `internal/search/e2e_test.go` — Pattern: t.TempDir() SQLite, UpsertCapability, NewService, Search

# KY|- `internal/search/candidate_path_test.go` — Matrix test pattern with metrics hook

# BQ|

# TZ|### Key Code Facts

# RJ|

# KH|- `keywordScore()` uses `strings.Count()` unbounded repetition: `count × (1 + 0.1×len(token))`

# VH|- `normalizeCosine()` maps `(v+1)/2` so cosine=0 → 0.5 × VectorMultiplier(6) = 3 pts

# KY|- FTS uses strict AND conjunction via `BuildFTSMatchQuery()` in `internal/storage/fts.go:78-89`

# VT|- Score weights: ExactCanonical=10, ExactName=8, Substring=2, VectorMultiplier=6

# NB|- `keyword_test.go` already exists but only has a minimal single test (TestKeywordScore)

# QM|- `candidate_path_test.go` already has the conversational query test case

# ZP|

# QW|### Test Patterns

# KW|

# ST|- All tests use `package search` (internal package tests)

# KJ|- Use `embeddings.Noop{}` for lexical-only tests

# MS|- Use `metrics.SearchCandidateGeneration` hook for candidate path testing

# WB|- `NewService(st, nil, embeddings.Noop{}, ScoreWeights{}, false)` for basic service creation

# SW|- `DefaultScoreWeights()` when you need actual default weights

# ZM|

# RN|### No Local Go Toolchain

# JQ|

# XK|- All `go test`, `go vet`, golangci-lint run via GitHub CI

# MS|- CI triggers on `push` with a draft PR open targeting main

# NV|- Verification flow: push branch → gh run watch → gh run view --exit-status

# NH|- Must create draft PR at start of each PR phase to enable CI

# RB|

# XY|### Git Context

# MS|

# WB|- Working on main branch (HEAD: 949dfd1)

# WH|- Need to create branch `search-improvement/pr1-lexical-cap` for Wave 1.1

# XN|

# VK|# Regression query baseline notes

# PB|

# ZP|- Built `TestRegressionQuerySet` with 15 subtests across exact routing, parameter/schema, paraphrase, conversational, and ambiguous-intent classes

# ZY|- Used `t.TempDir()` SQLite per subtest and behavior-based assertions on top ordering only

# VJ|

# HN|## [2026-04-10] Task: 4

# BN|

# ZX|- keywordScore() now uses binary presence cap: score += (1 + 0.1×len(token)) when count > 0

# YM|- normalization: divide by len(tokens)

# RJ|- TestInflationKeywordRepetition: t.Skip removed, now passes in CI

# ZM|- TestKeywordScore_repetitionInflation: updated to assert capped behavior

# YJ|

# KP|## [2026-04-10] Task: 5

# XN|

# XX|- `TestSearch_explainScores_keywordCapBounded` compares single-token vs 12x repeated `create` searchText and checks the explain breakdown stays bounded

# YH|- The test keeps the existing `sum(score_breakdown) ≈ score` assertion pattern and adds a relative boundedness check instead of exact score matching

# VV|- Reused the existing `ExplainScores:true` setup with temp SQLite and `UpsertCapability` records, matching the file's current style

# HQ|

# VH|## [2026-04-10] Task: 6

# WJ|- PR 1 description written using pr-schema.md 13-section template

# VV|- PR marked ready: gh pr ready 24 --repo cjnova/lazy-tool-x

# BS|- Before/after benchmark table included for 5 PR 1 queries

# PZ|

# VH|## [2026-04-10] Task: 6

# NB|- PR 1 description written (13-section schema)

# WW|- PR 24 marked ready via gh pr ready

# SW|- Before/after benchmark table for 5 PR1 queries included

# YY|

# NP|#VH|## [2026-04-10] Task: 7

# YN|#NB|- Created `internal/search/vector_scoring_test.go` with `TestVectorInflation` (skipped regression)

# KX|#WW|- Used `vector.NewInMemory()` plus `RebuildFromRecords()` to seed the neutral vector candidate

# JH|#SW|- Injected query embedding via `HasEmbedding=true` + `Embedding` so the vector leg runs in test

# TV|#RX|- Demonstrated cosine=0 → normalizeCosine()=0.5 → 3 pts inflation path

# PW|#NP|- Branch `search-improvement/pr2-vector-recal` created and draft PR opened

## [2026-04-10] Task: 8

- Added `TestLexicalOnlyInvariance` to `internal/search/vector_scoring_test.go`
- Verifies `LexicalOnly=true` prevents any `vector` contribution in `ScoreBreakdown`
- Uses `ExplainScores:true` with an explicit embedding to confirm lexical-only overrides vector scoring

## [2026-04-10] Task: 9

- `normalizeCosine()` now thresholds at `x <= 0.5`, so cosine `<= 0` contributes 0 vector points
- Neutral cosine `0` no longer maps to 3 points under `VectorMultiplier=6`
- Removed `t.Skip` from `TestVectorInflation`; CI passed with lexical winner ranking first
- No `VectorMultiplier` change needed for PR 2 Task 9
- After thresholding, the neutral-vector candidate can be dropped entirely when it has no lexical signal; `TestVectorInflation` should only require the lexical winner to rank first, not that a second result exists
PR 2 (Recalibrate Vector Contribution) description completed following the 13-section schema. Recalibrated normalizeCosine to use a 0.5 threshold, effectively suppressing neutral vector matches (cosine=0) from inflating scores by 3 points. Verified with TestVectorInflation and TestSearch_explainScores_vectorThresholdBounded.

## [2026-04-10] Task: 14

- BuildFTSMatchQuery now keeps strict AND for 1-3 surviving tokens and softens 4+ token queries by anchoring the three longest tokens with AND, then OR-ing remaining tokens for better recall.
- TestConversationalRecall now expects the FTS-hit candidate path, confirming conversational queries stay on the faster FTS-augmented path instead of full-catalog substring fallback.

## [2026-04-10] Task: 14 follow-up

- Search candidate-path regressions needed two expectation updates: sparse FTS hits now use substring_scan_augmented_fts_sparse, and the conversational recall test must use Limit: 1 to exercise substring_scan_skipped_fts_hit.
- The fallback-chain compact-search-text regression also moved from zero-row fallback to sparse FTS augmentation once long-query FTS matching was softened.

## [2026-04-10] Task: 15

- Reviewed `buildCandidates()` after softened `BuildFTSMatchQuery()`: long conversational queries now usually produce sparse anchor-based FTS hits, so keeping token conjunction after substring remains the cleanest fallback order.
- Added a clarifying comment in `internal/search/service.go` documenting that substring should still run before token conjunction because it can recover compact/near matches from those sparse FTS hits.
- No `CandidatePath` constants were needed; existing paths still describe the post-Task-14 chain accurately.

## [2026-04-10] Task: 16

- Added `softened_fts_long_query_skips_substring` to the existing `TestSearch_candidatePath_substringMatrix` rows slice.
- Introduced per-row `limit` support so the new case can use `Limit: 1` without changing the other matrix entries.
- The new fixture keeps the `office__word_from_markdown` hit on the softened FTS anchor path, so the top result remains stable while `SubstringSkippedFTSHit` is asserted.

## [2026-04-10] Task: 17

- PR 3 ready, CI green

## [2026-04-10] Task: 18

- Added a top-3 regression anchor test in `internal/search/e2e_test.go` using `capRec()` fixtures and `DefaultScoreWeights()` to lock current ordering for exact, paraphrase, conversational, parameter, and ambiguous queries.
- Kept assertions behavior-based by checking only relative ordering and top-hit `ProxyToolName`, with no score comparisons.

## [2026-04-10] Task: 20

- `scoreCandidate()` now separates scoring into commented exact-routing, relevance, and preference layers without changing any weights or branch conditions.
- Exact-routing remains additive and unchanged: canonical exact match still adds `wt.ExactCanonical` with `exact_canonical`, original-name exact still adds `wt.ExactName` with `exact_name`, and substring fallback still stays in the same mutually exclusive chain.
- Final verification for Task 20 passed on CI run `24258384298` after restoring the branch's empty-query regression expectation so PR 4 returned to a green baseline.
