package internal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func assertRuneAt(t *testing.T, s tcell.SimulationScreen, x, y int, want rune) {
	t.Helper()
	main, _, _, _ := s.GetContent(x, y)
	if main != want {
		t.Fatalf("cell (%d,%d) = %q, want %q", x, y, main, want)
	}
}

func filterTheme() ViewColors {
	return ViewColors{
		foreground:      GetColor("green"),
		background:      GetColor("black"),
		hintForeground:  GetColor("yellow"),
		hintBackground:  GetColor("black"),
		multiForeground: GetColor("red"),
		multiBackground: GetColor("black"),
	}
}

func TestProgressiveFilterHidesNonMatching(t *testing.T) {
	state := mustNewState(t, "alpha beta gamma", "abcd", nil)
	hq, hw, he := "q", "w", "e"
	matches := []Match{
		{Text: "alpha", X: 0, EndCol: 5, Y: 0, Hint: &hq},
		{Text: "beta", X: 6, EndCol: 10, Y: 0, Hint: &hw},
		{Text: "gamma", X: 11, EndCol: 16, Y: 0, Hint: &he},
	}
	screen := simulationScreen(t, 20, 1)
	r := newRenderer(state, matches, filterTheme(), false, "left")

	// No filter: every hint is drawn over its match's first column.
	r.render(screen, nil, nil, "", "")
	assertRuneAt(t, screen, 0, 0, 'q')
	assertRuneAt(t, screen, 6, 0, 'w')
	assertRuneAt(t, screen, 11, 0, 'e')

	// Typing "q" keeps only the q-hinted match; the others revert to base text.
	r.render(screen, nil, nil, "q", "")
	assertRuneAt(t, screen, 0, 0, 'q')
	assertRuneAt(t, screen, 6, 0, 'b')
	assertRuneAt(t, screen, 11, 0, 'g')

	// Clearing the prefix (backspace to empty) restores the full field.
	r.render(screen, nil, nil, "", "")
	assertRuneAt(t, screen, 6, 0, 'w')
	assertRuneAt(t, screen, 11, 0, 'e')
}

func TestProgressiveFilterKeepsChosen(t *testing.T) {
	state := mustNewState(t, "alpha beta gamma", "abcd", nil)
	hq, hw, he := "q", "w", "e"
	matches := []Match{
		{Text: "alpha", X: 0, EndCol: 5, Y: 0, Hint: &hq},
		{Text: "beta", X: 6, EndCol: 10, Y: 0, Hint: &hw},
		{Text: "gamma", X: 11, EndCol: 16, Y: 0, Hint: &he},
	}
	screen := simulationScreen(t, 20, 1)
	r := newRenderer(state, matches, filterTheme(), false, "left")

	// beta is already chosen; typing "q" must keep it visible (multi progress)
	// even though its hint does not match the prefix.
	chosen := []ChosenMatch{{Text: "beta"}}
	r.render(screen, nil, chosen, "q", "")
	assertRuneAt(t, screen, 0, 0, 'q') // alpha matches prefix
	assertRuneAt(t, screen, 6, 0, 'w') // beta stays (chosen)
	assertRuneAt(t, screen, 11, 0, 'g') // gamma hidden
}

func TestProgressiveFilterNarrowsByPrefix(t *testing.T) {
	state := mustNewState(t, "alpha beta gamma", "abcd", nil)
	hqa, hqb, hw := "qa", "qb", "w"
	matches := []Match{
		{Text: "alpha", X: 0, EndCol: 5, Y: 0, Hint: &hqa},
		{Text: "beta", X: 6, EndCol: 10, Y: 0, Hint: &hqb},
		{Text: "gamma", X: 11, EndCol: 16, Y: 0, Hint: &hw},
	}
	screen := simulationScreen(t, 20, 1)
	r := newRenderer(state, matches, filterTheme(), false, "left")

	// "q" keeps both qa and qb, hides w.
	r.render(screen, nil, nil, "q", "")
	assertRuneAt(t, screen, 0, 0, 'q')
	assertRuneAt(t, screen, 6, 0, 'q')
	assertRuneAt(t, screen, 11, 0, 'g')

	// "qa" keeps only qa; qb reverts to base.
	r.render(screen, nil, nil, "qa", "")
	assertRuneAt(t, screen, 0, 0, 'q')
	assertRuneAt(t, screen, 6, 0, 'b')
	assertRuneAt(t, screen, 11, 0, 'g')
}

func TestProgressiveFilterPrefixHighlight(t *testing.T) {
	state := mustNewState(t, "alpha", "abcd", nil)
	hq := "q"
	matches := []Match{{Text: "alpha", X: 0, EndCol: 5, Y: 0, Hint: &hq}}
	screen := simulationScreen(t, 8, 1)
	r := newRenderer(state, matches, filterTheme(), false, "left")

	fg := func() tcell.Color {
		_, _, style, _ := screen.GetContent(0, 0)
		c, _, _ := style.Decompose()
		return c
	}

	// Untyped: hint uses the hint color (yellow).
	r.render(screen, nil, nil, "", "")
	if fg() != tcell.ColorYellow {
		t.Fatalf("untyped hint fg = %v, want yellow", fg())
	}
	// Typed prefix: hint uses the prefix color (multi = red).
	r.render(screen, nil, nil, "q", "")
	if fg() != tcell.ColorRed {
		t.Fatalf("typed hint fg = %v, want red", fg())
	}
}
