package ps1parser

import (
	"fmt"
	"strings"
)

// NewParser creates a new PS1 parser with the given options.
// Use ParserOptions to control parsing behavior such as strict mode.
func NewParser(options ParserOptions) *Parser {
	return &Parser{
		options: options,
	}
}

// Parse parses a PS1 string into tokens.
// It processes the entire PS1 string character by character, identifying
// and categorizing each component into appropriate token types.
func (p *Parser) Parse(ps1 string) (*ParsedPS1, error) {
	var tokens []Token
	pos := 0

	for pos < len(ps1) {
		token, nextPos, err := p.parseNextToken(ps1, pos)
		if err != nil {
			return nil, fmt.Errorf("parse error at position %d: %w", pos, err)
		}

		if token.Type != TokenLiteral || len(token.Content) > 0 {
			tokens = append(tokens, token)
		}

		pos = nextPos
	}

	return &ParsedPS1{Tokens: tokens}, nil
}

// parseNextToken parses the next token starting at the given position
func (p *Parser) parseNextToken(ps1 string, pos int) (Token, int, error) {
	if pos >= len(ps1) {
		return Token{}, pos, nil
	}

	char := ps1[pos]

	switch char {
	case '%':
		return p.parsePercentToken(ps1, pos)
	case '$':
		if pos+1 < len(ps1) && (ps1[pos+1] == '(' || ps1[pos+1] == '{') {
			return p.parseCommandToken(ps1, pos)
		} else if pos+1 < len(ps1) && isVarStartChar(ps1[pos+1]) {
			// Handle $VAR format variables
			return p.parseSimpleVariable(ps1, pos)
		}
		fallthrough
	default:
		return p.parseLiteralToken(ps1, pos)
	}
}

// parsePercentToken parses % escape sequences
func (p *Parser) parsePercentToken(ps1 string, pos int) (Token, int, error) {
	if pos+1 >= len(ps1) {
		return Token{Type: TokenLiteral, Content: "%"}, pos + 1, nil
	}

	nextChar := ps1[pos+1]

	switch nextChar {
	case '{':
		// Color sequence %{...%}
		return p.parseColorSequence(ps1, pos)
	case '(':
		// Conditional expression %(...)
		return p.parseConditionalExpression(ps1, pos)
	case '$':
		// Variable reference %$ or %${...}
		return p.parseVariableReference(ps1, pos)
	case '%':
		// Escaped percent
		return Token{Type: TokenLiteral, Content: "%"}, pos + 2, nil
	case ')':
		// Closing parenthesis
		return Token{Type: TokenLiteral, Content: ")"}, pos + 2, nil
	default:
		// Check for numbered conditional like %1(...)
		if nextChar >= '0' && nextChar <= '9' {
			// Look ahead to see if it's a conditional
			numberEnd := pos + 1
			for numberEnd < len(ps1) && ps1[numberEnd] >= '0' && ps1[numberEnd] <= '9' {
				numberEnd++
			}
			if numberEnd < len(ps1) && ps1[numberEnd] == '(' {
				return p.parseConditionalExpression(ps1, pos)
			}
		}
		// Simple percent escape like %n, %m, %~, etc.
		return p.parseSimplePercent(ps1, pos)
	}
}

// parseColorSequence parses %{...%} sequences, including nested ones
func (p *Parser) parseColorSequence(ps1 string, pos int) (Token, int, error) {
	// Find the matching closing %} by counting braces
	braceCount := 1
	closePos := -1

	for i := pos + 2; i < len(ps1)-1; i++ {
		if ps1[i] == '%' && ps1[i+1] == '{' {
			braceCount++
			i++ // Skip the next character
		} else if ps1[i] == '%' && ps1[i+1] == '}' {
			braceCount--
			if braceCount == 0 {
				closePos = i
				break
			}
			i++ // Skip the next character
		}
	}

	if closePos == -1 {
		if p.options.StrictMode {
			return Token{}, pos, fmt.Errorf("unclosed color sequence")
		}
		// Treat as literal if not closed
		return Token{Type: TokenLiteral, Content: "%{"}, pos + 2, nil
	}

	content := ps1[pos+2 : closePos]

	// Check for nested color sequences
	colorType := "color"
	if strings.Contains(content, "%{") || strings.Contains(content, "%}") {
		colorType = "nested_color"
	}

	return Token{
		Type:    TokenColorSeq,
		Content: content,
		Params: map[string]string{
			"type": colorType,
		},
	}, closePos + 2, nil
}

