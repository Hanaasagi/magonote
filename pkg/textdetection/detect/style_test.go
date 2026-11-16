package detect

import (
	"slices"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

// styleContext builds a fully populated Context (screen + token chunks), the
// same way Detect does, so the style detector can snap runs to word boundaries.
func styleContext(input string) *Context {
	s := screen.Parse(input)
	chunks := make([][]token.Chunk, len(s.Lines))
	for line := range s.Lines {
		chunks[line] = token.Chunks(s, line)
	}
	return &Context{Screen: s, Chunks: chunks}
}

func styledTexts(input string) []string {
	candidates := NewStyleDetector().Detect(styleContext(input))
	var out []string
	for _, c := range candidates {
		if c.Kind == entity.KindStyled {
			out = append(out, c.Text)
		}
	}
	return out
}

func TestStyleSubTokenExpandsToWholeToken(t *testing.T) {
	// grep colors a match inside a larger hyphenated token (a pod name).
	got := styledTexts("\x1b[31mnginx\x1b[0m-deployment-7d4b8c9f5-x2k4p")
	want := []string{"nginx-deployment-7d4b8c9f5-x2k4p"}
	if !slices.Equal(got, want) {
		t.Fatalf("styled = %v, want %v", got, want)
	}
}

func TestStyleWholeTokenUnchanged(t *testing.T) {
	got := styledTexts("\x1b[31mnginx\x1b[0m")
	if !slices.Equal(got, []string{"nginx"}) {
		t.Fatalf("styled = %v, want [nginx]", got)
	}
}

func TestStyleSubTokenInIdentifierExpands(t *testing.T) {
	got := styledTexts("foo-\x1b[31mbar\x1b[0m-baz")
	if !slices.Equal(got, []string{"foo-bar-baz"}) {
		t.Fatalf("styled = %v, want [foo-bar-baz]", got)
	}
}

func TestStyleIndependentWordKeepsNeighbors(t *testing.T) {
	// Coloring one word must not turn its unstyled neighbors into fragments.
	got := styledTexts("pod-abc \x1b[31mRunning\x1b[0m 22h")
	if !slices.Equal(got, []string{"Running"}) {
		t.Fatalf("styled = %v, want [Running]", got)
	}
}

func TestStyleRunSpanningWordsKeepsThemTogether(t *testing.T) {
	got := styledTexts("\x1b[32mfoo=bar\x1b[0m")
	if !slices.Equal(got, []string{"foo=bar"}) {
		t.Fatalf("styled = %v, want [foo=bar]", got)
	}
}

func TestStyleShortFragmentExpandsToValidToken(t *testing.T) {
	// A 2-char highlight is below the styled minimum on its own, but the token
	// it belongs to is a valid selection.
	got := styledTexts("\x1b[31mng\x1b[0minx-deployment")
	if !slices.Equal(got, []string{"nginx-deployment"}) {
		t.Fatalf("styled = %v, want [nginx-deployment]", got)
	}
}

func TestStylePreservesPunctuationBoundaries(t *testing.T) {
	// token.Words excludes '(' and ')'; expanding to words must not shrink the
	// colored run and drop the closing parenthesis.
	got := styledTexts("\x1b[31mfoo(bar)\x1b[0m")
	if !slices.Equal(got, []string{"foo(bar)"}) {
		t.Fatalf("styled = %v, want [foo(bar)]", got)
	}
}

func TestStylePreservesShortPlusToken(t *testing.T) {
	// "C++" is one colored run whose only word is "C"; snapping must not shrink
	// it below the minimum length and drop the selection entirely.
	got := styledTexts("\x1b[31mC++\x1b[0m")
	if !slices.Equal(got, []string{"C++"}) {
		t.Fatalf("styled = %v, want [C++]", got)
	}
}

func TestStyleExpansionStopsAtHiddenText(t *testing.T) {
	// A visible highlight adjoining hidden text must not expand over the hidden
	// cells (which would get the whole candidate excluded during resolution).
	got := styledTexts("\x1b[31mdocs\x1b[8msecret\x1b[0m")
	if !slices.Equal(got, []string{"docs"}) {
		t.Fatalf("styled = %v, want [docs]", got)
	}
}
