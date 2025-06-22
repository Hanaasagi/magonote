package internal

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
func isCommonWord(word string) bool {
	// return commonWordsMap[word]
	if len(word) < 3 {
		return true
	}
	return false
}
