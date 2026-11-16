package clipboard

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// TmuxRunner executes tmux commands. It is the seam between this library and
// the host's tmux client (e.g. magonote's internal/tmux.Client). The interface
// is defined here, by the consumer, so the library depends on no tmux
// implementation and remains reusable by other projects.
type TmuxRunner interface {
	Run(ctx context.Context, args []string, stdin io.Reader) (string, error)
}

// LoadBuffer sets the tmux paste buffer from text via stdin.
//
// When osc52 is true, -w makes tmux also forward OSC 52 to the target client's
// terminal as tmux's own output. This requires tmux >= 3.2, an attached client,
// and the terminal's Ms capability; tmux silently skips OSC 52 otherwise but
// still sets the buffer. -w is NOT gated by tmux's set-clipboard option.
//
// A non-empty client targets a specific client (-t). Empty text is a no-op.
func LoadBuffer(ctx context.Context, r TmuxRunner, text string, osc52 bool, client string) error {
	if text == "" {
		return nil
	}
	args := make([]string, 0, 5)
	args = append(args, "load-buffer")
	if osc52 {
		args = append(args, "-w")
	}
	if client != "" {
		args = append(args, "-t", client)
	}
	args = append(args, "-")
	_, err := r.Run(ctx, args, strings.NewReader(text))
	return err
}

// execRunner is the library's default TmuxRunner backed by os/exec. It lets the
// convenience writers work standalone without caller wiring.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, args []string, stdin io.Reader) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}
