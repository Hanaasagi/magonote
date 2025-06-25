package clipboard

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}

	// Check default settings
	if !c.tmux || !c.system || !c.osc52 {
		t.Error("Default settings should enable all targets")
	}

	if c.output != os.Stderr {
		t.Error("Default output should be os.Stderr")
	}
}

func TestWithOptions(t *testing.T) {
	var buf bytes.Buffer

	c := New(
		WithTmux(false),
		WithSystem(false),
		WithOSC52(true),
		WithOutput(&buf),
	)

	if c.tmux || c.system {
		t.Error("Options should disable tmux and system")
	}

	if !c.osc52 {
		t.Error("Options should enable osc52")
	}

	if c.output != &buf {
		t.Error("Output should be set to buffer")
	}
}

func TestIsTmuxSession(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original) // nolint: errcheck

	// Test without TMUX
	os.Unsetenv("TMUX") // nolint: errcheck
	if isTmuxSession() {
		t.Error("Should not detect tmux when TMUX env is unset")
	}

	// Test with TMUX
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0") // nolint: errcheck
	if !isTmuxSession() {
		t.Error("Should detect tmux when TMUX env is set")
	}
}

func TestGetClipboardTools(t *testing.T) {
	tools := getClipboardTools()

	switch runtime.GOOS {
	case "darwin":
		if len(tools) == 0 || tools[0] != "pbcopy" {
			t.Error("macOS should include pbcopy")
		}
	case "linux":
		expectedTools := []string{"wl-copy", "xclip", "xsel"}
		if len(tools) != len(expectedTools) {
			t.Error("Linux should have expected tools")
		}
		for i, tool := range expectedTools {
			if tools[i] != tool {
				t.Errorf("Expected tool %s at index %d, got %s", tool, i, tools[i])
			}
		}
	case "windows":
		if len(tools) == 0 || tools[0] != "clip" {
			t.Error("Windows should include clip")
		}
	}
}

func TestOSC52Writer(t *testing.T) {
	original := os.Getenv("TMUX")
	defer func() {
		if original == "" {
			os.Unsetenv("TMUX") // nolint: errcheck
		} else {
			os.Setenv("TMUX", original) // nolint: errcheck
		}
	}()
	// make sure we're not in a tmux session
	os.Unsetenv("TMUX") // nolint: errcheck

	var buf bytes.Buffer
	writer := NewOSC52Writer(&buf)

	// Test empty text (clear clipboard)
	err := writer.Write("")
	if err != nil {
		t.Fatalf("Write empty string failed: %v", err)
	}

	output := buf.String()
	expected := "\033]52;c;\007"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}

	// Test with text
	buf.Reset()
	err = writer.Write("hello")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output = buf.String()
	// "hello" in base64 is "aGVsbG8="
	expected = "\033]52;c;aGVsbG8=\007"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestOSC52WriterWithTmux(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer func() {
		if original == "" {
			os.Unsetenv("TMUX") // nolint: errcheck
		} else {
			os.Setenv("TMUX", original) // nolint: errcheck
		}
	}()

	// Set tmux environment
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0") // nolint: errcheck

	var buf bytes.Buffer
	writer := NewOSC52Writer(&buf)

	err := writer.Write("hello")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// Should be wrapped in tmux DCS passthrough
	expected := "\033Ptmux;\033\033]52;c;aGVsbG8=\007\033\\"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestClipboardCopy(t *testing.T) {
	var buf bytes.Buffer

	// Create clipboard with only OSC52 enabled for testing
	c := New(
		WithTmux(false),
		WithSystem(false),
		WithOSC52(true),
		WithOutput(&buf),
	)

	err := c.Copy("test")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\033]52;c;") {
		t.Error("Output should contain OSC52 sequence")
	}
}

func TestTmuxWriter(t *testing.T) {
	writer := NewTmuxWriter()

	// This test will only work in actual tmux session
	// We'll just test that it returns appropriate error when not in tmux
	original := os.Getenv("TMUX")
	defer func() {
		if original == "" {
			os.Unsetenv("TMUX") // nolint: errcheck
		} else {
			os.Setenv("TMUX", original) // nolint: errcheck
		}
	}()

	os.Unsetenv("TMUX") // nolint: errcheck
	err := writer.Write("test")
	if err == nil {
		t.Error("Should return error when not in tmux session")
	}

	if !strings.Contains(err.Error(), "not in tmux session") {
		t.Errorf("Expected 'not in tmux session' error, got: %v", err)
	}
}

func TestSystemWriter(t *testing.T) {
	writer := NewSystemWriter()

	// This test will depend on system clipboard tools being available
	err := writer.Write("test")

	// We can't guarantee system tools are available in all test environments
	// So we just test that it doesn't panic
	if err != nil && !strings.Contains(err.Error(), "no system clipboard tool available") {
		t.Logf("System clipboard error (expected in some environments): %v", err)
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Test that convenience functions don't panic
	// Actual functionality depends on environment

	err := CopyWithOSC52("test")
	if err != nil {
		t.Errorf("CopyWithOSC52 failed: %v", err)
	}

	// Test other convenience functions (they may fail in test environment)
	_ = Copy("test")
	_ = CopyToTmux("test")
	_ = CopyToSystem("test")
}

func TestAvailable(t *testing.T) {
	available := Available()

	// OSC52 should always be available
	if !available["osc52"] {
		t.Error("OSC52 should always be available")
	}

	// Test that all expected keys are present
	expectedKeys := []string{"tmux", "system", "osc52"}
	for _, key := range expectedKeys {
		if _, exists := available[key]; !exists {
			t.Errorf("Available() should include key: %s", key)
		}
	}
}

func TestHasSystemClipboard(t *testing.T) {
	// Just test that it doesn't panic
	_ = HasSystemClipboard()
}

func TestIsTmuxSessionConvenience(t *testing.T) {
	// Test convenience function
	result1 := IsTmuxSession()
	result2 := isTmuxSession()

	if result1 != result2 {
		t.Error("IsTmuxSession() and isTmuxSession() should return same value")
	}
}

// Benchmark tests
func BenchmarkOSC52Write(b *testing.B) {
	var buf bytes.Buffer
	writer := NewOSC52Writer(&buf)
	text := "Hello, World! This is a test string for benchmarking."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.Write(text) // nolint: errcheck
	}
}

func BenchmarkClipboardCopy(b *testing.B) {
	var buf bytes.Buffer
	c := New(
		WithTmux(false),
		WithSystem(false),
		WithOSC52(true),
		WithOutput(&buf),
	)
	text := "Hello, World! This is a test string for benchmarking."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		c.Copy(text) // nolint: errcheck
	}
}
