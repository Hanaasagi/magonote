package internal

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// View represents the terminal UI for displaying and interacting with matches
type View struct {
	state      *State
	skip       int
	multi      bool
	contrast   bool
	position   string
	matches    []Match
	colors     ViewColors
	chosen     []ChosenMatch
	screen     tcell.Screen
	textBuffer *TextBuffer // Buffer for handling text wrapping
}

// ViewColors groups all color-related fields
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

// ChosenMatch represents a match that has been selected by the user
type ChosenMatch struct {
	Text           string
	Uppercase      bool
	ShouldOpenFile bool
}

// CaptureEvent represents the result of the user interaction
type CaptureEvent int

const (
	ExitEvent CaptureEvent = iota
	HintEvent
)

// NewView creates a new View instance
func NewView(
	state *State,
	multi bool,
	reverse bool,
	uniqueLevel int,
	contrast bool,
	position string,
	selectForegroundColor Color,
	selectBackgroundColor Color,
	multiForegroundColor Color,
	multiBackgroundColor Color,
	foregroundColor Color,
	backgroundColor Color,
	hintForegroundColor Color,
	hintBackgroundColor Color,
) *View {
	matches := state.Matches(reverse, uniqueLevel)
	skip := 0
	if reverse {
		skip = len(matches) - 1
	}

	return &View{
		state:      state,
		skip:       skip,
		multi:      multi,
		contrast:   contrast,
		position:   position,
		matches:    matches,
		textBuffer: nil, // Will be initialized when screen is available
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
		chosen: make([]ChosenMatch, 0),
	}
}

// Navigation methods
func (v *View) Prev() {
	if v.skip > 0 {
		v.skip--
	}
}

func (v *View) Next() {
	if v.skip < len(v.matches)-1 {
		v.skip++
	}
}

// makeHintText formats the hint text based on contrast setting
func (v *View) makeHintText(hint string) string {
	if v.contrast {
		return "[" + hint + "]"
	}
	return hint
}

// render displays the UI with all matches and highlights
func (v *View) render(typedHint string) {
	v.screen.Clear()

	// Initialize text buffer if not already done
	if v.textBuffer == nil {
		width, height := v.screen.Size()
		v.textBuffer = NewTextBuffer(v.state.Lines, width, height)
	} else {
		// Update buffer size if screen size changed
		width, height := v.screen.Size()
		if v.textBuffer.width != width || v.textBuffer.height != height {
			v.textBuffer = NewTextBuffer(v.state.Lines, width, height)
		} else {
			v.textBuffer.Clear()
		}
	}

	// Display the lines of text
	v.renderTextLines()

	// Get the selected match
	var selected *Match
	if v.skip < len(v.matches) {
		selected = &v.matches[v.skip]
	}

	// Display all matches with appropriate highlighting
	v.renderMatches(selected, typedHint)

	// Write buffer content to screen
	v.textBuffer.WriteToScreen(v.screen)

	v.screen.Show()
}

// renderTextLines renders the original text lines
func (v *View) renderTextLines() {
	for y, line := range v.state.Lines {
		cleanLine := strings.TrimRight(line, " \t\n\r")
		if cleanLine == "" {
			continue
		}

		// Use the text buffer to handle wrapping
		v.textBuffer.SetString(0, y, cleanLine, tcell.StyleDefault)
	}
}

// renderMatches renders all matches with highlighting
func (v *View) renderMatches(selected *Match, typedHint string) {
	chosenMap := make(map[string]bool, len(v.chosen))
	for _, chosen := range v.chosen {
		chosenMap[chosen.Text] = true
	}

	for _, mat := range v.matches {
		style := v.getMatchStyle(&mat, selected, chosenMap)
		v.renderSingleMatch(&mat, style, typedHint)
	}
}

// getMatchStyle determines the appropriate style for a match
func (v *View) getMatchStyle(mat *Match, selected *Match, chosenMap map[string]bool) tcell.Style {
	if chosenMap[mat.Text] {
		return tcell.StyleDefault.
			Foreground(colorToTcell(v.colors.multiForeground)).
			Background(colorToTcell(v.colors.multiBackground))
	}

	if selected != nil && mat.Equals(*selected) {
		return tcell.StyleDefault.
			Foreground(colorToTcell(v.colors.selectForeground)).
			Background(colorToTcell(v.colors.selectBackground))
	}

	return tcell.StyleDefault.
		Foreground(colorToTcell(v.colors.foreground)).
		Background(colorToTcell(v.colors.background))
}

