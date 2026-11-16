// Package tmux provides a small client for executing tmux commands.
//
// Client is the application's tmux command channel. It structurally satisfies
// the clipboard.TmuxRunner interface (defined in pkg/clipboard) without
// importing it, keeping the dependency direction cmd -> internal -> pkg.
package tmux

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Client executes tmux commands against a server.
type Client struct {
	bin    string
	socket string
}

// NewClient returns a Client using the tmux binary on PATH and the ambient
// server (the one identified by $TMUX).
func NewClient() *Client {
	return &Client{bin: "tmux"}
}

// NewClientWithSocket returns a Client targeting a specific server socket via
// -S. Useful for isolated test servers.
func NewClientWithSocket(socket string) *Client {
	return &Client{bin: "tmux", socket: socket}
}

// Run executes a tmux command, feeding stdin when non-nil, and returns stdout
// with trailing newlines trimmed. It matches the historical tmuxCommand
// behavior used throughout magonote-tmux.
func (c *Client) Run(ctx context.Context, args []string, stdin io.Reader) (string, error) {
	full := make([]string, 0, len(args)+2)
	if c.socket != "" {
		full = append(full, "-S", c.socket)
	}
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, c.bin, full...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// Query runs a tmux command that produces output and takes no stdin.
func (c *Client) Query(ctx context.Context, args ...string) (string, error) {
	return c.Run(ctx, args, nil)
}
