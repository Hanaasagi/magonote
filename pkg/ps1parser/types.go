// Package ps1parser provides functionality to parse zsh PS1 prompt strings
// and match them against terminal output to find prompt positions.
package ps1parser

import "regexp"

// Position represents the location of a matched prompt in the text.
// All line and column numbers are zero-based.
type Position struct {
	StartLine int // Starting line number (zero-based)
	StartCol  int // Starting column number (zero-based)
	EndLine   int // Ending line number (zero-based)
	EndCol    int // Ending column number (zero-based)
}

// TokenType represents different types of elements in a PS1 string.
type TokenType int

// Token types used in PS1 parsing
const (
	TokenLiteral   TokenType = iota // Regular text content
	TokenPercent                    // % escape sequences like %n, %m, %~
	TokenColorSeq                   // %{...%} color sequences and %F{color}
	TokenCondition                  // %(test.true.false) conditional expressions
	TokenCommand                    // $(command) and ${variable} substitutions
)

// Token represents a parsed element from the PS1 string.
// Params stores additional metadata specific to the token type.
type Token struct {
	Type    TokenType
	Content string
	Params  map[string]string // Parameters like condition test, colors, etc.
}

// ParsedPS1 represents a fully parsed PS1 string containing all tokens.
type ParsedPS1 struct {
	Tokens []Token
}

// MatchPattern represents a compiled pattern that can match against terminal output.
// It contains the compiled regex and metadata needed for matching.
type MatchPattern struct {
	regex   *regexp.Regexp
	tokens  []Token
	options MatchOptions
}

// MatchOptions controls how the pattern matching behaves.
// These options provide flexibility for different terminal environments and use cases.
type MatchOptions struct {
	IgnoreColors    bool // Whether to ignore ANSI color codes in the text
	IgnoreSpacing   bool // Whether to ignore extra whitespace
	CaseSensitive   bool // Whether matching should be case-sensitive
	MaxLineSpan     int  // Maximum lines a prompt can span (0 = unlimited)
	TimeoutPatterns bool // Whether to match timeout indicators like "took 5m30s"
	// When true, the compiled pattern is anchored at the beginning of each line.
	// Shell prompts appear at the start of a line, so this is enabled by default.
	AnchorAtLineStart bool
}

// MatchResult represents the result of a successful pattern match.
// It includes position information and captured groups.
type MatchResult struct {
	Position Position
	Matched  string
	Groups   map[string]string // Named capture groups from the regex
}

// Parser handles parsing PS1 strings into tokens.
type Parser struct {
	options ParserOptions
}

// ParserOptions controls parsing behavior and error handling.
type ParserOptions struct {
	StrictMode     bool // Whether to fail on unknown escape sequences
	ExpandCommands bool // Whether to expand command substitutions (future feature)
}

// Matcher handles matching parsed PS1 patterns against text.
// It compiles the tokens into a regex pattern and provides matching functionality.
type Matcher struct {
	pattern *MatchPattern
	options MatchOptions
}