// parseVariableReference parses %$ and %${...} variable references
func (p *Parser) parseVariableReference(ps1 string, pos int) (Token, int, error) {
	if pos+2 >= len(ps1) {
		// Just %$ at the end
		return Token{
			Type:    TokenPercent,
			Content: "$",
			Params: map[string]string{
				"escape":  "$",
				"meaning": "variable_reference",
			},
		}, pos + 2, nil
	}

	// Check if it's %${...} format
	if ps1[pos+2] == '{' {
		// Find the closing brace
		closePos := strings.Index(ps1[pos+3:], "}")
		if closePos == -1 {
			// Unclosed variable reference, treat as literal
			return Token{Type: TokenLiteral, Content: "%$"}, pos + 2, nil
		}

		closePos += pos + 3
		varName := ps1[pos+3 : closePos]

		return Token{
			Type:    TokenPercent,
			Content: "${" + varName + "}",
			Params: map[string]string{
				"escape":   "$",
				"meaning":  "variable_reference_braced",
				"var_name": varName,
			},
		}, closePos + 1, nil
	}

	// Find the end of the variable name (alphanumeric + underscore)
	nameEnd := pos + 2
	for nameEnd < len(ps1) && (isAlphanumeric(ps1[nameEnd]) || ps1[nameEnd] == '_') {
		nameEnd++
	}

	if nameEnd == pos+2 {
		// No variable name after %$, treat as literal
		return Token{
			Type:    TokenPercent,
			Content: "$",
			Params: map[string]string{
				"escape":  "$",
				"meaning": "variable_reference",
			},
		}, pos + 2, nil
	}

	varName := ps1[pos+2 : nameEnd]
	return Token{
		Type:    TokenPercent,
		Content: "$" + varName,
		Params: map[string]string{
			"escape":   "$",
			"meaning":  "variable_reference",
			"var_name": varName,
		},
	}, nameEnd, nil
}

// parseConditionalExpression parses %(test.true.false) and %number(test.true.false) expressions
func (p *Parser) parseConditionalExpression(ps1 string, pos int) (Token, int, error) {
	// Extract any leading number after %
	numberStart := pos + 1
	numberEnd := numberStart
	for numberEnd < len(ps1) && ps1[numberEnd] >= '0' && ps1[numberEnd] <= '9' {
		numberEnd++
	}

	// Check that we have the opening parenthesis
	if numberEnd >= len(ps1) || ps1[numberEnd] != '(' {
		return Token{Type: TokenLiteral, Content: "%"}, pos + 1, nil
	}

	number := ""
	if numberEnd > numberStart {
		number = ps1[numberStart:numberEnd]
	}

	// Find the matching closing parenthesis
	parenCount := 0
	closePos := -1

	for i := numberEnd + 1; i < len(ps1); i++ {
		char := ps1[i]
		if char == '(' {
			parenCount++
		} else if char == ')' {
			if parenCount == 0 {
				closePos = i
				break
			}
			parenCount--
		}
	}

	if closePos == -1 {
		if p.options.StrictMode {
			return Token{}, pos, fmt.Errorf("unclosed conditional expression")
		}
		return Token{Type: TokenLiteral, Content: "%("}, numberEnd + 1, nil
	}

	content := ps1[numberEnd+1 : closePos]

	// Parse the conditional content
	parts := p.parseConditionalParts(content)

	// Add the number if present
	if number != "" {
		parts["number"] = number
	}

	return Token{
		Type:    TokenCondition,
		Content: content,
		Params:  parts,
	}, closePos + 1, nil
}

