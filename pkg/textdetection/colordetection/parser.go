package colordetection

import (
	"strings"

	"github.com/leaanthony/go-ansi-parser"
)

// Parser handles ANSI escape sequence parsing
type Parser struct{}

// NewParser creates a new ANSI parser instance
func NewParser() *Parser {
	return &Parser{}
}

// Parse analyzes text and extracts styled content along with plain text
func (p *Parser) Parse(text string) (*ParseResult, error) {
	if text == "" {
		return &ParseResult{
			PlainText:  "",
			StyleSpans: []StyleSpan{},
		}, nil
	}

	lines := strings.Split(text, "\n")
	var allSpans []StyleSpan
	var plainTextLines []string

	for lineNum, line := range lines {
		spans, plainText := p.parseLineSpans(line, lineNum)
		allSpans = append(allSpans, spans...)
		plainTextLines = append(plainTextLines, plainText)
	}

	return &ParseResult{
		PlainText:  strings.Join(plainTextLines, "\n"),
		StyleSpans: allSpans,
	}, nil
}

// parseLineSpans extracts styled spans from a single line
func (p *Parser) parseLineSpans(line string, lineNum int) ([]StyleSpan, string) {
	elements, err := ansi.Parse(line)
	if err != nil {
		return nil, line
	}

	var spans []StyleSpan
	var plainTextBuilder strings.Builder
	currentCol := 0

	for _, element := range elements {
		if element.Label == "" {
			continue
		}

		text := element.Label
		endCol := currentCol + len(text)

		if p.hasStyled(element) {
			span := StyleSpan{
				Text:      text,
				StartLine: lineNum,
				StartCol:  currentCol,
				EndLine:   lineNum,
				EndCol:    endCol,
				Style:     p.extractStyle(element),
			}
			spans = append(spans, span)
		}

		plainTextBuilder.WriteString(text)
		currentCol = endCol
	}

	return spans, plainTextBuilder.String()
}

// hasStyled checks if an element has any styling applied
func (p *Parser) hasStyled(element *ansi.StyledText) bool {
	return element.FgCol != nil || element.BgCol != nil ||
		element.Bold() || element.Underlined() || element.Italic()
}

// extractStyle converts ansi.StyledText to our Style struct
func (p *Parser) extractStyle(element *ansi.StyledText) Style {
	style := Style{
		Bold:      element.Bold(),
		Underline: element.Underlined(),
		Italic:    element.Italic(),
	}

	if element.FgCol != nil {
		style.ForegroundColor = &Color{
			R: int(element.FgCol.Rgb.R),
			G: int(element.FgCol.Rgb.G),
			B: int(element.FgCol.Rgb.B),
		}
	}

	if element.BgCol != nil {
		style.BackgroundColor = &Color{
			R: int(element.BgCol.Rgb.R),
			G: int(element.BgCol.Rgb.G),
			B: int(element.BgCol.Rgb.B),
		}
	}

	return style
}

// ParseText is a convenience function for parsing text with a default parser
func ParseText(text string) (*ParseResult, error) {
	parser := NewParser()
	return parser.Parse(text)
}
