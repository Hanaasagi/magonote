package internal

import (
	"strings"
	"testing"
)

func split(output string) []string {
	// Go's strings.Split already returns a slice, so we don't need to collect
	return strings.Split(output, "\n")
}

func TestHintText(t *testing.T) {
	lines := split("lorem 127.0.0.1 lorem")
	custom := []string{}
	state := NewState(lines, "abcd", custom)

	view := NewView(
		state,
		false,               // multi
		false,               // reverse
		0,                   // uniqueLevel
		false,               // contrast
		"",                  // position
		GetColor("default"), // selectForegroundColor
		GetColor("default"), // selectBackgroundColor
		GetColor("default"), // multiForegroundColor
		GetColor("default"), // multiBackgroundColor
		GetColor("default"), // foregroundColor
		GetColor("default"), // backgroundColor
		GetColor("default"), // hintForegroundColor
		GetColor("default"), // hintBackgroundColor
	)

	// Test without contrast
	result := view.makeHintText("a")
	if result != "a" {
		t.Errorf("Expected 'a', got '%s'", result)
	}

	// Test with contrast
	view.contrast = true
	result = view.makeHintText("a")
	if result != "[a]" {
		t.Errorf("Expected '[a]', got '%s'", result)
	}
}
