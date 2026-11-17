package internal

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Hanaasagi/magonote/internal/action"
	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

// View is the interaction layer: it owns the input state machine and
// orchestrates the event loop. Presentation is delegated to the renderer.
type View struct {
	state        *State
	skip         int
	multi        bool
	matches      []Match
	chosen       []ChosenMatch
	screen       tcell.Screen
	renderer     *renderer
	registry     *action.Registry
	pendingGroup *action.Group
}

// ChosenMatch is a selected entity tagged with the action group chosen by the
// user. The core only selects; the tmux adapter resolves Group+Kind to a command
// and executes it (the Act stage).
type ChosenMatch struct {
	Text     string
	Kind     string
	Group    string
	Location *entity.FileLocation
}

// CaptureEvent represents the result of the user interaction.
type CaptureEvent int

const (
	ExitEvent CaptureEvent = iota
	HintEvent
)

// ViewOptions groups the view configuration, replacing a long positional
// parameter list.
type ViewOptions struct {
	Multi       bool
	Reverse     bool
	UniqueLevel int
	Contrast    bool
	Position    string
	Colors      ViewColors
	Actions     *action.Registry
}

// NewViewColors builds a ViewColors from individual color values, keeping the
// struct's fields encapsulated.
func NewViewColors(selectFg, selectBg, multiFg, multiBg, fg, bg, hintFg, hintBg Color) ViewColors {
	return ViewColors{
		selectForeground: selectFg,
		selectBackground: selectBg,
		multiForeground:  multiFg,
		multiBackground:  multiBg,
		foreground:       fg,
		background:       bg,
		hintForeground:   hintFg,
		hintBackground:   hintBg,
	}
}

// NewView creates a new View instance.
func NewView(state *State, opts ViewOptions) *View {
	matches := state.Matches(opts.Reverse, opts.UniqueLevel)
	skip := 0
	if opts.Reverse {
		skip = len(matches) - 1
	}

	registry := opts.Actions
	if registry == nil {
		registry = action.NewRegistry(action.DefaultGroups()...)
	}

	return &View{
		state:        state,
		skip:         skip,
		multi:        opts.Multi,
		matches:      matches,
		chosen:       make([]ChosenMatch, 0),
		renderer:     newRenderer(state, matches, opts.Colors, opts.Contrast, opts.Position),
		registry:     registry,
		pendingGroup: registry.Default(),
	}
}

// isActive reports whether a match remains selectable under the typed hint
// prefix. An empty prefix keeps every match active. It is the single predicate
// shared by rendering and interaction so the two never disagree.
func isActive(mat *Match, typedHint string) bool {
	return typedHint == "" || (mat.Hint != nil && strings.HasPrefix(*mat.Hint, typedHint))
}

// clampSkipToActive keeps the selection cursor on a visible match once a prefix
// filters the current one out, so Enter and arrows never act on a hidden match.
func (v *View) clampSkipToActive(typedHint string) {
	if typedHint == "" {
		return
	}
	if v.skip < len(v.matches) && isActive(&v.matches[v.skip], typedHint) {
		return
	}
	for i := range v.matches {
		if isActive(&v.matches[i], typedHint) {
			v.skip = i
			return
		}
	}
}

// Prev moves the selection to the previous active match.
func (v *View) Prev(typedHint string) {
	for i := v.skip - 1; i >= 0; i-- {
		if isActive(&v.matches[i], typedHint) {
			v.skip = i
			return
		}
	}
}

// Next moves the selection to the next active match.
func (v *View) Next(typedHint string) {
	for i := v.skip + 1; i < len(v.matches); i++ {
		if isActive(&v.matches[i], typedHint) {
			v.skip = i
			return
		}
	}
}

// statusLabel is shown only when a non-default group is pending, so the user
// knows what a completed hint will do.
func (v *View) statusLabel() string {
	if v.pendingGroup == v.registry.Default() {
		return ""
	}
	return v.pendingGroup.Name
}

// render delegates one frame to the renderer using the current visual state.
func (v *View) render(typedHint string) {
	var selected *Match
	if v.skip < len(v.matches) {
		selected = &v.matches[v.skip]
	}
	v.renderer.render(v.screen, selected, v.chosen, typedHint, v.statusLabel())
}

// visualState captures the fields that affect the rendered frame. Comparing
// snapshots lets the event loop redraw only when something actually changed.
type visualState struct {
	typedHint   string
	group       string
	skip        int
	chosenCount int
	multi       bool
}

