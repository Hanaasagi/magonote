package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// ---------------------------------------------------------------------------
// Custom patterns coexist with built-in detection
// ---------------------------------------------------------------------------

func TestDetectCustomPatternCoexistsWithBuiltins(t *testing.T) {
	f := newFramework(t)

	// Adding custom -x patterns must not suppress built-in entity detection.
	input := "server 192.168.1.1 ticket JIRA-999"
	result, err := f.RunDetect(input, "-x", `JIRA-\d+`)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if !result.HasEntity("ipv4", "192.168.1.1") {
		t.Error("built-in IPv4 should still be detected alongside custom patterns")
	}
	if !result.HasEntity("custom", "JIRA-999") {
		t.Error("custom pattern JIRA-999 should be detected")
	}
}

// ---------------------------------------------------------------------------
// Multiple custom regex patterns
// ---------------------------------------------------------------------------

func TestDetectMultipleCustomRegex(t *testing.T) {
	f := newFramework(t)

	input := "ticket JIRA-123 and build BUILD-456 done"
	result, err := f.RunDetect(input, "-x", `JIRA-\d+`, "-x", `BUILD-\d+`)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if !result.HasEntity("custom", "JIRA-123") {
		t.Errorf("expected JIRA-123; got %v", result.EntityTexts())
	}
	if !result.HasEntity("custom", "BUILD-456") {
		t.Errorf("expected BUILD-456; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Word detection combined with entity detection
// ---------------------------------------------------------------------------

func TestDetectWordsWithEntities(t *testing.T) {
	f := newFramework(t)

	input := "server 192.168.1.1 is running"
	resultNoWords, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect (no words): %v", err)
	}

	resultWithWords, err := f.RunDetect(input, "--words")
	if err != nil {
		t.Fatalf("detect (with words): %v", err)
	}

	// With --words, there should be more entities (words + IP)
	if len(resultWithWords.Entities) <= len(resultNoWords.Entities) {
		t.Errorf("--words should produce more entities: without=%d, with=%d",
			len(resultNoWords.Entities), len(resultWithWords.Entities))
	}

	// The IP should still be detected in both
	if !resultWithWords.HasEntity("ipv4", "192.168.1.1") {
		t.Error("IP should be detected even with --words")
	}
}

// ---------------------------------------------------------------------------
// Multi-line input with mixed entities
// ---------------------------------------------------------------------------

func TestDetectMultiLineEntities(t *testing.T) {
	f := newFramework(t)

	input := `line 0: 192.168.1.1
line 1: https://example.com
line 2: user@test.com
line 3: 550e8400-e29b-41d4-a716-446655440000`

	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// Verify line numbers in spans
	checks := []struct {
		text         string
		expectedLine int
	}{
		{"192.168.1.1", 0},
		{"https://example.com", 1},
		{"user@test.com", 2},
		{"550e8400-e29b-41d4-a716-446655440000", 3},
	}

	for _, check := range checks {
		found := false
		for _, e := range result.Entities {
			if e.Text == check.text {
				if e.Span.Line != check.expectedLine {
					t.Errorf("entity %q on line %d, expected line %d", check.text, e.Span.Line, check.expectedLine)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entity %q not found", check.text)
		}
	}
}

// ---------------------------------------------------------------------------
// Python traceback with file locations
// ---------------------------------------------------------------------------

func TestDetectPythonTraceback(t *testing.T) {
	f := newFramework(t)

	input := `Traceback (most recent call last):
  File "/app/api/core/app/main.py", line 74, in handle
    return process()
  File "/app/api/utils.py", line 126, in process
    return do_work()`

	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// Should detect file paths from Python traceback
	paths := result.EntitiesOfKind("path")
	if len(paths) < 2 {
		t.Errorf("expected at least 2 paths from traceback, got %d: %v", len(paths), textsOfKind(result, "path"))
	}

	// Python tracebacks include 'File "...", line N' — at least one path
	// should carry a Location with a line number.
	hasLocation := false
	for _, p := range paths {
		if p.Location != nil && p.Location.Line > 0 {
			hasLocation = true
			break
		}
	}
	if !hasLocation {
		t.Error("expected at least one path with file location (line number) from Python traceback")
	}
}

// ---------------------------------------------------------------------------
// Markdown links
// ---------------------------------------------------------------------------

func TestDetectMarkdownLinks(t *testing.T) {
	f := newFramework(t)

	input := "see [magonote](https://github.com/Hanaasagi/magonote) for details"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if !result.HasEntity("url", "https://github.com/Hanaasagi/magonote") {
		t.Errorf("expected URL from markdown link; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Large number of entities
// ---------------------------------------------------------------------------

func TestDetectHighDensity(t *testing.T) {
	f := newFramework(t)

	// Use the ip.txt fixture — very high density of entities
	result, err := f.RunDetectFile(f.FixturePath("ip.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// Should handle 160 IPs without issue
	if len(result.Entities) < 100 {
		t.Errorf("high density test: expected 100+ entities, got %d", len(result.Entities))
	}

	// All entities should have valid spans
	for i, e := range result.Entities {
		if e.Span.StartCol < 0 || e.Span.EndCol <= e.Span.StartCol {
			t.Errorf("entity %d (%q) has invalid span: %+v", i, e.Text, e.Span)
		}
		if e.Span.Line < 0 {
			t.Errorf("entity %d (%q) has negative line: %d", i, e.Text, e.Span.Line)
		}
	}
}

// ---------------------------------------------------------------------------
// detect --json without --json should error
// ---------------------------------------------------------------------------

func TestDetectRequiresJSON(t *testing.T) {
	f := newFramework(t)
	if err := f.BuildBinary(); err != nil {
		t.Fatalf("build: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "magonote-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("192.168.1.1")
	tmpFile.Close()

	cmd := exec.Command(f.BinaryPath, "detect", "-i", tmpFile.Name(), "--config", "NONE")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Error("detect without --json should fail, but it succeeded")
	}
}
