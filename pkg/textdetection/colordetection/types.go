package colordetection

import "strings"

// Style contains visual styling information
type Style struct {
	ForegroundColor *Color `json:"foreground_color,omitempty"`
	BackgroundColor *Color `json:"background_color,omitempty"`
	Bold            bool   `json:"bold,omitempty"`
	Underline       bool   `json:"underline,omitempty"`
	Italic          bool   `json:"italic,omitempty"`
}

// Color represents RGB color values
type Color struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// StyleSpan represents a styled text span with position and style information
type StyleSpan struct {
	Text      string `json:"text"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
	Style     Style  `json:"style"`
}

// HasForegroundColor returns true if the span has foreground color
func (s *StyleSpan) HasForegroundColor() bool {
	return s.Style.ForegroundColor != nil
}

// HasBackgroundColor returns true if the span has background color
func (s *StyleSpan) HasBackgroundColor() bool {
	return s.Style.BackgroundColor != nil
}

// HasStyling returns true if the span has any styling applied
func (s *StyleSpan) HasStyling() bool {
	return s.HasForegroundColor() || s.HasBackgroundColor() ||
		s.Style.Bold || s.Style.Underline || s.Style.Italic
}

// GetForegroundColor returns the foreground color if present
func (s *StyleSpan) GetForegroundColor() *Color {
	return s.Style.ForegroundColor
}

// GetBackgroundColor returns the background color if present
func (s *StyleSpan) GetBackgroundColor() *Color {
	return s.Style.BackgroundColor
}

// IsBold returns true if the text is bold
func (s *StyleSpan) IsBold() bool {
	return s.Style.Bold
}

// IsUnderlined returns true if the text is underlined
func (s *StyleSpan) IsUnderlined() bool {
	return s.Style.Underline
}

// IsItalic returns true if the text is italic
func (s *StyleSpan) IsItalic() bool {
	return s.Style.Italic
}

// Length returns the text length of the span
func (s *StyleSpan) Length() int {
	return len(s.Text)
}

// ParseResult represents the result of parsing ANSI-escaped text
type ParseResult struct {
	PlainText  string      `json:"plain_text"`
	StyleSpans []StyleSpan `json:"style_spans"`
}

// GetStyledSpansByLine groups style spans by line number
func (pr *ParseResult) GetStyledSpansByLine() map[int][]StyleSpan {
	result := make(map[int][]StyleSpan)
	for _, span := range pr.StyleSpans {
		result[span.StartLine] = append(result[span.StartLine], span)
	}
	return result
}

// GetStyledText returns only the text portions that have styling
func (pr *ParseResult) GetStyledText() []string {
	var result []string
	for _, span := range pr.StyleSpans {
		if span.Text != "" {
			result = append(result, span.Text)
		}
	}
	return result
}

// HasStyledContent returns true if the result contains any styled spans
func (pr *ParseResult) HasStyledContent() bool {
	return len(pr.StyleSpans) > 0
}

// GetLineCount returns the number of lines in the plain text
func (pr *ParseResult) GetLineCount() int {
	if pr.PlainText == "" {
		return 0
	}
	return len(strings.Split(pr.PlainText, "\n"))
}

// GetSpansForLine returns all style spans for a specific line
func (pr *ParseResult) GetSpansForLine(lineNum int) []StyleSpan {
	var spans []StyleSpan
	for _, span := range pr.StyleSpans {
		if span.StartLine == lineNum {
			spans = append(spans, span)
		}
	}
	return spans
}

// GetBoldSpans returns all spans that are bold
func (pr *ParseResult) GetBoldSpans() []StyleSpan {
	var spans []StyleSpan
	for _, span := range pr.StyleSpans {
		if span.IsBold() {
			spans = append(spans, span)
		}
	}
	return spans
}

// GetColoredSpans returns all spans that have foreground or background colors
func (pr *ParseResult) GetColoredSpans() []StyleSpan {
	var spans []StyleSpan
	for _, span := range pr.StyleSpans {
		if span.HasForegroundColor() || span.HasBackgroundColor() {
			spans = append(spans, span)
		}
	}
	return spans
}
