package clipboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	clip "github.com/Hanaasagi/magonote/pkg/clipboard"
)

// nativeTimeout bounds a native clipboard helper so a hung tool cannot block.
const nativeTimeout = 5 * time.Second

// Execute runs the plan for text against the given tmux runner, returning an
// honest report and any aggregated errors. Facts should be probed once by the
// caller and reused across selections.
func Execute(ctx context.Context, r clip.TmuxRunner, f Facts, p Prefs, text string) (Report, error) {
	plan := Decide(f, p, text)
	rep := Report{Warnings: plan.Warnings}
	if len(plan.Steps) == 0 {
		return rep, nil
	}

	var errs []error
	for _, s := range plan.Steps {
		switch s.Kind {
		case StepLoadBuffer:
			if err := clip.LoadBuffer(ctx, r, text, s.OSC52, f.ClientName); err != nil {
				errs = append(errs, fmt.Errorf("tmux buffer: %w", err))
				continue
			}
			rep.BufferSet = true
			rep.OSC52Sent = s.OSC52
		case StepNative:
			nctx, cancel := context.WithTimeout(ctx, nativeTimeout)
			err := clip.CopyNative(nctx, text)
			cancel()
			if err != nil {
				errs = append(errs, fmt.Errorf("native clipboard: %w", err))
				continue
			}
			rep.NativeDone = true
		}
	}
	return rep, errors.Join(errs...)
}