// parseConditionalParts parses the parts of a conditional expression
func (p *Parser) parseConditionalParts(content string) map[string]string {
	params := make(map[string]string)

	if len(content) == 0 {
		return params
	}

	// Extract the test condition (first character after optional number)
	testStart := 0
	for testStart < len(content) && (content[testStart] >= '0' && content[testStart] <= '9') {
		testStart++
	}

	if testStart < len(content) {
		testChar := content[testStart]
		params["test"] = string(testChar)
		if testStart > 0 {
			params["number"] = content[:testStart]
		}

		// The remaining content after the test character
		remaining := content[testStart+1:]

		// The first character of remaining should be the separator
		if len(remaining) > 0 {
			separator := string(remaining[0])

			// Split the content after the separator
			parts := strings.Split(remaining[1:], separator)
			if len(parts) >= 2 {
				params["true_text"] = parts[0]
				params["false_text"] = parts[1]
			} else if len(parts) == 1 {
				params["true_text"] = parts[0]
			}
		}
	}

	return params
}

// parseSimplePercent parses simple % escapes like %n, %m, %~
func (p *Parser) parseSimplePercent(ps1 string, pos int) (Token, int, error) {
	if pos+1 >= len(ps1) {
		return Token{Type: TokenLiteral, Content: "%"}, pos + 1, nil
	}

	// Check for numbered escape sequences like %1~, %2~, %30<...<%~
	numberStart := pos + 1
	numberEnd := numberStart

	// Extract any leading digits
	for numberEnd < len(ps1) && ps1[numberEnd] >= '0' && ps1[numberEnd] <= '9' {
		numberEnd++
	}

	var number string
	if numberEnd > numberStart {
		number = ps1[numberStart:numberEnd]
	}

	// Check for special patterns like %30<...<%~
	if numberEnd < len(ps1) && ps1[numberEnd] == '<' {
		return p.parseTruncationSequence(ps1, pos, number)
	}

	// Check for D{...} date format
	if numberEnd < len(ps1) && ps1[numberEnd] == 'D' && numberEnd+1 < len(ps1) && ps1[numberEnd+1] == '{' {
		return p.parseDateFormatSequence(ps1, pos)
	}

	// Check for F{...} foreground color or K{...} background color
	if numberEnd < len(ps1) && (ps1[numberEnd] == 'F' || ps1[numberEnd] == 'K') &&
		numberEnd+1 < len(ps1) && ps1[numberEnd+1] == '{' {
		return p.parseZshColorSequence(ps1, pos)
	}

	// Get the actual escape character (after any numbers)
	if numberEnd >= len(ps1) {
		return Token{Type: TokenLiteral, Content: "%"}, pos + 1, nil
	}

	escape := ps1[numberEnd]
	var meaning string

	switch escape {
	case 'n':
		meaning = "username"
	case 'm':
		meaning = "hostname_short"
	case 'M':
		meaning = "hostname_full"
	case '~':
		meaning = "current_dir_tilde"
	case 'd', '/':
		meaning = "current_dir"
	case 'c', '.':
		meaning = "current_dir_tail"
	case 'h', '!':
		meaning = "history_number"
	case 'T':
		meaning = "time_24h"
	case 't', '@':
		meaning = "time_12h"
	case '*':
		meaning = "time_24h_seconds"
	case 'w':
		meaning = "date_day_dd"
	case 'W':
		meaning = "date_mm_dd_yy"
	case 'D':
		meaning = "date_yy_mm_dd"
	case '#':
		meaning = "privilege_indicator"
	case '?':
		meaning = "exit_status"
	case 'j':
		meaning = "job_count"
	case 'l':
		meaning = "tty"
	case 'L':
		meaning = "shell_level"
	case '_':
		meaning = "parser_state"
	case '^':
		meaning = "parser_state_reverse"
	case 'B':
		meaning = "start_bold"
	case 'b':
		meaning = "end_bold"
	case 'U':
		meaning = "start_underline"
	case 'S':
		meaning = "start_standout"
	case 's':
		meaning = "end_standout"
	case 'F':
		meaning = "start_foreground_color"
	case 'f':
		meaning = "end_foreground_color"
	case 'K':
		meaning = "start_background_color"
	case 'k':
		meaning = "end_background_color"
	case 'E':
		meaning = "terminal_clear_eol"
	case ' ':
		meaning = "literal_space"
	case 'y':
		meaning = "tty_device"
	case 'i':
		meaning = "script_line_number"
	case 'I':
		meaning = "source_line_number"
	case 'N':
		meaning = "script_function_name"
	case 'x':
		meaning = "source_file_name"
	case 'C':
		meaning = "current_dir_tail_no_tilde"
	case 'v':
		meaning = "version"
	case 'V':
		meaning = "release_level"
	case 'A':
		meaning = "locale_date"
	case 'g':
		meaning = "effective_gid"
	case 'G':
		meaning = "effective_group"
	case 'u':
		if number != "" {
			meaning = "user_defined_" + number
		} else {
			meaning = "end_underline"
		}
	default:
		if p.options.StrictMode {
			return Token{}, pos, fmt.Errorf("unknown escape sequence: %%%s%c", number, escape)
		}
		meaning = "unknown"
	}

	content := ps1[pos+1 : numberEnd+1]
	params := map[string]string{
		"escape":  string(escape),
		"meaning": meaning,
	}

	if number != "" {
		params["number"] = number
		// For numbered escapes, adjust the meaning
		switch escape {
		case '~', 'd', '/':
			meaning = "current_dir_truncated_" + number
		case 'c', '.':
			meaning = "current_dir_tail_" + number
		default:
			meaning = "numbered_" + meaning + "_" + number
		}
		params["meaning"] = meaning
	}

	return Token{
		Type:    TokenPercent,
		Content: content,
		Params:  params,
	}, numberEnd + 1, nil
}

