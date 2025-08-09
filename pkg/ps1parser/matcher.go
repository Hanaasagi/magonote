package ps1parser

import (
	"fmt"
	"regexp"
	"strings"
)

// NewMatcher creates a new matcher from a parsed PS1.
// It compiles the PS1 tokens into a regex pattern that can be used to find prompts in text.
func NewMatcher(parsedPS1 *ParsedPS1, options MatchOptions) (*Matcher, error) {
	pattern, err := compilePattern(parsedPS1, options)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern: %w", err)
	}

	return &Matcher{
		pattern: pattern,
		options: options,
	}, nil
}

// Match finds all prompt matches in the given text.
// It returns a slice of MatchResult containing position and captured groups.
func (m *Matcher) Match(text string) ([]MatchResult, error) {
	// Preprocess text if needed
	processedText := m.preprocessText(text)

	// Find all matches
	matches := m.pattern.regex.FindAllStringSubmatch(processedText, -1)
	indices := m.pattern.regex.FindAllStringSubmatchIndex(processedText, -1)

	var results []MatchResult

	for i, match := range matches {
		if len(indices) <= i {
			continue
		}

		// Convert byte indices to line/column positions
		startPos := m.byteIndexToPosition(processedText, indices[i][0])
		endPos := m.byteIndexToPosition(processedText, indices[i][1])

		// Extract named groups
		groups := make(map[string]string)
		subexpNames := m.pattern.regex.SubexpNames()
		for j, name := range subexpNames {
			if name != "" && j < len(match) {
				groups[name] = match[j]
			}
		}

		results = append(results, MatchResult{
			Position: Position{
				StartLine: startPos.StartLine,
				StartCol:  startPos.StartCol,
				EndLine:   endPos.StartLine,
				EndCol:    endPos.StartCol,
			},
			Matched: match[0],
			Groups:  groups,
		})
	}

	return results, nil
}

// preprocessText prepares text for matching by removing/normalizing certain elements.
// It handles ANSI escape sequences and whitespace normalization based on match options.
func (m *Matcher) preprocessText(text string) string {
	processed := text

	if m.options.IgnoreColors {
		// Remove ANSI color codes and other escape sequences
		// Handle standard color codes
		ansiColorRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
		processed = ansiColorRegex.ReplaceAllString(processed, "")

		// Handle additional ANSI sequences like cursor positioning, clearing, etc.
		ansiOtherRegex := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
		processed = ansiOtherRegex.ReplaceAllString(processed, "")

		// Handle title sequences like \x1b]0;title\a
		titleRegex := regexp.MustCompile(`\x1b\]0;[^\a]*\a`)
		processed = titleRegex.ReplaceAllString(processed, "")

		// Handle other escape sequences like \x1b(B
		escapeRegex := regexp.MustCompile(`\x1b\([A-Za-z0-9]`)
		processed = escapeRegex.ReplaceAllString(processed, "")
	}

	if m.options.IgnoreSpacing {
		// Normalize whitespace
		spaceRegex := regexp.MustCompile(`\s+`)
		processed = spaceRegex.ReplaceAllString(processed, " ")
	}

	return processed
}

// byteIndexToPosition converts a byte index to line/column position
func (m *Matcher) byteIndexToPosition(text string, byteIndex int) Position {
	lines := strings.Split(text[:byteIndex], "\n")
	line := len(lines) - 1
	col := len(lines[line])

	return Position{
		StartLine: line,
		StartCol:  col,
	}
}

