package internal

import (
	"fmt"
	"strings"
)

var builtinAlphabets = []struct {
	name    string
	letters string
}{
	{"numeric", "1234567890"},
	{"abcd", "abcd"},
	{"qwerty", "asdfqwerzxcvjklmiuopghtybn"},
	{"qwerty-homerow", "asdfjklgh"},
	{"qwerty-left-hand", "asdfqwerzcxv"},
	{"qwerty-right-hand", "jkluiopmyhn"},
	{"azerty", "qsdfazerwxcvjklmuiopghtybn"},
	{"azerty-homerow", "qsdfjkmgh"},
	{"azerty-left-hand", "qsdfazerwxcv"},
	{"azerty-right-hand", "jklmuiophyn"},
	{"qwertz", "asdfqweryxcvjkluiopmghtzbn"},
	{"qwertz-homerow", "asdfghjkl"},
	{"qwertz-left-hand", "asdfqweryxcv"},
	{"qwertz-right-hand", "jkluiopmhzn"},
	{"dvorak", "aoeuqjkxpyhtnsgcrlmwvzfidb"},
	{"dvorak-homerow", "aoeuhtnsid"},
	{"dvorak-left-hand", "aoeupqjkyix"},
	{"dvorak-right-hand", "htnsgcrlmwvz"},
	{"colemak", "arstqwfpzxcvneioluymdhgjbk"},
	{"colemak-homerow", "arstneiodh"},
	{"colemak-left-hand", "arstqwfpzxcv"},
	{"colemak-right-hand", "neioluymjhk"},
}

type Alphabet struct {
	letters []string
}

func NewAlphabet(letters string) *Alphabet {
	return &Alphabet{letters: strings.Split(letters, "")}
}

func NewBuiltinAlphabet(name string) (*Alphabet, error) {
	for _, alphabet := range builtinAlphabets {
		if alphabet.name == name {
			return NewAlphabet(alphabet.letters), nil
		}
	}
	return nil, fmt.Errorf("unknown alphabet: %s", name)
}

// Len returns the number of usable hint characters.
func (a *Alphabet) Len() int { return len(a.letters) }

// Exclude returns an alphabet with the given keys removed, so action-group
// prefix keys can never appear as hint characters.
func (a *Alphabet) Exclude(keys []string) *Alphabet {
	if len(keys) == 0 {
		return a
	}
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	var letters []string
	for _, l := range a.letters {
		if !drop[l] {
			letters = append(letters, l)
		}
	}
	return &Alphabet{letters: letters}
}

func (a *Alphabet) Hints(matches int) []string {
	if matches <= 0 {
		return nil
	}

	lettersCount := len(a.letters)
	if lettersCount == 0 {
		return nil
	}

	// Start with single letters
	expansion := make([]string, lettersCount)
	copy(expansion, a.letters)

	var expanded []string

	for len(expansion) > 0 && len(expansion)+len(expanded) < matches {
		// Take the last element from expansion
		prefix := expansion[len(expansion)-1]
		expansion = expansion[:len(expansion)-1]

		limit := matches - len(expansion) - len(expanded)
		var subExpansion []string
		for i := 0; i < lettersCount && i < limit; i++ {
			subExpansion = append(subExpansion, prefix+a.letters[i])
		}

		// Insert at beginning of expanded
		expanded = append(subExpansion, expanded...)
	}

	// Limit expansion if we have too many
	if len(expansion) > matches-len(expanded) {
		expansion = expansion[:matches-len(expanded)]
	}

	result := append(expansion, expanded...)
	return result
}
