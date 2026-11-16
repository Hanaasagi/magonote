package internal

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

// View is the interaction layer: it owns the input state machine and
// orchestrates the event loop. Presentation is delegated to the renderer.
type View struct {
	state    *State
	skip     int
	multi    bool
	matches  []Match
	chosen   []ChosenMatch
	screen   tcell.Screen
	renderer *renderer
}

// ChosenMatch represents a match that has been selected by the user.
type ChosenMatch struct {
	Text           string
	Uppercase      bool
	ShouldOpenFile bool
	Location       *entity.FileLocation
}

// CaptureEvent represents the result of the user interaction.
type CaptureEvent int

const (
	ExitEvent CaptureEvent = iota
	HintEvent
)

// NewView creates a new View instance.
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

	theme := ViewColors{
		selectForeground: selectForegroundColor,
		selectBackground: selectBackgroundColor,
		multiForeground:  multiForegroundColor,
		multiBackground:  multiBackgroundColor,
		foreground:       foregroundColor,
		background:       backgroundColor,
		hintForeground:   hintForegroundColor,
		hintBackground:   hintBackgroundColor,
	}

	return &View{
		state:    state,
		skip:     skip,
		multi:    multi,
		matches:  matches,
		chosen:   make([]ChosenMatch, 0),
		renderer: newRenderer(state, matches, theme, contrast, position),
	}
}

// Prev moves the selection to the previous match.
func (v *View) Prev() {
	if v.skip > 0 {
		v.skip--
	}
}

// Next moves the selection to the next match.
func (v *View) Next() {
	if v.skip < len(v.matches)-1 {
		v.skip++
	}
}

// render delegates one frame to the renderer using the current visual state.
func (v *View) render(typedHint string) {
	var selected *Match
	if v.skip < len(v.matches) {
		selected = &v.matches[v.skip]
	}
	v.renderer.render(v.screen, selected, v.chosen, typedHint)
}

// visualState captures the fields that affect the rendered frame. Comparing
// snapshots lets the event loop redraw only when something actually changed,
// without threading dirty flags through the input handlers.
type visualState struct {
	typedHint    string
	hasUppercase bool
	skip         int
	chosenCount  int
	multi        bool
}

func (v *View) visualState(typedHint string, hasUppercase bool) visualState {
	return visualState{
		typedHint:    typedHint,
		hasUppercase: hasUppercase,
		skip:         v.skip,
		chosenCount:  len(v.chosen),
		multi:        v.multi,
	}
}

// listen runs the event loop. PollEvent blocks, so no sleep is needed; frames
// are composed only on visual state changes or resize.
func (v *View) listen() CaptureEvent {
	if len(v.matches) == 0 {
		return ExitEvent
	}

	typedHint := ""
	hasUppercase := false
	longestHint := v.findLongestHint()

	renderStart := time.Now()
	v.render(typedHint)
	slog.Info("first render completed", "duration_ms", time.Since(renderStart).Milliseconds())

	for {
		ev := v.screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			before := v.visualState(typedHint, hasUppercase)
			action := v.handleKeyEvent(ev, &typedHint, &hasUppercase, longestHint)
			if action != nil {
				return *action
			}
			if v.visualState(typedHint, hasUppercase) != before {
				v.render(typedHint)
			}
		case *tcell.EventResize:
			v.screen.Sync()
			v.render(typedHint)
		case *tcell.EventError:
			return ExitEvent
		}
	}
}

// findLongestHint finds the longest hint for reference.
func (v *View) findLongestHint() string {
	longest := ""
	for _, match := range v.matches {
		if match.Hint != nil && len(*match.Hint) > len(longest) {
			longest = *match.Hint
		}
	}
	return longest
}

// handleKeyEvent processes a key event and returns an action if needed.
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

// handleEscapeKey handles escape key press.
func (v *View) handleEscapeKey(typedHint *string, hasUppercase *bool) *CaptureEvent {
	if v.multi && *typedHint != "" {
		*typedHint = ""
		*hasUppercase = false
		return nil
	}
	action := ExitEvent
	return &action
}

// handleBackspace handles backspace key press.
func (v *View) handleBackspace(typedHint *string, hasUppercase *bool) *CaptureEvent {
	if len(*typedHint) > 0 {
		*typedHint = (*typedHint)[:len(*typedHint)-1]
		*hasUppercase = false
	}
	return nil
}

// handleEnter handles enter key press.
func (v *View) handleEnter() *CaptureEvent {
	if v.skip < len(v.matches) {
		v.chosen = append(v.chosen, ChosenMatch{
			Text:           v.matches[v.skip].Text,
			Uppercase:      false,
			ShouldOpenFile: false,
			Location:       v.matches[v.skip].Location,
		})

		if !v.multi {
			action := HintEvent
			return &action
		}
	}
	return nil
}

// handleRuneKey handles character input.
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
				Text:           mat.Text,
				Uppercase:      *hasUppercase,
				ShouldOpenFile: *hasUppercase && (mat.Location != nil || isLikelyFilePath(mat.Text)),
				Location:       mat.Location,
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

// handleSpaceKey handles space key press.
func (v *View) handleSpaceKey() *CaptureEvent {
	if v.multi {
		action := HintEvent
		return &action
	}
	v.multi = true
	return nil
}

// Present displays the UI and returns the chosen matches.
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
