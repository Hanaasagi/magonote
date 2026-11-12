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
	s := Parse("A中é👩‍💻B https://example.com")
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
		{4, "👩‍💻", 2, 7},
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
