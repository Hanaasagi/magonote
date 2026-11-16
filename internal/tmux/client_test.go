package tmux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeTmux creates an executable script that echoes its arguments and
// stdin so Client.Run mechanics can be tested without a real tmux server.
func writeFakeTmux(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "faketmux")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunTrimsAndCapturesStdout(t *testing.T) {
	bin := writeFakeTmux(t, "printf 'hello\\n\\n'\n")
	c := &Client{bin: bin}
	out, err := c.Run(context.Background(), []string{"display", "-p"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}

func TestRunPrependsSocket(t *testing.T) {
	bin := writeFakeTmux(t, "echo \"$@\"\n")
	c := &Client{bin: bin, socket: "/tmp/test.sock"}
	out, err := c.Run(context.Background(), []string{"load-buffer", "-w", "-"}, strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	if out != "-S /tmp/test.sock load-buffer -w -" {
		t.Errorf("socket not prepended correctly: %q", out)
	}
}

func TestRunFeedsStdin(t *testing.T) {
	bin := writeFakeTmux(t, "cat\n")
	c := &Client{bin: bin}
	out, err := c.Run(context.Background(), []string{"load-buffer", "-"}, strings.NewReader("payload text"))
	if err != nil {
		t.Fatal(err)
	}
	if out != "payload text" {
		t.Errorf("stdin not fed: %q", out)
	}
}

func TestRunErrorIncludesStderr(t *testing.T) {
	bin := writeFakeTmux(t, "echo 'boom' >&2; exit 1\n")
	c := &Client{bin: bin}
	_, err := c.Run(context.Background(), []string{"bad"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should include stderr: %v", err)
	}
}
