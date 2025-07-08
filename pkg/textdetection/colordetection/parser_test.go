package colordetection

import (
	"strings"
	"testing"
)

func TestNewParser_ReturnsValidInstance(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Error("Expected NewParser to return a non-nil parser")
	}
}

func TestParseText_EmptyInput(t *testing.T) {
	result, err := ParseText("")
	if err != nil {
		t.Errorf("Expected no error for empty input, got %v", err)
	}
	if result.PlainText != "" {
		t.Errorf("Expected empty plain text, got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 0 {
		t.Errorf("Expected 0 style spans for empty input, got %d", len(result.StyleSpans))
	}
}

func TestParseText_PlainTextOnly(t *testing.T) {
	input := "This is plain text\nAnother plain line\nNo styling here"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for plain text, got %v", err)
	}
	if result.PlainText != input {
		t.Errorf("Expected plain text '%s', got '%s'", input, result.PlainText)
	}
	if len(result.StyleSpans) != 0 {
		t.Errorf("Expected 0 style spans for plain text, got %d", len(result.StyleSpans))
	}
}

func TestParseText_BoldText(t *testing.T) {
	input := "\x1b[1mbold text\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for bold text, got %v", err)
	}
	if result.PlainText != "bold text" {
		t.Errorf("Expected plain text 'bold text', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for bold text, got %d", len(result.StyleSpans))
	}
	if !result.StyleSpans[0].IsBold() {
		t.Error("Expected first span to be bold")
	}
}

func TestParseText_UnderlinedText(t *testing.T) {
	input := "\x1b[4munderlined text\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for underlined text, got %v", err)
	}
	if result.PlainText != "underlined text" {
		t.Errorf("Expected plain text 'underlined text', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for underlined text, got %d", len(result.StyleSpans))
	}
	if !result.StyleSpans[0].IsUnderlined() {
		t.Error("Expected first span to be underlined")
	}
}

func TestParseText_ItalicText(t *testing.T) {
	input := "\x1b[3mitalic text\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for italic text, got %v", err)
	}
	if result.PlainText != "italic text" {
		t.Errorf("Expected plain text 'italic text', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for italic text, got %d", len(result.StyleSpans))
	}
	if !result.StyleSpans[0].IsItalic() {
		t.Error("Expected first span to be italic")
	}
}

func TestParseText_ColoredText(t *testing.T) {
	input := "\x1b[38;2;255;255;255mwhite text\x1b[39m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for colored text, got %v", err)
	}
	if result.PlainText != "white text" {
		t.Errorf("Expected plain text 'white text', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for colored text, got %d", len(result.StyleSpans))
	}
	if !result.StyleSpans[0].HasForegroundColor() {
		t.Error("Expected first span to have foreground color")
	}
	color := result.StyleSpans[0].GetForegroundColor()
	if color.R != 255 || color.G != 255 || color.B != 255 {
		t.Errorf("Expected white color (255,255,255), got (%d,%d,%d)", color.R, color.G, color.B)
	}
}

func TestParseText_BackgroundColoredText(t *testing.T) {
	input := "\x1b[48;2;255;0;0mred background\x1b[49m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for background colored text, got %v", err)
	}
	if !result.StyleSpans[0].HasBackgroundColor() {
		t.Error("Expected first span to have background color")
	}
	color := result.StyleSpans[0].GetBackgroundColor()
	if color.R != 255 || color.G != 0 || color.B != 0 {
		t.Errorf("Expected red background (255,0,0), got (%d,%d,%d)", color.R, color.G, color.B)
	}
}

func TestParseText_MixedStyledText(t *testing.T) {
	input := "\x1b[38;2;255;0;0mred\x1b[39m normal \x1b[1mbold\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for mixed styled text, got %v", err)
	}
	if result.PlainText != "red normal bold" {
		t.Errorf("Expected plain text 'red normal bold', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 2 {
		t.Errorf("Expected 2 style spans for mixed text, got %d", len(result.StyleSpans))
	}
}

func TestParseText_MultilineText(t *testing.T) {
	input := "\x1b[1mfirst line\x1b[0m\n\x1b[4msecond line\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for multiline text, got %v", err)
	}
	if result.PlainText != "first line\nsecond line" {
		t.Errorf("Expected plain text 'first line\\nsecond line', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 2 {
		t.Errorf("Expected 2 style spans for multiline text, got %d", len(result.StyleSpans))
	}
	if result.StyleSpans[0].StartLine != 0 || result.StyleSpans[1].StartLine != 1 {
		t.Error("Expected spans to be on different lines")
	}
}

