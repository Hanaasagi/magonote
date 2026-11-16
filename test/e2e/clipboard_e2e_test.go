package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

// buildBoth builds magonote and magonote-tmux into the project build dir,
// reusing the framework's project-root discovery and magonote build.
func buildBoth(t *testing.T) string {
	t.Helper()
	f := framework.NewFramework()
	if err := f.BuildBinary(); err != nil {
		t.Fatalf("build magonote: %v", err)
	}
	bindir := filepath.Join(f.ProjectRoot, "build")
	cmd := exec.Command("go", "build", "-o", filepath.Join(bindir, "magonote-tmux"), "./cmd/magonote-tmux")
	cmd.Dir = f.ProjectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build magonote-tmux: %v: %s", err, out)
	}
	return bindir
}

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// waitMagonotePane returns the magonote TUI pane. swap-pane moves panes between
// windows, so neither the [magonote] window name nor pane_current_command (the
// "zsh -c" wrapper) identifies it reliably. Pane IDs are stable: the magonote
// pane is the one that is not the original fixture pane.
// stubNativeTools places no-op clipboard helpers first on PATH so the real
// magonote-tmux child cannot overwrite the host's system clipboard. The OSC 52
// tests run locally (non-SSH) with a native tool present, so Decide schedules
// StepNative; without stubbing, every run would clobber the developer's
// clipboard with the fixture URL. tmux/sh/cat/sleep still resolve from the
// original PATH appended after the stub dir.
func stubNativeTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range []string{"pbcopy", "wl-copy", "xclip", "xsel", "clip"} {
		if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func waitMagonotePane(t *testing.T, s *framework.TmuxServer, originalPane string) string {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := s.Run("list-panes", "-a", "-F", "#{pane_id}")
		for _, line := range strings.Split(out, "\n") {
			id := strings.TrimSpace(line)
			if id != "" && id != originalPane {
				return id
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("magonote pane never appeared")
	return ""
}

func waitPaneReady(t *testing.T, s *framework.TmuxServer, pane, marker string) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := s.Run("capture-pane", "-t", pane, "-p")
		if strings.Contains(out, marker) {
			time.Sleep(150 * time.Millisecond) // let tcell finish initializing
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("pane %s never showed %q", pane, marker)
}

// runPick drives the real magonote-tmux binary against the isolated server:
// fixture pane -> hint selection -> clipboard. TMUX points the binary's tmux
// commands at the test socket; XDG dirs isolate state/config.
func runPick(t *testing.T, s *framework.TmuxServer, ft *framework.FakeTerminal, bindir, fixture, keys string, osc52 bool) {
	t.Helper()
	if _, err := s.Run("respawn-pane", "-t", "test", "-k", "cat "+fixture+"; sleep 300"); err != nil {
		t.Fatalf("respawn pane: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	ft.Drain()

	originalPane, _ := s.Run("display-message", "-p", "#{pane_id}")
	pid, _ := s.Run("display-message", "-p", "#{pid}")
	env := append(os.Environ(),
		"TMUX="+s.Socket+","+pid+",0",
		"XDG_STATE_HOME="+t.TempDir(),
		"XDG_CONFIG_HOME="+t.TempDir(),
		"TERM=xterm-256color",
		"PATH="+stubNativeTools(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	args := []string{"--dir", bindir}
	if osc52 {
		args = append(args, "--osc52")
	}
	cmd := exec.Command(filepath.Join(bindir, "magonote-tmux"), args...)
	cmd.Env = env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	pane := waitMagonotePane(t, s, originalPane)
	waitPaneReady(t, s, pane, "example.com")
	if _, err := s.Run("send-keys", "-t", pane, keys); err != nil {
		t.Fatalf("send keys: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Logf("magonote-tmux exited: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("magonote-tmux timed out")
	}
}

const testURL = "https://example.com/test"

// L4: full binary flow delivers OSC 52 to the client terminal with --osc52.
func TestMagonoteTmuxOSC52EndToEnd(t *testing.T) {
	bindir := buildBoth(t)
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	fixture := writeFixture(t, "visit "+testURL+"\n")

	runPick(t, s, ft, bindir, fixture, "a", true)

	if !ft.WaitForPayload(testURL, 3*time.Second) {
		t.Fatalf("OSC 52 not delivered end-to-end; payloads=%v", ft.Payloads())
	}
}

// L4: without --osc52 the default flow sets the tmux buffer (via the user
// command) but emits no OSC 52 — the default path is unchanged.
func TestMagonoteTmuxDefaultNoOSC52(t *testing.T) {
	bindir := buildBoth(t)
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	fixture := writeFixture(t, "visit "+testURL+"\n")

	runPick(t, s, ft, bindir, fixture, "a", false)

	buf, _ := s.Run("show-buffer")
	if strings.TrimSpace(buf) != testURL {
		t.Errorf("buffer = %q, want %q", buf, testURL)
	}
	time.Sleep(300 * time.Millisecond)
	for _, p := range ft.Payloads() {
		if p == testURL {
			t.Errorf("default flow must not emit OSC 52; payloads=%v", ft.Payloads())
		}
	}
}

// L4 race regression: repeated rapid selections must each deliver OSC 52.
// This guards the removal of the old 100ms "wait for redraw" sleep.
func TestMagonoteTmuxRepeatedSelections(t *testing.T) {
	bindir := buildBoth(t)
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	fixture := writeFixture(t, "visit "+testURL+"\n")

	for i := 0; i < 3; i++ {
		ft.Drain()
		runPick(t, s, ft, bindir, fixture, "a", true)
		if !ft.WaitForPayload(testURL, 3*time.Second) {
			t.Fatalf("iteration %d: OSC 52 lost; payloads=%v", i, ft.Payloads())
		}
	}
}
