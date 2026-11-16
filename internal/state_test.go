package internal

import (
	"strings"
	"testing"
)

func TestStyledTextMatching(t *testing.T) {
	text := "\x1b[1;31merror\x1b[0m normal \x1b[4mwarning\x1b[0m"
	state := mustNewState(t, text, "abcd", nil, WithColorDetection())
	results := state.Matches(false, 0)
	if len(results) != 2 || results[0].Text != "error" || results[1].Text != "warning" {
		t.Fatalf("styled matches = %#v", results)
	}
	if state.Lines[0] != "error normal warning" {
		t.Fatalf("plain lines = %#v", state.Lines)
	}
}

func TestMatchDefaultHints(t *testing.T) {
	text := "lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem"
	results := mustNewState(t, text, "abcd", nil).Matches(false, 0)
	if len(results) != 3 || *results[0].Hint != "a" || *results[len(results)-1].Hint != "c" {
		t.Fatalf("default hints = %#v", results)
	}
}

func TestMatchUnique(t *testing.T) {
	text := "lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem"
	results := mustNewState(t, text, "abcd", nil).Matches(false, 1)
	if len(results) != 3 || *results[0].Hint != "a" || *results[len(results)-1].Hint != "a" {
		t.Fatalf("unique hints = %#v", results)
	}
}

func TestMatchSuperUnique(t *testing.T) {
	text := "lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem"
	results := mustNewState(t, text, "abcd", nil).Matches(false, 2)
	if len(results) != 2 {
		t.Fatalf("super unique matches = %#v", results)
	}
	found := map[string]bool{}
	for _, result := range results {
		found[result.Text] = true
	}
	if !found["127.0.0.1"] || !found["255.255.255.255"] {
		t.Fatalf("unique IPs = %#v", found)
	}
}

func TestMatchSuperUniqueMiddleSelection(t *testing.T) {
	lines := strings.Split("127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1", "\n")
	results := mustNewStateFromLines(t, lines, "abcd", nil).Matches(false, 2)
	if len(results) != 1 || results[0].Y != 2 {
		t.Fatalf("middle selection = %#v", results)
	}
}

func TestMatchSuperUniqueEarlySelection(t *testing.T) {
	results := mustNewState(t, "127.0.0.1\n127.0.0.1", "abcd", nil).Matches(false, 2)
	if len(results) != 1 || results[0].Y != 0 {
		t.Fatalf("early selection = %#v", results)
	}
}

func TestMatchSuperUniqueComplexScenario(t *testing.T) {
	results := mustNewState(t, "127.0.0.1\n127.0.0.1\n127.0.0.2\n127.0.0.1", "abcd", nil).Matches(false, 2)
	if len(results) != 2 {
		t.Fatalf("complex unique matches = %#v", results)
	}
	lines := map[string]int{}
	for _, result := range results {
		lines[result.Text] = result.Y
	}
	if lines["127.0.0.1"] != 0 || lines["127.0.0.2"] != 2 {
		t.Fatalf("spaced selection = %#v", lines)
	}
}

func TestMatchSuperUniqueWithManyLines(t *testing.T) {
	results := mustNewState(t, "127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1", "abcd", nil).Matches(false, 2)
	if len(results) != 1 || results[0].Y != 3 {
		t.Fatalf("many-line selection = %#v", results)
	}
}

func TestCustomPatterns(t *testing.T) {
	results := mustNewState(t, "CUSTOM-52463 ISSUE-123", "abcd", []string{"CUSTOM-[0-9]{4,}", "ISSUE-[0-9]{3}"}).Matches(false, 0)
	if len(results) != 2 || results[0].Text != "CUSTOM-52463" || results[1].Text != "ISSUE-123" {
		t.Fatalf("custom matches = %#v", results)
	}
}

func TestExclusionRulesThroughState(t *testing.T) {
	state := mustNewState(t, "https://a.com https://b.com main.go", "abcd", nil, WithExclusionRules([]ExclusionRule{{Type: "text", Pattern: "a.com"}, {Type: "regex", Pattern: `main\.go`}}))
	results := state.Matches(false, 0)
	if len(results) != 1 || results[0].Text != "https://b.com" {
		t.Fatalf("excluded matches = %#v", results)
	}
}

func TestUnicodeExclusionDoesNotShiftMatch(t *testing.T) {
	state := mustNewState(t, "中 https://example.com", "abcd", nil, WithExclusionRules([]ExclusionRule{{Type: "text", Pattern: "中"}}))
	results := state.Matches(false, 0)
	if len(results) != 1 || results[0].X != 3 {
		t.Fatalf("Unicode exclusion = %#v", results)
	}
}

func TestExclusionBeforeUniqueSelectsRemainingRepresentative(t *testing.T) {
	state := mustNewState(t, "127.0.0.1\nfoo 127.0.0.1\n127.0.0.1", "abcd", nil, WithExclusionRules([]ExclusionRule{{Type: "regex", Pattern: "^foo"}}))
	results := state.Matches(false, 2)
	if len(results) != 1 || results[0].Y == 1 {
		t.Fatalf("unique representative after exclusion = %#v", results)
	}
}

func TestNewStateRejectsInvalidAlphabet(t *testing.T) {
	_, err := NewState("hello", "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for invalid alphabet")
	}
}
