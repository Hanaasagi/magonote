package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adrg/xdg"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"

	tdscreen "github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

// ViewColors groups the theme colors used to style matches and hints.
type ViewColors struct {
	selectForeground Color
	selectBackground Color
	multiForeground  Color
	multiBackground  Color
	foreground       Color
	background       Color
	hintForeground   Color
	hintBackground   Color
}

// renderer is the presentation layer. It composes each frame from a cached
// static base (the screen replica) plus precomputed match glyphs, then flushes
// to a tcell screen. Interaction state lives in View; the renderer only reads
// the visual state it is given.
//
// Performance contract: everything that is invariant for the session (base
// cells, glyph geometry, theme styles) is derived once. A steady-state frame
// performs no rune splitting, no grapheme measurement, no color conversion and
// no heap allocation in the composition path — important because the process
// runs with GC disabled.
type renderer struct {
	lines    []string
	screen   *tdscreen.Screen
	matches  []Match
	contrast bool
	position string

	styles renderStyles
	glyphs []matchGlyphs

	base  *TextBuffer
	frame *TextBuffer
}

// renderStyles holds the theme converted to tcell styles once, so per-frame
// styling is a plain selection rather than a color conversion.
type renderStyles struct {
	normal     tcell.Style
	selected   tcell.Style
	chosen     tcell.Style
	hint       tcell.Style
	hintPrefix tcell.Style
}

func newRenderStyles(c ViewColors) renderStyles {
	return renderStyles{
		normal:     tcell.StyleDefault.Foreground(colorToTcell(c.foreground)).Background(colorToTcell(c.background)),
		selected:   tcell.StyleDefault.Foreground(colorToTcell(c.selectForeground)).Background(colorToTcell(c.selectBackground)),
		chosen:     tcell.StyleDefault.Foreground(colorToTcell(c.multiForeground)).Background(colorToTcell(c.multiBackground)),
		hint:       tcell.StyleDefault.Foreground(colorToTcell(c.hintForeground)).Background(colorToTcell(c.hintBackground)),
		hintPrefix: tcell.StyleDefault.Foreground(colorToTcell(c.multiForeground)).Background(colorToTcell(c.multiBackground)),
	}
}

// glyph is one precomputed display cell of a match's text or hint, at an
// absolute logical column. Style is applied at composition time.
type glyph struct {
	x     int
	main  rune
	comb  []rune
	width int
}

// matchGlyphs caches the geometry of a match. Hint glyph slice order equals the
// grapheme index used for prefix highlighting (brackets included, matching the
// historical behavior).
type matchGlyphs struct {
	text    []glyph
	hint    []glyph
	hintStr string
}

func newRenderer(state *State, matches []Match, theme ViewColors, contrast bool, position string) *renderer {
	r := &renderer{
		lines:    state.Lines,
		screen:   state.Screen(),
		matches:  matches,
		contrast: contrast,
		position: position,
		styles:   newRenderStyles(theme),
	}
	r.glyphs = r.precomputeGlyphs(matches)
	return r
}

// render composes and flushes one frame. selected may be nil.
func (r *renderer) render(s tcell.Screen, selected *Match, chosen []ChosenMatch, typedHint string) {
	width, height := s.Size()
	r.ensureBase(width, height)
	r.compose(selected, chosen, typedHint)
	r.flush(s)
}

// ensureBase builds the static screen replica once per screen size.
func (r *renderer) ensureBase(width, height int) {
	if r.base != nil && r.base.width == width && r.base.height == height {
		return
	}
	r.base = NewTextBuffer(r.lines, width, height)
	r.renderBase()
	r.frame = NewTextBuffer(r.lines, width, height)
}

func (r *renderer) renderBase() {
	for y, line := range r.screen.Lines {
		x := 0
		for _, cell := range line.Cells {
			grapheme := cell.Grapheme
			if cell.Style.Attr&tdscreen.AttrHidden != 0 {
				grapheme = " "
			}
			r.base.SetGrapheme(x, y, grapheme, int(cell.Width), screenStyleToTcell(cell.Style))
			x += int(cell.Width)
		}
	}
}

// compose resets the working frame to the base and overlays matches. It touches
// no tcell screen and allocates nothing in steady state.
func (r *renderer) compose(selected *Match, chosen []ChosenMatch, typedHint string) {
	r.frame.CopyFrom(r.base)
	r.composeOverlays(selected, chosen, typedHint)
}

