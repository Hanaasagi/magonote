package colordetection

import (
	"github.com/leaanthony/go-ansi-parser"
)

// StyledTextSegment represents a text segment that contains styled content
type StyledTextSegment struct {
	Lines      []string     // The lines that contain styled text
	StartLine  int          // Starting line number in the original text
	EndLine    int          // Ending line number in the original text
	StyledText []StyledSpan // Information about styled text spans
	PlainText  []string     // The same lines but with ANSI sequences stripped
}

// StyledSpan represents a span of text with style information
type StyledSpan struct {
	Text        string // The actual text content (without ANSI codes)
	LineIndex   int    // Which line this span belongs to (relative to segment)
	StartPos    int    // Start position in the line
	EndPos      int    // End position in the line
	ForegroundR *int   // Foreground color RGB values (nil if not set)
	ForegroundG *int
	ForegroundB *int
	BackgroundR *int // Background color RGB values (nil if not set)
	BackgroundG *int
	BackgroundB *int
	Bold        bool // Whether text is bold
	Underline   bool // Whether text is underlined
	Italic      bool // Whether text is italic
}

// StyledTextDetector detects styled text segments in ANSI-escaped content
type StyledTextDetector struct {
	// Currently no configuration needed since we detect any styled text
}

// NewStyledTextDetector creates a new styled text detector
func NewStyledTextDetector() *StyledTextDetector {
	return &StyledTextDetector{}
}

// DetectStyledSegments analyzes text lines and returns segments that contain styled content
func (std *StyledTextDetector) DetectStyledSegments(lines []string) []StyledTextSegment {
	if len(lines) == 0 {
		return nil
	}

	var segments []StyledTextSegment

	// Process text to find styled segments
	for i := 0; i < len(lines); {
		segment := std.analyzeStyledSegment(lines, i)
		if segment != nil {
			segments = append(segments, *segment)
			i = segment.EndLine + 1
		} else {
			i++
		}
	}

	return segments
}

// analyzeStyledSegment analyzes a potential styled segment starting from the given line
func (std *StyledTextDetector) analyzeStyledSegment(lines []string, startIdx int) *StyledTextSegment {
	// Skip empty lines at the start
	for startIdx < len(lines) && len(lines[startIdx]) == 0 {
		startIdx++
	}

	if startIdx >= len(lines) {
		return nil
	}

	// Collect consecutive lines with styled content
	var segmentLines []string
	var lineIndices []int
	var allStyledSpans []StyledSpan
	var plainTextLines []string

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]

		// Parse styles in this line
		styledSpans, plainText := std.parseStylesInLine(line, len(segmentLines))

		// If this line has styled content, include it
		if len(styledSpans) > 0 {
			segmentLines = append(segmentLines, line)
			lineIndices = append(lineIndices, i)
			allStyledSpans = append(allStyledSpans, styledSpans...)
			plainTextLines = append(plainTextLines, plainText)
		} else {
			// If we already have styled lines, stop here
			if len(segmentLines) > 0 {
				break
			}
			// Otherwise, continue looking for styled content
		}
	}

	// Return segment if we found any styled content
	if len(segmentLines) > 0 {
		return &StyledTextSegment{
			Lines:      segmentLines,
			StartLine:  lineIndices[0],
			EndLine:    lineIndices[len(lineIndices)-1],
			StyledText: allStyledSpans,
			PlainText:  plainTextLines,
		}
	}

	return nil
}

// parseStylesInLine parses ANSI style codes in a line and returns styled spans
func (std *StyledTextDetector) parseStylesInLine(line string, lineIndex int) ([]StyledSpan, string) {
	// Parse the line using go-ansi-parser
	elements, err := ansi.Parse(line)
	if err != nil {
		// If parsing fails, return empty result
		return nil, line
	}

	var styledSpans []StyledSpan
	var plainText string
	currentPos := 0

	for _, element := range elements {
		if element.Label == "" {
			continue
		}

		// Check if this element has any styling
		hasStyle := element.FgCol != nil || element.BgCol != nil ||
			element.Bold() || element.Underlined() || element.Italic()

		if hasStyle {
			// Create styled span
			span := StyledSpan{
				Text:      element.Label,
				LineIndex: lineIndex,
				StartPos:  currentPos,
				EndPos:    currentPos + len(element.Label),
				Bold:      element.Bold(),
				Underline: element.Underlined(),
				Italic:    element.Italic(),
			}

			// Set color information
			if element.FgCol != nil {
				r := int(element.FgCol.Rgb.R)
				g := int(element.FgCol.Rgb.G)
				b := int(element.FgCol.Rgb.B)
				span.ForegroundR = &r
				span.ForegroundG = &g
				span.ForegroundB = &b
			}

			if element.BgCol != nil {
				r := int(element.BgCol.Rgb.R)
				g := int(element.BgCol.Rgb.G)
				b := int(element.BgCol.Rgb.B)
				span.BackgroundR = &r
				span.BackgroundG = &g
				span.BackgroundB = &b
			}

			styledSpans = append(styledSpans, span)
		}

		plainText += element.Label
		currentPos += len(element.Label)
	}

	return styledSpans, plainText
}

// GetStyledText returns only the text portions that have style information
func (sts *StyledTextSegment) GetStyledText() []string {
	var result []string
	for _, span := range sts.StyledText {
		if span.Text != "" {
			result = append(result, span.Text)
		}
	}
	return result
}

// GetStyledSpansByLine returns styled spans grouped by line index
func (sts *StyledTextSegment) GetStyledSpansByLine() map[int][]StyledSpan {
	result := make(map[int][]StyledSpan)
	for _, span := range sts.StyledText {
		result[span.LineIndex] = append(result[span.LineIndex], span)
	}
	return result
}
