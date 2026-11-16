package clipboard

// MaxOSC52Bytes caps the raw text sent via OSC 52. Terminals impose payload
// limits (commonly ~100KB); beyond this we set the tmux buffer only and warn.
const MaxOSC52Bytes = 64 * 1024

// Prefs are the user's clipboard preferences, mapped from CLI flags.
type Prefs struct {
	// OSC52 syncs the system clipboard in addition to the tmux buffer.
	OSC52 bool
}

// StepKind enumerates clipboard mechanisms.
type StepKind int

const (
	// StepLoadBuffer sets the tmux buffer (and optionally OSC 52 via -w).
	StepLoadBuffer StepKind = iota
	// StepNative copies via the platform clipboard tool.
	StepNative
)

// Step is a single planned clipboard action.
type Step struct {
	Kind  StepKind
	OSC52 bool // StepLoadBuffer: add -w
}

// Plan is the ordered set of actions plus advisory warnings.
type Plan struct {
	Steps    []Step
	Warnings []string
}

// Decide computes the clipboard plan. It is a pure function of facts, prefs,
// and text. The entire mechanism is gated by Prefs.OSC52: when false (or text
// empty), the plan is empty and the caller's existing command flow runs
// unchanged.
func Decide(f Facts, p Prefs, text string) Plan {
	if !p.OSC52 || text == "" {
		return Plan{}
	}

	var warns []string
	osc52 := true
	if len(text) > MaxOSC52Bytes {
		osc52 = false
		warns = append(warns, "selection exceeds OSC 52 size limit; copied to tmux buffer only")
	}
	if !f.ClientAttached {
		warns = append(warns, "no attached tmux client; tmux will skip OSC 52 (buffer still set)")
	}
	if f.Nested {
		warns = append(warns, "nested tmux detected; system clipboard sync needs outer set-clipboard on")
	}

	steps := []Step{{Kind: StepLoadBuffer, OSC52: osc52}}
	// Local native fallback covers terminals without OSC 52 (Terminal.app,
	// VTE). Suppressed over SSH, where native tools write the remote clipboard.
	if !f.SSH && f.NativeTool != "" {
		steps = append(steps, Step{Kind: StepNative})
	}
	return Plan{Steps: steps, Warnings: warns}
}
