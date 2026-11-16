// Package clipboard implements magonote's clipboard routing policy.
//
// It is application-private (internal/): the reusable mechanisms live in
// pkg/clipboard, while this package decides which mechanism to use given the
// environment and user preferences. The policy is gated entirely by the
// --osc52 preference; when disabled, the caller's existing command flow runs
// unchanged.
package clipboard

import (
	"context"
	"os"
	"strings"

	"github.com/Hanaasagi/magonote/internal/tmux"
	clip "github.com/Hanaasagi/magonote/pkg/clipboard"
)

// Facts are the environment facts used to route clipboard operations.
type Facts struct {
	Version        tmux.Version
	ClientAttached bool
	ClientName     string
	SSH            bool
	NativeTool     string
	Nested         bool
}

// tmuxQueryer is the subset of *tmux.Client needed for probing. Defined here so
// Probe can be unit-tested with a fake.
type tmuxQueryer interface {
	Query(ctx context.Context, args ...string) (string, error)
	Version(ctx context.Context) (tmux.Version, error)
}

// Probe gathers clipboard-relevant facts from tmux and the environment. It is
// best-effort: individual query failures leave zero values rather than
// aborting, so a partial environment still yields a usable plan.
func Probe(ctx context.Context, c tmuxQueryer) Facts {
	f := Facts{
		SSH:        os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "",
		NativeTool: clip.NativeTool(),
	}
	f.Version, _ = c.Version(ctx)

	// A non-empty current client name implies an attached client.
	if name, err := c.Query(ctx, "display-message", "-p", "#{client_name}"); err == nil {
		f.ClientName = strings.TrimSpace(name)
		f.ClientAttached = f.ClientName != ""
	}

	f.Nested = detectNested(ctx, c)
	return f
}

// detectNested reports whether this tmux is likely nested inside another.
// Same-machine: the server's global environment carries the outer $TMUX.
// Cross-SSH: the client TERM is typically tmux*/screen* (heuristic). Used only
// for diagnostics/warnings; nesting is not actively supported.
func detectNested(ctx context.Context, c tmuxQueryer) bool {
	if env, err := c.Query(ctx, "show-environment", "-g", "TMUX"); err == nil && strings.TrimSpace(env) != "" {
		return true
	}
	if term, err := c.Query(ctx, "display-message", "-p", "#{client_termname}"); err == nil {
		term = strings.TrimSpace(term)
		if strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen") {
			return true
		}
	}
	return false
}
