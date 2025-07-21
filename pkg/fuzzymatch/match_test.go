package fuzzymatch

import (
	"testing"
)

// TestExactMatch tests exact fuzzy matching
func TestExactMatch(t *testing.T) {
	matcher := NewFuzzyMatcher(false)
	query := "abc"
	candidates := []string{"abc", "acb", "a_bc"}

	results := matcher.Match(query, candidates)
	if len(results) == 0 || results[0].Text != "abc" {
		t.Errorf("Expected 'abc' to match first, got: %+v", results)
	}
}

// TestPartialMatch tests a case where characters are present but not in order
func TestPartialMatch(t *testing.T) {
	matcher := NewFuzzyMatcher(false)
	query := "abc"
	candidates := []string{"acb", "cab", "bac"}

	results := matcher.Match(query, candidates)
	if len(results) != 0 {
		t.Errorf("Expected no matches for disordered input, got: %+v", results)
	}
}
