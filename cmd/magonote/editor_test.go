package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

func TestOpenFileWithEditorUsesStructuredLocation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	argumentsPath := filepath.Join(dir, "arguments.txt")
	editor := filepath.Join(dir, "editor.sh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argumentsPath + "\"\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)

	if err := openFileWithEditor(entity.FileLocation{Path: path, Line: 12, Column: 4}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argumentsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"+12", path}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("editor arguments = %#v, want %#v", got, want)
	}
}

func TestOpenFileWithEditorExpandsHomePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	argumentsPath := filepath.Join(home, "arguments.txt")
	editor := filepath.Join(home, "editor.sh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argumentsPath + "\"\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)

	for _, input := range []string{"~/main.go", "$HOME/main.go"} {
		if err := openFileWithEditor(entity.FileLocation{Path: input}); err != nil {
			t.Errorf("open %q: %v", input, err)
		}
		data, err := os.ReadFile(argumentsPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(data)); got != path {
			t.Errorf("open %q editor path = %q, want %q", input, got, path)
		}
	}
}

func TestOpenFileWithEditorChecksRealPath(t *testing.T) {
	location := entity.FileLocation{Path: filepath.Join(t.TempDir(), "missing.go"), Line: 12}
	if err := openFileWithEditor(location); err == nil || !strings.Contains(err.Error(), "file does not exist") {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestExpandEditorPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("TESTVAR", "/opt/test")

	tests := []struct {
		name string
		path string
		want string
	}{
		{"plain path", "/usr/bin/go", "/usr/bin/go"},
		{"relative path", "main.go", "main.go"},
		{"home tilde only", "~", home},
		{"home prefix", "~/src/main.go", filepath.Join(home, "src/main.go")},
		{"env $VAR", "$TESTVAR/bin", filepath.Join("/opt/test", "bin")},
		{"env ${VAR}", "${TESTVAR}/bin", filepath.Join("/opt/test", "bin")},
		{"env $VAR alone", "$TESTVAR", "/opt/test"},
		{"env ${VAR} alone", "${TESTVAR}", "/opt/test"},
		{"unset var $NOVAR", "$NOVAR/file", "$NOVAR/file"},
		{"unset var ${NOVAR}", "${NOVAR}/file", "${NOVAR}/file"},
		{"empty name $", "$/file", "$/file"},
		{"env ${VAR} trailing slash", "${TESTVAR}/", "/opt/test"},
		{"unclosed brace", "${UNCLOSED/file", "${UNCLOSED/file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandEditorPath(tt.path); got != tt.want {
				t.Errorf("expandEditorPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
