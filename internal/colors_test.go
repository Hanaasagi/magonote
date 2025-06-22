package internal

import (
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestMatchColor(t *testing.T) {
	color1 := GetColor("green").FgString("foo")

	// Create a direct color reference for comparison
	color2 := color.New(color.FgGreen).Sprint("foo")

	if color1 != color2 {
		t.Errorf("Expected %q, got %q", color2, color1)
	}
}

func TestParseRGB(t *testing.T) {
	color1 := GetColor("#1b1cbf").FgString("foo")

	// RGB format should be "\x1b[38;2;27;28;191mfoo\x1b[0m"
	if !strings.Contains(color1, "27;28;191") {
		t.Errorf("Expected RGB color with 27;28;191, got %q", color1)
	}
}

func TestInvalidRGB(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for invalid RGB")
		}
	}()
	_ = GetColor("#1b1cbj")
}

func TestUnknownColor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for unknown color")
		}
	}()
	_ = GetColor("wat")
}
