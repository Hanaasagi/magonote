package screen

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

func (s *Screen) line(line int) *Line {
	if line < 0 || line >= len(s.Lines) {
		return nil
	}
	return &s.Lines[line]
}