// compilePattern converts a parsed PS1 into a regex pattern.
// It processes each token and combines them into a single regex that can match prompts.
func compilePattern(parsedPS1 *ParsedPS1, options MatchOptions) (*MatchPattern, error) {
	var patternParts []string

	for i, token := range parsedPS1.Tokens {
		part, err := tokenToRegexPart(token, options)
		if err != nil {
			return nil, fmt.Errorf("failed to convert token to regex: %w", err)
		}

		// Add the token pattern
		if part != "" {
			patternParts = append(patternParts, part)
		}

		// Add optional color codes and spacing between tokens (except after the last token)
		if i < len(parsedPS1.Tokens)-1 && part != "" {
			nextToken := parsedPS1.Tokens[i+1]
			// Check if next token is a literal space - if so, be more flexible
			if nextToken.Type == TokenLiteral && strings.TrimSpace(nextToken.Content) == "" {
				// Next token is whitespace, allow more flexible spacing
				if options.IgnoreColors {
					// Allow flexible whitespace since colors are stripped
					patternParts = append(patternParts, `\s*`)
				} else {
					// Allow optional color codes and flexible spacing
					patternParts = append(patternParts, `(?:\x1b\[[0-9;]*m|\s)*`)
				}
			} else {
				// Normal token separation
				if options.IgnoreColors {
					// No color codes in the pattern since we strip them from input
				} else {
					// Allow optional color codes
					patternParts = append(patternParts, `(?:\x1b\[[0-9;]*m)*`)
				}
			}
		}
	}

	// Join all parts
	pattern := strings.Join(patternParts, "")

	// Add line boundary handling if needed
	if options.MaxLineSpan > 0 {
		// Limit to specified number of lines - apply restriction
		pattern = fmt.Sprintf("(?ms)^%s$", pattern)
	} else {
		// Allow multiline matching
		pattern = fmt.Sprintf("(?ms)%s", pattern)
	}

	// Compile regex with appropriate flags
	var flags string
	if !options.CaseSensitive {
		flags = "(?i)"
	}

	finalPattern := flags + pattern
	regex, err := regexp.Compile(finalPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex pattern '%s': %w", finalPattern, err)
	}

	return &MatchPattern{
		regex:   regex,
		tokens:  parsedPS1.Tokens,
		options: options,
	}, nil
}

// tokenToRegexPart converts a single token to a regex pattern part
func tokenToRegexPart(token Token, options MatchOptions) (string, error) {
	switch token.Type {
	case TokenLiteral:
		// Handle whitespace more flexibly
		if strings.TrimSpace(token.Content) == "" && token.Content != "" {
			// This is whitespace - be flexible about it
			if options.IgnoreSpacing {
				return `\s*`, nil
			} else {
				return `\s+`, nil // At least one whitespace
			}
		}

		// Handle mixed content (whitespace + non-whitespace)
		content := token.Content
		if strings.Contains(content, " ") || strings.Contains(content, "\t") || strings.Contains(content, "\n") {
			// Break down mixed content
			parts := []string{}
			i := 0
			for i < len(content) {
				if content[i] == ' ' || content[i] == '\t' || content[i] == '\n' {
					// Found whitespace
					j := i
					for j < len(content) && (content[j] == ' ' || content[j] == '\t' || content[j] == '\n') {
						j++
					}
					if options.IgnoreSpacing {
						parts = append(parts, `\s*`)
					} else {
						parts = append(parts, `\s+`)
					}
					i = j
				} else {
					// Found non-whitespace
					j := i
					for j < len(content) && content[j] != ' ' && content[j] != '\t' && content[j] != '\n' {
						j++
					}
					parts = append(parts, regexp.QuoteMeta(content[i:j]))
					i = j
				}
			}
			return strings.Join(parts, ""), nil
		}

		// Escape special regex characters for non-whitespace literals
		// But handle newlines specially since they've been unescaped from \\\n
		tokenContent := token.Content
		if strings.Contains(tokenContent, "\n") {
			// Replace literal newlines with flexible whitespace matching
			tokenContent = strings.ReplaceAll(tokenContent, "\n", `\s*`)
		}
		return regexp.QuoteMeta(tokenContent), nil

	case TokenPercent:
		return percentToRegexPart(token, options)

	case TokenColorSeq:
		if options.IgnoreColors {
			return "", nil // Ignore color sequences
		}
		// Match any ANSI escape sequence
		return `\x1b\[[0-9;]*m`, nil

	case TokenCondition:
		return conditionalToRegexPart(token, options)

	case TokenCommand:
		return commandToRegexPart(token, options)

	default:
		return "", fmt.Errorf("unknown token type: %d", token.Type)
	}
}

