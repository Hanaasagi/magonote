package internal

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	fz "github.com/Hanaasagi/magonote/pkg/fuzzymatch"
	"github.com/fatih/color"
	"golang.org/x/term"
)

const (
	// Default configuration
	defaultMaxVisibleItems = 10
	defaultWidth           = 80
	defaultHeight          = 24

	// Control characters
	ctrlC = 3   // Ctrl+C
	esc   = 27  // ESC
	del   = 127 // Backspace/Delete
	bs    = 8   // Backspace
	enter = 13  // Enter
	ctrlU = 21  // Ctrl+U (clear input)
	ctrlP = 16  // Ctrl+P (up)
	ctrlN = 14  // Ctrl+N (down)
	ctrlJ = 10  // Ctrl+J (down)
	ctrlK = 11  // Ctrl+K (up)
	tab   = 9   // Tab
)

// ListView represents a direct terminal-based dropdown selector
type ListView struct {
	// Core state
	state           *State
	candidates      []string
	filteredMatches []fz.FuzzyMatch
	selectedIndex   int
	scrollOffset    int
	query           string
	fuzzyMatcher    *fz.FuzzyMatcher
	multi           bool
	chosen          []ChosenMatch

	// Display configuration
	maxVisibleItems    int
	originalTotalWidth int // Width based on original total count for consistent layout

	// Terminal state
	originalState *term.State
	width         int
	height        int
	startRow      int
	startCol      int

	// Terminal I/O
	ttyin  *os.File
	ttyout *os.File

	// Colors
	colors ViewColors

	// Color functions using fatih/color
	selectColor *color.Color
	chosenColor *color.Color
	normalColor *color.Color
}

// NewListView creates a new direct terminal ListView instance
func NewListView(
	state *State,
	multi bool,
	selectForegroundColor Color,
	selectBackgroundColor Color,
	multiForegroundColor Color,
	multiBackgroundColor Color,
	foregroundColor Color,
	backgroundColor Color,
	hintForegroundColor Color,
	hintBackgroundColor Color,
) *ListView {
	// Extract candidate texts from matches
	matches := state.Matches(false, 2) // list view should only show unique matches
	candidates := make([]string, len(matches))
	for i, match := range matches {
		candidates[i] = match.Text
	}

	lv := &ListView{
		state:              state,
		candidates:         candidates,
		filteredMatches:    []fz.FuzzyMatch{},
		selectedIndex:      0,
		scrollOffset:       0,
		query:              "",
		fuzzyMatcher:       fz.NewFuzzyMatcher(false),
		maxVisibleItems:    defaultMaxVisibleItems,
		multi:              multi,
		chosen:             make([]ChosenMatch, 0),
		originalTotalWidth: len(fmt.Sprintf("%d", len(candidates))),
		colors: ViewColors{
			selectForeground: selectForegroundColor,
			selectBackground: selectBackgroundColor,
			multiForeground:  multiForegroundColor,
			multiBackground:  multiBackgroundColor,
			foreground:       foregroundColor,
			background:       backgroundColor,
			hintForeground:   hintForegroundColor,
			hintBackground:   hintBackgroundColor,
		},
		selectColor: color.New(color.BgCyan, color.FgBlack),
		chosenColor: color.New(color.FgGreen, color.Bold),
		normalColor: color.New(color.Reset),
	}

	return lv
}

// initTerminal initializes terminal for direct manipulation
func (lv *ListView) initTerminal() error {
	if err := lv.openTTY(); err != nil {
		return err
	}

	if err := lv.setupRawMode(); err != nil {
		return err
	}

	if err := lv.getTerminalSize(); err != nil {
		lv.setDefaultSize()
	}

	if err := lv.getCurrentPosition(); err != nil {
		lv.setFallbackPosition()
	}

	return nil
}

// openTTY opens the TTY for direct I/O
func (lv *ListView) openTTY() error {
	var err error
	lv.ttyin, err = os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	lv.ttyout = lv.ttyin
	return nil
}

// setupRawMode configures the terminal for raw input
func (lv *ListView) setupRawMode() error {
	var err error
	lv.originalState, err = term.MakeRaw(int(lv.ttyin.Fd()))
	return err
}

