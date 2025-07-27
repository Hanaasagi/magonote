# Color Detection Module

This module provides ANSI escape sequence parsing functionality to extract styled text content and position information.

## Features

- Parse ANSI-escaped text and extract plain text
- Identify styled text spans with position information
- Support for colors, bold, underline, and italic formatting
- Easy-to-use API with convenience methods
- Modular design with clear separation of concerns

## Quick Start

```go
import "github.com/Hanaasagi/magonote/pkg/textdetection/colordetection"

// Parse ANSI-escaped text
result, err := colordetection.ParseText("\x1b[1m\x1b[31mBold red text\x1b[0m")
if err != nil {
    log.Fatal(err)
}

// Get plain text without ANSI codes
fmt.Println("Plain text:", result.PlainText)

// Access styled spans
for _, span := range result.StyleSpans {
    fmt.Printf("Span: '%s' at line %d, col %d-%d\n", 
        span.Text, span.StartLine, span.StartCol, span.EndCol)
    
    if span.IsBold() {
        fmt.Println("  - Bold")
    }
    if span.HasForegroundColor() {
        color := span.GetForegroundColor()
        fmt.Printf("  - Color: RGB(%d,%d,%d)\n", color.R, color.G, color.B)
    }
}
```

## API Reference

### Main Types

#### `ParseResult`
Contains the result of parsing ANSI-escaped text.
- `PlainText string` - Text with ANSI codes removed
- `StyleSpans []StyleSpan` - Array of styled text spans

#### `StyleSpan`
Represents a styled text span with position and style information.
- `Text string` - The actual text content
- `StartLine int` - Starting line number (0-based)
- `StartCol int` - Starting column position
- `EndLine int` - Ending line number
- `EndCol int` - Ending column position
- `Style Style` - Style information

#### `Style`
Contains visual styling information.
- `ForegroundColor *Color` - Foreground color (nil if not set)
- `BackgroundColor *Color` - Background color (nil if not set)
- `Bold bool` - Whether text is bold
- `Underline bool` - Whether text is underlined
- `Italic bool` - Whether text is italic

#### `Color`
RGB color values.
- `R int` - Red component (0-255)
- `G int` - Green component (0-255)
- `B int` - Blue component (0-255)

### Functions

#### `ParseText(text string) (*ParseResult, error)`
Convenience function to parse text with a default parser.

#### `NewParser() *Parser`
Creates a new parser instance.

#### `(p *Parser) Parse(text string) (*ParseResult, error)`
Parses ANSI-escaped text and returns styled content.

### StyleSpan Methods

- `HasForegroundColor() bool` - Returns true if span has foreground color
- `HasBackgroundColor() bool` - Returns true if span has background color
- `HasStyling() bool` - Returns true if span has any styling
- `GetForegroundColor() *Color` - Returns foreground color if present
- `GetBackgroundColor() *Color` - Returns background color if present
- `IsBold() bool` - Returns true if text is bold
- `IsUnderlined() bool` - Returns true if text is underlined
- `IsItalic() bool` - Returns true if text is italic
- `Length() int` - Returns text length

### ParseResult Methods

- `GetStyledSpansByLine() map[int][]StyleSpan` - Groups spans by line number
- `GetStyledText() []string` - Returns only styled text portions
- `HasStyledContent() bool` - Returns true if result contains styled spans
- `GetLineCount() int` - Returns number of lines in plain text
- `GetSpansForLine(lineNum int) []StyleSpan` - Returns spans for specific line
- `GetBoldSpans() []StyleSpan` - Returns all bold spans
- `GetColoredSpans() []StyleSpan` - Returns all colored spans

## Architecture

The module is organized into separate files for clarity:

- `types.go` - Data structure definitions
- `parser.go` - Core parsing logic
- `utils.go` - Convenience methods and utilities
- `color_test.go` - Comprehensive test suite
- `example_test.go` - Usage examples

## Testing

Run tests with:
```bash
go test -v
```

The test suite covers:
- Basic functionality (bold, italic, underline, colors)
- Complex real-world scenarios (shell prompts, file listings)
- Edge cases (empty input, very long lines, special characters)
- Utility methods and convenience functions 