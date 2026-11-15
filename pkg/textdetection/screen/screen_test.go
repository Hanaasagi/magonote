package screen

import (
	"reflect"
	"strings"
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
		prefix    string
		start     int
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

func TestANSISplitGraphemeCursorUsesCombinedWidth(t *testing.T) {
	// The variation selector widens ❤ after the fact; the cursor must track
	// the combined width or the next append overwrites X.
	s := Parse("❤\x1b[31m\ufe0fX\x1b[0mY")
	line := s.Lines[0]
	if line.Plain != "❤\ufe0fXY" {
		t.Fatalf("plain = %q, want %q", line.Plain, "❤\ufe0fXY")
	}
	if line.Width != 4 {
		t.Fatalf("width = %d, want 4", line.Width)
	}
	if got := s.ColOf(0, len(line.Plain)-1); got != 3 {
		t.Fatalf("Y column = %d, want 3", got)
	}
}

func TestANSISplitGraphemeCursorDoesNotRunAhead(t *testing.T) {
	// The ZWJ sequence narrows the combined width; the cursor must not run
	// ahead of the line or the tab overshoots its stop.
	s := Parse("👩\x1b[31m\u200d💻\x1b[0m\tX", 4)
	line := s.Lines[0]
	if line.Plain != "👩\u200d💻  X" {
		t.Fatalf("plain = %q, want %q", line.Plain, "👩\u200d💻  X")
	}
	if got := s.ColOf(0, len(line.Plain)-1); got != 4 {
		t.Fatalf("X column = %d, want 4", got)
	}
}

func TestCROverwriteCursorAfterGraphemeMerge(t *testing.T) {
	// CR + tab positions the cursor mid-line; the inserted variation selector
	// merges with the preceding grapheme, so the cursor must land after the
	// widened grapheme instead of overwriting Y.
	s := Parse("a❤b\r\t\ufe0fY\x1b[31mZ\x1b[0mW", 2)
	line := s.Lines[0]
	if line.Plain != "a❤\ufe0fYZW" {
		t.Fatalf("plain = %q, want %q", line.Plain, "a❤\ufe0fYZW")
	}
	if got := s.ColOf(0, len("a❤\ufe0fY")); got != 4 {
		t.Fatalf("Z column = %d, want 4", got)
	}
}

func TestCROverwritePartialWideGrapheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantCol  int // display column of the final rune
		tabWidth int
	}{
		{"pad remaining column", "界x\ra", "a x", 2, 8},
		{"styled append hits padding", "界x\ra\x1b[31mb\x1b[0m", "abx", 2, 8},
		{"pad before retained wide grapheme", "中中x\ra", "a 中x", 4, 8},
		{"boundary needs no padding", "界x\rab", "abx", 2, 8},
		{"zero-width insertion does not pad", "界x\r\t\ufe0f", "界\ufe0fx", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Parse(tt.input, tt.tabWidth)
			line := s.Lines[0]
			if line.Plain != tt.want {
				t.Fatalf("plain = %q, want %q", line.Plain, tt.want)
			}
			if got := s.ColOf(0, len(line.Plain)-1); got != tt.wantCol {
				t.Fatalf("final rune column = %d, want %d", got, tt.wantCol)
			}
		})
	}
}

func TestCROverwriteStartsInsideWideGrapheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantCol  int // display column of the final rune
		tabWidth int
	}{
		{"pad leading columns", "1234567界x\r\tA", "1234567 Ax", 9, 8},
		{"styled append after straddle", "1234567界x\r\tA\x1b[31mB\x1b[0m", "1234567 AB", 9, 8},
		{"straddle both ends", "界界x\r\tab", " ab x", 4, 1},
		{"zero-width insertion keeps grapheme", "1234567界x\r\t\ufe0f", "1234567界\ufe0fx", 9, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Parse(tt.input, tt.tabWidth)
			line := s.Lines[0]
			if line.Plain != tt.want {
				t.Fatalf("plain = %q, want %q", line.Plain, tt.want)
			}
			if got := s.ColOf(0, len(line.Plain)-1); got != tt.wantCol {
				t.Fatalf("final rune column = %d, want %d", got, tt.wantCol)
			}
			if line.Width != 10 && tt.name != "straddle both ends" {
				t.Fatalf("width = %d, want 10", line.Width)
			}
		})
	}
}

