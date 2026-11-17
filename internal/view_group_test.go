package internal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestGroupKeySelectsAction(t *testing.T) {
	hq, hw := "q", "w"
	matches := []Match{{Text: "alpha", Hint: &hq}, {Text: "beta", Hint: &hw}}
	v := newTestView(matches, 0, false)
	typedHint := ""

	// 'o' is the open group key: switches group, resets hint input.
	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone), &typedHint, "w")
	if v.pendingGroup.Name != "open" {
		t.Fatalf("group = %s, want open", v.pendingGroup.Name)
	}
	if typedHint != "" {
		t.Fatalf("typedHint = %q, want empty after group key", typedHint)
	}

	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), &typedHint, "w")
	if len(v.chosen) != 1 || v.chosen[0].Group != "open" || v.chosen[0].Text != "alpha" {
		t.Fatalf("chosen = %#v, want alpha under open group", v.chosen)
	}
}

func TestBareHintCopies(t *testing.T) {
	hq := "q"
	matches := []Match{{Text: "alpha", Hint: &hq}}
	v := newTestView(matches, 0, false)
	typedHint := ""
	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), &typedHint, "q")
	if len(v.chosen) != 1 || v.chosen[0].Group != "copy" {
		t.Fatalf("chosen = %#v, want copy group", v.chosen)
	}
}

func TestUppercaseIsOpenAlias(t *testing.T) {
	hq := "q"
	matches := []Match{{Text: "alpha", Hint: &hq}}
	v := newTestView(matches, 0, false)
	typedHint := ""
	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'Q', tcell.ModNone), &typedHint, "q")
	if len(v.chosen) != 1 || v.chosen[0].Group != "open" {
		t.Fatalf("chosen = %#v, want open group via uppercase", v.chosen)
	}
}

func TestBackspaceResetsGroup(t *testing.T) {
	hq := "q"
	matches := []Match{{Text: "alpha", Hint: &hq}}
	v := newTestView(matches, 0, false)
	typedHint := ""
	v.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone), &typedHint, "q")
	if v.pendingGroup.Name != "open" {
		t.Fatalf("group = %s, want open", v.pendingGroup.Name)
	}
	v.handleBackspace(&typedHint)
	if v.pendingGroup.Name != "copy" {
		t.Fatalf("group = %s, want copy after backspace", v.pendingGroup.Name)
	}
}
