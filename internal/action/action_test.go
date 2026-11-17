package action

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryDefaultAndKeys(t *testing.T) {
	r := NewRegistry(DefaultGroups()...)
	if r.Default() == nil || r.Default().Name != "copy" {
		t.Fatalf("default = %#v, want copy", r.Default())
	}
	if g, ok := r.ByKey("o"); !ok || g.Name != "open" {
		t.Fatalf("ByKey(o) = %#v %v", g, ok)
	}
	if g, ok := r.ByKey("i"); !ok || g.Name != "insert" {
		t.Fatalf("ByKey(i) = %#v %v", g, ok)
	}
	keys := r.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys() = %v, want 2 reserved keys", keys)
	}
}

func TestRegistrySynthesizesDefault(t *testing.T) {
	r := NewRegistry(&Group{Name: "open", Key: "o"})
	if r.Default() == nil || r.Default().Key != "" {
		t.Fatalf("expected synthesized default group, got %#v", r.Default())
	}
}

func TestCommandForFallsBackToDefault(t *testing.T) {
	g := &Group{
		Commands: map[string]Command{"url": {Type: Run, Template: "open {{.Text}}"}},
		Default:  Command{Type: Run, Template: "copy {{.Text}}"},
	}
	if c := g.CommandFor("url"); c.Template != "open {{.Text}}" {
		t.Fatalf("url command = %#v", c)
	}
	if c := g.CommandFor("unknown"); c.Template != "copy {{.Text}}" {
		t.Fatalf("fallback command = %#v", c)
	}
}

