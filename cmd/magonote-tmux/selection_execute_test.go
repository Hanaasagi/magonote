package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hanaasagi/magonote/internal/action"
)

// runSelection executes a selection whose copy command writes the text to a
// file, so the test can assert what the command actually received.
func runSelection(t *testing.T, text string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.txt")
	reg := action.NewRegistry(&action.Group{
		Name: "copy", Key: "",
		Default: action.Command{Type: action.Run, Template: `printf '%s' "{{.Text}}" > ` + out},
	})
	m := &Magonote{registry: reg, activePaneInfo: &PaneInfo{ID: "%1"}}
	if err := m.executeSelection(selection{Group: "copy", Kind: "word", Text: text}); err != nil {
		t.Fatalf("executeSelection: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestExecuteSelectionPreservesSpaces(t *testing.T) {
	if got := runSelection(t, "hello world"); got != "hello world" {
		t.Fatalf("text = %q, want %q", got, "hello world")
	}
}

func TestExecuteSelectionInjectionSafe(t *testing.T) {
	// Shell metacharacters in selected text must be treated as literal data,
	// never executed: the value travels as a bash variable, not spliced code.
	malicious := `$(echo hacked) " ; rm -rf / #`
	if got := runSelection(t, malicious); got != malicious {
		t.Fatalf("text = %q, want literal %q", got, malicious)
	}
}
