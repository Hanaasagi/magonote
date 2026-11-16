package internal

import (
	"testing"
)

func TestHintText(t *testing.T) {
	state := mustNewStateFromLines(t, []string{"lorem 127.0.0.1 lorem"}, "abcd", []string{})
	r := newRenderer(state, state.Matches(false, 0), ViewColors{}, false, "")

	// Without contrast the hint text is unchanged.
	if got := r.makeText("a"); got != "a" {
		t.Errorf("Expected 'a', got '%s'", got)
	}

	// With contrast the hint text is bracketed.
	r.contrast = true
	if got := r.makeText("a"); got != "[a]" {
		t.Errorf("Expected '[a]', got '%s'", got)
	}
}
