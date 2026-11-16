package internal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEnterRespectsFilteredField(t *testing.T) {
	hq, hw := "q", "w"
	matches := []Match{
		{Text: "alpha", X: 0, Y: 0, Hint: &hq},
		{Text: "beta", X: 6, Y: 0, Hint: &hw},
	}
	// Cursor sits on beta, but the typed prefix "q" filters beta out.
	v := &View{matches: matches, skip: 1}
	action := v.handleEnter("q")
	if action == nil || *action != HintEvent {
		t.Fatalf("action = %v, want HintEvent", action)
	}
	if len(v.chosen) != 1 || v.chosen[0].Text != "alpha" {
		t.Fatalf("chosen = %#v, want alpha (first active), not the filtered-out beta", v.chosen)
	}
}

func TestNavigationStaysInFilteredField(t *testing.T) {
	hq, hw := "q", "w"
	matches := []Match{
		{Text: "alpha", Hint: &hq},
		{Text: "beta", Hint: &hw},
		{Text: "gamma", Hint: &hq},
	}
	v := &View{matches: matches, skip: 0}
	v.Next("q") // skips beta (w), lands on gamma (q)
	if v.skip != 2 {
		t.Fatalf("Next skip = %d, want 2", v.skip)
	}
	v.Prev("q") // back over beta to alpha
	if v.skip != 0 {
		t.Fatalf("Prev skip = %d, want 0", v.skip)
	}
}

func TestTypingClampsSkipToVisible(t *testing.T) {
	hqa, hwb := "qa", "wb"
	matches := []Match{
		{Text: "alpha", Hint: &hqa},
		{Text: "beta", Hint: &hwb},
	}
	v := &View{matches: matches, skip: 1, multi: true}
	typedHint, hasUppercase := "", false
	// Partial prefix "q" filters to alpha; the cursor on beta must clamp to it.
	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), &typedHint, &hasUppercase, "wb")
	if typedHint != "q" {
		t.Fatalf("typedHint = %q, want q", typedHint)
	}
	if v.skip != 0 {
		t.Fatalf("skip = %d, want 0 (clamped to the visible match)", v.skip)
	}
}

func TestEnterNoActiveDoesNothing(t *testing.T) {
	hq := "q"
	matches := []Match{{Text: "alpha", Hint: &hq}}
	v := &View{matches: matches, skip: 0, multi: true}
	// Prefix "z" matches nothing; Enter must not select a hidden match.
	if action := v.handleEnter("z"); action != nil {
		t.Fatalf("action = %v, want nil", action)
	}
	if len(v.chosen) != 0 {
		t.Fatalf("chosen = %#v, want empty", v.chosen)
	}
}
