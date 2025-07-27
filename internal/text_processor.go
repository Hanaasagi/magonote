package internal

import (
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/colordetection"
)

// TextProcessor defines how to process different types of text input
type TextProcessor interface {
	// Process analyzes the input text and returns plain text lines and style-based matches
	Process(text string) (lines []string, styleMatches []Match, err error)
	// HasStyledContent returns true if the processor detected styled content
	HasStyledContent() bool
}

// PlainTextProcessor handles plain text without ANSI styling
type PlainTextProcessor struct{}

// NewPlainTextProcessor creates a new plain text processor
func NewPlainTextProcessor() *PlainTextProcessor {
	return &PlainTextProcessor{}
}

// Process splits plain text into lines and returns no style matches
func (p *PlainTextProcessor) Process(text string) ([]string, []Match, error) {
	lines := strings.Split(text, "\n")
	return lines, nil, nil
}

// HasStyledContent always returns false for plain text
func (p *PlainTextProcessor) HasStyledContent() bool {
	return false
}

// StyledTextProcessor handles ANSI-styled text using colordetection
type StyledTextProcessor struct {
	result *colordetection.ParseResult
}

// NewStyledTextProcessor creates a new styled text processor
func NewStyledTextProcessor() *StyledTextProcessor {
	return &StyledTextProcessor{}
}

// Process analyzes styled text and extracts both plain text and style-based matches
func (s *StyledTextProcessor) Process(text string) ([]string, []Match, error) {
	result, err := colordetection.ParseText(text)
	if err != nil {
		return nil, nil, err
	}

	s.result = result
	lines := strings.Split(result.PlainText, "\n")

	// Convert style spans to matches
	var styleMatches []Match
	for _, span := range result.StyleSpans {
		// Only include spans that have visible styling
		if span.HasStyling() {
			styleMatches = append(styleMatches, Match{
				X:       span.StartCol,
				Y:       span.StartLine,
				Pattern: "styled",
				Text:    span.Text,
				Hint:    nil,
			})
		}
	}

	return lines, styleMatches, nil
}

// HasStyledContent returns true if styled content was detected
func (s *StyledTextProcessor) HasStyledContent() bool {
	return s.result != nil && s.result.HasStyledContent()
}

// CreateTextProcessor automatically selects the appropriate processor based on content
func CreateTextProcessor(text string) TextProcessor {
	// Quick check for ANSI escape sequences
	if strings.ContainsAny(text, "\033\x1b") {
		return NewStyledTextProcessor()
	}
	return NewPlainTextProcessor()
}
