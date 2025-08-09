// Package ps1parser provides functionality to parse zsh PS1 prompt strings
// and match them against terminal output to find prompt positions.
//
// This package supports all major zsh prompt escape sequences including:
//   - Basic sequences (%n, %m, %~, %d, %c, etc.)
//   - Color sequences (%{...%}, %F{color}, %K{color})
//   - Conditional expressions (%(test.true.false))
//   - Command substitution ($(command), ${variable})
//   - Date/time formatting (%D{format}, %T, %t, etc.)
//
// The package provides both high-level convenience functions and low-level
// APIs for detailed control over parsing and matching behavior.
//
// Example usage:
//
//	ps1 := "%n@%m:%~ $ "
//	terminalOutput := "user@host:~/project $ ls"
//	matches, err := ps1parser.FindPrompts(ps1, terminalOutput)
package ps1parser

import (
	"fmt"
)

// ParseAndMatch is a convenience function that parses a PS1 string and immediately
// creates a matcher for finding prompts in text. This is the recommended high-level API.
func ParseAndMatch(ps1 string, text string, options MatchOptions) ([]MatchResult, error) {
	parser := NewParser(ParserOptions{})

	parsed, err := parser.Parse(ps1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PS1: %w", err)
	}

	matcher, err := NewMatcher(parsed, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create matcher: %w", err)
	}

	return matcher.Match(text)
}

// DefaultMatchOptions returns sensible default options for matching.
// These options work well for most terminal environments and use cases.
func DefaultMatchOptions() MatchOptions {
	return MatchOptions{
		IgnoreColors:    true,
		IgnoreSpacing:   false,
		CaseSensitive:   false,
		MaxLineSpan:     3, // Most prompts span at most 3 lines
		TimeoutPatterns: true,
	}
}

// StrictMatchOptions returns options for strict matching.
// Use these when you need exact matching including colors and spacing.
func StrictMatchOptions() MatchOptions {
	return MatchOptions{
		IgnoreColors:    false,
		IgnoreSpacing:   false,
		CaseSensitive:   true,
		MaxLineSpan:     0, // No limit
		TimeoutPatterns: false,
	}
}

// FindPrompts is a high-level function to find all prompts in terminal output
// using a PS1 pattern with default options. This is the simplest way to use the library.
func FindPrompts(ps1 string, terminalOutput string) ([]MatchResult, error) {
	return ParseAndMatch(ps1, terminalOutput, DefaultMatchOptions())
}

// FindPromptsStrict is like FindPrompts but uses strict matching options
func FindPromptsStrict(ps1 string, terminalOutput string) ([]MatchResult, error) {
	return ParseAndMatch(ps1, terminalOutput, StrictMatchOptions())
}

// ValidatePS1 checks if a PS1 string is valid and parseable.
// Returns an error if the PS1 contains invalid syntax.
func ValidatePS1(ps1 string) error {
	parser := NewParser(ParserOptions{StrictMode: true})
	_, err := parser.Parse(ps1)
	return err
}

// AnalyzePS1 returns detailed information about a PS1 string.
// Use this function to inspect the structure and tokens of a PS1 string.
func AnalyzePS1(ps1 string) (*ParsedPS1, error) {
	parser := NewParser(ParserOptions{})
	return parser.Parse(ps1)
}

// TokenTypeString returns a human-readable string for a token type
func (t TokenType) String() string {
	switch t {
	case TokenLiteral:
		return "Literal"
	case TokenPercent:
		return "Percent"
	case TokenColorSeq:
		return "ColorSeq"
	case TokenCondition:
		return "Condition"
	case TokenCommand:
		return "Command"
	default:
		return "Unknown"
	}
}

// String returns a string representation of a token
func (t Token) String() string {
	return fmt.Sprintf("%s(%q)", t.Type, t.Content)
}

// String returns a string representation of a position
func (p Position) String() string {
	if p.StartLine == p.EndLine {
		return fmt.Sprintf("line %d, cols %d-%d", p.StartLine+1, p.StartCol+1, p.EndCol+1)
	}
	return fmt.Sprintf("lines %d-%d, cols %d-%d", p.StartLine+1, p.EndLine+1, p.StartCol+1, p.EndCol+1)
}
