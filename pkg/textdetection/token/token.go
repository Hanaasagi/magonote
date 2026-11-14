package token

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/rivo/uniseg"
)

type Chunk struct {
	Text               string
	Span               screen.Span
	ByteStart, ByteEnd int
}

type Word struct {
	Text               string
	Span               screen.Span
	ByteStart, ByteEnd int
}

// Chunks returns maximal non-whitespace regions on one Screen line.
func Chunks(s *screen.Screen, line int) []Chunk {
	if line < 0 || line >= len(s.Lines) {
		return nil
	}
	plain := s.Lines[line].Plain
	var chunks []Chunk
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		chunks = append(chunks, Chunk{
			Text: plain[start:end], Span: s.SpanOf(line, start, end), ByteStart: start, ByteEnd: end,
		})
		start = -1
	}

	graphemes := uniseg.NewGraphemes(plain)
	for graphemes.Next() {
		byteStart, _ := graphemes.Positions()
		r, _ := utf8.DecodeRuneInString(graphemes.Str())
		if unicode.IsSpace(r) {
			flush(byteStart)
		} else if start < 0 {
			start = byteStart
		}
	}
	flush(len(plain))
	return chunks
}

// Words splits a chunk on punctuation while retaining characters commonly
// found in paths, URLs, identifiers, versions, and addresses.
func Words(chunk Chunk) []Word {
	var words []Word
	start := -1
	startCol := 0
	col := 0
	flush := func(end int) {
		if start < 0 {
			return
		}
		words = append(words, Word{
			Text:      chunk.Text[start:end],
			Span:      screen.Span{Line: chunk.Span.Line, StartCol: startCol, EndCol: chunk.Span.StartCol + col},
			ByteStart: chunk.ByteStart + start, ByteEnd: chunk.ByteStart + end,
		})
		start = -1
	}

	graphemes := uniseg.NewGraphemes(chunk.Text)
	seenGrapheme := false
	for graphemes.Next() {
		byteStart, _ := graphemes.Positions()
		text := graphemes.Str()
		r, _ := utf8.DecodeRuneInString(text)
		separator := !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && !strings.ContainsRune("/._-:@~$%", r)
		if separator {
			flush(byteStart)
		} else {
			if start < 0 {
				start = byteStart
				startCol = chunk.Span.StartCol + col
			}
		}
		w := uniseg.StringWidth(text)
		if w == 0 && seenGrapheme {
			// Zero-width character merged with previous cell; no column advance
		} else {
			col += screen.GraphemeWidth(text)
		}
		seenGrapheme = true
	}
	flush(len(chunk.Text))
	return words
}

func IsSeparatorLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	for _, r := range line {
		if unicode.IsSpace(r) || strings.ContainsRune("-=_~┈─━│┼+|^:┌┐└┘├┤┬┴╭╮╰╯═║╬╞╡╦╩", r) {
			continue
		}
		return false
	}
	return true
}