// getTerminalSize retrieves the current terminal dimensions
func (lv *ListView) getTerminalSize() error {
	var err error
	lv.width, lv.height, err = term.GetSize(int(lv.ttyin.Fd()))
	return err
}

// setDefaultSize sets fallback terminal dimensions
func (lv *ListView) setDefaultSize() {
	lv.width = defaultWidth
	lv.height = defaultHeight
}

// setFallbackPosition sets a fallback cursor position
func (lv *ListView) setFallbackPosition() {
	lv.startRow = lv.height - 1
	lv.startCol = 0
}

// getCurrentPosition gets the current cursor position using ANSI sequence
func (lv *ListView) getCurrentPosition() error {
	// Send cursor position request
	lv.write("\x1b[6n")

	// Read response
	buf := make([]byte, 32)
	n, err := lv.ttyin.Read(buf)
	if err != nil {
		return err
	}

	// Parse response: \x1b[row;colR
	re := regexp.MustCompile(`\x1b\[(\d+);(\d+)R`)
	matches := re.FindSubmatch(buf[:n])
	if len(matches) >= 3 {
		row, _ := strconv.Atoi(string(matches[1]))
		col, _ := strconv.Atoi(string(matches[2]))
		lv.startRow = row - 1 // Convert to 0-based
		lv.startCol = col - 1
		return nil
	}

	return fmt.Errorf("failed to parse cursor position")
}

// cleanup restores terminal state
func (lv *ListView) cleanup() {
	if lv.originalState != nil {
		_ = term.Restore(int(lv.ttyin.Fd()), lv.originalState)
	}
	if lv.ttyin != nil && lv.ttyin != os.Stdin {
		_ = lv.ttyin.Close()
	}
}

// write sends text to terminal
func (lv *ListView) write(text string) {
	_, err := lv.ttyout.WriteString(text)
	if err != nil {
		panic(err)
	}
}

// writeColored sends colored text to terminal using fatih/color
func (lv *ListView) writeColored(text string, selected bool, chosen bool) {
	if selected {
		_, _ = lv.selectColor.Print(text)
	} else if chosen {
		_, _ = lv.chosenColor.Print(text)
	} else {
		_, _ = lv.normalColor.Print(text)
	}
}

// moveCursor moves cursor to specific position
func (lv *ListView) moveCursor(row, col int) {
	lv.write(fmt.Sprintf("\x1b[%d;%dH", row+1, col+1)) // Convert to 1-based
}

// clearLine clears the current line
func (lv *ListView) clearLine() {
	lv.write("\x1b[2K") // Clear entire line
}

// makeSpace creates space for the popup by moving content down
func (lv *ListView) makeSpace(lines int) {
	lv.moveCursor(lv.startRow, lv.startCol)

	for i := 0; i < lines; i++ {
		lv.write("\n")
	}

	lv.moveCursor(lv.startRow, lv.startCol)
}

// clearPopupArea clears the popup display area
func (lv *ListView) clearPopupArea(totalLines int) {
	for i := 0; i < totalLines; i++ {
		lv.moveCursor(lv.startRow+i, 0)
		lv.clearLine()
	}
}

// updateFilter updates the filtered matches based on current query
func (lv *ListView) updateFilter() {
	lv.filteredMatches = lv.fuzzyMatcher.Match(lv.query, lv.candidates)

	// Reset selection if it's out of bounds
	if lv.selectedIndex >= len(lv.filteredMatches) {
		lv.selectedIndex = 0
	}

	// Constrain scroll offset to keep selection visible
	lv.constrainSelection()
}

// constrainSelection adjusts the scroll offset to ensure the selected item is visible
func (lv *ListView) constrainSelection() {
	count := len(lv.filteredMatches)
	if count == 0 {
		lv.scrollOffset = 0
		return
	}

	// Ensure selected index is within bounds
	if lv.selectedIndex >= count {
		lv.selectedIndex = count - 1
	}
	if lv.selectedIndex < 0 {
		lv.selectedIndex = 0
	}

	// Calculate constraints based on logic
	numItems := min(lv.maxVisibleItems, count)

	// minOffset: when current item is at bottom of visible area
	minOffset := max(lv.selectedIndex-numItems+1, 0)

	// maxOffset: when current item is at top of visible area
	maxOffset := max(min(count-numItems, lv.selectedIndex), 0)

	// Constrain offset within calculated bounds
	if lv.scrollOffset < minOffset {
		lv.scrollOffset = minOffset
	}
	if lv.scrollOffset > maxOffset {
		lv.scrollOffset = maxOffset
	}
}

