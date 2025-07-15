package fuzzymatch

import (
	"strings"
	"unicode"
)

// FuzzyMatch represents a fuzzy match result
type FuzzyMatch struct {
	Text     string
	Score    int
	Indices  []int // positions of matched characters
	Original int   // original index in the input slice
}

// FuzzyMatcher provides fuzzy matching capabilities
type FuzzyMatcher struct {
	caseSensitive bool
}

// NewFuzzyMatcher creates a new fuzzy matcher
func NewFuzzyMatcher(caseSensitive bool) *FuzzyMatcher {
	return &FuzzyMatcher{
		caseSensitive: caseSensitive,
	}
}

// Match performs fuzzy matching on a slice of strings
func (fm *FuzzyMatcher) Match(query string, candidates []string) []FuzzyMatch {
	if query == "" {
		results := make([]FuzzyMatch, len(candidates))
		for i, candidate := range candidates {
			results[i] = FuzzyMatch{
				Text:     candidate,
				Score:    0,
				Indices:  []int{},
				Original: i,
			}
		}
		return results
	}

	var results []FuzzyMatch

	for i, candidate := range candidates {
		if match := fm.matchString(query, candidate, i); match != nil {
			results = append(results, *match)
		}
	}

	// Sort by score (higher is better)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// matchString performs fuzzy matching on a single string
func (fm *FuzzyMatcher) matchString(query, candidate string, originalIndex int) *FuzzyMatch {
	if query == "" {
		return &FuzzyMatch{
			Text:     candidate,
			Score:    0,
			Indices:  []int{},
			Original: originalIndex,
		}
	}

	queryRunes := []rune(query)
	candidateRunes := []rune(candidate)

	if !fm.caseSensitive {
		queryRunes = []rune(strings.ToLower(query))
		candidateRunes = []rune(strings.ToLower(candidate))
	}

	// Check if all characters in query exist in candidate
	queryIdx := 0
	var indices []int
	score := 0

	for i, candidateRune := range candidateRunes {
		if queryIdx < len(queryRunes) && candidateRune == queryRunes[queryIdx] {
			indices = append(indices, i)
			queryIdx++

			// Scoring logic
			if queryIdx == 1 {
				// First character match
				if i == 0 {
					score += 100 // Perfect start
				} else {
					score += 50 // Not at start
				}
			} else {
				// Consecutive matches are better
				if len(indices) > 1 && indices[len(indices)-1] == indices[len(indices)-2]+1 {
					score += 50
				} else {
					score += 20
				}
			}
		}
	}

	// If not all characters matched, return nil
	if queryIdx < len(queryRunes) {
		return nil
	}

	// Bonus for shorter strings
	score += (1000 - len(candidateRunes)) / 10

	// Bonus for word boundary matches
	score += fm.calculateWordBoundaryBonus(query, candidate, indices)

	return &FuzzyMatch{
		Text:     candidate,
		Score:    score,
		Indices:  indices,
		Original: originalIndex,
	}
}

// calculateWordBoundaryBonus calculates bonus for word boundary matches
func (fm *FuzzyMatcher) calculateWordBoundaryBonus(query, candidate string, indices []int) int {
	if len(indices) == 0 {
		return 0
	}

	bonus := 0
	candidateRunes := []rune(candidate)

	for _, idx := range indices {
		if idx == 0 {
			// First character
			bonus += 10
		} else if idx < len(candidateRunes) {
			prevChar := candidateRunes[idx-1]
			if unicode.IsSpace(prevChar) || prevChar == '/' || prevChar == '\\' || prevChar == '.' {
				// Word boundary
				bonus += 15
			} else if unicode.IsLower(prevChar) && unicode.IsUpper(candidateRunes[idx]) {
				// CamelCase boundary
				bonus += 10
			}
		}
	}

	return bonus
}

// FilterMatches filters matches based on a minimum score threshold
func (fm *FuzzyMatcher) FilterMatches(matches []FuzzyMatch, minScore int) []FuzzyMatch {
	var filtered []FuzzyMatch
	for _, match := range matches {
		if match.Score >= minScore {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

// HighlightMatch returns a highlighted version of the match
func (fm *FuzzyMatcher) HighlightMatch(match FuzzyMatch, startTag, endTag string) string {
	if len(match.Indices) == 0 {
		return match.Text
	}

	runes := []rune(match.Text)
	result := make([]rune, 0, len(runes)*2)

	matchSet := make(map[int]bool)
	for _, idx := range match.Indices {
		matchSet[idx] = true
	}

	inHighlight := false
	for i, r := range runes {
		if matchSet[i] && !inHighlight {
			result = append(result, []rune(startTag)...)
			inHighlight = true
		} else if !matchSet[i] && inHighlight {
			result = append(result, []rune(endTag)...)
			inHighlight = false
		}
		result = append(result, r)
	}

	if inHighlight {
		result = append(result, []rune(endTag)...)
	}

	return string(result)
}
