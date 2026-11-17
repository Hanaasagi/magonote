package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func benchState(b *testing.B, text string) *State {
	b.Helper()
	state, err := NewState(text, "abcd", nil)
	if err != nil {
		b.Fatal(err)
	}
	return state
}

// BenchmarkRenderCompose measures the steady-state composition hot path
// (base copy + overlay blit). It must stay allocation-free.
func BenchmarkRenderCompose(b *testing.B) {
	state := benchState(b, "192.168.1.1 10.0.0.1 https://example.com/very/long/path main.go:12:4")
	matches := state.Matches(false, 0)
	r := newRenderer(state, matches, ViewColors{}, false, "left")
	r.ensureBase(120, 5)
	r.compose(nil, nil, "")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.compose(nil, nil, "a")
	}
}

// BenchmarkRenderComposeDense exercises a table-like screen with many matches.
func BenchmarkRenderComposeDense(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "pod-%d  1/1  Running  0  22h  10.0.%d.%d\n", i, i/250, i%250)
	}
	state := benchState(b, sb.String())
	matches := state.Matches(false, 0)
	r := newRenderer(state, matches, ViewColors{}, false, "left")
	r.ensureBase(120, 60)
	r.compose(nil, nil, "")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.compose(nil, nil, "a")
	}
}

// BenchmarkRenderFrame measures a full frame including projection and tcell
// presentation on a simulation screen.
func BenchmarkRenderFrame(b *testing.B) {
	state := benchState(b, "192.168.1.1 10.0.0.1 https://example.com/very/long/path main.go:12:4")
	matches := state.Matches(false, 0)
	screen := simulationScreenB(b, 120, 5)
	r := newRenderer(state, matches, ViewColors{}, false, "left")
	r.render(screen, nil, nil, "", "")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.render(screen, nil, nil, "a", "")
	}
}

func simulationScreenB(b *testing.B, width, height int) tcell.SimulationScreen {
	b.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	screen.SetSize(width, height)
	b.Cleanup(screen.Fini)
	return screen
}