// percentToRegexPart converts percent escape tokens to regex
func percentToRegexPart(token Token, options MatchOptions) (string, error) {
	meaning := token.Params["meaning"]

	switch meaning {
	case "username":
		return `(?P<username>\w+)`, nil
	case "hostname_short", "hostname_full":
		return `(?P<hostname>[\w\.-]+)`, nil
	case "current_dir", "current_dir_tilde":
		return `(?P<current_dir>[^\s]+)`, nil
	case "current_dir_tail":
		return `(?P<current_dir_tail>[^\s/]+)`, nil
	case "history_number":
		return `(?P<history_number>\d+)`, nil
	case "time_24h", "time_12h":
		return `(?P<time>\d{1,2}:\d{2}(?::\d{2})?(?:\s*[AP]M)?)`, nil
	case "time_24h_seconds":
		return `(?P<time>\d{1,2}:\d{2}:\d{2})`, nil
	case "date_day_dd", "date_mm_dd_yy", "date_yy_mm_dd":
		return `(?P<date>\d{1,2}[-/]\d{1,2}[-/]\d{2,4})`, nil
	case "privilege_indicator":
		return `(?P<privilege>[#%])`, nil
	case "exit_status":
		return `(?P<exit_status>\d+)`, nil
	case "job_count":
		return `(?P<job_count>\d+)`, nil
	case "tty":
		return `(?P<tty>\w+)`, nil
	case "shell_level":
		return `(?P<shell_level>\d+)`, nil
	case "parser_state", "parser_state_reverse":
		return `(?P<parser_state>\w*)`, nil
	case "start_bold", "end_bold", "start_underline", "end_underline",
		"start_standout", "end_standout", "start_foreground_color",
		"end_foreground_color", "start_background_color", "end_background_color":
		if options.IgnoreColors {
			return "", nil
		}
		return `\x1b\[[0-9;]*m`, nil
	default:
		// For unknown escapes, match any non-whitespace
		return `\S*`, nil
	}
}

// conditionalToRegexPart converts conditional expressions to regex
func conditionalToRegexPart(token Token, options MatchOptions) (string, error) {
	trueText := token.Params["true_text"]
	falseText := token.Params["false_text"]

	// Create a regex that matches either the true or false branch
	var alternatives []string

	// Always add true branch, even if empty
	if trueText != "" {
		// Parse the true text as PS1 and convert to regex
		parser := NewParser(ParserOptions{})
		trueParsed, err := parser.Parse(trueText)
		if err != nil {
			return "", fmt.Errorf("failed to parse true branch: %w", err)
		}

		var trueParts []string
		for _, subToken := range trueParsed.Tokens {
			part, err := tokenToRegexPart(subToken, options)
			if err != nil {
				return "", fmt.Errorf("failed to convert true branch token: %w", err)
			}
			trueParts = append(trueParts, part)
		}
		alternatives = append(alternatives, strings.Join(trueParts, ""))
	} else {
		// True branch is empty
		alternatives = append(alternatives, "")
	}

	// Always add false branch, even if empty
	if falseText != "" {
		// Parse the false text as PS1 and convert to regex
		parser := NewParser(ParserOptions{})
		falseParsed, err := parser.Parse(falseText)
		if err != nil {
			return "", fmt.Errorf("failed to parse false branch: %w", err)
		}

		var falseParts []string
		for _, subToken := range falseParsed.Tokens {
			part, err := tokenToRegexPart(subToken, options)
			if err != nil {
				return "", fmt.Errorf("failed to convert false branch token: %w", err)
			}
			falseParts = append(falseParts, part)
		}
		alternatives = append(alternatives, strings.Join(falseParts, ""))
	} else {
		// False branch is empty
		alternatives = append(alternatives, "")
	}

	// If we have alternatives, create a group
	if len(alternatives) > 0 {
		// Check if any alternative is empty - if so, make the whole thing optional
		hasEmpty := false
		for _, alt := range alternatives {
			if strings.TrimSpace(alt) == "" {
				hasEmpty = true
				break
			}
		}

		if hasEmpty {
			// Remove empty alternatives and make the whole group optional
			nonEmptyAlts := []string{}
			for _, alt := range alternatives {
				if strings.TrimSpace(alt) != "" {
					nonEmptyAlts = append(nonEmptyAlts, alt)
				}
			}

			if len(nonEmptyAlts) > 0 {
				return fmt.Sprintf("(?:%s)?", strings.Join(nonEmptyAlts, "|")), nil
			} else {
				return "", nil // All alternatives were empty
			}
		}

		return fmt.Sprintf("(?:%s)", strings.Join(alternatives, "|")), nil
	}

	// If no alternatives, match empty or any reasonable text
	return ".*?", nil
}

