package internal

import (
	"strings"

	tdscreen "github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

const extraCapacity = 16

// TextCell is one logical display cell: a base rune, optional combining runes,
// its display width, and its style.
type TextCell struct {
	Rune  rune
	Comb  []rune
	Width int
	Style tcell.Style
}

// TextBuffer is a logical framebuffer. Rows may be wider than the screen;
// WriteToScreen projects (wraps) them onto a tcell screen at grapheme
// boundaries. It carries no I/O concerns — callers own any snapshot output.
type TextBuffer struct {
	content [][]TextCell
	width   int
	height  int
	maxX    int
}

func (tb *TextBuffer) String() string {
	var sb strings.Builder
	for _, row := range tb.content {
		for _, cell := range row {
			if cell.Rune != 0 {
				sb.WriteRune(cell.Rune)
				for _, r := range cell.Comb {
					sb.WriteRune(r)
				}
			}
		}
	}
	return sb.String()
}

func NewTextBuffer(lines []string, width, height int) *TextBuffer {
	maxLineWidth := 0
	for _, line := range lines {
		lineWidth := tdscreen.GraphemeWidth(line) + extraCapacity
		if lineWidth > maxLineWidth {
			maxLineWidth = lineWidth
		}
	}
	content := make([][]TextCell, len(lines))
	for i := range content {
		content[i] = make([]TextCell, maxLineWidth)
	}
	return &TextBuffer{content: content, width: width, height: height, maxX: maxLineWidth}
}

func (tb *TextBuffer) Clear() {
	for i := range tb.content {
		for j := range tb.content[i] {
			tb.content[i][j] = TextCell{}
		}
	}
}

// CopyFrom resets tb to src's contents, reusing row storage. Combining slices
// are shared safely: cells are replaced wholesale, never mutated in place, so
// src stays pristine. A steady-state copy allocates nothing.
func (tb *TextBuffer) CopyFrom(src *TextBuffer) {
	if len(tb.content) != len(src.content) {
		tb.content = make([][]TextCell, len(src.content))
	}
	for y, srcRow := range src.content {
		if cap(tb.content[y]) < len(srcRow) {
			tb.content[y] = make([]TextCell, len(srcRow))
		}
		tb.content[y] = tb.content[y][:len(srcRow)]
		copy(tb.content[y], srcRow)
	}
	tb.width, tb.height, tb.maxX = src.width, src.height, src.maxX
}

func (tb *TextBuffer) SetCell(x, y int, r rune, style tcell.Style) {
	width := tdscreen.GraphemeWidth(string(r))
	tb.SetGrapheme(x, y, string(r), width, style)
}

// SetGrapheme stores one grapheme at a display-column coordinate. Width-zero
// leading runes use a space as tcell's main rune and remain in the comb slice.
func (tb *TextBuffer) SetGrapheme(x, y int, text string, width int, style tcell.Style) {
	if y < 0 || y >= len(tb.content) {
		return
	}
	if len(tb.content[y]) <= x {
		newRow := make([]TextCell, x+extraCapacity)
		copy(newRow, tb.content[y])
		tb.content[y] = newRow
		if x+extraCapacity > tb.maxX {
			tb.maxX = x + extraCapacity
		}
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return
	}
	main, comb := runes[0], runes[1:]
	if uniseg.StringWidth(string(main)) == 0 {
		main, comb = ' ', runes
	}
	tb.content[y][x] = TextCell{Rune: main, Comb: comb, Width: width, Style: style}
}

// Put writes a precomputed cell without re-measuring its grapheme. This is the
// allocation-free hot path the renderer uses for cached glyphs.
func (tb *TextBuffer) Put(x, y int, cell TextCell) {
	if y < 0 || y >= len(tb.content) || x < 0 {
		return
	}
	if len(tb.content[y]) <= x {
		newRow := make([]TextCell, x+extraCapacity)
		copy(newRow, tb.content[y])
		tb.content[y] = newRow
		if x+extraCapacity > tb.maxX {
			tb.maxX = x + extraCapacity
		}
	}
	tb.content[y][x] = cell
}

func (tb *TextBuffer) SetString(x, y int, text string, style tcell.Style) {
	currentX := x
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		width := tdscreen.GraphemeWidth(graphemes.Str())
		if width <= 0 {
			width = 1
		}
		tb.SetGrapheme(currentX, y, graphemes.Str(), width, style)
		currentX += width
	}
}

// WriteToScreen wraps logical rows at grapheme boundaries. A wide grapheme
// that cannot fit in the remaining columns moves to the next screen row.
func (tb *TextBuffer) WriteToScreen(screen tcell.Screen) {
	if tb.width <= 0 {
		return
	}
	screenY := 0
	for y := 0; y < len(tb.content); y++ {
		if screenY >= tb.height {
			break
		}
		row := tb.content[y]
		if len(row) == 0 {
			screenY++
			continue
		}
		maxX := -1
		for x := range row {
			if row[x].Rune != 0 {
				maxX = x
			}
		}
		if maxX == -1 {
			screenY++
			continue
		}

		screenX := 0
		for x := 0; x <= maxX; x++ {
			cell := row[x]
			if screenX >= tb.width {
				screenY++
				screenX = 0
			}
			width := 1
			if cell.Rune != 0 {
				width = cell.Width
				if width <= 0 {
					width = tdscreen.GraphemeWidth(string(cell.Rune))
				}
			}
			if cell.Rune != 0 && screenX+width > tb.width {
				screenY++
				screenX = 0
			}
			if screenY >= tb.height {
				break
			}
			if cell.Rune != 0 {
				screen.SetContent(screenX, screenY, cell.Rune, cell.Comb, cell.Style)
			}
			screenX++
		}
		screenY++
	}
}
