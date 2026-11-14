package screen

import (
	"reflect"
	"testing"
)

func cellAt(t *testing.T, s *Screen, line, col int) Cell {
	t.Helper()
	x := 0
	for _, cell := range s.Lines[line].Cells {
		if col >= x && col < x+int(cell.Width) {
			return cell
		}
		x += int(cell.Width)
	}
	t.Fatalf("no cell at line %d col %d", line, col)
	return Cell{}
}

func TestParsePlainAndEmptyLines(t *testing.T) {
	s := Parse("a\n\n")
	want := []string{"a", "", ""}
	if got := s.PlainLines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("PlainLines() = %#v, want %#v", got, want)
	}
	if got := Parse("").PlainLines(); !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("empty input lines = %#v", got)
	}
}

func TestParseUnicodeUsesDisplayColumns(t *testing.T) {
	s := Parse("A中é👩\u200d💻B https://example.com")
	line := s.Lines[0]
	if line.Width != 27 {
		t.Fatalf("line width = %d, want 27", line.Width)
	}

	checks := []struct {
		col      int
		grapheme string
		width    int8
		byteOff  int
	}{
		{0, "A", 1, 0},
		{1, "中", 2, 1},
		{3, "é", 1, 4},
		{4, "👩\u200d💻", 2, 7},
		{6, "B", 1, 18},
		{8, "h", 1, 20},
	}
	for _, check := range checks {
		cell := cellAt(t, s, 0, check.col)
		if cell.Grapheme != check.grapheme || cell.Width != check.width || cell.ByteOff != check.byteOff {
			t.Errorf("cell at col %d = %#v, want grapheme %q width %d byte %d", check.col, cell, check.grapheme, check.width, check.byteOff)
		}
	}

	if got := s.ColOf(0, 20); got != 8 {
		t.Errorf("ColOf(URL) = %d, want 8", got)
	}
	if got := s.ColOf(0, 2); got != 1 {
		t.Errorf("ColOf(inside CJK) = %d, want 1", got)
	}
	if got := s.EndColOf(0, 2); got != 3 {
		t.Errorf("EndColOf(inside CJK) = %d, want 3", got)
	}
	if got := s.ByteOf(0, 2); got != 1 {
		t.Errorf("ByteOf(second CJK column) = %d, want 1", got)
	}
	if got := s.ByteOf(0, 8); got != 20 {
		t.Errorf("ByteOf(URL) = %d, want 20", got)
	}
	if got := s.SpanOf(0, 2, 4); got != (Span{Line: 0, StartCol: 1, EndCol: 3}) {
		t.Errorf("SpanOf(CJK bytes) = %#v", got)
	}
}

func TestParseEmojiUsesGraphemeWidth(t *testing.T) {
	tests := []struct {
		prefix     string
		start      int
		lineWidth int
	}{
		{prefix: "🇺🇸", start: 3, lineWidth: 22},
		{prefix: "1️⃣", start: 3, lineWidth: 22},
		{prefix: "❤️", start: 3, lineWidth: 22},
		{prefix: "α", start: 2, lineWidth: 21},
	}
	for _, test := range tests {
		s := Parse(test.prefix + " https://example.com")
		if got := s.ColOf(0, len(test.prefix)+1); got != test.start {
			t.Errorf("%q URL start = %d, want %d", test.prefix, got, test.start)
		}
		if got := s.Lines[0].Width; got != test.lineWidth {
			t.Errorf("%q line width = %d, want %d", test.prefix, got, test.lineWidth)
		}
	}
}

func TestANSISplitGraphemeKeepsWidthAndMappings(t *testing.T) {
	for _, input := range []string{"❤\x1b[31m️\x1b[0m", "👩\x1b[31m\u200d💻\x1b[0m"} {
		s := Parse(input)
		line := s.Lines[0]
		if len(line.Cells) != 1 || line.Cells[0].Width != 2 || line.Width != 2 {
			t.Errorf("%q cells/width = %#v/%d", input, line.Cells, line.Width)
		}
		if got := s.ByteOf(0, 2); got != len(line.Plain) {
			t.Errorf("%q ByteOf(end) = %d, want %d", input, got, len(line.Plain))
		}
	}
}

func TestANSISplitGraphemeBeforeTabUsesFinalWidth(t *testing.T) {
	s := Parse("👩\x1b[31m\u200d💻\x1b[0m\tX")
	if got := s.ColOf(0, len("👩\u200d💻")+6); got != 8 {
		t.Fatalf("X column = %d, want 8", got)
	}
}

func TestParseTabsExpandsAtDisplayColumn(t *testing.T) {
	s := Parse("中\tX")
	if got := s.Lines[0].Plain; got != "中      X" {
		t.Fatalf("expanded plain = %q", got)
	}
	if got := s.ColOf(0, 9); got != 8 {
		t.Fatalf("X column = %d, want 8", got)
	}
	if got := s.ByteOf(0, 8); got != 9 {
		t.Fatalf("X byte = %d, want 9", got)
	}

	s = Parse("a\tb", 4)
	if got := s.Lines[0].Plain; got != "a   b" {
		t.Fatalf("tab width 4 plain = %q", got)
	}
}

func TestHiddenSpans(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Span
	}{
		{
			name:  "no hidden text",
			input: "hello world",
			want:  nil,
		},
		{
			name:  "entire line hidden",
			input: "\x1b[8msecret\x1b[28m",
			want:  []Span{{Line: 0, StartCol: 0, EndCol: 6}},
		},
		{
			name:  "hidden in middle",
			input: "before\x1b[8msecret\x1b[28mafter",
			want:  []Span{{Line: 0, StartCol: 6, EndCol: 12}},
		},
		{
			name:  "multiple hidden spans",
			input: "a\x1b[8mXX\x1b[28mb\x1b[8mYY\x1b[28mc",
			want:  []Span{{Line: 0, StartCol: 1, EndCol: 3}, {Line: 0, StartCol: 4, EndCol: 6}},
		},
		{
			name:  "hidden until end of line",
			input: "visible\x1b[8mhidden",
			want:  []Span{{Line: 0, StartCol: 7, EndCol: 13}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Parse(tt.input)
			got := s.HiddenSpans()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HiddenSpans() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCarriageReturn(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		lines int
	}{
		{
			name:  "overwrite from start",
			input: "old text\rnew!",
			want:  "new!text",
			lines: 1,
		},
		{
			name:  "full overwrite",
			input: "loading\rdone!!!",
			want:  "done!!!",
			lines: 1,
		},
		{
			name:  "CRLF is newline",
			input: "line1\r\nline2",
			want:  "line1",
			lines: 2,
		},
		{
			name:  "multiple CR",
			input: "aaa\rbbb\rccc",
			want:  "ccc",
			lines: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Parse(tt.input)
			if len(s.Lines) != tt.lines {
				t.Fatalf("lines = %d, want %d", len(s.Lines), tt.lines)
			}
			if s.Lines[0].Plain != tt.want {
				t.Errorf("plain = %q, want %q", s.Lines[0].Plain, tt.want)
			}
		})
	}
}
