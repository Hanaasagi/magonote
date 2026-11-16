package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"reflect"
	"testing"
)

type fakeRunner struct {
	calls int
	args  []string
	stdin string
	err   error
}

func (f *fakeRunner) Run(_ context.Context, args []string, stdin io.Reader) (string, error) {
	f.calls++
	f.args = args
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		f.stdin = string(b)
	}
	return "", f.err
}

func TestEncodeOSC52(t *testing.T) {
	got := EncodeOSC52("hello")
	want := "\x1b]52;c;aGVsbG8=\x07"
	if string(got) != want {
		t.Errorf("EncodeOSC52(hello) = %q, want %q", got, want)
	}
	if EncodeOSC52("") != nil {
		t.Error("EncodeOSC52(empty) must be nil (no clipboard clear)")
	}
	// Unicode round-trips through base64.
	text := "路径/🚀/café"
	seq := EncodeOSC52(text)
	b64 := string(seq[len("\x1b]52;c;") : len(seq)-1])
	dec, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || string(dec) != text {
		t.Errorf("unicode roundtrip failed: %q err=%v", dec, err)
	}
}

func TestLoadBufferArgs(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		osc52  bool
		client string
		want   []string
		calls  int
	}{
		{"osc52+client", "hi", true, "c1", []string{"load-buffer", "-w", "-t", "c1", "-"}, 1},
		{"plain", "hi", false, "", []string{"load-buffer", "-"}, 1},
		{"osc52 noclient", "hi", true, "", []string{"load-buffer", "-w", "-"}, 1},
		{"empty noop", "", true, "c1", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{}
			err := LoadBuffer(context.Background(), f, tc.text, tc.osc52, tc.client)
			if err != nil {
				t.Fatal(err)
			}
			if f.calls != tc.calls {
				t.Errorf("calls = %d, want %d", f.calls, tc.calls)
			}
			if tc.calls > 0 {
				if !reflect.DeepEqual(f.args, tc.want) {
					t.Errorf("args = %v, want %v", f.args, tc.want)
				}
				if f.stdin != tc.text {
					t.Errorf("stdin = %q, want %q", f.stdin, tc.text)
				}
			}
		})
	}
}

func TestLoadBufferError(t *testing.T) {
	f := &fakeRunner{err: errors.New("tmux down")}
	err := LoadBuffer(context.Background(), f, "hi", true, "")
	if err == nil || err.Error() != "tmux down" {
		t.Errorf("expected propagated error, got %v", err)
	}
}

func TestCopyNativeEmptyNoop(t *testing.T) {
	if err := CopyNative(context.Background(), ""); err != nil {
		t.Errorf("empty CopyNative should be no-op, got %v", err)
	}
}

func TestCopyEmptyNoop(t *testing.T) {
	var buf bytes.Buffer
	c := New(WithOutput(&buf))
	if err := c.Copy(""); err != nil {
		t.Errorf("empty Copy should be no-op, got %v", err)
	}
	if buf.Len() != 0 {
		t.Error("empty Copy should write nothing")
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestCopyAggregatesErrors(t *testing.T) {
	c := New(WithTmux(false), WithSystem(false), WithOSC52(true), WithOutput(errWriter{}))
	err := c.Copy("x")
	if err == nil {
		t.Fatal("expected error from failing OSC52 writer")
	}
	if !errors.Is(err, err) || err.Error() == "" {
		t.Errorf("unexpected error: %v", err)
	}
}
