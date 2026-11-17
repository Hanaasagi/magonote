// Package action implements magonote's user-definable action groups.
//
// An action group binds a prefix key to a set of kind-specific commands. The
// default group (Key == "") is triggered by a bare hint and copies the
// selection; other groups (open, insert, or user-defined) run a different
// command per entity kind. The engine is generic: "open" and "insert" are just
// shipped default groups, not special-cased behavior.
//
// The package is split into a pure domain (Group/Command/Registry), template
// rendering, and configuration loading, so the policy is testable without tmux.
package action

import "sort"

// CommandType distinguishes how a command is delivered to the system.
type CommandType int

const (
	// Run executes the rendered template as a background shell command.
	Run CommandType = iota
	// Keys sends the rendered template as keystrokes to the active pane.
	Keys
)

// Command is a templated action. Template uses {{.Text}}/{{.Kind}}/{{.Pane}}/
// {{.Line}}/{{.Column}} placeholders; for Keys it may also use key notation such
// as <CR> (Enter), <Esc>, <Tab>, <C-c>.
type Command struct {
	Type     CommandType
	Template string
}

// Group is a named set of kind-specific commands bound to a prefix key.
type Group struct {
	Name     string
	Key      string // "" marks the default (bare-hint) group
	Commands map[string]Command
	Default  Command // fallback for kinds without an explicit command
}

// CommandFor returns the command for a kind, falling back to the group default.
func (g *Group) CommandFor(kind string) Command {
	if c, ok := g.Commands[kind]; ok {
		return c
	}
	return g.Default
}

// Registry indexes groups by prefix key and by name.
type Registry struct {
	byKey  map[string]*Group
	byName map[string]*Group
	def    *Group
	keys   []string
}

// NewRegistry builds a registry from groups. Exactly one group should have an
// empty Key (the default); if none is given, a copy default is synthesized so
// the registry always has a usable default and bare hints always work.
func NewRegistry(groups ...*Group) *Registry {
	r := &Registry{
		byKey:  make(map[string]*Group),
		byName: make(map[string]*Group),
	}
	// Sort by name so key/default resolution is deterministic regardless of
	// caller ordering (config maps are unordered).
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	for _, g := range groups {
		r.byName[g.Name] = g
		if g.Key == "" {
			r.def = g
		} else {
			r.byKey[g.Key] = g
			r.keys = append(r.keys, g.Key)
		}
	}
	if r.def == nil {
		r.def = defaultCopyGroup()
		r.byName[r.def.Name] = r.def
	}
	return r
}

// Default returns the bare-hint group.
func (r *Registry) Default() *Group { return r.def }

// ByKey resolves a prefix key to its group.
func (r *Registry) ByKey(key string) (*Group, bool) {
	g, ok := r.byKey[key]
	return g, ok
}

// ByName resolves a group by name (used by the selection protocol).
func (r *Registry) ByName(name string) (*Group, bool) {
	g, ok := r.byName[name]
	return g, ok
}

// Keys returns the non-empty prefix keys, which must be excluded from the hint
// alphabet so a group key can never be mistaken for a hint character.
func (r *Registry) Keys() []string { return r.keys }
