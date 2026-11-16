// Package clipboard copies text to terminal, tmux, and native clipboards.
//
// It is a reusable, stdlib-only library. The low-level mechanisms are:
//   - EncodeOSC52: pure OSC 52 sequence encoding (no tmux knowledge)
//   - LoadBuffer: tmux paste buffer (+ optional OSC 52 via -w) over a TmuxRunner
//   - CopyNative / NativeTool: platform clipboard tools
//
// Application-specific routing policy (which mechanism to use when) lives
// outside this package, in the host application.
package clipboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// Writer represents a clipboard destination.
type Writer interface {
	Write(text string) error
}

// Option configures a Clipboard.
type Option func(*Clipboard)

// Clipboard provides unified access to multiple clipboard targets.
type Clipboard struct {
	tmux   bool
	system bool
	osc52  bool
	output io.Writer
}

// New creates a Clipboard with all targets enabled and OSC 52 output on stderr.
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

// WithTmux enables/disables tmux buffer copying.
func WithTmux(enabled bool) Option {
	return func(c *Clipboard) { c.tmux = enabled }
}

// WithSystem enables/disables system clipboard copying.
func WithSystem(enabled bool) Option {
	return func(c *Clipboard) { c.system = enabled }
}

// WithOSC52 enables/disables OSC 52 terminal copying.
func WithOSC52(enabled bool) Option {
	return func(c *Clipboard) { c.osc52 = enabled }
}

// WithOutput sets the output destination for OSC 52 sequences.
func WithOutput(w io.Writer) Option {
	return func(c *Clipboard) { c.output = w }
}

// Copy writes text to all enabled clipboard targets. Empty text is a no-op.
// Errors from individual targets are aggregated (errors.Join) rather than
// swallowed, so a partial failure still reports what went wrong.
func (c *Clipboard) Copy(text string) error {
	if text == "" {
		return nil
	}
	ctx := context.Background()
	var errs []error

	if c.tmux && isTmuxSession() {
		if err := LoadBuffer(ctx, execRunner{}, text, false, ""); err != nil {
			errs = append(errs, fmt.Errorf("tmux buffer: %w", err))
		}
	}
	if c.system {
		if err := CopyNative(ctx, text); err != nil {
			errs = append(errs, fmt.Errorf("system clipboard: %w", err))
		}
	}
	if c.osc52 {
		if _, err := c.output.Write(EncodeOSC52(text)); err != nil {
			errs = append(errs, fmt.Errorf("osc52: %w", err))
		}
	}
	return errors.Join(errs...)
}

// TmuxWriter provides tmux-only clipboard access.
type TmuxWriter struct{}

// NewTmuxWriter creates a tmux-only clipboard writer.
func NewTmuxWriter() *TmuxWriter { return &TmuxWriter{} }

// Write copies text to the tmux buffer. Empty text is a no-op (it does not
// delete the buffer). Returns an error when not inside tmux.
func (t *TmuxWriter) Write(text string) error {
	if !isTmuxSession() {
		return fmt.Errorf("not in tmux session")
	}
	return LoadBuffer(context.Background(), execRunner{}, text, false, "")
}

// SystemWriter provides system-only clipboard access.
type SystemWriter struct{}

// NewSystemWriter creates a system-only clipboard writer.
func NewSystemWriter() *SystemWriter { return &SystemWriter{} }

// Write copies text to the system clipboard via the platform tool.
func (s *SystemWriter) Write(text string) error {
	return CopyNative(context.Background(), text)
}

// OSC52Writer provides OSC 52-only clipboard access to a caller-supplied writer.
// It emits a raw OSC 52 sequence with no tmux DCS wrapping; the caller is
// responsible for choosing an appropriate transport. Empty text is a no-op.
type OSC52Writer struct {
	output io.Writer
}

// NewOSC52Writer creates an OSC 52-only clipboard writer.
func NewOSC52Writer(output io.Writer) *OSC52Writer {
	return &OSC52Writer{output: output}
}

// Write emits the OSC 52 sequence for text. Empty text writes nothing.
func (o *OSC52Writer) Write(text string) error {
	seq := EncodeOSC52(text)
	if seq == nil {
		return nil
	}
	_, err := o.output.Write(seq)
	return err
}

// Copy is a convenience function for quick clipboard operations.
func Copy(text string) error { return New().Copy(text) }

// CopyToTmux is a convenience function for tmux-only copying.
func CopyToTmux(text string) error { return NewTmuxWriter().Write(text) }

// CopyToSystem is a convenience function for system-only copying.
func CopyToSystem(text string) error { return NewSystemWriter().Write(text) }

// CopyWithOSC52 is a convenience function for OSC 52-only copying to stderr.
func CopyWithOSC52(text string) error { return NewOSC52Writer(os.Stderr).Write(text) }

// IsTmuxSession returns true if running inside tmux.
func IsTmuxSession() bool { return isTmuxSession() }

func isTmuxSession() bool { return os.Getenv("TMUX") != "" }

// HasSystemClipboard returns true if a system clipboard tool is available.
func HasSystemClipboard() bool { return NativeTool() != "" }

// Available reports which clipboard targets are available.
func Available() map[string]bool {
	return map[string]bool{
		"tmux":   isTmuxSession(),
		"system": NativeTool() != "",
		"osc52":  true,
	}
}
