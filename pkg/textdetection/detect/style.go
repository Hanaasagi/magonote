package detect

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

type styleDetector struct{}

func NewStyleDetector() Detector   { return styleDetector{} }
func (styleDetector) Name() string { return "style" }

func (styleDetector) Detect(ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for line := range ctx.Screen.Lines {
		cells := ctx.Screen.Lines[line].Cells
		candidates = append(candidates, hyperlinkCandidates(cells, line)...)
		words := lineWords(ctx, line)
		for start, col := 0, 0; start < len(cells); {
			style := cells[start].Style
			end := start + 1
			for end < len(cells) && cells[end].Style == style {
				end++
			}
			if style != (screen.Style{}) && style.Attr&screen.AttrHidden == 0 {
				chunks := styledChunks(cells[start:end], line, col)
				for _, chunk := range chunks {
					// A highlight is a relevance signal, not a selection boundary:
					// snap the styled run outward to the words it covers so a
					// colored sub-token fragment (e.g. a grep match inside a pod
					// name) selects the whole token instead of suppressing it.
					span := expandToWords(words, chunk.span)
					text := spanText(ctx, line, span)
					if validStyledChunk(text, span, len(chunks)) {
						candidates = append(candidates, entity.Candidate{
							Kind: entity.KindStyled, Text: text, Span: span,
							Class: entity.ClassStructure, Source: "style", Confidence: styleConfidence(style),
						})
					}
				}
			}
			for _, cell := range cells[start:end] {
				col += int(cell.Width)
			}
			start = end
		}
	}
	return candidates
}

// lineWords collects the word tokens of a line, used to snap styled runs to
// token boundaries.
func lineWords(ctx *Context, line int) []token.Word {
	var words []token.Word
	for _, chunk := range ctx.Chunks[line] {
		words = append(words, token.Words(chunk)...)
	}
	return words
}

// expandToWords snaps span outward to the minimal range covering every word it
// intersects. A run equal to a token is unchanged; a run spanning several words
// keeps them together; a run with no intersecting word is returned as-is.
func expandToWords(words []token.Word, span screen.Span) screen.Span {
	out := span
	found := false
	for _, w := range words {
		if w.Span.Line != span.Line || w.Span.StartCol >= span.EndCol || span.StartCol >= w.Span.EndCol {
			continue
		}
		if !found {
			out, found = w.Span, true
			continue
		}
		if w.Span.StartCol < out.StartCol {
			out.StartCol = w.Span.StartCol
		}
		if w.Span.EndCol > out.EndCol {
			out.EndCol = w.Span.EndCol
		}
	}
	return out
}

func spanText(ctx *Context, line int, span screen.Span) string {
	byteStart := ctx.Screen.ByteOf(line, span.StartCol)
	byteEnd := ctx.Screen.ByteOf(line, span.EndCol)
	return ctx.Screen.Lines[line].Plain[byteStart:byteEnd]
}

func hyperlinkCandidates(cells []screen.Cell, line int) []entity.Candidate {
	var candidates []entity.Candidate
	for start, col := 0, 0; start < len(cells); {
		link := cells[start].Style.Hyperlink
		hidden := cells[start].Style.Attr&screen.AttrHidden != 0
		end := start + 1
		// Split runs at hidden-attribute boundaries so a link label mixing
		// hidden and visible text still offers its visible portion: a run
		// starting hidden would suppress the visible cells, while a run
		// starting visible would span hidden cells and be discarded by the
		// exclusion pipeline.
		for end < len(cells) && cells[end].Style.Hyperlink == link &&
			(cells[end].Style.Attr&screen.AttrHidden != 0) == hidden {
			end++
		}
		if link != "" && !hidden {
			startCol, endCol := col, col
			for _, cell := range cells[start:end] {
				endCol += int(cell.Width)
			}
			first, last := start, end
			for first < last && isSpaceCell(cells[first]) {
				startCol += int(cells[first].Width)
				first++
			}
			for last > first && isSpaceCell(cells[last-1]) {
				endCol -= int(cells[last-1].Width)
				last--
			}
			if endCol > startCol {
				candidates = append(candidates, entity.Candidate{
					Kind: entity.KindURL, Text: link, Span: screen.Span{Line: line, StartCol: startCol, EndCol: endCol},
					Class: entity.ClassSemantic, Source: "style", Confidence: 0.95,
				})
			}
		}
		for _, cell := range cells[start:end] {
			col += int(cell.Width)
		}
		start = end
	}
	return candidates
}

func isSpaceCell(cell screen.Cell) bool {
	r, _ := utf8.DecodeRuneInString(cell.Grapheme)
	return unicode.IsSpace(r)
}

type styledChunk struct {
	text string
	span screen.Span
}

func styledChunks(cells []screen.Cell, line, offset int) []styledChunk {
	var chunks []styledChunk
	start := -1
	col, startCol := offset, offset
	var text strings.Builder
	flush := func() {
		if start >= 0 {
			chunks = append(chunks, styledChunk{text: text.String(), span: screen.Span{Line: line, StartCol: startCol, EndCol: col}})
			start, text = -1, strings.Builder{}
		}
	}
	for _, cell := range cells {
		if isSpaceCell(cell) {
			flush()
		} else {
			if start < 0 {
				start, startCol = 0, col
			}
			text.WriteString(cell.Grapheme)
		}
		col += int(cell.Width)
	}
	flush()
	return chunks
}

func validStyledChunk(text string, span screen.Span, runChunks int) bool {
	if span.EndCol-span.StartCol < 3 || !hasLetterOrDigit(text) || repeatedRune(text) {
		return false
	}
	if runChunks > 4 && !strings.ContainsAny(text, "/._-:@") && !strings.ContainsFunc(text, unicode.IsDigit) {
		return false
	}
	return true
}

func styleConfidence(style screen.Style) float64 {
	if style.Fg.Mode != screen.ColorDefault || style.Bg.Mode != screen.ColorDefault {
		return 0.8
	}
	if style.Attr&screen.AttrUnderline != 0 {
		return 0.7
	}
	if style.Attr&screen.AttrBold != 0 {
		return 0.6
	}
	return 0.3
}
