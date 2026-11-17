package action

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// ExpandPath expands a leading ~ or $VAR/${VAR} prefix in a detected path using
// the home directory and environment. It performs controlled prefix expansion
// only (no arbitrary shell evaluation), so the result can be safely shell-quoted
// before being sent to a pane. Unsupported or unset prefixes are left intact.
func ExpandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
		return path
	}
	if !strings.HasPrefix(path, "$") {
		return path
	}
	rest := path[1:]
	var name string
	if strings.HasPrefix(rest, "{") {
		close := strings.Index(rest, "}")
		if close < 0 {
			return path
		}
		name, rest = rest[1:close], rest[close+1:]
	} else {
		nameEnd := strings.Index(rest, "/")
		if nameEnd < 0 {
			name, rest = rest, ""
		} else {
			name, rest = rest[:nameEnd], rest[nameEnd:]
		}
	}
	if name == "" {
		return path
	}
	value := os.Getenv(name)
	if value == "" {
		return path
	}
	if rest == "" || rest == "/" {
		return value
	}
	return filepath.Join(value, strings.TrimPrefix(rest, "/"))
}

// validateGroups rejects ambiguous bindings: a nonempty key may belong to only
// one group, and at most one group may be the default (empty key).
func validateGroups(groups []*Group) error {
	keys := make(map[string]string)
	defaults := 0
	for _, g := range groups {
		if g.Key == "" {
			defaults++
			continue
		}
		if utf8.RuneCountInString(g.Key) != 1 {
			return fmt.Errorf("action group %q key %q must be a single character", g.Name, g.Key)
		}
		if r, _ := utf8.DecodeRuneInString(g.Key); !unicode.IsPrint(r) {
			return fmt.Errorf("action group %q key %q must be a printable character", g.Name, g.Key)
		}
		if other, ok := keys[g.Key]; ok {
			return fmt.Errorf("action groups %q and %q share key %q", other, g.Name, g.Key)
		}
		keys[g.Key] = g.Name
	}
	if defaults > 1 {
		return fmt.Errorf("multiple default action groups (empty key); exactly one is allowed")
	}
	return nil
}

// DefaultConfigPath is the shared config.toml location used by both the core and
// the tmux adapter, so action groups are defined once and honored by both.
func DefaultConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "magonote", "config.toml")
}

func defaultCopyGroup() *Group {
	return &Group{
		Name:    "copy",
		Key:     "",
		Default: Command{Type: Run, Template: `tmux set-buffer -- "{{.Text}}"`},
	}
}

// openCommand picks the platform's URL/file opener for the given argument.
func openCommand(arg string) string {
	if runtime.GOOS == "darwin" {
		return `open ` + arg
	}
	return `xdg-open ` + arg
}

// DefaultGroups returns the shipped groups: copy (bare hint), open (o), insert
// (i). They are ordinary groups; users may override or add their own.
func DefaultGroups() []*Group {
	return []*Group{
		defaultCopyGroup(),
		{
			Name: "open",
			Key:  "o",
			Commands: map[string]Command{
				"url":      {Type: Run, Template: openCommand(`"{{.Text}}"`)},
				"email":    {Type: Run, Template: openCommand(`"mailto:{{.Text}}"`)},
				"path":     {Type: Keys, Template: `${EDITOR:-vi} {{.LineArg}}{{.PathSh}}<CR>`},
				"filename": {Type: Keys, Template: `${EDITOR:-vi} {{.LineArg}}{{.PathSh}}<CR>`},
				"sha":      {Type: Keys, Template: `git show {{.TextSh}}<CR>`},
				"digest":   {Type: Keys, Template: `git show {{.TextSh}}<CR>`},
				"branch":   {Type: Keys, Template: `git checkout {{.TextSh}}<CR>`},
			},
			Default: Command{Type: Run, Template: `tmux set-buffer -- "{{.Text}}"`},
		},
		{
			Name:    "insert",
			Key:     "i",
			Default: Command{Type: Keys, Template: `{{.Text}}`},
		},
	}
}

// UnmarshalTOML accepts either a plain string (a Run command) or a table with a
// "keys" or "run" field, so config can write `url = "xdg-open {{.Text}}"` or
// `sha = { keys = "git show {{.Text}}<CR>" }`.
func (c *Command) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		c.Type, c.Template = Run, v
	case map[string]interface{}:
		if keys, ok := v["keys"].(string); ok {
			c.Type, c.Template = Keys, keys
		} else if run, ok := v["run"].(string); ok {
			c.Type, c.Template = Run, run
		} else {
			return fmt.Errorf("command table must have a 'keys' or 'run' string field")
		}
	default:
		return fmt.Errorf("command must be a string or a {keys/run} table, got %T", data)
	}
	return nil
}

// groupConfig is the TOML shape of a group: a "key", a "default" command, and
// any number of kind -> command entries.
type groupConfig struct {
	Key      string
	Default  Command
	Commands map[string]Command
}

func (g *groupConfig) UnmarshalTOML(data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("group must be a table, got %T", data)
	}
	g.Commands = make(map[string]Command)
	for k, v := range m {
		switch k {
		case "key":
			key, ok := v.(string)
			if !ok {
				return fmt.Errorf("group key must be a string")
			}
			g.Key = key
		case "default":
			if err := g.Default.UnmarshalTOML(v); err != nil {
				return err
			}
		default:
			var c Command
			if err := c.UnmarshalTOML(v); err != nil {
				return fmt.Errorf("kind %q: %w", k, err)
			}
			g.Commands[k] = c
		}
	}
	return nil
}

func (g *groupConfig) toGroup(name string) *Group {
	grp := &Group{Name: name, Key: g.Key, Commands: g.Commands, Default: g.Default}
	if grp.Default.Template == "" {
		grp.Default = defaultCopyGroup().Default
	}
	if grp.Commands == nil {
		grp.Commands = make(map[string]Command)
	}
	return grp
}

type groupsFile struct {
	Groups map[string]*groupConfig `toml:"groups"`
}

// LoadRegistry builds the action registry from the shipped defaults overlaid
// with any user-defined [groups] in the config file. A missing file yields the
// defaults; a malformed file is an error.
func LoadRegistry(path string) (*Registry, error) {
	byName := make(map[string]*Group)
	for _, g := range DefaultGroups() {
		byName[g.Name] = g
	}

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			var f groupsFile
			if err := toml.Unmarshal(data, &f); err != nil {
				return nil, fmt.Errorf("parse action groups: %w", err)
			}
			for name, gc := range f.Groups {
				byName[name] = gc.toGroup(name)
			}
		case os.IsNotExist(err):
			// defaults only
		default:
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
	}

	groups := make([]*Group, 0, len(byName))
	for _, g := range byName {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	if err := validateGroups(groups); err != nil {
		return nil, err
	}
	return NewRegistry(groups...), nil
}
