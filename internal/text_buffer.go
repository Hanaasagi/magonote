package internal

import (
	"fmt"
	"github.com/adrg/xdg"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const extraCapacity = 16

type TextCell struct {
	Rune  rune
	Style tcell.Style
}

// TextBuffer manages text rendering with automatic wrapping
type TextBuffer struct {
	content [][]TextCell // [row][column] -> TextCell, stores original positions
	width   int          // Terminal width
	height  int          // Terminal height
	maxX    int          // Maximum X coordinate for each line
}

func (tb *TextBuffer) String() string {
	var sb strings.Builder
	for _, row := range tb.content {
		for _, cell := range row {
			if cell.Rune != 0 {
				sb.WriteRune(cell.Rune)
			}
		}
	}
	return sb.String()
}

func NewTextBuffer(lines []string, width, height int) *TextBuffer {
	maxLineWidth := 0
	for _, line := range lines {
		lineWidth := len([]rune(line)) + extraCapacity
		if lineWidth > maxLineWidth {
			maxLineWidth = lineWidth
		}
	}

	content := make([][]TextCell, len(lines))
	for i := range content {
		content[i] = make([]TextCell, maxLineWidth)
	}

	return &TextBuffer{
		content: content,
		width:   width,
		height:  height,
		maxX:    maxLineWidth,
	}
}

// Clear clears the buffer
func (tb *TextBuffer) Clear() {
	for i := range tb.content {
		for j := range tb.content[i] {
			tb.content[i][j] = TextCell{}
		}
	}
}

// SetCell sets a character at the specified original coordinates
// The buffer stores content without wrapping - wrapping is applied only when writing to screen
func (tb *TextBuffer) SetCell(x, y int, r rune, style tcell.Style) {
	// Ensure the row is wide enough
	if len(tb.content[y]) <= x {
		newRow := make([]TextCell, x+extraCapacity) // Add some buffer
		copy(newRow, tb.content[y])
		tb.content[y] = newRow
		if x+extraCapacity > tb.maxX {
			tb.maxX = x + extraCapacity
		}
	}

	// Store the cell at its original coordinates
	tb.content[y][x] = TextCell{
		Rune:  r,
		Style: style,
	}
}

// SetString sets a string at the specified original coordinates
func (tb *TextBuffer) SetString(x, y int, text string, style tcell.Style) {
	currentX := x
	for _, r := range text {
		tb.SetCell(currentX, y, r, style)

		// Calculate the width of the current rune
		width := runewidth.RuneWidth(r)
		if width <= 0 {
			width = 1
		}
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
	defer f.Close() // nolint

	_, err = f.WriteString(tb.String())
	return err
}

// WriteToScreen writes the buffer content to a tcell screen with automatic wrapping
func (tb *TextBuffer) WriteToScreen(screen tcell.Screen) {
	if tb.width <= 0 {
		return
	}

	if IsDebugMode() {
		tb.dumpSnapshot() // nolint
	}

	screenY := 0

	// Process each line in order
	for y := 0; y < len(tb.content); y++ {
		if screenY >= tb.height {
			break // Screen is full
		}

		row := tb.content[y]
		if len(row) == 0 {
			screenY++ // Empty line takes one screen line
			continue
		}

		// Find the last non-empty cell in this row
		maxX := -1
		for x := 0; x < len(row); x++ {
			cell := row[x]
			if cell.Rune != 0 {
				maxX = x
			}
		}

		if maxX == -1 {
			screenY++ // No content in this line
			continue
		}

		// Render this line with wrapping
		for x := 0; x <= maxX; x++ {
			if screenY >= tb.height {
				break
			}

			cell := row[x]

			// Calculate screen position considering wrapping
			screenX := x % tb.width
			if x > 0 && screenX == 0 {
				// We've wrapped to a new line
				screenY++
				if screenY >= tb.height {
					break
				}
			}

			// Set content on screen
			if cell.Rune != 0 && cell.Rune != ' ' {
				screen.SetContent(screenX, screenY, cell.Rune, nil, cell.Style)
			}
		}

		screenY++ // Move to next line after processing this original line
	}
}