// renderSingleMatch renders a single match with its hint
func (v *View) renderSingleMatch(mat *Match, style tcell.Style, typedHint string) {
	// Calculate display position accounting for wide characters
	line := v.state.Lines[mat.Y]
	prefix := line[:mat.X]

	// Calculate the actual display position by summing up character widths
	offset := 0
	for _, r := range prefix {
		width := runewidth.RuneWidth(r)
		if width <= 0 {
			width = 1
		}
		offset += width
	}

	// Display the match text
	text := v.makeHintText(mat.Text)
	currentX := offset
	for _, r := range text {
		v.textBuffer.SetCell(currentX, mat.Y, r, style)
		width := runewidth.RuneWidth(r)
		if width <= 0 {
			width = 1
		}
		currentX += width
	}

	// Display the hint if available
	if mat.Hint != nil {
		v.renderHint(mat, offset, text, typedHint)
	}
}

// renderHint renders the hint for a match
func (v *View) renderHint(mat *Match, offset int, text string, typedHint string) {
	hint := *mat.Hint

	// Calculate hint position
	extraPosition := v.calculateHintPosition(text, hint)
	finalPosition := max(0, offset+extraPosition)

	// Display the hint
	hintText := v.makeHintText(hint)
	currentX := finalPosition
	hintRunes := []rune(hintText)
	for i, r := range hintRunes {
		hintStyle := v.getHintStyle(hint, typedHint, i)
		v.textBuffer.SetCell(currentX, mat.Y, r, hintStyle)
		width := runewidth.RuneWidth(r)
		if width <= 0 {
			width = 1
		}
		currentX += width
	}
}

// calculateHintPosition calculates where to position the hint
func (v *View) calculateHintPosition(text, hint string) int {
	switch v.position {
	case "right":
		// Calculate actual display width of the text
		textWidth := 0
		for _, r := range text {
			width := runewidth.RuneWidth(r)
			if width <= 0 {
				width = 1
			}
			textWidth += width
		}
		return textWidth - len([]rune(hint))
	case "off_left":
		offset := -len([]rune(hint))
		if v.contrast {
			offset -= 2
		}
		return offset
	case "off_right":
		// Calculate actual display width of the text
		textWidth := 0
		for _, r := range text {
			width := runewidth.RuneWidth(r)
			if width <= 0 {
				width = 1
			}
			textWidth += width
		}
		return textWidth
	default: // "left"
		return 0
	}
}

// getHintStyle determines the style for hint characters
func (v *View) getHintStyle(hint, typedHint string, charIndex int) tcell.Style {
	baseStyle := tcell.StyleDefault.
		Foreground(colorToTcell(v.colors.hintForeground)).
		Background(colorToTcell(v.colors.hintBackground))

	// Highlight matching portion of the hint
	if strings.HasPrefix(hint, typedHint) && charIndex < len([]rune(typedHint)) {
		return tcell.StyleDefault.
			Foreground(colorToTcell(v.colors.multiForeground)).
			Background(colorToTcell(v.colors.multiBackground))
	}

	return baseStyle
}

// listen handles user input and interaction
func (v *View) listen() CaptureEvent {
	if len(v.matches) == 0 {
		return ExitEvent
	}

	typedHint := ""
	hasUppercase := false
	longestHint := v.findLongestHint()

	renderStart := time.Now()
	v.render(typedHint)
	firstRenderDuration := time.Since(renderStart)
	slog.Info("first render completed", "duration_ms", firstRenderDuration.Milliseconds())

	for {
		ev := v.screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			action := v.handleKeyEvent(ev, &typedHint, &hasUppercase, longestHint)
			if action != nil {
				return *action
			}
		case *tcell.EventResize:
			v.screen.Sync()
		case *tcell.EventError:
			return ExitEvent
		}

		v.render(typedHint)
		time.Sleep(time.Millisecond * 10)
	}
}

// findLongestHint finds the longest hint for reference
func (v *View) findLongestHint() string {
	longest := ""
	for _, match := range v.matches {
		if match.Hint != nil && len(*match.Hint) > len(longest) {
			longest = *match.Hint
		}
	}
	return longest
}

// handleKeyEvent processes a key event and returns an action if needed
func (v *View) handleKeyEvent(ev *tcell.EventKey, typedHint *string, hasUppercase *bool, longestHint string) *CaptureEvent {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return v.handleEscapeKey(typedHint, hasUppercase)
	case tcell.KeyUp, tcell.KeyLeft:
		v.Prev()
	case tcell.KeyDown, tcell.KeyRight:
		v.Next()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return v.handleBackspace(typedHint, hasUppercase)
	case tcell.KeyEnter:
		return v.handleEnter()
	case tcell.KeyRune:
		return v.handleRuneKey(ev, typedHint, hasUppercase, longestHint)
	}
	return nil
}

// handleEscapeKey handles escape key press
func (v *View) handleEscapeKey(typedHint *string, hasUppercase *bool) *CaptureEvent {
	if v.multi && *typedHint != "" {
		*typedHint = ""
		*hasUppercase = false
		return nil
	}
	action := ExitEvent
	return &action
}