// parseTruncationSequence parses truncation sequences like %30<...<%~
func (p *Parser) parseTruncationSequence(ps1 string, pos int, number string) (Token, int, error) {
	// Find the pattern %number<...<%escape or %number>...>%escape
	currentPos := pos + 1 + len(number) + 1 // Skip %number<

	// Find the second occurrence of the truncation character
	truncChar := ps1[pos+1+len(number)]
	dots := ""
	endPos := currentPos

	for endPos < len(ps1) {
		if ps1[endPos] == truncChar {
			// Found potential end, check if followed by %
			if endPos+1 < len(ps1) && ps1[endPos+1] == '%' {
				dots = ps1[currentPos:endPos]
				// Parse the following escape sequence
				if endPos+2 < len(ps1) {
					escapeChar := ps1[endPos+2]
					endTokenPos := endPos + 3

					// Check for trailing %<< or %>>
					if endTokenPos+2 < len(ps1) && ps1[endTokenPos] == '%' &&
						ps1[endTokenPos+1] == truncChar && ps1[endTokenPos+2] == truncChar {
						endTokenPos += 3
					}

					content := ps1[pos+1 : endTokenPos]

					return Token{
						Type:    TokenPercent,
						Content: content,
						Params: map[string]string{
							"escape":     string(escapeChar),
							"meaning":    "truncated_" + string(escapeChar),
							"number":     number,
							"truncation": string(truncChar),
							"dots":       dots,
						},
					}, endTokenPos, nil
				}
				break
			}
		}
		endPos++
	}

	// If we couldn't parse it as a truncation, treat as unknown
	return Token{
		Type:    TokenPercent,
		Content: ps1[pos+1 : pos+2],
		Params: map[string]string{
			"escape":  string(ps1[pos+1]),
			"meaning": "unknown",
		},
	}, pos + 2, nil
}

