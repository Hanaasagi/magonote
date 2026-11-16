package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/internal/tmux"
)

type fakeDoctorClient struct {
	version   tmux.Version
	verErr    error
	responses map[string]string
}

func (f fakeDoctorClient) Query(_ context.Context, args ...string) (string, error) {
	if v, ok := f.responses[strings.Join(args, " ")]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown")
}

func (f fakeDoctorClient) Version(_ context.Context) (tmux.Version, error) {
	return f.version, f.verErr
}

func TestDiagnoseVersionFloor(t *testing.T) {
	t.Setenv("TMUX", "")
	var buf bytes.Buffer
	c := fakeDoctorClient{version: tmux.Version{Major: 3, Minor: 1, Raw: "tmux 3.1"}}
	if err := diagnose(context.Background(), &buf, c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "below required 3.2") {
		t.Errorf("expected version floor failure:\n%s", buf.String())
	}
}

func TestDiagnoseUnreachable(t *testing.T) {
	t.Setenv("TMUX", "")
	var buf bytes.Buffer
	c := fakeDoctorClient{verErr: fmt.Errorf("no tmux")}
	_ = diagnose(context.Background(), &buf, c)
	if !strings.Contains(buf.String(), "tmux not reachable") {
		t.Errorf("expected unreachable message:\n%s", buf.String())
	}
}

func TestDiagnoseNestedWarning(t *testing.T) {
	t.Setenv("TMUX", "/tmp/x,1,0")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	var buf bytes.Buffer
	c := fakeDoctorClient{
		version: tmux.Version{Major: 3, Minor: 8, Raw: "tmux 3.8"},
		responses: map[string]string{
			"display-message -p #{client_name}":         "c1",
			"show-environment -g TMUX":                  "TMUX=/tmp/outer,1,0",
			"display-message -p #{client_termname}":     "tmux-256color",
			"display-message -p #{client_termfeatures}": "clipboard,RGB",
			"show -g set-clipboard":                     "external",
		},
	}
	if err := diagnose(context.Background(), &buf, c); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"[ OK ] tmux version", `attached client "c1"`, "nested tmux detected",
		"set-clipboard = external", "clipboard feature",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
