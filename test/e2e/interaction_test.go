package e2e

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

// ---------------------------------------------------------------------------
// Entity type selection — verify different entity types can be selected via PTY
// ---------------------------------------------------------------------------

func TestInteractionSelectURL(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select URL",
		Input:          "visit https://example.com/test",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "https://example.com/test",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

func TestInteractionSelectUUID(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select UUID",
		Input:          "id 550e8400-e29b-41d4-a716-446655440000 end",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "550e8400-e29b-41d4-a716-446655440000",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

func TestInteractionSelectPath(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select file path",
		Input:          "edit /var/log/nginx.log",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "/var/log/nginx.log",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

func TestInteractionSelectEmail(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select email",
		Input:          "contact admin@example.com",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "admin@example.com",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

func TestInteractionSelectSHA(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select SHA hash",
		Input:          "commit f79010f2 merged",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "f79010f2",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

func TestInteractionSelectColor(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Select color code",
		Input:          "use #FF0000 as primary",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "#FF0000",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// --reverse flag
// ---------------------------------------------------------------------------

func TestInteractionReverse(t *testing.T) {
	f := newFramework(t)

	// Default: 'a' → first/top match (192.168.1.1).
	// With --reverse: hint assignment is reversed, so 'a' → last/bottom match (10.0.0.1).
	tc := framework.TestCase{
		Name:           "Reverse hint order",
		Input:          "first 192.168.1.1\nsecond 10.0.0.1",
		Args:           []string{"--reverse"},
		Keys:           "a",
		ExpectedOutput: "10.0.0.1",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// --format flag
// ---------------------------------------------------------------------------

func TestInteractionFormat(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Custom format",
		Input:          "192.168.1.1",
		Args:           []string{"--format", "SELECTED:%H"},
		Keys:           "a",
		ExpectedOutput: "SELECTED:192.168.1.1",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// --target flag — output to file
// ---------------------------------------------------------------------------

func TestInteractionTarget(t *testing.T) {
	f := newFramework(t)

	content, err := f.RunWithTarget("192.168.1.1", "a")
	if err != nil {
		t.Fatalf("run with target: %v", err)
	}

	if !strings.Contains(content, "192.168.1.1") {
		t.Errorf("target file should contain 192.168.1.1, got %q", content)
	}
}

func TestInteractionTargetMulti(t *testing.T) {
	f := newFramework(t)

	content, err := f.RunWithTarget("192.168.1.1\n10.0.0.1", "as ", "--multi")
	if err != nil {
		t.Fatalf("run with target: %v", err)
	}

	if !strings.Contains(content, "192.168.1.1") {
		t.Errorf("target file should contain 192.168.1.1, got %q", content)
	}
	if !strings.Contains(content, "10.0.0.1") {
		t.Errorf("target file should contain 10.0.0.1, got %q", content)
	}
}

// ---------------------------------------------------------------------------
// --regexp flag (custom pattern via PTY)
// ---------------------------------------------------------------------------

func TestInteractionCustomRegex(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Custom regex selection",
		Input:          "ticket JIRA-12345 filed",
		Args:           []string{"-x", `JIRA-\d+`},
		Keys:           "a",
		ExpectedOutput: "JIRA-12345",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// --contrast flag
// ---------------------------------------------------------------------------

func TestInteractionContrast(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Contrast mode",
		Input:          "192.168.1.1",
		Args:           []string{"--contrast"},
		Keys:           "a",
		ExpectedOutput: "192.168.1.1",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// Multi-selection with multiple hint presses
// ---------------------------------------------------------------------------

func TestInteractionMultiThreeItems(t *testing.T) {
	f := newFramework(t)

	// Select all 3 IPs via hints a, s, d then space to finish.
	// Verify all three IPs appear in target file output.
	content, err := f.RunWithTarget("192.168.1.1\n10.0.0.1\n172.16.0.1", "asd ", "--multi")
	if err != nil {
		t.Fatalf("run with target: %v", err)
	}

	for _, ip := range []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"} {
		if !strings.Contains(content, ip) {
			t.Errorf("expected %q in multi-select output, got %q", ip, content)
		}
	}
}

// ---------------------------------------------------------------------------
// Escape key exits without output
// ---------------------------------------------------------------------------

func TestInteractionEscape(t *testing.T) {
	f := newFramework(t)

	content, err := f.RunWithTarget("192.168.1.1", "\x1b")
	if err != nil {
		if errors.Is(err, framework.ErrTimeout) {
			t.Fatalf("process timed out — ESC should cause timely exit, not hang: %v", err)
		}
		// Non-timeout errors (e.g. process exited quickly before keys) are tolerable
		t.Logf("escape: %v (non-timeout exit — checking target file)", err)
	}

	if strings.TrimSpace(content) != "" {
		t.Errorf("escape should produce empty target file, got %q", content)
	}
}

// ---------------------------------------------------------------------------
// No matches — should exit cleanly
// ---------------------------------------------------------------------------

func TestInteractionNoMatches(t *testing.T) {
	f := newFramework(t)

	content, err := f.RunWithTarget("just plain text nothing here", "a")
	if err != nil {
		if errors.Is(err, framework.ErrTimeout) {
			t.Fatalf("process timed out — no-match input should cause timely exit: %v", err)
		}
		// Non-timeout errors (e.g. process exits immediately when no matches) are expected
		t.Logf("no matches: %v (expected: process exits before keys are sent)", err)
	}

	if strings.TrimSpace(content) != "" {
		t.Errorf("no matches should produce empty target file, got %q", content)
	}
}

// ---------------------------------------------------------------------------
// --words flag via PTY
// ---------------------------------------------------------------------------

func TestInteractionWords(t *testing.T) {
	f := newFramework(t)

	tc := framework.TestCase{
		Name:           "Words mode selection",
		Input:          "hello world",
		Args:           []string{"--words"},
		Keys:           "a",
		ExpectedOutput: "hello",
	}

	result := f.RunTest(tc)
	if !result.Passed {
		t.Errorf("failed: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// Version flag (non-interactive)
// ---------------------------------------------------------------------------

func TestVersionFlag(t *testing.T) {
	f := newFramework(t)
	if err := f.BuildBinary(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(f.BinaryPath, "-v")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("version: %v", err)
	}

	if !strings.Contains(stdout.String(), "magonote version") {
		t.Errorf("expected version output, got %q", stdout.String())
	}
}

// ---------------------------------------------------------------------------
// Detect subcommand as non-interactive (no PTY needed)
// ---------------------------------------------------------------------------

func TestDetectCLIBasic(t *testing.T) {
	f := newFramework(t)
	if err := f.BuildBinary(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(f.BinaryPath, "detect", "--json", "--config", "NONE")
	cmd.Stdin = strings.NewReader("192.168.1.1")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("detect CLI: %v", err)
	}

	if !strings.Contains(stdout.String(), "192.168.1.1") {
		t.Errorf("detect CLI should output entity text; got %q", stdout.String())
	}
}
