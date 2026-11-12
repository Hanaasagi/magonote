package resolve

import (
	"reflect"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

func candidate(kind entity.Kind, text string, start, end, class int, confidence float64) entity.Candidate {
	return entity.Candidate{Kind: kind, Text: text, Span: screen.Span{Line: 0, StartCol: start, EndCol: end}, Class: class, Confidence: confidence}
}

func TestResolveRanksClassLengthAndConfidence(t *testing.T) {
	semantic := candidate(entity.KindURL, "semantic", 2, 8, entity.ClassSemantic, 0.9)
	custom := candidate(entity.KindCustom, "custom", 2, 8, entity.ClassCustom, 0.1)
	if got := Candidates([]entity.Candidate{semantic, custom}, nil); len(got) != 1 || got[0].Text != "custom" {
		t.Fatalf("class priority = %#v", got)
	}

	longer := candidate(entity.KindPath, "/a/long", 0, 7, entity.ClassSemantic, 0.1)
	shorter := candidate(entity.KindFilename, "a", 0, 1, entity.ClassSemantic, 1)
	if got := Candidates([]entity.Candidate{shorter, longer}, nil); len(got) != 1 || got[0].Text != "/a/long" {
		t.Fatalf("length priority = %#v", got)
	}

	low := candidate(entity.KindURL, "low", 0, 3, entity.ClassSemantic, 0.1)
	high := candidate(entity.KindURL, "high", 0, 3, entity.ClassSemantic, 0.9)
	if got := Candidates([]entity.Candidate{low, high}, nil); len(got) != 1 || got[0].Text != "high" {
		t.Fatalf("confidence priority = %#v", got)
	}
}

func TestResolveExcludesOverlapsAndReturnsDocumentOrder(t *testing.T) {
	first := candidate(entity.KindFilename, "first.go", 0, 8, entity.ClassSemantic, 0.9)
	second := candidate(entity.KindURL, "https://x", 20, 29, entity.ClassSemantic, 0.9)
	got := Candidates([]entity.Candidate{second, first}, []screen.Span{{Line: 0, StartCol: 20, EndCol: 21}})
	if !reflect.DeepEqual(got, []entity.Candidate{first}) {
		t.Fatalf("excluded result = %#v", got)
	}

	first.Span.Line = 1
	second.Span.Line = 0
	got = Candidates([]entity.Candidate{first, second}, nil)
	if got[0].Span.Line != 0 || got[1].Span.Line != 1 {
		t.Fatalf("document order = %#v", got)
	}
}

func TestResolveDeduplicatesIdenticalSpans(t *testing.T) {
	a := candidate(entity.KindURL, "a", 1, 4, entity.ClassSemantic, 0.9)
	b := candidate(entity.KindURL, "b", 1, 4, entity.ClassSemantic, 0.9)
	if got := Candidates([]entity.Candidate{a, b}, nil); len(got) != 1 || got[0].Text != "a" {
		t.Fatalf("duplicate spans = %#v", got)
	}
}
