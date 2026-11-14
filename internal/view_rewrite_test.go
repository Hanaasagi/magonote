package internal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func simulationScreen(t *testing.T, width, height int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(width, height)
	t.Cleanup(screen.Fini)
	return screen
}

func TestTextBufferWritesGraphemesAndWrapsWideCells(t *testing.T) {
	screen := simulationScreen(t, 4, 3)
	buffer := NewTextBuffer([]string{""}, 4, 3)
	buffer.SetGrapheme(0, 0, "A", 1, tcell.StyleDefault)
	buffer.SetGrapheme(1, 0, "中", 2, tcell.StyleDefault)
	buffer.SetGrapheme(3, 0, "B", 1, tcell.StyleDefault)
	buffer.SetGrapheme(4, 0, "👩‍💻", 2, tcell.StyleDefault)
	buffer.WriteToScreen(screen)

	assertRune := func(x, y int, want rune) {
		t.Helper()
		main, _, _, _ := screen.GetContent(x, y)
		if main != want {
			t.Errorf("cell %d,%d = %q, want %q", x, y, main, want)
		}
	}
	assertRune(0, 0, 'A')
	assertRune(1, 0, '中')
	assertRune(3, 0, 'B')
	assertRune(0, 1, '👩')
}

func TestTextBufferHintOverlayPreservesWideCharacterColumns(t *testing.T) {
	screen := simulationScreen(t, 6, 1)
	buffer := NewTextBuffer([]string{""}, 6, 1)
	buffer.SetGrapheme(0, 0, "中", 2, tcell.StyleDefault)
	buffer.SetGrapheme(2, 0, "文", 2, tcell.StyleDefault)
	buffer.SetGrapheme(0, 0, "a", 1, tcell.StyleDefault)
	buffer.WriteToScreen(screen)
	main, _, _, _ := screen.GetContent(2, 0)
	if main != '文' {
		t.Fatalf("character after overlay is at column 1 or missing; column 2 = %q", main)
	}
}

func TestTextBufferFullWidthLineDoesNotSkipNextRow(t *testing.T) {
	screen := simulationScreen(t, 4, 3)
	buffer := NewTextBuffer([]string{"ABCD", "E"}, 4, 3)
	buffer.SetString(0, 0, "ABCD", tcell.StyleDefault)
	buffer.SetString(0, 1, "E", tcell.StyleDefault)
	buffer.WriteToScreen(screen)
	main, _, _, _ := screen.GetContent(0, 1)
	if main != 'E' {
		t.Fatalf("second logical line at (0,1) = %q, want E", main)
	}
}

func TestTextBufferKeepsCombiningGraphemeInOneCell(t *testing.T) {
	screen := simulationScreen(t, 4, 1)
	buffer := NewTextBuffer([]string{""}, 4, 1)
	buffer.SetString(0, 0, "éx", tcell.StyleDefault)
	buffer.WriteToScreen(screen)
	main, comb, _, _ := screen.GetContent(0, 0)
	if main != 'e' || len(comb) == 0 {
		t.Fatalf("combining cell = %q %#v", main, comb)
	}
	main, _, _, _ = screen.GetContent(1, 0)
	if main != 'x' {
		t.Fatalf("next cell = %q, want x", main)
	}
}

func TestTextBufferWritesStyledSpaces(t *testing.T) {
	screen := simulationScreen(t, 2, 1)
	style := tcell.StyleDefault.Background(tcell.ColorRed)
	buffer := NewTextBuffer([]string{""}, 2, 1)
	buffer.SetGrapheme(0, 0, " ", 1, style)
	buffer.WriteToScreen(screen)
	_, _, got, _ := screen.GetContent(0, 0)
	_, background, _ := got.Decompose()
	if background != tcell.ColorRed {
		t.Fatalf("space background = %v", background)
	}
}

func TestViewHyperlinkKeepsVisibleLabel(t *testing.T) {
	state := mustNewState(t, "\x1b]8;;https://example.com\adocs\x1b]8;;\a", "abcd", nil, WithColorDetection())
	matches := state.Matches(false, 0)
	matches[0].Hint = nil
	screen := simulationScreen(t, 20, 1)
	view := &View{state: state, screen: screen, matches: matches, position: "left", colors: ViewColors{hintForeground: GetColor("yellow"), hintBackground: GetColor("black")}}
	view.textBuffer = NewTextBuffer(state.Lines, 20, 1)
	view.renderTextLines()
	view.renderSingleMatch(&matches[0], tcell.StyleDefault, "")
	view.textBuffer.WriteToScreen(screen)
	for x, want := range []rune("docs") {
		main, _, _, _ := screen.GetContent(x, 0)
		if main != want {
			t.Fatalf("visible cell %d = %q, want %q", x, main, want)
		}
	}
	main, _, _, _ := screen.GetContent(4, 0)
	if main == 'h' {
		t.Fatalf("URI overwrote text after visible label")
	}
}

func TestViewSelectionPreservesFileLocation(t *testing.T) {
	state := mustNewState(t, "main.go:12:4", "abcd", nil)
	matches := state.Matches(false, 0)
	view := &View{matches: matches}
	typedHint := ""
	hasUppercase := false
	longestHint := "a"
	view.handleRuneKey(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone), &typedHint, &hasUppercase, longestHint)
	if len(view.chosen) != 1 || view.chosen[0].Location == nil {
		t.Fatalf("chosen = %#v", view.chosen)
	}
	location := view.chosen[0].Location
	if location.Path != "main.go" || location.Line != 12 || location.Column != 4 {
		t.Fatalf("chosen location = %#v", location)
	}
}

func TestViewConcealsHiddenText(t *testing.T) {
	state := mustNewState(t, "\x1b[8msecret\x1b[0m", "abcd", nil)
	screen := simulationScreen(t, 8, 1)
	view := &View{state: state, screen: screen}
	view.textBuffer = NewTextBuffer(state.Lines, 8, 1)
	view.renderTextLines()
	view.textBuffer.WriteToScreen(screen)
	for x := 0; x < len("secret"); x++ {
		main, _, _, _ := screen.GetContent(x, 0)
		if main != ' ' {
			t.Fatalf("hidden cell %d = %q, want space", x, main)
		}
	}
}

func TestViewRestoresANSIColor(t *testing.T) {
	state := mustNewState(t, "\x1b[31mred\x1b[0m", "abcd", nil)
	screen := simulationScreen(t, 8, 1)
	view := &View{state: state, screen: screen}
	view.textBuffer = NewTextBuffer(state.Lines, 8, 1)
	view.renderTextLines()
	view.textBuffer.WriteToScreen(screen)
	_, _, style, _ := screen.GetContent(0, 0)
	foreground, _, _ := style.Decompose()
	if foreground != tcell.PaletteColor(1) {
		t.Fatalf("foreground = %v, want ANSI red", foreground)
	}
}