// parseDateFormatSequence parses %D{...} date format sequences
func (p *Parser) parseDateFormatSequence(ps1 string, pos int) (Token, int, error) {
	// Find the opening brace
	openPos := pos + 2 // Skip %D
	if openPos >= len(ps1) || ps1[openPos] != '{' {
		return Token{Type: TokenLiteral, Content: "%D"}, pos + 2, nil
	}

	// Find the closing brace
	closePos := openPos + 1
	braceCount := 1

	for closePos < len(ps1) && braceCount > 0 {
		switch ps1[closePos] {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
		closePos++
	}

	if braceCount > 0 {
		// Unclosed brace
		return Token{Type: TokenLiteral, Content: "%D"}, pos + 2, nil
	}

	formatString := ps1[openPos+1 : closePos-1]

	return Token{
		Type:    TokenPercent,
		Content: ps1[pos+1 : closePos],
		Params: map[string]string{
			"escape":  "D",
			"meaning": "date_format",
			"format":  formatString,
		},
	}, closePos, nil
}

// parseZshColorSequence parses %F{color} and %K{color} sequences
func (p *Parser) parseZshColorSequence(ps1 string, pos int) (Token, int, error) {
	// Find the opening brace
	colorType := "foreground"

	// Skip any leading numbers
	numberStart := pos + 1
	numberEnd := numberStart
	for numberEnd < len(ps1) && ps1[numberEnd] >= '0' && ps1[numberEnd] <= '9' {
		numberEnd++
	}

	if numberEnd >= len(ps1) {
		return Token{Type: TokenLiteral, Content: "%"}, pos + 1, nil
	}

	escapeChar := ps1[numberEnd]
	if escapeChar == 'K' {
		colorType = "background"
	}

	openPos := numberEnd + 1
	if openPos >= len(ps1) || ps1[openPos] != '{' {
		// Not a color sequence after all
		return Token{Type: TokenLiteral, Content: string(ps1[pos:openPos])}, openPos, nil
	}

	// Find the closing brace
	closePos := openPos + 1
	braceCount := 1

	for closePos < len(ps1) && braceCount > 0 {
		switch ps1[closePos] {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
		closePos++
	}

	if braceCount > 0 {
		// Unclosed brace
		return Token{Type: TokenLiteral, Content: string(ps1[pos : openPos+1])}, openPos + 1, nil
	}

	colorValue := ps1[openPos+1 : closePos-1]

	return Token{
		Type:    TokenColorSeq,
		Content: colorValue,
		Params: map[string]string{
			"escape": string(escapeChar),
			"type":   colorType,
			"color":  colorValue,
		},
	}, closePos, nil
}

// parseCommandToken parses $(command) and ${variable} substitutions
func (p *Parser) parseCommandToken(ps1 string, pos int) (Token, int, error) {
	if pos+1 >= len(ps1) {
		return Token{Type: TokenLiteral, Content: "$"}, pos + 1, nil
	}

	openChar := ps1[pos+1]
	var closeChar byte
	var startPos int

	switch openChar {
	case '(':
		closeChar = ')'
		startPos = pos + 2
	case '{':
		closeChar = '}'
		startPos = pos + 2
	default:
		return Token{Type: TokenLiteral, Content: "$"}, pos + 1, nil
	}

	// Find the closing character
	count := 0
	closePos := -1

	for i := startPos; i < len(ps1); i++ {
		char := ps1[i]
		if char == openChar {
			count++
		} else if char == closeChar {
			if count == 0 {
				closePos = i
				break
			}
			count--
		}
	}

	if closePos == -1 {
		if p.options.StrictMode {
			return Token{}, pos, fmt.Errorf("unclosed command/variable substitution")
		}
		return Token{Type: TokenLiteral, Content: string(ps1[pos : pos+2])}, pos + 2, nil
	}

	command := ps1[startPos:closePos]
	tokenType := "command"
	if openChar == '{' {
		tokenType = "variable"
	}

	return Token{
		Type:    TokenCommand,
		Content: command,
		Params: map[string]string{
			"command": command,
			"type":    tokenType,
		},
	}, closePos + 1, nil
}

// parseLiteralToken parses regular text
func (p *Parser) parseLiteralToken(ps1 string, pos int) (Token, int, error) {
	start := pos

	// Find the next special character
	for pos < len(ps1) {
		char := ps1[pos]
		if char == '%' || char == '$' {
			break
		}
		pos++
	}

	if pos == start {
		// Single character
		return Token{Type: TokenLiteral, Content: string(ps1[start])}, start + 1, nil
	}

	content := ps1[start:pos]

	// Handle escape sequences in literals
	content = unescapeString(content)

	return Token{Type: TokenLiteral, Content: content}, pos, nil
}

// parseSimpleVariable parses $VAR format variables
func (p *Parser) parseSimpleVariable(ps1 string, pos int) (Token, int, error) {
	if pos+1 >= len(ps1) || ps1[pos] != '$' {
		return Token{Type: TokenLiteral, Content: "$"}, pos + 1, nil
	}

	start := pos + 1
	end := start

	// Find the end of the variable name
	for end < len(ps1) && isVarChar(ps1[end]) {
		end++
	}

	if end == start {
		// No variable name after $
		return Token{Type: TokenLiteral, Content: "$"}, pos + 1, nil
	}

	varName := ps1[start:end]

	return Token{
		Type:    TokenCommand,
		Content: varName,
		Params: map[string]string{
			"command": varName,
			"type":    "variable",
		},
	}, end, nil
}