// moveUp moves selection up
func (lv *ListView) moveUp() {
	if lv.selectedIndex > 0 {
		lv.selectedIndex--
		lv.constrainSelection()
	}
}

// moveDown moves selection down
func (lv *ListView) moveDown() {
	if lv.selectedIndex < len(lv.filteredMatches)-1 {
		lv.selectedIndex++
		lv.constrainSelection()
	}
}

// clearQuery clears the search query
func (lv *ListView) clearQuery() {
	lv.query = ""
	lv.updateFilter()
}

// appendToQuery adds a character to the search query
func (lv *ListView) appendToQuery(ch byte) {
	lv.query += string(ch)
	lv.updateFilter()
}

// backspaceQuery removes the last character from the search query
func (lv *ListView) backspaceQuery() {
	if len(lv.query) > 0 {
		lv.query = lv.query[:len(lv.query)-1]
		lv.updateFilter()
	}
}

// calculateDisplayMetrics calculates the display dimensions
func (lv *ListView) calculateDisplayMetrics() (visibleCount, totalLines int) {
	visibleCount = min(lv.maxVisibleItems, len(lv.filteredMatches))
	totalLines = visibleCount + 1 // +1 for prompt
	return
}

// ensureSpace ensures there's enough space for the popup
func (lv *ListView) ensureSpace(totalLines int) {
	if lv.startRow+totalLines >= lv.height {
		// Not enough space below, move up
		lv.startRow = lv.height - totalLines - 1
		if lv.startRow < 0 {
			lv.startRow = 0
		}
	}
}

// renderPrompt renders the search prompt line
func (lv *ListView) renderPrompt() {
	lv.moveCursor(lv.startRow, 0)

	var counterText string
	if len(lv.filteredMatches) > 0 {
		counterText = fmt.Sprintf("[ %*d/%-*d ]",
			lv.originalTotalWidth, lv.selectedIndex+1,
			lv.originalTotalWidth, len(lv.filteredMatches))
	} else {
		counterText = fmt.Sprintf("[ %*d/%-*d ]",
			lv.originalTotalWidth, 0,
			lv.originalTotalWidth, 0)
	}

	promptText := fmt.Sprintf("%s > %s", counterText, lv.query)
	lv.write(promptText)
}

// createChosenMap creates a map for quick lookup of chosen items
func (lv *ListView) createChosenMap() map[string]bool {
	chosenMap := make(map[string]bool)
	for _, chosen := range lv.chosen {
		chosenMap[chosen.Text] = true
	}
	return chosenMap
}

// renderMatches renders the filtered matches
func (lv *ListView) renderMatches(visibleCount int) {
	chosenMap := lv.createChosenMap()

	for i := 0; i < visibleCount; i++ {
		matchIndex := lv.scrollOffset + i
		if matchIndex >= len(lv.filteredMatches) {
			break
		}

		match := lv.filteredMatches[matchIndex]
		lv.moveCursor(lv.startRow+1+i, 0)

		// Determine item state
		isSelected := matchIndex == lv.selectedIndex
		isChosen := chosenMap[match.Text]

		lv.renderSingleMatch(match, isSelected, isChosen)
	}
}

// renderSingleMatch renders a single match item
func (lv *ListView) renderSingleMatch(match fz.FuzzyMatch, selected, chosen bool) {
	// Render indicator
	var indicator string
	if selected {
		indicator = " > "
	} else {
		indicator = "   "
	}

	// Truncate text if too long
	text := match.Text
	maxTextWidth := lv.width - len(indicator)
	if len(text) > maxTextWidth {
		text = text[:maxTextWidth-3] + "..."
	}

	// Write indicator without color
	lv.write(indicator)

	// Write text with appropriate coloring
	lv.writeColored(text, selected, chosen)
}