func TestLeadingZeroWidthFallback(t *testing.T) {
	// The cached width must agree with finishLine's fallback column for a
	// standalone zero-width grapheme at the start of the line, so tab
	// expansion lands on the stop the screen mappings report.
	s := Parse("\u200b\tX")
	line := s.Lines[0]
	if line.Plain != "\u200b       X" {
		t.Fatalf("plain = %q, want %q", line.Plain, "\u200b       X")
	}
	if got := s.ColOf(0, len(line.Plain)-1); got != 8 {
		t.Fatalf("X column = %d, want 8", got)
	}
	if line.Width != 9 {
		t.Fatalf("width = %d, want 9", line.Width)
	}

	if got := Parse("\u200b").Lines[0].Width; got != 1 {
		t.Fatalf("leading zero-width line width = %d, want 1", got)
	}

	// CR overwrite consumes the fallback column like any other cell.
	if got := Parse("\u200bab\rX").Lines[0].Plain; got != "Xab" {
		t.Fatalf("CR overwrite plain = %q, want %q", got, "Xab")
	}

	// A fragment becoming the line start gains the fallback column when
	// measuring the overwrite range.
	s = Parse("ab\r\u200bX")
	line = s.Lines[0]
	if line.Plain != "\u200bX" || line.Width != 2 {
		t.Fatalf("plain/width = %q/%d, want %q/2", line.Plain, line.Width, "\u200bX")
	}
	if got := s.ColOf(0, len(line.Plain)-1); got != 1 {
		t.Fatalf("X column = %d, want 1", got)
	}
}

func TestANSISplitGraphemeWidthCache(t *testing.T) {
	// Splitting graphemes with ANSI codes must not change the parsed content
	// or width: the cached line width has to track every merge at the append
	// boundary (variation selectors, ZWJ sequences, regional indicators).
	for _, base := range []string{
		"❤️X", "👩‍💻Y", "🇨🇳🇺🇸Z", "1️⃣2️⃣", "a\u200bb", "éx", "👨‍👩‍👧‍👦!", "🏳️‍🌈", "abc",
		"\u200bX", "\ufe0f👩",
	} {
		want := Parse(base)
		var split strings.Builder
		for i, r := range base {
			if i > 0 {
				split.WriteString("\x1b[31m")
			}
			split.WriteRune(r)
		}
		got := Parse(split.String())
		wl, gl := want.Lines[0], got.Lines[0]
		if gl.Plain != wl.Plain || gl.Width != wl.Width || len(gl.Cells) != len(wl.Cells) {
			t.Errorf("%q split: plain/width/cells = %q/%d/%d, want %q/%d/%d",
				base, gl.Plain, gl.Width, len(gl.Cells), wl.Plain, wl.Width, len(wl.Cells))
			continue
		}
		for i := range wl.Cells {
			if gl.Cells[i].Grapheme != wl.Cells[i].Grapheme || gl.Cells[i].Width != wl.Cells[i].Width {
				t.Errorf("%q split cell %d = %q/%d, want %q/%d", base, i,
					gl.Cells[i].Grapheme, gl.Cells[i].Width, wl.Cells[i].Grapheme, wl.Cells[i].Width)
			}
		}
	}
}

func BenchmarkParseStyledFragments(b *testing.B) {
	input := strings.Repeat("\x1b[31ma\x1b[0m", 2000)
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(input)
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
		{
			name:  "multibyte overwrite",
			input: "éx\ra",
			want:  "ax",
			lines: 1,
		},
		{
			name:  "CJK partial overwrite",
			input: "中文\rab",
			want:  "ab文",
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

func TestCROverwriteReplacesStyleRuns(t *testing.T) {
	s := Parse("abc\r\x1b[8mXYZ")
	spans := s.HiddenSpans()
	if len(spans) == 0 {
		t.Fatal("expected hidden spans after CR + ESC[8m overwrite")
	}
	if spans[0] != (Span{Line: 0, StartCol: 0, EndCol: 3}) {
		t.Errorf("hidden span = %v, want {0 0 3}", spans[0])
	}
}

func TestTabAfterCR(t *testing.T) {
	s := Parse("abcdef\r\tX")
	plain := s.Lines[0].Plain
	if plain != "abcdef  X" {
		t.Fatalf("plain = %q, want %q", plain, "abcdef  X")
	}
	if got := s.ColOf(0, 8); got != 8 {
		t.Errorf("X column = %d, want 8", got)
	}
}

func TestZeroWidthBeforeTab(t *testing.T) {
	s := Parse("a\u200b\tX")
	if got := s.ColOf(0, len("a\u200b")+7); got != 8 {
		t.Fatalf("X column = %d, want 8", got)
	}
}
