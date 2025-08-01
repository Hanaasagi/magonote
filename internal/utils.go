package internal

import (
	"os"
	"strings"

	"github.com/mattn/go-runewidth"
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

func isTextNoise(text string) bool {
	if len(text) == 0 {
		return true
	}

	width := runewidth.StringWidth(text)
	if width < 3 {
		return true
	}

	isAllSame := true

	first := text[0]
	for i := 1; i < len(text); i++ {
		if text[i] != first {
			isAllSame = false
			break
		}
	}
	return isAllSame
}
