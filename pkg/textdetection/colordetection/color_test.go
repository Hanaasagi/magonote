package colordetection

import (
	"testing"
)

func TestStyledTextDetector_NewStyledTextDetector(t *testing.T) {
	detector := NewStyledTextDetector()
	if detector == nil {
		t.Error("Expected NewStyledTextDetector to return a non-nil detector")
	}
}

func TestStyledTextDetector_DetectStyledSegments_EmptyInput(t *testing.T) {
	detector := NewStyledTextDetector()

	// Test empty slice
	segments := detector.DetectStyledSegments([]string{})
	if segments != nil {
		t.Error("Expected nil segments for empty input")
	}

	// Test nil input
	segments = detector.DetectStyledSegments(nil)
	if segments != nil {
		t.Error("Expected nil segments for nil input")
	}
}

func TestStyledTextDetector_DetectStyledSegments_PlainText(t *testing.T) {
	detector := NewStyledTextDetector()

	lines := []string{
		"This is plain text",
		"Another plain line",
		"No styling here",
	}

	segments := detector.DetectStyledSegments(lines)
	if len(segments) != 0 {
		t.Errorf("Expected 0 segments for plain text, got %d", len(segments))
	}
}

func TestStyledTextDetector_ParseStylesInLine(t *testing.T) {
	detector := NewStyledTextDetector()

	tests := []struct {
		name               string
		input              string
		expectedSpanCount  int
		expectedPlainText  string
		expectedBold       bool
		expectedUnderline  bool
		expectedForeground bool
	}{
		{
			name:              "Plain text",
			input:             "plain text",
			expectedSpanCount: 0,
			expectedPlainText: "plain text",
		},
		{
			name:              "Bold text",
			input:             "\x1b[1mbold text\x1b[0m",
			expectedSpanCount: 1,
			expectedPlainText: "bold text",
			expectedBold:      true,
		},
		{
			name:              "Underlined text",
			input:             "\x1b[4munderlined text\x1b[0m",
			expectedSpanCount: 1,
			expectedPlainText: "underlined text",
			expectedUnderline: true,
		},
		{
			name:               "RGB colored text",
			input:              "\x1b[38;2;255;255;255mwhite text\x1b[39m",
			expectedSpanCount:  1,
			expectedPlainText:  "white text",
			expectedForeground: true,
		},
		{
			name:              "Mixed styled text",
			input:             "\x1b[38;2;255;0;0mred\x1b[39m normal \x1b[1mbold\x1b[0m",
			expectedSpanCount: 3,
			expectedPlainText: "red normal bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans, plainText := detector.parseStylesInLine(tt.input, 0)

			if len(spans) != tt.expectedSpanCount {
				t.Errorf("Expected %d spans, got %d", tt.expectedSpanCount, len(spans))
			}

			if plainText != tt.expectedPlainText {
				t.Errorf("Expected plain text '%s', got '%s'", tt.expectedPlainText, plainText)
			}

			if len(spans) > 0 {
				span := spans[0]
				if tt.expectedBold && !span.Bold {
					t.Error("Expected first span to be bold")
				}
				if tt.expectedUnderline && !span.Underline {
					t.Error("Expected first span to be underlined")
				}
				if tt.expectedForeground && span.ForegroundR == nil {
					t.Error("Expected first span to have foreground color")
				}
			}
		})
	}
}

