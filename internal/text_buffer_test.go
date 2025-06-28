package internal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTextBuffer_BasicFunctionality(t *testing.T) {
	lines := []string{"hello", "world"}
	buffer := NewTextBuffer(lines, 10, 5)

	// Test setting a simple string
	buffer.SetString(0, 0, "hello", tcell.StyleDefault)

	// Check if content was stored correctly
	if len(buffer.content) == 0 {
		t.Error("Buffer should have content")
		return
	}

	// Check individual characters
	expectedChars := []rune("hello")
	for i, expected := range expectedChars {
		cell := buffer.content[0][i]
		if cell.Rune != expected {
			t.Errorf("Expected rune %c at position %d, got %c", expected, i, cell.Rune)
		}
	}
}

func TestTextBuffer_SetString(t *testing.T) {
	lines := []string{"test line 1", "test line 2"}
	buffer := NewTextBuffer(lines, 10, 5)

	// Test setting a simple string
	buffer.SetString(0, 0, "hello", tcell.StyleDefault)

	// Check if content was stored correctly
	if len(buffer.content) == 0 {
		t.Error("Buffer should have content")
		return
	}

	cell := buffer.content[0][0]
	if cell.Rune != 'h' {
		t.Error("First character should be 'h'")
	}

	// Test setting text at an offset
	buffer.SetString(2, 1, "world", tcell.StyleDefault)
	if len(buffer.content) < 2 {
		t.Error("Buffer should have at least 2 rows")
		return
	}

	cell = buffer.content[1][2]
	if cell.Rune != 'w' {
		t.Error("Character at position (2,1) should be 'w'")
	}
}

func TestTextBuffer_SetCell(t *testing.T) {
	lines := []string{"line1", "line2"}
	buffer := NewTextBuffer(lines, 10, 5)

	// Test setting individual cells
	buffer.SetCell(0, 0, 'A', tcell.StyleDefault)
	buffer.SetCell(1, 0, 'B', tcell.StyleDefault)
	buffer.SetCell(0, 1, 'C', tcell.StyleDefault)

	// Check the cells
	if len(buffer.content) < 2 {
		t.Error("Buffer should have at least 2 rows")
		return
	}

	if cell := buffer.content[0][0]; cell.Rune != 'A' {
		t.Error("Cell (0,0) should contain 'A'")
	}

	if cell := buffer.content[0][1]; cell.Rune != 'B' {
		t.Error("Cell (1,0) should contain 'B'")
	}

	if cell := buffer.content[1][0]; cell.Rune != 'C' {
		t.Error("Cell (0,1) should contain 'C'")
	}
}

func TestTextBuffer_Clear(t *testing.T) {
	lines := []string{"test line"}
	buffer := NewTextBuffer(lines, 10, 5)

	// Add some content
	buffer.SetString(0, 0, "hello", tcell.StyleDefault)

	// Verify content exists
	if len(buffer.content) == 0 {
		t.Error("Buffer should have content before clear")
	}

	// Clear and verify
	buffer.Clear()

	// Check that content is cleared (all cells should be empty)
	hasContent := false
	for i := range buffer.content {
		for j := range buffer.content[i] {
			if buffer.content[i][j].Rune != 0 {
				hasContent = true
				break
			}
		}
		if hasContent {
			break
		}
	}

	if hasContent {
		t.Error("Buffer should be empty after clear")
	}
}

func TestTextBuffer_WriteToScreen(t *testing.T) {
	lines := []string{"hello", "world"}
	buffer := NewTextBuffer(lines, 10, 5)

	// Create a mock screen for testing
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()
	if err != nil {
		t.Fatalf("Failed to initialize simulation screen: %v", err)
	}
	defer screen.Fini()

	// Set some content
	buffer.SetString(0, 0, "hello", tcell.StyleDefault)
	buffer.SetString(0, 1, "world", tcell.StyleDefault)

	// Write to screen (should not panic)
	buffer.WriteToScreen(screen)
	screen.Show()

	// Basic check that screen dimensions are valid
	width, height := screen.Size()
	if width <= 0 || height <= 0 {
		t.Error("Screen dimensions should be positive")
	}
}

func TestTextBuffer_LongLineWrapping(t *testing.T) {
	// Test with a narrow buffer to force wrapping
	longText := "This is a very long line that should wrap"
	lines := []string{longText}
	buffer := NewTextBuffer(lines, 5, 10)

	// Set a long line that should wrap
	buffer.SetString(0, 0, longText, tcell.StyleDefault)

	// Create a mock screen
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()
	if err != nil {
		t.Fatalf("Failed to initialize simulation screen: %v", err)
	}
	defer screen.Fini()

	// Write to screen - should handle wrapping without errors
	buffer.WriteToScreen(screen)
	screen.Show()

	// Verify that the content was stored correctly in the buffer
	if len(buffer.content) == 0 {
		t.Error("Buffer should have content")
		return
	}

	// Check that at least some characters were stored
	hasContent := false
	for j := range buffer.content[0] {
		if buffer.content[0][j].Rune != 0 {
			hasContent = true
			break
		}
	}

	if !hasContent {
		t.Error("Row should contain some characters")
	}

	// Check first few characters
	expectedChars := []rune(longText)
	for i := 0; i < min(len(expectedChars), 10); i++ {
		if cell := buffer.content[0][i]; cell.Rune != expectedChars[i] {
			t.Errorf("Character at position %d should be '%c', got '%c'",
				i, expectedChars[i], cell.Rune)
		}
	}
}

func TestTextBuffer_MultipleLines(t *testing.T) {
	lines := []string{
		"Line 1",
		"Line 2 is longer",
		"Line 3",
	}
	buffer := NewTextBuffer(lines, 20, 10)

	// Set multiple lines
	for i, line := range lines {
		buffer.SetString(0, i, line, tcell.StyleDefault)
	}

	// Verify all lines are stored
	for i, expectedLine := range lines {
		if i >= len(buffer.content) {
			t.Errorf("Row %d should exist", i)
			continue
		}

		expectedChars := []rune(expectedLine)
		for j, expectedChar := range expectedChars {
			if j >= len(buffer.content[i]) {
				t.Errorf("Column %d in row %d should exist", j, i)
				continue
			}
			if cell := buffer.content[i][j]; cell.Rune != expectedChar {
				t.Errorf("Character at position (%d,%d) should be '%c', got '%c'",
					j, i, expectedChar, cell.Rune)
			}
		}
	}
}
