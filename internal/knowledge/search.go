// internal/knowledge/search.go
package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Result struct {
	ID      string
	Score   int
	Snippet string
	Terms   []string
}

func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 2 {
			out = append(out, b.String())
		}
		b.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func countOccurrences(tokens []string, term string) int {
	n := 0
	for _, t := range tokens {
		if t == term {
			n++
		}
	}
	return n
}

func snippetFor(body string, terms []string) string {
	lower := strings.ToLower(body)
	best, bestByte := "", -1
	for _, term := range terms {
		if idx := strings.Index(lower, term); idx >= 0 && (bestByte < 0 || idx < bestByte) {
			bestByte = idx
			best = term
		}
	}
	runes := []rune(body)
	if bestByte < 0 {
		if len(runes) > 120 {
			return string(runes[:120]) + "…"
		}
		return body
	}
	start := len([]rune(lower[:bestByte]))
	width := len([]rune(best))
	from := start - 60
	if from < 0 {
		from = 0
	}
	to := start + width + 60
	if to > len(runes) {
		to = len(runes)
	}
	window := string(runes[from:to])
	if from > 0 {
		window = "…" + window
	}
	if to < len(runes) {
		window += "…"
	}
	return window
}

func Search(bundle Bundle, query string, limit int) ([]Result, error) {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil, fmt.Errorf("%w: empty query", ErrKnowledge)
	}
	if limit <= 0 || limit > 50 {
		return nil, fmt.Errorf("%w: limit outside 1..50", ErrKnowledge)
	}
	var out []Result
	for id, concept := range bundle.Concepts {
		score := 0
		matched := map[string]bool{}
		titleTokens := tokenize(concept.Title)
		bodyTokens := tokenize(concept.Title + " " + concept.Body)
		for _, term := range terms {
			titleHits := countOccurrences(titleTokens, term)
			bodyHits := countOccurrences(bodyTokens, term) - titleHits
			if bodyHits < 0 {
				bodyHits = 0
			}
			tagHit := false
			for _, tag := range concept.Tags {
				if strings.ToLower(tag) == term {
					tagHit = true
					break
				}
			}
			if titleHits == 0 && bodyHits == 0 && !tagHit {
				continue
			}
			matched[term] = true
			score += 3*titleHits + bodyHits
			if tagHit {
				score += 5
			}
		}
		if score == 0 {
			continue
		}
		termList := make([]string, 0, len(matched))
		for term := range matched {
			termList = append(termList, term)
		}
		sort.Strings(termList)
		out = append(out, Result{ID: id, Score: score, Snippet: snippetFor(concept.Body, termList), Terms: termList})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