// commandToRegexPart converts command substitution to regex
func commandToRegexPart(token Token, options MatchOptions) (string, error) {
	command := token.Params["command"]
	tokenType := token.Params["type"] // "command" or "variable"

	// Create a named group for the command output
	groupName := strings.ReplaceAll(command, " ", "_")
	groupName = regexp.MustCompile(`[^\w]`).ReplaceAllString(groupName, "_")

	// Handle variables (${var}) with specific patterns
	if tokenType == "variable" {
		switch command {
		case "ret_status":
			// Return status often displays as symbols
			return `(?P<ret_status>[^\s]+)`, nil

		case "_LIBERTY":
			// Bureau theme variable that becomes a symbol
			return `(?P<_LIBERTY>[^\s]+)`, nil
		case "ZSH_THEME_CLOUD_PREFIX":
			// Cloud theme prefix that typically expands to ☁ or similar symbol
			return `(?P<ZSH_THEME_CLOUD_PREFIX>[^\s]+)`, nil
		case "user", "host", "pwd":
			// Common variables
			return fmt.Sprintf(`(?P<%s>[^\s@]+)`, groupName), nil
		case "time":
			// Time variable
			return `(?P<time>\d{1,2}:\d{2}(?::\d{2})?)`, nil
		case "TotalSize", "suffix":
			// Size-related variables (like in humza theme)
			return fmt.Sprintf(`(?P<%s>[^\s\]]+)`, groupName), nil
		case "smiley":
			// Emoticon variables (could be Unicode)
			return fmt.Sprintf(`(?P<%s>[^\s]+)`, groupName), nil
		case "vcs_info_msg_0_", "retcode":
			// VCS and return code info
			return fmt.Sprintf(`(?P<%s>[^\s>]*)?`, groupName), nil
		default:
			// Handle PR_ variables and other special patterns
			if strings.HasPrefix(command, "PR_") {
				// These are often ANSI codes or empty
				return fmt.Sprintf(`(?P<%s>.*?)`, groupName), nil
			}
			if strings.Contains(command, "prompt") || strings.Contains(command, "info") {
				// Prompt functions are often empty or short
				return fmt.Sprintf(`(?P<%s>[^\s]*)?`, groupName), nil
			}
			if strings.Contains(command, "size") || strings.Contains(command, "Size") {
				// Size-related variables
				return fmt.Sprintf(`(?P<%s>[^\s\]]+)`, groupName), nil
			}
			// Generic variable - could be anything
			return fmt.Sprintf(`(?P<%s>[^\s]*)?`, groupName), nil
		}
	}

	// Handle command substitutions $(cmd)
	switch {
	case strings.Contains(command, "git"):
		// Git related commands usually output branch names, status info
		return fmt.Sprintf(`(?P<%s>[^\s]*(?:\s+[^\s]*)*)?`, groupName), nil
	case strings.Contains(command, "time"):
		// Time commands
		return `(?P<command_time>\d+m\d+s)?`, nil
	case command == "toon":
		// Apple logo or other special characters
		return `(?P<toon>[^\s]*)`, nil
	default:
		// Generic command output - could be anything or nothing
		return fmt.Sprintf(`(?P<%s>[^\n]*)?`, groupName), nil
	}
}
