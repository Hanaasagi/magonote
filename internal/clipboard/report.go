package clipboard

// Report describes what the clipboard executor actually did.
//
// OSC52Sent means "requested via tmux -w", not "the terminal accepted it":
// OSC 52 is fire-and-forget, and terminal-side settings may still refuse it.
type Report struct {
	BufferSet  bool
	OSC52Sent  bool
	NativeDone bool
	Warnings   []string
}
