package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	clipolicy "github.com/Hanaasagi/magonote/internal/clipboard"
	clip "github.com/Hanaasagi/magonote/pkg/clipboard"
	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

const osc52Timeout = 3 * time.Second

func clientName(t *testing.T, s *framework.TmuxServer) string {
	t.Helper()
	out, err := s.Run("list-clients", "-F", "#{client_name}")
	if err != nil {
		t.Fatalf("list-clients: %v", err)
	}
	return strings.TrimSpace(strings.Split(out, "\n")[0])
}

// L3: the core mechanism delivers OSC 52 to the client terminal and sets the
// tmux buffer, against a real isolated tmux server.
func TestLoadBufferWDeliversOSC52(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	r := framework.SocketRunner{Socket: s.Socket}

	if err := clip.LoadBuffer(context.Background(), r, "hello-osc52", true, ""); err != nil {
		t.Fatalf("LoadBuffer: %v", err)
	}
	if !ft.WaitForPayload("hello-osc52", osc52Timeout) {
		t.Fatalf("OSC 52 not delivered; payloads=%v", ft.Payloads())
	}
	buf, err := s.Run("show-buffer")
	if err != nil || buf != "hello-osc52" {
		t.Errorf("buffer = %q err=%v, want hello-osc52", buf, err)
	}
}

// L3: without -w the buffer is set but no OSC 52 is emitted.
func TestLoadBufferPlainNoOSC52(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	r := framework.SocketRunner{Socket: s.Socket}

	if err := clip.LoadBuffer(context.Background(), r, "plain-text", false, ""); err != nil {
		t.Fatalf("LoadBuffer: %v", err)
	}
	buf, _ := s.Run("show-buffer")
	if buf != "plain-text" {
		t.Errorf("buffer = %q, want plain-text", buf)
	}
	time.Sleep(300 * time.Millisecond)
	for _, p := range ft.Payloads() {
		if p == "plain-text" {
			t.Errorf("plain load-buffer must not emit OSC 52, got %v", ft.Payloads())
		}
	}
}

// L3: -w is NOT gated by set-clipboard (verified from tmux source). Even with
// set-clipboard off, OSC 52 reaches the terminal.
func TestOSC52IgnoresSetClipboard(t *testing.T) {
	for _, mode := range []string{"off", "external", "on"} {
		t.Run(mode, func(t *testing.T) {
			s := framework.NewTmuxServer(t)
			ft := framework.AttachFakeTerminal(t, s)
			if _, err := s.Run("set", "-g", "set-clipboard", mode); err != nil {
				t.Fatal(err)
			}
			ft.Drain()
			r := framework.SocketRunner{Socket: s.Socket}
			if err := clip.LoadBuffer(context.Background(), r, "mode-"+mode, true, ""); err != nil {
				t.Fatal(err)
			}
			if !ft.WaitForPayload("mode-"+mode, osc52Timeout) {
				t.Errorf("set-clipboard=%s: OSC 52 not delivered; payloads=%v", mode, ft.Payloads())
			}
		})
	}
}

// L3: explicit -t client targeting works.
func TestLoadBufferTargetsClient(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	name := clientName(t, s)
	r := framework.SocketRunner{Socket: s.Socket}

	if err := clip.LoadBuffer(context.Background(), r, "targeted", true, name); err != nil {
		t.Fatal(err)
	}
	if !ft.WaitForPayload("targeted", osc52Timeout) {
		t.Errorf("OSC 52 not delivered to client %q; payloads=%v", name, ft.Payloads())
	}
}

// L3: the policy executor (internal/clipboard) drives the mechanism correctly.
func TestExecutePolicyDeliversOSC52(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	r := framework.SocketRunner{Socket: s.Socket}
	facts := clipolicy.Facts{ClientAttached: true, ClientName: clientName(t, s)}

	rep, err := clipolicy.Execute(context.Background(), r, facts, clipolicy.Prefs{OSC52: true}, "policy-text")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.BufferSet || !rep.OSC52Sent {
		t.Errorf("report = %+v", rep)
	}
	if !ft.WaitForPayload("policy-text", osc52Timeout) {
		t.Errorf("OSC 52 not delivered; payloads=%v", ft.Payloads())
	}
}

// L3: empty text is a no-op (no buffer, no OSC 52, no clipboard clear).
func TestExecuteEmptyNoop(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	r := framework.SocketRunner{Socket: s.Socket}
	facts := clipolicy.Facts{ClientAttached: true}

	if _, err := clipolicy.Execute(context.Background(), r, facts, clipolicy.Prefs{OSC52: true}, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if len(ft.Payloads()) != 0 {
		t.Errorf("empty selection emitted OSC 52: %v", ft.Payloads())
	}
}

// L3: oversized text sets the buffer but skips OSC 52.
func TestExecuteOversizedSkipsOSC52(t *testing.T) {
	s := framework.NewTmuxServer(t)
	ft := framework.AttachFakeTerminal(t, s)
	r := framework.SocketRunner{Socket: s.Socket}
	facts := clipolicy.Facts{ClientAttached: true}
	big := strings.Repeat("y", clipolicy.MaxOSC52Bytes+1)

	rep, err := clipolicy.Execute(context.Background(), r, facts, clipolicy.Prefs{OSC52: true}, big)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.BufferSet || rep.OSC52Sent {
		t.Errorf("oversized report = %+v, want buffer set / osc52 not sent", rep)
	}
	buf, _ := s.Run("show-buffer")
	if len(buf) != len(big) {
		t.Errorf("buffer len = %d, want %d", len(buf), len(big))
	}
	time.Sleep(300 * time.Millisecond)
	for _, p := range ft.Payloads() {
		if p == big {
			t.Error("oversized text must not be sent via OSC 52")
		}
	}
}
