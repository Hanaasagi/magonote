package ps1parser

import (
	"fmt"
	"strings"
)

// Character classification utilities for parsing

// isVarStartChar checks if a character can start a variable name.
// Variable names can start with letters or underscore.
func isVarStartChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// isVarChar checks if a character can be part of a variable name.
// Variable names can contain letters, numbers, or underscore.
func isVarChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// isAlphanumeric checks if a character is alphanumeric.
func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// String processing utilities

// unescapeString handles escape sequences in literal strings.
// It processes ZSH-specific escape sequences and common escape sequences.
func unescapeString(s string) string {
	// Handle ZSH-specific escape sequences first
	// In ZSH PS1, \\\n means backslash-escaped-backslash followed by newline
	// This appears in terminal as just newline (no visible backslash)
	// We need to handle this before \\n processing
	s = strings.ReplaceAll(s, "\\\n", "\n") // \n -> \n

	// Handle common escape sequences (after handling the above)
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\\\", "\\")

	// Handle hex escapes like \x1b
	i := 0
	result := make([]byte, 0, len(s))
	for i < len(s) {
		if i+3 < len(s) && s[i] == '\\' && s[i+1] == 'x' {
			// Try to parse hex escape
			hex := s[i+2 : i+4]
			var val byte
			if n, err := fmt.Sscanf(hex, "%x", &val); n == 1 && err == nil {
				result = append(result, val)
				i += 4
				continue
			}
		}
		result = append(result, s[i])
		i++
	}

	return string(result)
}
