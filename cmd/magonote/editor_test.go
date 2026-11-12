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

func TestOpenFileWithEditorChecksRealPath(t *testing.T) {
	location := entity.FileLocation{Path: filepath.Join(t.TempDir(), "missing.go"), Line: 12}
	if err := openFileWithEditor(location); err == nil || !strings.Contains(err.Error(), "file does not exist") {
		t.Fatalf("missing file error = %v", err)
	}
}