func TestStyledTextDetector_ScreenTextData(t *testing.T) {
	detector := NewStyledTextDetector()

	screenData := []string{
		"",
		"\x1b[38;2;255;255;255madd_param command        string\x1b[39m",
		"\x1b[38;2;255;255;255madd_param upcase-command string\x1b[39m",
		"\x1b[38;2;255;255;255madd_param multi-command  string\x1b[39m",
		"\x1b[38;2;255;255;255madd_param osc52          boolean\x1b[39m",
		"",
		"\x1b[38;2;255;255;255m\"$\x1b[38;2;230;219;116m{\x1b[38;2;255;255;255mBINARY\x1b[38;2;230;219;116m}\x1b[38;2;255;255;255m\" \"$\x1b[38;2;230;219;116m{\x1b[38;2;255;255;255mPARAMS[@]\x1b[38;2;230;219;116m}\x1b[38;2;255;255;255m\" \x1b[38;2;249;38;114m||\x1b[38;2;255;255;255m true\x1b[39m",
		"\x1b[1m\x1b[32m➜  \x1b[36mmagonote\x1b[0m \x1b[1m\x1b[34mgit:(\x1b[31mmaster\x1b[34m) \x1b[33m✗\x1b[0m ls -alh",
		"\x1b[4mPermissions\x1b[0m \x1b[4mSize\x1b[0m \x1b[4mUser\x1b[0m   \x1b[4mDate Modified\x1b[0m    \x1b[4mName",
		"\x1b[0;1m\x1b[36md\x1b[33mr\x1b[31mw\x1b[32mx\x1b[0m\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[39m@    \x1b[1m\x1b[90m-\x1b[0m \x1b[1m\x1b[33mkumiko\x1b[0m \x1b[34m2025-07-08 23:20\x1b[39m \x1b[1m\x1b[36m.git",
	}

	segments := detector.DetectStyledSegments(screenData)

	if len(segments) == 0 {
		t.Error("Expected to detect styled segments from screen data")
	}

	if len(segments) > 0 {
		firstSegment := segments[0]
		if len(firstSegment.StyledText) == 0 {
			t.Error("Expected first segment to have styled text")
		}

		found := false
		for _, span := range firstSegment.StyledText {
			if span.ForegroundR != nil && *span.ForegroundR == 255 &&
				span.ForegroundG != nil && *span.ForegroundG == 255 &&
				span.ForegroundB != nil && *span.ForegroundB == 255 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find white foreground color in first segment")
		}
	}
}

func TestStyledTextDetector_ShellPromptWithColors(t *testing.T) {
	detector := NewStyledTextDetector()

	promptLine := "\x1b[1m\x1b[32m➜  \x1b[36mmagonote\x1b[0m \x1b[1m\x1b[34mgit:(\x1b[31mmaster\x1b[34m) \x1b[33m✗\x1b[0m ls -alh"

	segments := detector.DetectStyledSegments([]string{promptLine})

	if len(segments) != 1 {
		t.Errorf("Expected 1 segment for shell prompt, got %d", len(segments))
	}

	if len(segments) > 0 {
		segment := segments[0]

		if len(segment.StyledText) == 0 {
			t.Error("Expected shell prompt to have styled spans")
		}

		hasBold := false
		for _, span := range segment.StyledText {
			if span.Bold {
				hasBold = true
				break
			}
		}
		if !hasBold {
			t.Error("Expected shell prompt to contain bold text")
		}

		expectedPlainText := "➜  magonote git:(master) ✗ ls -alh"
		if len(segment.PlainText) > 0 && segment.PlainText[0] != expectedPlainText {
			t.Errorf("Expected plain text '%s', got '%s'", expectedPlainText, segment.PlainText[0])
		}
	}
}

func TestStyledTextDetector_FilePermissionsWithColors(t *testing.T) {
	detector := NewStyledTextDetector()

	permissionLine := "\x1b[0;1m\x1b[36md\x1b[33mr\x1b[31mw\x1b[32mx\x1b[0m\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[39m@    \x1b[1m\x1b[90m-\x1b[0m \x1b[1m\x1b[33mkumiko\x1b[0m \x1b[34m2025-07-08 23:20\x1b[39m \x1b[1m\x1b[36m.git"

	segments := detector.DetectStyledSegments([]string{permissionLine})

	if len(segments) != 1 {
		t.Errorf("Expected 1 segment for permission line, got %d", len(segments))
	}

	if len(segments) > 0 {
		segment := segments[0]

		if len(segment.StyledText) == 0 {
			t.Error("Expected permission line to have styled spans")
		}

		colors := make(map[string]bool)
		for _, span := range segment.StyledText {
			if span.ForegroundR != nil {
				colorKey := string(rune(*span.ForegroundR)) + string(rune(*span.ForegroundG)) + string(rune(*span.ForegroundB))
				colors[colorKey] = true
			}
		}

		if len(colors) < 3 {
			t.Errorf("Expected multiple colors in permission line, got %d unique colors", len(colors))
		}
	}
}

