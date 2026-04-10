package search

import (
	"sort"
	"strings"

	"lazy-tool/pkg/models"
)

// scoreLexical applies spec §14 signals: name, summary, argument names (tags), source id.
func scoreLexical(needle string, tokens []string, rec *models.CapabilityRecord) (float64, []string) {
	seen := make(map[string]struct{})
	var sc float64
	add := func(key string, pts float64) {
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		sc += pts
	}

	on := strings.ToLower(rec.OriginalName)
	sum := strings.ToLower(rec.EffectiveSummary())
	src := strings.ToLower(rec.SourceID)

	if needle != "" {
		if strings.Contains(on, needle) && !strings.EqualFold(on, needle) {
			add("name:"+truncateWhy(needle, 32), 2.5)
		}
		if sum != "" && strings.Contains(sum, needle) {
			add("summary:"+truncateWhy(needle, 32), 2.0)
		}
		if strings.Contains(src, needle) || strings.EqualFold(needle, rec.SourceID) {
			add("source:"+rec.SourceID, 2.5)
		}
		for _, tag := range rec.Tags {
			lt := strings.ToLower(tag)
			if lt != "" && (strings.Contains(needle, lt) || strings.Contains(lt, needle)) {
				add("arg:"+tag, 3.0)
			}
		}
	}

	for _, tok := range tokens {
		if len(tok) < 2 {
			continue
		}
		if strings.Contains(on, tok) {
			add("name:"+tok, 1.2)
		}
		if sum != "" && strings.Contains(sum, tok) {
			add("summary:"+tok, 1.0)
		}
		if strings.Contains(src, tok) {
			add("source:"+rec.SourceID, 1.5)
		}
		for _, tag := range rec.Tags {
			if strings.EqualFold(tag, tok) {
				add("arg:"+tag, 2.5)
			} else if lt := strings.ToLower(tag); strings.Contains(lt, tok) || strings.Contains(tok, lt) {
				add("arg:"+tag, 1.2)
			}
		}
	}

	kw := keywordScore(rec.SearchText, tokens)
	if kw > 0 {
		add("keyword:tokens", kw)
	}

	return sc, sortedWhy(seen)
}

func truncateWhy(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func sortedWhy(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normalizeCosine maps chromem similarity from [-1,1] to [0,1].
func normalizeCosine(v float32) float64 {
	x := (float64(v) + 1) / 2
	if x <= 0.5 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