func TestParseText_ComplexShellPrompt(t *testing.T) {
	input := "\x1b[1m\x1b[32m➜  \x1b[36mmagonote\x1b[0m \x1b[1m\x1b[34mgit:(\x1b[31mmaster\x1b[34m) \x1b[33m✗\x1b[0m ls -alh"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for shell prompt, got %v", err)
	}
	if !result.HasStyledContent() {
		t.Error("Expected shell prompt to have styled content")
	}
	expectedPlainText := "➜  magonote git:(master) ✗ ls -alh"
	if result.PlainText != expectedPlainText {
		t.Errorf("Expected plain text '%s', got '%s'", expectedPlainText, result.PlainText)
	}
	boldSpans := result.GetBoldSpans()
	if len(boldSpans) == 0 {
		t.Error("Expected shell prompt to contain bold text")
	}
}

func TestParseText_FilePermissionsWithColors(t *testing.T) {
	input := "\x1b[0;1m\x1b[36md\x1b[33mr\x1b[31mw\x1b[32mx\x1b[0m\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[33mr\x1b[1m\x1b[90m-\x1b[0m\x1b[32mx\x1b[39m@"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for permission text, got %v", err)
	}
	coloredSpans := result.GetColoredSpans()
	if len(coloredSpans) < 3 {
		t.Errorf("Expected multiple colored spans, got %d", len(coloredSpans))
	}
}

func TestStyleSpan_PositionMethods(t *testing.T) {
	span := StyleSpan{
		Text:      "test text",
		StartLine: 2,
		StartCol:  5,
		EndLine:   2,
		EndCol:    14,
		Style: Style{
			Bold:            true,
			ForegroundColor: &Color{R: 255, G: 0, B: 0},
		},
	}

	if span.Length() != 9 {
		t.Errorf("Expected length 9, got %d", span.Length())
	}
	if !span.HasStyling() {
		t.Error("Expected span to have styling")
	}
	if !span.HasForegroundColor() {
		t.Error("Expected span to have foreground color")
	}
	if span.HasBackgroundColor() {
		t.Error("Expected span to not have background color")
	}
}

func TestParseResult_UtilityMethods(t *testing.T) {
	input := "\x1b[1mline1\x1b[0m\n\x1b[4mline2\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.GetLineCount() != 2 {
		t.Errorf("Expected 2 lines, got %d", result.GetLineCount())
	}

	spansByLine := result.GetStyledSpansByLine()
	if len(spansByLine) != 2 {
		t.Errorf("Expected spans for 2 lines, got %d", len(spansByLine))
	}

	line0Spans := result.GetSpansForLine(0)
	if len(line0Spans) != 1 {
		t.Errorf("Expected 1 span for line 0, got %d", len(line0Spans))
	}

	styledTexts := result.GetStyledText()
	expectedTexts := []string{"line1", "line2"}
	if len(styledTexts) != len(expectedTexts) {
		t.Errorf("Expected %d styled texts, got %d", len(expectedTexts), len(styledTexts))
	}
}

func TestParseText_OnlyNewlines(t *testing.T) {
	input := "\n\n\n"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for newlines only, got %v", err)
	}
	if result.PlainText != input {
		t.Errorf("Expected plain text to match input, got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 0 {
		t.Errorf("Expected 0 style spans for newlines only, got %d", len(result.StyleSpans))
	}
}

func TestParseText_MixedContentWithEmptyLines(t *testing.T) {
	input := "\x1b[1mbold\x1b[0m\n\nplain text\n\n\x1b[4munderlined\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for mixed content with empty lines, got %v", err)
	}
	if result.GetLineCount() != 5 {
		t.Errorf("Expected 5 lines, got %d", result.GetLineCount())
	}
	if len(result.StyleSpans) != 2 {
		t.Errorf("Expected 2 style spans, got %d", len(result.StyleSpans))
	}
}

func TestParseText_VeryLongLine(t *testing.T) {
	longText := strings.Repeat("a", 10000)
	input := "\x1b[1m" + longText + "\x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for very long line, got %v", err)
	}
	if result.PlainText != longText {
		t.Error("Expected plain text to match long text")
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for long line, got %d", len(result.StyleSpans))
	}
	if result.StyleSpans[0].Length() != 10000 {
		t.Errorf("Expected span length 10000, got %d", result.StyleSpans[0].Length())
	}
}

func TestParseText_SpecialCharacters(t *testing.T) {
	input := "\x1b[1m你好世界 🌍 ➜ \x1b[0m"
	result, err := ParseText(input)
	if err != nil {
		t.Errorf("Expected no error for special characters, got %v", err)
	}
	if result.PlainText != "你好世界 🌍 ➜ " {
		t.Errorf("Expected plain text '你好世界 🌍 ➜ ', got '%s'", result.PlainText)
	}
	if len(result.StyleSpans) != 1 {
		t.Errorf("Expected 1 style span for special characters, got %d", len(result.StyleSpans))
	}
}
