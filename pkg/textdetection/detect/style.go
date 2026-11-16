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
		visRuns := visibleRuns(cells, line)
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
					span = clampToVisible(visRuns, chunk.span, span)
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

// expandToWords extends span outward to cover every word it intersects, starting
// from the original styled bounds. It only ever grows the span: a run may include
// punctuation that token.Words excludes (e.g. "foo(bar)", "C++"), which must stay
// selectable rather than being shrunk away. A run equal to a token is unchanged;
// a run spanning several words keeps them together.
func expandToWords(words []token.Word, span screen.Span) screen.Span {
	out := span
	for _, w := range words {
		if w.Span.Line != span.Line || w.Span.StartCol >= span.EndCol || span.StartCol >= w.Span.EndCol {
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

// visibleRuns returns the contiguous non-hidden column intervals of a line. It
// is the exact dual of screen.HiddenSpans, so its boundaries align with the
// hidden-span exclusions applied during resolution.
func visibleRuns(cells []screen.Cell, line int) []screen.Span {
	var runs []screen.Span
	col, startCol := 0, 0
	inVisible := false
	for _, cell := range cells {
		visible := cell.Style.Attr&screen.AttrHidden == 0
		if visible && !inVisible {
			startCol, inVisible = col, true
		} else if !visible && inVisible {
			runs = append(runs, screen.Span{Line: line, StartCol: startCol, EndCol: col})
			inVisible = false
		}
		col += int(cell.Width)
	}
	if inVisible {
		runs = append(runs, screen.Span{Line: line, StartCol: startCol, EndCol: col})
	}
	return runs
}

// clampToVisible limits expanded to the visible run containing chunk, so word
// expansion never crosses into hidden text. Without this, a visible highlight
// adjoining concealed text (e.g. "docs" + hidden "secret") would expand over the
// hidden cells and be discarded wholesale by the hidden-span exclusion.
func clampToVisible(runs []screen.Span, chunk, expanded screen.Span) screen.Span {
	for _, r := range runs {
		if r.StartCol <= chunk.StartCol && chunk.StartCol < r.EndCol {
			if r.StartCol > expanded.StartCol {
				expanded.StartCol = r.StartCol
			}
			if r.EndCol < expanded.EndCol {
				expanded.EndCol = r.EndCol
			}
			return expanded
		}
	}
	return expanded
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