// positionCursor positions cursor in the search box
func (lv *ListView) positionCursor() {
	counterLen := len(fmt.Sprintf("[ %*d/%-*d ]",
		lv.originalTotalWidth, 0, lv.originalTotalWidth, 0))
	lv.moveCursor(lv.startRow, counterLen+3+len(lv.query)) // +3 for " > "
}

// render renders the complete popup interface
func (lv *ListView) render() {
	visibleCount, totalLines := lv.calculateDisplayMetrics()
	lv.ensureSpace(totalLines)
	lv.clearPopupArea(lv.height)
	lv.renderPrompt()
	lv.renderMatches(visibleCount)
	lv.positionCursor()
}

// handleArrowKeys handles arrow key sequences
func (lv *ListView) handleArrowKeys(buf []byte) bool {
	if len(buf) >= 3 && buf[0] == esc && buf[1] == 91 { // ESC [
		switch buf[2] {
		case 65: // Up arrow
			lv.moveUp()
		case 66: // Down arrow
			lv.moveDown()
		case 67, 68: // Right/Left arrow (ignore)
			// Do nothing
		default:
			// Unknown escape sequence, treat as ESC
			return true
		}
	}
	return false
}

// handleControlChars handles control character sequences
func (lv *ListView) handleControlChars(ch byte) bool {
	switch ch {
	case ctrlC, esc:
		return true // Exit
	case del, bs:
		lv.backspaceQuery()
	case enter:
		return lv.selectCurrentItem()
	case ctrlU:
		lv.clearQuery()
	case ctrlP, ctrlK:
		lv.moveUp()
	case ctrlN, ctrlJ:
		lv.moveDown()
	case tab:
		if lv.multi {
			lv.selectCurrentItem()
			return false
		}
	default:
		if ch >= 32 && ch < 127 { // Printable ASCII
			lv.appendToQuery(ch)
		}
	}
	return false
}

// handleInput processes keyboard input
func (lv *ListView) handleInput() bool {
	buf := make([]byte, 6) // Large enough for escape sequences
	n, err := lv.ttyin.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	// Handle escape sequences (like arrow keys)
	if n >= 3 {
		if lv.handleArrowKeys(buf) {
			return true
		}
	}

	// Handle single characters
	if n == 1 {
		return lv.handleControlChars(buf[0])
	}

	return false
}

// selectCurrentItem selects the current item
func (lv *ListView) selectCurrentItem() bool {
	if lv.selectedIndex < len(lv.filteredMatches) {
		match := lv.filteredMatches[lv.selectedIndex]
		lv.chosen = append(lv.chosen, ChosenMatch{
			Text:           match.Text,
			Uppercase:      false,
			ShouldOpenFile: false,
		})

		if !lv.multi {
			return true // Exit after single selection
		}
	}
	return false
}

// getDefaultSelection returns the highlighted item if no explicit selection was made
func (lv *ListView) getDefaultSelection() []ChosenMatch {
	if len(lv.chosen) == 0 && len(lv.filteredMatches) > 0 {
		match := lv.filteredMatches[lv.selectedIndex]
		return []ChosenMatch{
			{
				Text:           match.Text,
				Uppercase:      false,
				ShouldOpenFile: false,
			},
		}
	}
	return lv.chosen
}

// Present displays the list interface and returns chosen matches
func (lv *ListView) Present() []ChosenMatch {
	if len(lv.candidates) == 0 {
		return []ChosenMatch{}
	}

	if err := lv.initTerminal(); err != nil {
		return []ChosenMatch{}
	}
	defer lv.cleanup()

	// Initialize with all candidates
	lv.updateFilter()

	if len(lv.filteredMatches) == 0 {
		return []ChosenMatch{}
	}

	// Ensure initial state is properly constrained
	lv.constrainSelection()

	// Make space for our popup
	_, totalLines := lv.calculateDisplayMetrics()
	lv.makeSpace(totalLines)

	// Main event loop
	for {
		lv.render()

		if lv.handleInput() {
			break
		}

		time.Sleep(time.Millisecond * 10)
	}

	// Clear our popup area
	lv.clearPopupArea(totalLines)

	return lv.getDefaultSelection()
}
