package internal

import "testing"

func TestStateScreenAvailableBeforeMatches(t *testing.T) {
	state := NewState("\x1b[31mred\x1b[0m", "abcd", nil)
	if got := state.Lines; len(got) != 1 || got[0] != "red" {
		t.Fatalf("Lines = %#v", got)
	}
	if state.Screen() == nil || len(state.Screen().Lines) != 1 || state.Screen().Lines[0].Cells[0].Style.Fg.Index != 1 {
		t.Fatalf("Screen is unavailable or lost style")
	}
}

func TestStateMatchKeepsDisplaySpan(t *testing.T) {
	matches := NewState("中 https://example.com", "abcd", nil).Matches(false, 0)
	if len(matches) != 1 || matches[0].X != 3 || matches[0].EndCol != 22 {
		t.Fatalf("match span = %#v", matches)
	}
}

func TestStateHyperlinkTextDoesNotReplaceVisibleLabel(t *testing.T) {
	state := NewState("\x1b]8;;https://example.com\adocs\x1b]8;;\a", "abcd", nil, WithColorDetection())
	matches := state.Matches(false, 0)
	if len(matches) != 1 || matches[0].Text != "https://example.com" || matches[0].X != 0 || matches[0].EndCol != 4 {
		t.Fatalf("hyperlink match = %#v", matches)
	}
	if state.Lines[0] != "docs" {
		t.Fatalf("visible label = %q", state.Lines[0])
	}
}

func TestStatePreservesFileLocation(t *testing.T) {
	matches := NewState("main.go:12:4", "abcd", nil).Matches(false, 0)
	if len(matches) != 1 || matches[0].Location == nil {
		t.Fatalf("matches = %#v", matches)
	}
	location := matches[0].Location
	if location.Path != "main.go" || location.Line != 12 || location.Column != 4 {
		t.Fatalf("location = %#v", location)
	}
}

func TestStateMatchesRefreshesExclusions(t *testing.T) {
	state := NewState("https://example.com https://example.org", "abcd", nil)
	state.ExclusionConfig = &ExclusionConfig{Rules: []ExclusionRule{{Type: "text", Pattern: "example.com"}}}
	matches := state.Matches(false, 0)
	if len(matches) != 1 || matches[0].Text != "https://example.org" {
		t.Fatalf("first exclusions = %#v", matches)
	}
	state.ExclusionConfig.Rules = []ExclusionRule{{Type: "text", Pattern: "example.org"}}
	matches = state.Matches(false, 0)
	if len(matches) != 1 || matches[0].Text != "https://example.com" {
		t.Fatalf("updated exclusions = %#v", matches)
	}
}

func TestMatchReverseTrueAssignsLastMatchA(t *testing.T) {
	matches := NewState("127.0.0.1 192.168.1.1", "abcd", nil).Matches(true, 0)
	if len(matches) != 2 || *matches[0].Hint != "b" || *matches[1].Hint != "a" {
		t.Fatalf("reverse matches = %#v", matches)
	}
}
