// nolint:errcheck
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	clipolicy "github.com/Hanaasagi/magonote/internal/clipboard"
	"github.com/Hanaasagi/magonote/internal/tmux"
	clip "github.com/Hanaasagi/magonote/pkg/clipboard"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose tmux clipboard (OSC 52) integration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(cmd.OutOrStdout())
		},
	}
}

// doctorClient is the tmux surface doctor needs; satisfied by *tmux.Client.
type doctorClient interface {
	Query(ctx context.Context, args ...string) (string, error)
	Version(ctx context.Context) (tmux.Version, error)
}

func runDoctor(out io.Writer) error {
	return diagnose(context.Background(), out, tmux.NewClient())
}

func diagnose(ctx context.Context, out io.Writer, c doctorClient) error {
	fmt.Fprintln(out, "magonote doctor — clipboard integration")
	fmt.Fprintln(out)

	v, verr := c.Version(ctx)
	switch {
	case verr != nil:
		fmt.Fprintf(out, "[FAIL] tmux not reachable: %v\n", verr)
		fmt.Fprintf(out, "       magonote requires tmux >= %d.%d on PATH.\n", tmux.MinMajor, tmux.MinMinor)
	case v.AtLeast(tmux.MinMajor, tmux.MinMinor):
		fmt.Fprintf(out, "[ OK ] tmux version %s (>= %d.%d)\n", v.Raw, tmux.MinMajor, tmux.MinMinor)
	default:
		fmt.Fprintf(out, "[FAIL] tmux version %s is below required %d.%d; please upgrade\n",
			v.Raw, tmux.MinMajor, tmux.MinMinor)
	}

	if os.Getenv("TMUX") == "" {
		fmt.Fprintln(out, "[INFO] not inside tmux ($TMUX unset); tmux clipboard checks skipped")
		reportNative(out, clip.NativeTool())
		return nil
	}

	f := clipolicy.Probe(ctx, c)

	if f.ClientAttached {
		fmt.Fprintf(out, "[ OK ] attached client %q\n", f.ClientName)
	} else {
		fmt.Fprintln(out, "[WARN] no attached client; tmux will skip OSC 52 (buffer still set)")
	}

	if f.SSH {
		fmt.Fprintln(out, "[INFO] SSH session detected; native clipboard tools suppressed (OSC 52 only)")
	}
	reportNative(out, f.NativeTool)

	if f.Nested {
		fmt.Fprintln(out, "[WARN] nested tmux detected; system clipboard sync is best-effort.")
		fmt.Fprintln(out, "       For reliable sync, set 'set-clipboard on' in the OUTER tmux.")
	}

	if sc, err := c.Query(ctx, "show", "-g", "set-clipboard"); err == nil {
		fmt.Fprintf(out, "[INFO] set-clipboard = %s (tmux -w ignores this; it only gates app OSC 52)\n",
			strings.TrimSpace(sc))
	}

	term, _ := c.Query(ctx, "display-message", "-p", "#{client_termname}")
	feats, _ := c.Query(ctx, "display-message", "-p", "#{client_termfeatures}")
	fmt.Fprintf(out, "[INFO] client TERM = %s\n", strings.TrimSpace(term))
	if strings.Contains(feats, "clipboard") {
		fmt.Fprintln(out, "[ OK ] terminal advertises clipboard feature (OSC 52 likely supported)")
	} else {
		fmt.Fprintln(out, "[WARN] terminal clipboard feature not detected; if OSC 52 copy fails, try:")
		fmt.Fprintln(out, "       tmux set -as terminal-features ',xterm*:clipboard'")
		fmt.Fprintln(out, "       (Terminal.app / GNOME VTE lack OSC 52; the native tool covers them locally)")
	}

	plan := clipolicy.Decide(f, clipolicy.Prefs{OSC52: true}, "sample")
	fmt.Fprintln(out, "[INFO] with --osc52, a selection would:")
	for _, s := range plan.Steps {
		switch s.Kind {
		case clipolicy.StepLoadBuffer:
			if s.OSC52 {
				fmt.Fprintln(out, "       - tmux load-buffer -w (buffer + OSC 52)")
			} else {
				fmt.Fprintln(out, "       - tmux load-buffer (buffer only)")
			}
		case clipolicy.StepNative:
			fmt.Fprintln(out, "       - native clipboard tool")
		}
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(out, "       warning: %s\n", w)
	}
	return nil
}

func reportNative(out io.Writer, tool string) {
	if tool != "" {
		fmt.Fprintf(out, "[ OK ] native clipboard tool: %s\n", tool)
	} else {
		fmt.Fprintln(out, "[WARN] no native clipboard tool found (pbcopy/wl-copy/xclip/xsel)")
	}
}
