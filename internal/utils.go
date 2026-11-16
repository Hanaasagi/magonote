package internal

import (
	"os"
	"strings"
)

func IsDebugMode() bool {
	isDebug := strings.ToLower(os.Getenv("MAGONOTE_DEBUG"))
	if isDebug == "true" || isDebug == "1" {
		return true
	}
	return false
}

func abs[T int | int8 | int16 | int32 | int64 | float32 | float64](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

// var commonWordsMap = map[string]bool{
// 	"path": true, "and": true, "or": true, "but": true, "not": true, "with": true,
// 	"from": true, "for": true, "this": true, "that": true, "all": true,
// 	"any": true, "some": true, "can": true, "will": true, "would": true,
// 	"could": true, "should": true, "must": true, "may": true, "might": true,
// }

// isCommonWord checks if a word is too common to be useful as a match
// func isCommonWord(word string) bool {
// 	// return commonWordsMap[word]
// 	if len(word) < 3 {
// 		return true
// 	}
// 	return false
// }

// isLikelyFilePath checks if the given text looks like a file path.
func isLikelyFilePath(text string) bool {
	if len(text) < 2 {
		return false
	}

	// Skip URLs and other protocols
	protocols := []string{"http://", "https://", "ftp://", "ssh://"}
	for _, protocol := range protocols {
		if strings.HasPrefix(text, protocol) {
			return false
		}
	}

	// Check for path separators
	if strings.ContainsAny(text, "/\\") {
		return true
	}

	// Check for file extension
	if strings.Contains(text, ".") {
		parts := strings.Split(text, ".")
		if len(parts) > 1 && len(parts[len(parts)-1]) <= 10 {
			return true
		}
	}

	return false
}
