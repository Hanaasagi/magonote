package clipboard

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
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

func TestExecuteLoadBufferWithOSC52(t *testing.T) {
	f := &fakeRunner{}
	facts := Facts{ClientAttached: true, ClientName: "c1"} // no native tool
	rep, err := Execute(context.Background(), f, facts, Prefs{OSC52: true}, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.BufferSet || !rep.OSC52Sent || rep.NativeDone {
		t.Errorf("report = %+v", rep)
	}
	want := []string{"load-buffer", "-w", "-t", "c1", "-"}
	if !reflect.DeepEqual(f.args, want) {
		t.Errorf("args = %v, want %v", f.args, want)
	}
	if f.stdin != "hi" {
		t.Errorf("stdin = %q", f.stdin)
	}
}

func TestExecuteGatedOff(t *testing.T) {
	f := &fakeRunner{}
	rep, err := Execute(context.Background(), f, Facts{ClientAttached: true}, Prefs{OSC52: false}, "hi")
	if err != nil || f.calls != 0 || rep.BufferSet {
		t.Errorf("OSC52=false should do nothing: calls=%d rep=%+v err=%v", f.calls, rep, err)
	}
}

func TestExecuteEmptyNoop(t *testing.T) {
	f := &fakeRunner{}
	_, err := Execute(context.Background(), f, Facts{ClientAttached: true}, Prefs{OSC52: true}, "")
	if err != nil || f.calls != 0 {
		t.Errorf("empty should do nothing: calls=%d err=%v", f.calls, err)
	}
}

func TestExecuteOversizedNoW(t *testing.T) {
	f := &fakeRunner{}
	big := strings.Repeat("x", MaxOSC52Bytes+1)
	rep, err := Execute(context.Background(), f, Facts{ClientAttached: true}, Prefs{OSC52: true}, big)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OSC52Sent || !rep.BufferSet {
		t.Errorf("oversized: buffer set, osc52 not sent; got %+v", rep)
	}
	for _, a := range f.args {
		if a == "-w" {
			t.Errorf("oversized must not pass -w: %v", f.args)
		}
	}
	if !hasWarning(rep.Warnings, "size limit") {
		t.Errorf("expected size warning, got %v", rep.Warnings)
	}
}

func TestExecuteErrorPropagates(t *testing.T) {
	f := &fakeRunner{err: errors.New("tmux down")}
	rep, err := Execute(context.Background(), f, Facts{ClientAttached: true}, Prefs{OSC52: true}, "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if rep.BufferSet {
		t.Error("buffer should not be marked set on failure")
	}
}