func TestRenderVariableReferences(t *testing.T) {
	got, err := Render(`tmux send-keys -t {{.Pane}} -l {{.Text}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `tmux send-keys -t ${MAGONOTE_PANE} -l ${MAGONOTE_TEXT}`
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

func TestRenderLegacyAlias(t *testing.T) {
	got, err := Render(`tmux set-buffer -- "{}"`)
	if err != nil {
		t.Fatal(err)
	}
	want := `tmux set-buffer -- "${MAGONOTE_TEXT}"`
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

func TestKeysCommandEnterIsSeparate(t *testing.T) {
	got := KeysCommand(`git show "${MAGONOTE_TEXT}"<CR>`)
	want := `tmux send-keys -t "${MAGONOTE_PANE}" -l "git show \"${MAGONOTE_TEXT}\"" ; tmux send-keys -t "${MAGONOTE_PANE}" Enter`
	if got != want {
		t.Fatalf("KeysCommand =\n%q\nwant\n%q", got, want)
	}
}

func TestKeysCommandInsertNoEnter(t *testing.T) {
	got := KeysCommand(`${MAGONOTE_TEXT}`)
	want := `tmux send-keys -t "${MAGONOTE_PANE}" -l "${MAGONOTE_TEXT}"`
	if got != want {
		t.Fatalf("KeysCommand = %q, want %q", got, want)
	}
}

func TestKeysCommandControlChord(t *testing.T) {
	got := KeysCommand(`<C-c>`)
	want := `tmux send-keys -t "${MAGONOTE_PANE}" C-c`
	if got != want {
		t.Fatalf("KeysCommand = %q, want %q", got, want)
	}
}

func TestLoadRegistryDefaults(t *testing.T) {
	r, err := LoadRegistry(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.ByKey("o"); !ok {
		t.Fatal("default open group missing")
	}
}

func TestLoadRegistryUserOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := `
[groups.open]
key = "o"
url = "firefox {{.Text}}"
sha = { keys = "git log {{.Text}}<CR>" }

[groups.logs]
key = "l"
default = { keys = "kubectl logs {{.Text}}<CR>" }
`
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	open, _ := r.ByKey("o")
	if c := open.CommandFor("url"); c.Type != Run || c.Template != "firefox {{.Text}}" {
		t.Fatalf("url command = %#v", c)
	}
	if c := open.CommandFor("sha"); c.Type != Keys || c.Template != "git log {{.Text}}<CR>" {
		t.Fatalf("sha command = %#v", c)
	}
	logs, ok := r.ByKey("l")
	if !ok || logs.Name != "logs" {
		t.Fatalf("user group logs missing: %#v %v", logs, ok)
	}
	if c := logs.CommandFor("anything"); c.Type != Keys {
		t.Fatalf("logs default = %#v", c)
	}
}

func TestRenderShellSafeVars(t *testing.T) {
	got, err := Render(`${EDITOR:-vi} {{.LineArg}}{{.PathSh}}<CR>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `${EDITOR:-vi} ${MAGONOTE_LINE_ARG}${MAGONOTE_PATH_SH}<CR>`
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

// TestBindScriptShellEscapesText verifies the printf %q escaping round-trips:
// evaluating the escaped form in a shell yields the original literal text and
// never executes embedded substitutions (the pane-shell injection vector).
func TestBindScriptShellEscapesText(t *testing.T) {
	malicious := `$(echo HACKED) "x"`
	cmd := exec.Command("bash", "-c", BindScript, "--",
		malicious, "word", "%1", "0", "0", malicious, `printf '%s' "$MAGONOTE_TEXT_SH"`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	escaped := string(out)
	if escaped == malicious {
		t.Fatalf("text was not shell-escaped: %q", escaped)
	}
	// Re-evaluating the escaped form must produce the literal, not run it.
	cmd2 := exec.Command("bash", "-c", `printf '%s' `+escaped)
	out2, err := cmd2.Output()
	if err != nil {
		t.Fatalf("escaped form failed to evaluate: %v", err)
	}
	if string(out2) != malicious {
		t.Fatalf("round-trip = %q, want literal %q", out2, malicious)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	r, err := LoadRegistry("../../config.example.toml")
	if err != nil {
		t.Fatalf("example config invalid: %v", err)
	}
	if _, ok := r.ByKey("o"); !ok {
		t.Fatal("open group missing from example")
	}
	if _, ok := r.ByKey("i"); !ok {
		t.Fatal("insert group missing from example")
	}
	if r.Default() == nil || r.Default().Key != "" {
		t.Fatal("default copy group missing from example")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("MG_TEST_VAR", "/opt/test")
	cases := []struct{ in, want string }{
		{"~", home},
		{"~/src/main.go", filepath.Join(home, "src/main.go")},
		{"$MG_TEST_VAR/bin", "/opt/test/bin"},
		{"${MG_TEST_VAR}/bin", "/opt/test/bin"},
		{"$MG_TEST_VAR", "/opt/test"},
		{"/usr/bin/go", "/usr/bin/go"},
		{"main.go", "main.go"},
		{"$UNSET_VAR/file", "$UNSET_VAR/file"},
		{"$(echo hacked)/file", "$(echo hacked)/file"}, // never evaluated
	}
	for _, c := range cases {
		if got := ExpandPath(c.in); got != c.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateGroups(t *testing.T) {
	if err := validateGroups([]*Group{{Name: "open", Key: "o"}, {Name: "logs", Key: "o"}}); err == nil {
		t.Fatal("expected duplicate-key error")
	}
	if err := validateGroups([]*Group{{Name: "a", Key: ""}, {Name: "b", Key: ""}}); err == nil {
		t.Fatal("expected multiple-default error")
	}
	if err := validateGroups([]*Group{{Name: "copy", Key: ""}, {Name: "open", Key: "o"}}); err != nil {
		t.Fatalf("valid groups rejected: %v", err)
	}
}

func TestLoadRegistryRejectsKeyConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := "[groups.open]\nkey = \"o\"\n[groups.logs]\nkey = \"o\"\n"
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("expected key-conflict error from LoadRegistry")
	}
}

func TestDefaultEmailOpenUsesMailto(t *testing.T) {
	r := NewRegistry(DefaultGroups()...)
	open, _ := r.ByKey("o")
	cmd := open.CommandFor("email")
	if !strings.Contains(cmd.Template, "mailto:{{.Text}}") {
		t.Fatalf("email open template = %q, want mailto: prefix", cmd.Template)
	}
	url := open.CommandFor("url")
	if strings.Contains(url.Template, "mailto:") {
		t.Fatalf("url open template should not use mailto: %q", url.Template)
	}
}

func TestValidateGroupsRejectsUnreachableKeys(t *testing.T) {
	if err := validateGroups([]*Group{{Name: "logs", Key: "gg"}}); err == nil {
		t.Fatal("expected multi-rune key rejection")
	}
	if err := validateGroups([]*Group{{Name: "logs", Key: "\n"}}); err == nil {
		t.Fatal("expected non-printable key rejection")
	}
	if err := validateGroups([]*Group{{Name: "copy", Key: ""}, {Name: "logs", Key: "l"}}); err != nil {
		t.Fatalf("single printable key should be valid: %v", err)
	}
}
