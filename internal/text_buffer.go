package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

const extraCapacity = 16

type TextCell struct {
	Rune  rune
	Comb  []rune
	Width int
	Style tcell.Style
}

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
		lineWidth := runewidth.StringWidth(line) + extraCapacity
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

func (tb *TextBuffer) SetCell(x, y int, r rune, style tcell.Style) {
	width := runewidth.RuneWidth(r)
	if width <= 0 {
		width = 1
	}
	tb.SetGrapheme(x, y, string(r), width, style)
}

// SetGrapheme stores one grapheme at a display-column coordinate. Width-zero
// leading runes use a space as tcell's main rune and remain in the comb slice.
func (tb *TextBuffer) SetGrapheme(x, y int, text string, width int, style tcell.Style) {
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
	if runewidth.RuneWidth(main) == 0 {
		main, comb = ' ', runes
	}
	tb.content[y][x] = TextCell{Rune: main, Comb: comb, Width: width, Style: style}
}

func (tb *TextBuffer) SetString(x, y int, text string, style tcell.Style) {
	currentX := x
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		width := runewidth.StringWidth(graphemes.Str())
		if width <= 0 {
			width = 1
		}
		tb.SetGrapheme(currentX, y, graphemes.Str(), width, style)
		currentX += width
	}
}

func (tb *TextBuffer) dumpSnapshot() error {
	unixMilli := time.Now().UnixMilli()
	appDir := filepath.Join(xdg.StateHome, "magonote")
	filePath := filepath.Join(appDir, fmt.Sprintf("snapshot-%d.txt", unixMilli))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close() // nolint: errcheck
	_, err = f.WriteString(tb.String())
	return err
}

// WriteToScreen wraps logical rows at grapheme boundaries. A wide grapheme
// that cannot fit in the remaining columns moves to the next screen row.
func (tb *TextBuffer) WriteToScreen(screen tcell.Screen) {
	if tb.width <= 0 {
		return
	}
	if IsDebugMode() {
		tb.dumpSnapshot() // nolint: errcheck
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
					width = runewidth.RuneWidth(cell.Rune)
					if width <= 0 {
						width = 1
					}
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
