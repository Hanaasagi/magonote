package clipboard

import (
	"strings"
	"testing"
)

func hasWarning(warns []string, substr string) bool {
	for _, w := range warns {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestDecideGatedByOSC52(t *testing.T) {
	f := Facts{ClientAttached: true, NativeTool: "pbcopy"}
	if p := Decide(f, Prefs{OSC52: false}, "hi"); len(p.Steps) != 0 {
		t.Errorf("OSC52=false must yield empty plan, got %+v", p)
	}
	if p := Decide(f, Prefs{OSC52: true}, ""); len(p.Steps) != 0 {
		t.Errorf("empty text must yield empty plan, got %+v", p)
	}
}

func TestDecideMatrix(t *testing.T) {
	big := strings.Repeat("x", MaxOSC52Bytes+1)
	cases := []struct {
		name      string
		facts     Facts
		text      string
		wantKinds []StepKind
		wantOSC52 bool
		wantWarn  string // substring expected present, "" = no warnings
	}{
		{
			name:      "local attached native",
			facts:     Facts{ClientAttached: true, NativeTool: "pbcopy"},
			text:      "hi",
			wantKinds: []StepKind{StepLoadBuffer, StepNative},
			wantOSC52: true,
		},
		{
			name:      "ssh suppresses native",
			facts:     Facts{ClientAttached: true, SSH: true, NativeTool: "pbcopy"},
			text:      "hi",
			wantKinds: []StepKind{StepLoadBuffer},
			wantOSC52: true,
		},
		{
			name:      "no native tool",
			facts:     Facts{ClientAttached: true},
			text:      "hi",
			wantKinds: []StepKind{StepLoadBuffer},
			wantOSC52: true,
		},
		{
			name:      "oversized drops osc52",
			facts:     Facts{ClientAttached: true},
			text:      big,
			wantKinds: []StepKind{StepLoadBuffer},
			wantOSC52: false,
			wantWarn:  "size limit",
		},
		{
			name:      "detached warns",
			facts:     Facts{ClientAttached: false},
			text:      "hi",
			wantKinds: []StepKind{StepLoadBuffer},
			wantOSC52: true,
			wantWarn:  "no attached",
		},
		{
			name:      "nested warns",
			facts:     Facts{ClientAttached: true, Nested: true},
			text:      "hi",
			wantKinds: []StepKind{StepLoadBuffer},
			wantOSC52: true,
			wantWarn:  "nested",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Decide(tc.facts, Prefs{OSC52: true}, tc.text)
			if len(p.Steps) != len(tc.wantKinds) {
				t.Fatalf("steps = %+v, want kinds %v", p.Steps, tc.wantKinds)
			}
			for i, k := range tc.wantKinds {
				if p.Steps[i].Kind != k {
					t.Errorf("step[%d].Kind = %v, want %v", i, p.Steps[i].Kind, k)
				}
			}
			if p.Steps[0].OSC52 != tc.wantOSC52 {
				t.Errorf("LoadBuffer.OSC52 = %v, want %v", p.Steps[0].OSC52, tc.wantOSC52)
			}
			if tc.wantWarn == "" {
				if len(p.Warnings) != 0 {
					t.Errorf("expected no warnings, got %v", p.Warnings)
				}
			} else if !hasWarning(p.Warnings, tc.wantWarn) {
				t.Errorf("expected warning containing %q, got %v", tc.wantWarn, p.Warnings)
			}
		})
	}
}
