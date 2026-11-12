package token

import (
	"reflect"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

func TestChunksUseUnicodeWhitespaceAndDisplayColumns(t *testing.T) {
	s := screen.Parse("  foo  中 bar baz")
	got := Chunks(s, 0)
	want := []Chunk{
		{Text: "foo", Span: screen.Span{Line: 0, StartCol: 2, EndCol: 5}, ByteStart: 2, ByteEnd: 5},
		{Text: "中", Span: screen.Span{Line: 0, StartCol: 7, EndCol: 9}, ByteStart: 7, ByteEnd: 10},
		{Text: "bar", Span: screen.Span{Line: 0, StartCol: 10, EndCol: 13}, ByteStart: 11, ByteEnd: 14},
		{Text: "baz", Span: screen.Span{Line: 0, StartCol: 14, EndCol: 17}, ByteStart: 16, ByteEnd: 19},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Chunks() = %#v, want %#v", got, want)
	}
}

func TestWordsSplitPunctuationButKeepEntityCharacters(t *testing.T) {
	s := screen.Parse("(foo/bar.go:12),")
	chunks := Chunks(s, 0)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %#v", chunks)
	}
	got := Words(chunks[0])
	want := []Word{{
		Text: "foo/bar.go:12", Span: screen.Span{Line: 0, StartCol: 1, EndCol: 14}, ByteStart: 1, ByteEnd: 14,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Words() = %#v, want %#v", got, want)
	}
}

func TestWordsKeepAbsoluteDisplayColumns(t *testing.T) {
	s := screen.Parse("  (foo)")
	words := Words(Chunks(s, 0)[0])
	if len(words) != 1 || words[0].Span.StartCol != 3 || words[0].Span.EndCol != 6 {
		t.Fatalf("word span = %#v, want [3,6)", words)
	}
}

func TestIsSeparatorLine(t *testing.T) {
	for _, input := range []string{"  ├─:──^  ", "|---|", "++++", "^^^^"} {
		if !IsSeparatorLine(input) {
			t.Errorf("IsSeparatorLine(%q) = false", input)
		}
	}
	for _, input := range []string{"", "   ", "abc---", "a | b"} {
		if IsSeparatorLine(input) {
			t.Errorf("IsSeparatorLine(%q) = true", input)
		}
	}
}