func (v *View) visualState(typedHint string) visualState {
	return visualState{
		typedHint:   typedHint,
		group:       v.pendingGroup.Name,
		skip:        v.skip,
		chosenCount: len(v.chosen),
		multi:       v.multi,
	}
}

// listen runs the event loop. PollEvent blocks, so no sleep is needed; frames
// are composed only on visual state changes or resize.
func (v *View) listen() CaptureEvent {
	if len(v.matches) == 0 {
		return ExitEvent
	}

	typedHint := ""
	longestHint := v.findLongestHint()

	renderStart := time.Now()
	v.render(typedHint)
	slog.Info("first render completed", "duration_ms", time.Since(renderStart).Milliseconds())

	for {
		ev := v.screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			before := v.visualState(typedHint)
			action := v.handleKeyEvent(ev, &typedHint, longestHint)
			if action != nil {
				return *action
			}
			if v.visualState(typedHint) != before {
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
func (v *View) handleKeyEvent(ev *tcell.EventKey, typedHint *string, longestHint string) *CaptureEvent {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return v.handleEscapeKey(typedHint)
	case tcell.KeyUp, tcell.KeyLeft:
		v.Prev(*typedHint)
	case tcell.KeyDown, tcell.KeyRight:
		v.Next(*typedHint)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return v.handleBackspace(typedHint)
	case tcell.KeyEnter:
		return v.handleEnter(*typedHint)
	case tcell.KeyRune:
		return v.handleRuneKey(ev, typedHint, longestHint)
	}
	return nil
}

// handleEscapeKey clears pending input/group in multi mode, otherwise exits.
func (v *View) handleEscapeKey(typedHint *string) *CaptureEvent {
	if v.multi && (*typedHint != "" || v.pendingGroup != v.registry.Default()) {
		*typedHint = ""
		v.pendingGroup = v.registry.Default()
		return nil
	}
	action := ExitEvent
	return &action
}

// handleBackspace removes a hint character, or resets the pending group when no
// hint characters remain.
func (v *View) handleBackspace(typedHint *string) *CaptureEvent {
	if len(*typedHint) > 0 {
		*typedHint = (*typedHint)[:len(*typedHint)-1]
		v.clampSkipToActive(*typedHint)
	} else if v.pendingGroup != v.registry.Default() {
		v.pendingGroup = v.registry.Default()
	}
	return nil
}

// handleEnter selects the active match under the pending group.
func (v *View) handleEnter(typedHint string) *CaptureEvent {
	v.clampSkipToActive(typedHint)
	if v.skip < len(v.matches) && isActive(&v.matches[v.skip], typedHint) {
		v.choose(&v.matches[v.skip])
		if !v.multi {
			action := HintEvent
			return &action
		}
	}
	return nil
}

// choose records a selection tagged with the pending group and entity kind.
func (v *View) choose(mat *Match) {
	v.chosen = append(v.chosen, ChosenMatch{
		Text:     mat.Text,
		Kind:     mat.Pattern,
		Group:    v.pendingGroup.Name,
		Location: mat.Location,
	})
}

// handleRuneKey handles group prefix keys and hint characters.
func (v *View) handleRuneKey(ev *tcell.EventKey, typedHint *string, longestHint string) *CaptureEvent {
	ch := string(ev.Rune())

	if ch == " " {
		return v.handleSpaceKey()
	}

	// A reserved group key switches the pending action and resets hint input.
	if g, ok := v.registry.ByKey(ch); ok {
		v.pendingGroup = g
		*typedHint = ""
		return nil
	}

	lowerCh := strings.ToLower(ch)
	if ch != lowerCh {
		// Uppercase is a backward-compatible alias for the open group.
		if g, ok := v.registry.ByName("open"); ok {
			v.pendingGroup = g
		}
	}
	*typedHint += lowerCh

	for i := range v.matches {
		mat := &v.matches[i]
		if mat.Hint != nil && *mat.Hint == *typedHint {
			v.choose(mat)
			if v.multi {
				*typedHint = ""
				v.pendingGroup = v.registry.Default()
			} else {
				action := HintEvent
				return &action
			}
			break
		}
	}

	if !v.multi && len(*typedHint) >= len(longestHint) {
		action := ExitEvent
		return &action
	}

	v.clampSkipToActive(*typedHint)
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
