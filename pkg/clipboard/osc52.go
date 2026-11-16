package clipboard

import "encoding/base64"

// EncodeOSC52 returns the raw OSC 52 escape sequence that sets the terminal
// clipboard to text. It performs no tmux detection and no DCS wrapping: the
// caller chooses the transport. Inside tmux, prefer LoadBuffer with osc52=true
// so tmux emits the sequence on its own output channel (see tmux.go).
//
// Empty text returns nil: emitting an empty OSC 52 would clear the user's
// clipboard, which is never the intent of a copy operation.
func EncodeOSC52(text string) []byte {
	if text == "" {
		return nil
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	return []byte("\x1b]52;c;" + encoded + "\x07")
}