func TestStyledTextSegment_GetStyledText(t *testing.T) {
	segment := StyledTextSegment{
		Lines: []string{"\x1b[1mtest\x1b[0m"},
		StyledText: []StyledSpan{
			{Text: "bold", Bold: true},
			{Text: "normal"},
		},
	}

	styledTexts := segment.GetStyledText()

	if len(styledTexts) != 2 {
		t.Errorf("Expected 2 styled texts, got %d", len(styledTexts))
	}

	if styledTexts[0] != "bold" || styledTexts[1] != "normal" {
		t.Errorf("Expected ['bold', 'normal'], got %v", styledTexts)
	}
}

func TestStyledTextSegment_GetStyledSpansByLine(t *testing.T) {
	segment := StyledTextSegment{
		StyledText: []StyledSpan{
			{Text: "line0-span1", LineIndex: 0},
			{Text: "line0-span2", LineIndex: 0},
			{Text: "line1-span1", LineIndex: 1},
		},
	}

	spansByLine := segment.GetStyledSpansByLine()

	if len(spansByLine) != 2 {
		t.Errorf("Expected 2 lines with spans, got %d", len(spansByLine))
	}

	if len(spansByLine[0]) != 2 {
		t.Errorf("Expected 2 spans for line 0, got %d", len(spansByLine[0]))
	}

	if len(spansByLine[1]) != 1 {
		t.Errorf("Expected 1 span for line 1, got %d", len(spansByLine[1]))
	}
}

func TestStyledSpan_ColorProperties(t *testing.T) {
	fgR, fgG, fgB := 255, 128, 64
	span := StyledSpan{
		Text:        "colored text",
		ForegroundR: &fgR,
		ForegroundG: &fgG,
		ForegroundB: &fgB,
	}

	if span.ForegroundR == nil || *span.ForegroundR != 255 {
		t.Error("Expected ForegroundR to be 255")
	}
	if span.ForegroundG == nil || *span.ForegroundG != 128 {
		t.Error("Expected ForegroundG to be 128")
	}
	if span.ForegroundB == nil || *span.ForegroundB != 64 {
		t.Error("Expected ForegroundB to be 64")
	}

	bgR, bgG, bgB := 32, 64, 128
	span.BackgroundR = &bgR
	span.BackgroundG = &bgG
	span.BackgroundB = &bgB

	if span.BackgroundR == nil || *span.BackgroundR != 32 {
		t.Error("Expected BackgroundR to be 32")
	}
	if span.BackgroundG == nil || *span.BackgroundG != 64 {
		t.Error("Expected BackgroundG to be 64")
	}
	if span.BackgroundB == nil || *span.BackgroundB != 128 {
		t.Error("Expected BackgroundB to be 128")
	}
}

func TestStyledTextDetector_ComplexScreenData(t *testing.T) {
	detector := NewStyledTextDetector()

	complexData := []string{
		"\x1b[38;2;255;255;255m\"$\x1b[38;2;230;219;116m{\x1b[38;2;255;255;255mBINARY\x1b[38;2;230;219;116m}\x1b[38;2;255;255;255m\" \"$\x1b[38;2;230;219;116m{\x1b[38;2;255;255;255mPARAMS[@]\x1b[38;2;230;219;116m}\x1b[38;2;255;255;255m\" \x1b[38;2;249;38;114m||\x1b[38;2;255;255;255m true\x1b[39m",
		"\x1b[4mPermissions\x1b[0m \x1b[4mSize\x1b[0m \x1b[4mUser\x1b[0m   \x1b[4mDate Modified\x1b[0m    \x1b[4mName",
	}

	segments := detector.DetectStyledSegments(complexData)

	if len(segments) == 0 {
		t.Error("Expected to detect segments from complex data")
	}

	hasUnderline := false
	for _, segment := range segments {
		for _, span := range segment.StyledText {
			if span.Underline {
				hasUnderline = true
				break
			}
		}
		if hasUnderline {
			break
		}
	}

	if !hasUnderline {
		t.Error("Expected to find underlined text in complex data")
	}
}