func (r *renderer) composeOverlays(selected *Match, chosen []ChosenMatch, typedHint string) {
	prefixLen := utf8.RuneCountInString(typedHint)
	filtering := typedHint != ""
	for i := range r.matches {
		mat := &r.matches[i]
		g := &r.glyphs[i]

		isChosen := chosenContains(chosen, mat.Text)
		// Progressive filtering: once a hint prefix is typed, hide every
		// non-matching match (revert to the base screen) so the visual field
		// narrows instead of staying cluttered. Already-chosen matches stay
		// visible to preserve multi-select progress.
		if filtering && !isChosen && !isActive(mat, typedHint) {
			continue
		}

		style := r.styles.normal
		switch {
		case isChosen:
			style = r.styles.chosen
		case selected != nil && mat.Equals(*selected):
			style = r.styles.selected
		}
		for _, gl := range g.text {
			r.frame.Put(gl.x, mat.Y, TextCell{Rune: gl.main, Comb: gl.comb, Width: gl.width, Style: style})
		}

		if mat.Hint == nil {
			continue
		}
		prefix := strings.HasPrefix(g.hintStr, typedHint)
		for ci, gl := range g.hint {
			hs := r.styles.hint
			if prefix && ci < prefixLen {
				hs = r.styles.hintPrefix
			}
			r.frame.Put(gl.x, mat.Y, TextCell{Rune: gl.main, Comb: gl.comb, Width: gl.width, Style: hs})
		}
	}
}

// chosenContains reports whether text is already chosen. A linear scan keeps
// the composition path allocation-free; chosen sets are small.
func chosenContains(chosen []ChosenMatch, text string) bool {
	for _, c := range chosen {
		if c.Text == text {
			return true
		}
	}
	return false
}

// flush is the output adapter: project the frame onto the screen and present it.
// tcell's Show diffs cells, so only changed cells reach the terminal.
func (r *renderer) flush(s tcell.Screen) {
	s.Clear()
	r.frame.WriteToScreen(s)
	s.Show()
	if IsDebugMode() {
		writeSnapshot(r.frame.String())
	}
}

// precomputeGlyphs derives match text/hint geometry once.
func (r *renderer) precomputeGlyphs(matches []Match) []matchGlyphs {
	out := make([]matchGlyphs, len(matches))
	for i, mat := range matches {
		visible := mat.Text
		if mat.EndCol > mat.X {
			line := r.screen.Lines[mat.Y]
			byteStart := r.screen.ByteOf(mat.Y, mat.X)
			byteEnd := r.screen.ByteOf(mat.Y, mat.EndCol)
			visible = line.Plain[byteStart:byteEnd]
		}
		text := r.makeText(visible)
		out[i].text = layoutGlyphs(text, mat.X)

		if mat.Hint != nil {
			hint := *mat.Hint
			out[i].hintStr = hint
			offset := mat.X + r.hintPosition(text, hint)
			if offset < 0 {
				offset = 0
			}
			out[i].hint = layoutGlyphs(r.makeText(hint), offset)
		}
	}
	return out
}

// layoutGlyphs splits text into graphemes at absolute logical columns.
func layoutGlyphs(text string, startX int) []glyph {
	var gs []glyph
	x := startX
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		width := tdscreen.GraphemeWidth(graphemes.Str())
		if width <= 0 {
			width = 1
		}
		main, comb := splitGrapheme(graphemes.Str())
		gs = append(gs, glyph{x: x, main: main, comb: comb, width: width})
		x += width
	}
	return gs
}

// splitGrapheme mirrors TextBuffer.SetGrapheme's main/combining handling.
func splitGrapheme(s string) (rune, []rune) {
	runes := []rune(s)
	if len(runes) == 0 {
		return 0, nil
	}
	main, comb := runes[0], runes[1:]
	if uniseg.StringWidth(string(main)) == 0 {
		main, comb = ' ', runes
	}
	return main, comb
}

func (r *renderer) makeText(s string) string {
	if r.contrast {
		return "[" + s + "]"
	}
	return s
}

func (r *renderer) hintPosition(text, hint string) int {
	textWidth := tdscreen.GraphemeWidth(text)
	hintWidth := tdscreen.GraphemeWidth(r.makeText(hint))
	switch r.position {
	case "right":
		return textWidth - hintWidth
	case "off_left":
		return -hintWidth
	case "off_right":
		return textWidth
	default: // "left"
		return 0
	}
}

// writeSnapshot is an explicit debug aid, kept out of the framebuffer.
func writeSnapshot(content string) {
	dir := filepath.Join(xdg.StateHome, "magonote")
	path := filepath.Join(dir, fmt.Sprintf("snapshot-%d.txt", time.Now().UnixMilli()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close() // nolint: errcheck
	_, _ = f.WriteString(content)
}
