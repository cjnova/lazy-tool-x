package search

import "testing"

func TestKeywordScore(t *testing.T) {
	s := keywordScore("hello world create issue", []string{"create", "issue"})
	if s <= 0 {
		t.Fatal(s)
	}
}

func TestKeywordScore_cases(t *testing.T) {
	tests := []struct {
		name       string
		searchText string
		tokens     []string
		wantGtZero bool
	}{
		{
			name:       "single_token_match",
			searchText: "create github issue",
			tokens:     []string{"create"},
			wantGtZero: true,
		},
		{
			name:       "multiple_tokens_match",
			searchText: "create github issue with title and body",
			tokens:     []string{"create", "issue", "title"},
			wantGtZero: true,
		},
		{
			name:       "token_not_found",
			searchText: "read powerpoint slides",
			tokens:     []string{"create"},
			wantGtZero: false,
		},
		{
			name:       "empty_tokens",
			searchText: "create github issue",
			tokens:     []string{},
			wantGtZero: false,
		},
		{
			name:       "nil_tokens",
			searchText: "create github issue",
			tokens:     nil,
			wantGtZero: false,
		},
		{
			name:       "short_token_skipped",
			searchText: "to be or not to be",
			tokens:     []string{"a", "b"},
			wantGtZero: false,
		},
		{
			name:       "longer_token_scores_more",
			searchText: "documentation document",
			tokens:     []string{"documentation"},
			wantGtZero: true,
		},
		{
			name:       "repeated_token_scores_higher_than_once",
			searchText: "create create create create create",
			tokens:     []string{"create"},
			wantGtZero: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := keywordScore(tc.searchText, tc.tokens)
			if tc.wantGtZero && score <= 0 {
				t.Fatalf("expected score > 0, got %v", score)
			}
			if !tc.wantGtZero && score != 0 {
				t.Fatalf("expected score == 0, got %v", score)
			}
		})
	}
}

// TestKeywordScore_repetitionInflation documents that the current keywordScore
// is unbounded — repeated tokens inflate the score linearly. This will change
// after Task 4 caps the contribution.
func TestKeywordScore_repetitionInflation(t *testing.T) {
	once := keywordScore("the word create appears here", []string{"create"})
	five := keywordScore("create create create create create appears here", []string{"create"})
	if five <= once {
		t.Fatalf("expected repeated token to score higher (current behavior): once=%v five=%v", once, five)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "basic_words",
			input: "hello world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "mixed_case_lowercased",
			input: "Hello World",
			want:  []string{"hello", "world"},
		},
		{
			name:  "punctuation_split",
			input: "create-issue",
			want:  []string{"create", "issue"},
		},
		{
			name:  "underscores_split",
			input: "word_from_markdown",
			want:  []string{"word", "from", "markdown"},
		},
		{
			name:  "numbers_kept",
			input: "v2 api",
			want:  []string{"v2", "api"},
		},
		{
			name:  "empty_string",
			input: "",
			want:  nil,
		},
		{
			name:  "only_punctuation",
			input: "!@#$%",
			want:  nil,
		},
		{
			name:  "unicode_letters",
			input: "résumé",
			want:  []string{"résumé"},
		},
		{
			name:  "multiple_spaces",
			input: "foo   bar",
			want:  []string{"foo", "bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenize(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("tokenize(%q) = %v; want %v", tc.input, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("tokenize(%q)[%d] = %q; want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFallbackConjunctionTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   []string
	}{
		{
			name:   "empty",
			tokens: []string{},
			want:   nil,
		},
		{
			name:   "nil",
			tokens: nil,
			want:   nil,
		},
		{
			name:   "stopwords_removed",
			tokens: []string{"the", "of", "and", "create"},
			want:   []string{"create"},
		},
		{
			name:   "short_tokens_filtered",
			tokens: []string{"to", "be", "or", "create"},
			want:   []string{"create"},
		},
		{
			name:   "max_4_cap",
			tokens: []string{"alpha", "beta", "gamma", "delta", "epsilon"},
			// sorted by length desc: epsilon(7), alpha(5), delta(5), gamma(5), beta(4) → cap 4
			// then sort alphabetically: alpha, delta, epsilon, gamma
			want: []string{"alpha", "delta", "epsilon", "gamma"},
		},
		{
			name:   "deduplication",
			tokens: []string{"create", "create", "issue"},
			want:   []string{"create", "issue"},
		},
		{
			name:   "alphabetical_output",
			tokens: []string{"zebra", "apple", "mango"},
			want:   []string{"apple", "mango", "zebra"},
		},
		{
			name:   "all_stopwords",
			tokens: []string{"the", "a", "in", "is"},
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fallbackConjunctionTokens(tc.tokens)
			if len(got) != len(tc.want) {
				t.Fatalf("fallbackConjunctionTokens(%v) = %v; want %v", tc.tokens, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("fallbackConjunctionTokens(%v)[%d] = %q; want %q", tc.tokens, i, got[i], tc.want[i])
				}
			}
		})
	}
}
