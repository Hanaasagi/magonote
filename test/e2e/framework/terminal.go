package framework

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// tmuxTestConf forces the Ms (OSC 52) capability regardless of the host
// terminfo database (macOS ships an old ncurses whose xterm entries lack Ms),
// and reduces redraw noise. This makes clipboard tests deterministic across
// Linux and macOS.
const tmuxTestConf = `set -as terminal-overrides ',*:Ms=\E]52;%p1%s;%p2%s\007'
set -g set-clipboard on
set -g status off
set -g default-terminal "screen-256color"
`

// TmuxServer is an isolated tmux server (own socket, no user config) for tests.
type TmuxServer struct {
	Socket string
}

// NewTmuxServer starts a detached test session and registers cleanup.
func NewTmuxServer(t *testing.T) *TmuxServer {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
	dir := t.TempDir()
	socket := filepath.Join(dir, "s.sock")
	conf := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(conf, []byte(tmuxTestConf), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("tmux", "-S", socket, "-f", conf,
		"new-session", "-d", "-x", "80", "-y", "24", "-s", "test").CombinedOutput()
	if err != nil {
		t.Fatalf("start tmux server: %v: %s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
	})
	return &TmuxServer{Socket: socket}
}

// Run executes a tmux command against this server.
func (s *TmuxServer) Run(args ...string) (string, error) {
	out, err := exec.Command("tmux", append([]string{"-S", s.Socket}, args...)...).CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// RunStdin executes a tmux command feeding stdin (e.g. load-buffer -).
func (s *TmuxServer) RunStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("tmux", append([]string{"-S", s.Socket}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// SocketRunner adapts a TmuxServer to clipboard.TmuxRunner so the real
// mechanism can be exercised against an isolated server.
type SocketRunner struct{ Socket string }

func (r SocketRunner) Run(ctx context.Context, args []string, stdin io.Reader) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", append([]string{"-S", r.Socket}, args...)...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// FakeTerminal is a pty-attached tmux client that scans the output byte stream
// for OSC 52 sequences. It replaces a real terminal emulator's parsing role; it
// does not set any OS clipboard. The scanner is intentionally independent of
// magonote's own screen parser.
type FakeTerminal struct {
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
	buf  []byte
}

// AttachFakeTerminal attaches a pty client to the server's "test" session.
func AttachFakeTerminal(t *testing.T, s *TmuxServer) *FakeTerminal {
	t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("tmux", "-S", s.Socket, "attach", "-t", "test")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = tty.Close()
	ft := &FakeTerminal{ptmx: ptmx, cmd: cmd}
	go ft.readLoop()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
	})
	time.Sleep(400 * time.Millisecond) // let attach + initial redraw settle
	ft.Drain()
	return ft
}

func (ft *FakeTerminal) readLoop() {
	buf := make([]byte, 65536)
	for {
		n, err := ft.ptmx.Read(buf)
		if n > 0 {
			ft.mu.Lock()
			ft.buf = append(ft.buf, buf[:n]...)
			ft.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// Drain clears accumulated output.
func (ft *FakeTerminal) Drain() {
	ft.mu.Lock()
	ft.buf = nil
	ft.mu.Unlock()
}

// osc52Re matches a raw OSC 52 sequence (BEL or ST terminated). The base64
// alphabet excludes ESC, so this cannot false-match inside other sequences.
var osc52Re = regexp.MustCompile("\x1b\\]52;[cp]?;([A-Za-z0-9+/=]*)(?:\x07|\x1b\\\\)")

// Payloads returns decoded clipboard payloads seen since the last Drain.
func (ft *FakeTerminal) Payloads() []string {
	ft.mu.Lock()
	data := append([]byte(nil), ft.buf...)
	ft.mu.Unlock()
	var out []string
	for _, m := range osc52Re.FindAllSubmatch(data, -1) {
		if dec, err := base64.StdEncoding.DecodeString(string(m[1])); err == nil {
			out = append(out, string(dec))
		}
	}
	return out
}

// WaitForPayload polls until want appears or timeout elapses.
func (ft *FakeTerminal) WaitForPayload(want string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, p := range ft.Payloads() {
			if p == want {
				return true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
