package detect

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

type wordDetector struct{}

func NewWordDetector() Detector   { return wordDetector{} }
func (wordDetector) Name() string { return "words" }

func (wordDetector) Detect(ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for _, chunks := range ctx.Chunks {
		for _, chunk := range chunks {
			for _, word := range token.Words(chunk) {
				if word.Span.EndCol-word.Span.StartCol < 3 || !hasLetterOrDigit(word.Text) {
					continue
				}
				candidates = append(candidates, entity.Candidate{
					Kind: entity.KindWord, Text: word.Text, Span: word.Span,
					Class: entity.ClassWord, Source: "words", Confidence: 0.1,
				})
			}
		}
	}
	return candidates
}

func hasLetterOrDigit(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
}

func repeatedRune(text string) bool {
	first, _ := utf8.DecodeRuneInString(text)
	for _, r := range text {
		if r != first {
			return false
		}
	}
	return true
}
