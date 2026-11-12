package screen

import "testing"

func TestSGRStatePersistsAcrossLinesAndResets(t *testing.T) {
	s := Parse("\x1b[31;1mred\nnext\x1b[22;39m plain")
	for _, col := range []int{0, 1, 2} {
		cell := cellAt(t, s, 0, col)
		if cell.Style.Fg.Mode != ColorANSI16 || cell.Style.Fg.Index != 1 || cell.Style.Attr&AttrBold == 0 {
			t.Errorf("first line cell %d style = %#v", col, cell.Style)
		}
	}
	for _, col := range []int{0, 1, 2, 3} {
		cell := cellAt(t, s, 1, col)
		if cell.Style.Fg.Mode != ColorANSI16 || cell.Style.Attr&AttrBold == 0 {
			t.Errorf("second line cell %d lost style: %#v", col, cell.Style)
		}
	}
	cell := cellAt(t, s, 1, 5)
	if cell.Style.Fg.Mode != ColorDefault || cell.Style.Attr&AttrBold != 0 {
		t.Fatalf("reset style = %#v", cell.Style)
	}
}

func TestSGRExtendedColors(t *testing.T) {
	s := Parse("\x1b[38:2::1:2:3mA\x1b[48;5;200mB\x1b[0mC")
	a := cellAt(t, s, 0, 0)
	if a.Style.Fg.Mode != ColorRGB || a.Style.Fg.R != 1 || a.Style.Fg.G != 2 || a.Style.Fg.B != 3 {
		t.Fatalf("RGB foreground = %#v", a.Style.Fg)
	}
	b := cellAt(t, s, 0, 1)
	if b.Style.Bg.Mode != ColorANSI256 || b.Style.Bg.Index != 200 {
		t.Fatalf("palette background = %#v", b.Style.Bg)
	}
	c := cellAt(t, s, 0, 2)
	if c.Style.Fg.Mode != ColorDefault || c.Style.Bg.Mode != ColorDefault {
		t.Fatalf("reset colors = %#v", c.Style)
	}
}

func TestSGRAttributesAndCurlyUnderline(t *testing.T) {
	s := Parse("\x1b[1;2;3;5;7;8;9;4:3mX\x1b[22;23;24;25;27;28;29mY")
	all := AttrBold | AttrDim | AttrItalic | AttrUnderline | AttrBlink | AttrReverse | AttrHidden | AttrStrike
	if got := cellAt(t, s, 0, 0).Style.Attr; got != all {
		t.Fatalf("attributes = %b, want %b", got, all)
	}
	if got := cellAt(t, s, 0, 1).Style.Attr; got != 0 {
		t.Fatalf("cleared attributes = %b", got)
	}
}

func TestOSC8HyperlinkPersistsUntilClosed(t *testing.T) {
	s := Parse("\x1b]8;;https://example.com\aA\nB\x1b]8;;\aC")
	for _, tc := range []struct{ line, col int }{{0, 0}, {1, 0}} {
		if got := cellAt(t, s, tc.line, tc.col).Style.Hyperlink; got != "https://example.com" {
			t.Errorf("hyperlink at %d:%d = %q", tc.line, tc.col, got)
		}
	}
	if got := cellAt(t, s, 1, 1).Style.Hyperlink; got != "" {
		t.Fatalf("closed hyperlink = %q", got)
	}
}

func TestControlSequencesAreDiscardedWithoutLeaking(t *testing.T) {
	s := Parse("A\x1b[2KB\x1b]0;title\aC\x1b[?25lD\x1bP1;2|payload\x1b\\E")
	if got := s.Lines[0].Plain; got != "ABCDE" {
		t.Fatalf("plain = %q, want ABCDE", got)
	}
}

func TestMalformedCSIReturnsToGroundAtNewline(t *testing.T) {
	s := Parse("A\x1b[31\nB")
	if got := s.PlainLines(); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("plain lines = %#v", got)
	}
}
