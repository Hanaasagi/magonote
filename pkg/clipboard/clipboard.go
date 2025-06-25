package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Writer represents a clipboard destination
type Writer interface {
	Write(text string) error
}

// Option configures a Clipboard
type Option func(*Clipboard)

// Clipboard provides unified access to multiple clipboard targets
type Clipboard struct {
	tmux   bool
	system bool
	osc52  bool
	output io.Writer
}

// New creates a new Clipboard with default settings
func New(opts ...Option) *Clipboard {
	c := &Clipboard{
		tmux:   true,
		system: true,
		osc52:  true,
		output: os.Stderr,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithTmux enables/disables tmux buffer copying
func WithTmux(enabled bool) Option {
	return func(c *Clipboard) {
		c.tmux = enabled
	}
}

// WithSystem enables/disables system clipboard copying
func WithSystem(enabled bool) Option {
	return func(c *Clipboard) {
		c.system = enabled
	}
}

// WithOSC52 enables/disables OSC52 terminal copying
func WithOSC52(enabled bool) Option {
	return func(c *Clipboard) {
		c.osc52 = enabled
	}
}

// WithOutput sets the output destination for OSC52 sequences
func WithOutput(w io.Writer) Option {
	return func(c *Clipboard) {
		c.output = w
	}
}

// Copy writes text to all enabled clipboard targets
func (c *Clipboard) Copy(text string) error {
	var lastErr error

	if c.tmux && isTmuxSession() {
		if err := c.copyToTmux(text); err != nil {
			lastErr = err
		}
	}

	if c.system {
		if err := c.copyToSystem(text); err != nil {
			lastErr = err
		}
	}

	if c.osc52 {
		if err := c.copyWithOSC52(text); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// copyToTmux copies text to tmux buffer
func (c *Clipboard) copyToTmux(text string) error {
	if text == "" {
		return exec.Command("tmux", "delete-buffer").Run()
	}

	cmd := exec.Command("tmux", "load-buffer", "-")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// copyToSystem copies text to system clipboard
func (c *Clipboard) copyToSystem(text string) error {
	tool := findSystemClipboardTool()
	if tool == "" {
		return fmt.Errorf("no system clipboard tool available")
	}

	cmd := exec.Command(tool)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// copyWithOSC52 copies text using OSC52 escape sequence
func (c *Clipboard) copyWithOSC52(text string) error {
	if text == "" {
		_, err := c.output.Write([]byte("\033]52;c;\007"))
		return err
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))

	var sequence string
	if isTmuxSession() {
		// Wrap OSC52 in tmux DCS passthrough
		sequence = fmt.Sprintf("\033Ptmux;\033\033]52;c;%s\007\033\\", encoded)
	} else {
		sequence = fmt.Sprintf("\033]52;c;%s\007", encoded)
	}

	_, err := c.output.Write([]byte(sequence))
	return err
}

// isTmuxSession checks if running inside tmux
func isTmuxSession() bool {
	return os.Getenv("TMUX") != ""
}

// findSystemClipboardTool finds available system clipboard tool
func findSystemClipboardTool() string {
	tools := getClipboardTools()

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}

	return ""
}

// getClipboardTools returns platform-specific clipboard tools
func getClipboardTools() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"pbcopy"}
	case "linux":
		return []string{"wl-copy", "xclip", "xsel"}
	case "windows":
		return []string{"clip"}
	default:
		return []string{}
	}
}

// TmuxWriter provides tmux-only clipboard access
type TmuxWriter struct{}

// NewTmuxWriter creates a tmux-only clipboard writer
func NewTmuxWriter() *TmuxWriter {
	return &TmuxWriter{}
}

// Write copies text to tmux buffer
func (t *TmuxWriter) Write(text string) error {
	if !isTmuxSession() {
		return fmt.Errorf("not in tmux session")
	}

	if text == "" {
		return exec.Command("tmux", "delete-buffer").Run()
	}

	cmd := exec.Command("tmux", "load-buffer", "-")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// SystemWriter provides system-only clipboard access
type SystemWriter struct{}

// NewSystemWriter creates a system-only clipboard writer
func NewSystemWriter() *SystemWriter {
	return &SystemWriter{}
}

// Write copies text to system clipboard
func (s *SystemWriter) Write(text string) error {
	tool := findSystemClipboardTool()
	if tool == "" {
		return fmt.Errorf("no system clipboard tool available")
	}

	cmd := exec.Command(tool)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// OSC52Writer provides OSC52-only clipboard access
type OSC52Writer struct {
	output io.Writer
}

// NewOSC52Writer creates an OSC52-only clipboard writer
func NewOSC52Writer(output io.Writer) *OSC52Writer {
	return &OSC52Writer{output: output}
}

// Write copies text using OSC52 escape sequence
func (o *OSC52Writer) Write(text string) error {
	if text == "" {
		_, err := o.output.Write([]byte("\033]52;c;\007"))
		return err
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))

	var sequence string
	if isTmuxSession() {
		sequence = fmt.Sprintf("\033Ptmux;\033\033]52;c;%s\007\033\\", encoded)
	} else {
		sequence = fmt.Sprintf("\033]52;c;%s\007", encoded)
	}

	_, err := o.output.Write([]byte(sequence))
	return err
}

// Copy is a convenience function for quick clipboard operations
func Copy(text string) error {
	return New().Copy(text)
}

// CopyToTmux is a convenience function for tmux-only copying
func CopyToTmux(text string) error {
	return NewTmuxWriter().Write(text)
}

// CopyToSystem is a convenience function for system-only copying
func CopyToSystem(text string) error {
	return NewSystemWriter().Write(text)
}

// CopyWithOSC52 is a convenience function for OSC52-only copying
func CopyWithOSC52(text string) error {
	return NewOSC52Writer(os.Stderr).Write(text)
}

// IsTmuxSession returns true if running inside tmux
func IsTmuxSession() bool {
	return isTmuxSession()
}

// HasSystemClipboard returns true if system clipboard tools are available
func HasSystemClipboard() bool {
	return findSystemClipboardTool() != ""
}

// Available returns which clipboard targets are available
func Available() map[string]bool {
	return map[string]bool{
		"tmux":   isTmuxSession(),
		"system": findSystemClipboardTool() != "",
		"osc52":  true, // OSC52 is always available
	}
}
