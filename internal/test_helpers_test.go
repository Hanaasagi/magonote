package internal

import "testing"

func mustNewState(t *testing.T, text string, alphabet string, patterns []string, opts ...Option) *State {
	t.Helper()
	s, err := NewState(text, alphabet, patterns, opts...)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

func mustNewStateFromLines(t *testing.T, lines []string, alphabet string, patterns []string, opts ...Option) *State {
	t.Helper()
	s, err := NewStateFromLines(lines, alphabet, patterns, opts...)
	if err != nil {
		t.Fatalf("NewStateFromLines: %v", err)
	}
	return s
}