// handleBackspace handles backspace key press
func (v *View) handleBackspace(typedHint *string, hasUppercase *bool) *CaptureEvent {
	if len(*typedHint) > 0 {
		*typedHint = (*typedHint)[:len(*typedHint)-1]
		*hasUppercase = false
	}
	return nil
}

// handleEnter handles enter key press
func (v *View) handleEnter() *CaptureEvent {
	if v.skip < len(v.matches) {
		v.chosen = append(v.chosen, ChosenMatch{
			Text:           v.matches[v.skip].Text,
			Uppercase:      false,
			ShouldOpenFile: false,
		})

		if !v.multi {
			action := HintEvent
			return &action
		}
	}
	return nil
}

// handleRuneKey handles character input
func (v *View) handleRuneKey(ev *tcell.EventKey, typedHint *string, hasUppercase *bool, longestHint string) *CaptureEvent {
	ch := string(ev.Rune())

	if ch == " " {
		return v.handleSpaceKey()
	}

	lowerCh := strings.ToLower(ch)
	if ch != lowerCh {
		*hasUppercase = true
	}
	*typedHint += lowerCh

	// Check for hint match
	for _, mat := range v.matches {
		if mat.Hint != nil && *mat.Hint == *typedHint {
			v.chosen = append(v.chosen, ChosenMatch{
				Text:      mat.Text,
				Uppercase: *hasUppercase,
				// ShouldOpenFile: *hasUppercase && isLikelyFilePath(mat.Text),
				ShouldOpenFile: *hasUppercase,
			})

			if v.multi {
				*typedHint = ""
				*hasUppercase = false
			} else {
				action := HintEvent
				return &action
			}
			break
		}
	}

	// Exit if typed too much in single mode
	if !v.multi && len(*typedHint) >= len(longestHint) {
		action := ExitEvent
		return &action
	}

	return nil
}

// handleSpaceKey handles space key press
func (v *View) handleSpaceKey() *CaptureEvent {
	if v.multi {
		action := HintEvent
		return &action
	}
	v.multi = true
	return nil
}

// Present displays the UI and returns the chosen matches
func (v *View) Present() []ChosenMatch {
	// fast path
	if len(v.matches) == 0 {
		return []ChosenMatch{}
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		slog.Error("Failed to create tcell screen", "error", err)
		return []ChosenMatch{}
	}

	if err := screen.Init(); err != nil {
		slog.Error("Failed to initialize tcell screen", "error", err)
		return []ChosenMatch{}
	}

	v.screen = screen
	defer screen.Fini()

	screen.SetStyle(tcell.StyleDefault)
	screen.EnableMouse()
	screen.Clear()

	event := v.listen()
	if event == ExitEvent {
		return []ChosenMatch{}
	}

	return v.chosen
}

// Pre-compiled pattern for RGB color matching
var rgbColorPattern = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)

// colorToTcell converts a Color to tcell.Color
func colorToTcell(c Color) tcell.Color {
	if cw, ok := c.(ColorWrapper); ok {
		if cw.isRGB {
			return tcell.NewRGBColor(int32(cw.r), int32(cw.g), int32(cw.b))
		}

		// Map color attributes to tcell colors
		colorMap := map[color.Attribute]tcell.Color{
			color.FgBlack:   tcell.ColorBlack,
			color.FgRed:     tcell.ColorRed,
			color.FgGreen:   tcell.ColorGreen,
			color.FgYellow:  tcell.ColorYellow,
			color.FgBlue:    tcell.ColorBlue,
			color.FgMagenta: tcell.ColorFuchsia,
			color.FgCyan:    tcell.ColorAqua,
			color.FgWhite:   tcell.ColorWhite,
			color.Reset:     tcell.ColorDefault,
		}

		if tcellColor, exists := colorMap[cw.GetFgColor()]; exists {
			return tcellColor
		}
	}

	// Fallback: parse RGB from colored text
	coloredText := c.FgString("test")
	if matches := rgbColorPattern.FindStringSubmatch(coloredText); len(matches) == 4 {
		r, _ := strconv.Atoi(matches[1])
		g, _ := strconv.Atoi(matches[2])
		b, _ := strconv.Atoi(matches[3])
		return tcell.NewRGBColor(int32(r), int32(g), int32(b))
	}

	return tcell.ColorDefault
}

// isLikelyFilePath checks if the given text looks like a file path
func isLikelyFilePath(text string) bool { // nolint: unused
	if len(text) < 2 {
		return false
	}

	// Skip URLs and other protocols
	protocols := []string{"http://", "https://", "ftp://", "ssh://"}
	for _, protocol := range protocols {
		if strings.HasPrefix(text, protocol) {
			return false
		}
	}

	// Check for path separators
	if strings.ContainsAny(text, "/\\") {
		return true
	}

	// Check for file extension
	if strings.Contains(text, ".") {
		parts := strings.Split(text, ".")
		if len(parts) > 1 && len(parts[len(parts)-1]) <= 10 {
			return true
		}
	}

	return false
}
