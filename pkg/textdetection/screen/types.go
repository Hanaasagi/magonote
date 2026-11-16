package screen

import "github.com/rivo/uniseg"

// GraphemeWidth returns a deterministic terminal width for a grapheme or text
// sequence. Standalone zero-width graphemes occupy one fallback column.
func GraphemeWidth(text string) int {
	if width := uniseg.StringWidth(text); width > 0 {
		return width
	}
	return 1
}

// Span identifies a half-open range on one logical line, in display columns.
type Span struct {
	Line     int `json:"line"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

// Cell is one grapheme. A wide grapheme occupies Width display columns but is
// still one element in Line.Cells.
type Cell struct {
	Grapheme string `json:"grapheme"`
	Width    int8   `json:"width"`
	Style    Style  `json:"style"`
	ByteOff  int    `json:"byte_off"` // Offset into Line.Plain.
}

type Line struct {
	Cells []Cell
	Plain string
	Width int

	colToByte    []int
	byteToCol    []int
	byteToEndCol []int
}

type Screen struct {
	Lines []Line
}

func (s *Screen) PlainLines() []string {
	lines := make([]string, len(s.Lines))
	for i := range s.Lines {
		lines[i] = s.Lines[i].Plain
	}
	return lines
}

// ColOf rounds a byte offset down to the containing grapheme's start column.
func (s *Screen) ColOf(line, byteOff int) int {
	l := s.line(line)
	if l == nil {
		return 0
	}
	byteOff = max(0, min(byteOff, len(l.Plain)))
	return l.byteToCol[byteOff]
}

// EndColOf rounds a byte offset up to the containing grapheme's end column.
func (s *Screen) EndColOf(line, byteOff int) int {
	l := s.line(line)
	if l == nil {
		return 0
	}
	byteOff = max(0, min(byteOff, len(l.Plain)))
	return l.byteToEndCol[byteOff]
}

// ByteOf maps either column of a wide grapheme to that grapheme's byte start.
func (s *Screen) ByteOf(line, col int) int {
	l := s.line(line)
	if l == nil {
		return 0
	}
	col = max(0, min(col, l.Width))
	return l.colToByte[col]
}

func (s *Screen) SpanOf(line, byteStart, byteEnd int) Span {
	if byteEnd < byteStart {
		byteEnd = byteStart
	}
	return Span{Line: line, StartCol: s.ColOf(line, byteStart), EndCol: s.EndColOf(line, byteEnd)}
}

// HiddenSpans returns display-column spans where the hidden attribute is set.
// Callers can pass these to the exclusion pipeline so detectors skip invisible
// terminal content.
func (s *Screen) HiddenSpans() []Span {
	var spans []Span
	for lineIdx, line := range s.Lines {
		col := 0
		inHidden := false
		startCol := 0
		for _, cell := range line.Cells {
			hidden := cell.Style.Attr&AttrHidden != 0
			if hidden && !inHidden {
				startCol = col
				inHidden = true
			} else if !hidden && inHidden {
				spans = append(spans, Span{Line: lineIdx, StartCol: startCol, EndCol: col})
				inHidden = false
			}
			col += int(cell.Width)
		}
		if inHidden {
			spans = append(spans, Span{Line: lineIdx, StartCol: startCol, EndCol: col})
		}
	}
	return spans
}

func (s *Screen) line(line int) *Line {
	if line < 0 || line >= len(s.Lines) {
		return nil
	}
	return &s.Lines[line]
}
