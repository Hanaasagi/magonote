package detect

import (
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

func TestHasURISchemeRejectsSingleLetter(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"https://example.com", true},
		{"ftp://files.example.com", true},
		{"mailto:user@example.com", true},
		{"c:foo", false},
		{"d:\\windows\\path", false},
		{"a:", false},
		{"noscheme", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := hasURIScheme(tt.text); got != tt.want {
				t.Errorf("hasURIScheme(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestMarkdownFileLocationBeforeScheme(t *testing.T) {
	ctx := testContext("[source](README:12)")
	candidates := markdownCandidates(ctx, 0)
	if len(candidates) == 0 {
		t.Fatal("expected candidate for README:12")
	}
	c := candidates[0]
	if c.Kind != entity.KindPath {
		t.Errorf("kind = %v, want KindPath", c.Kind)
	}
	if c.Location == nil || c.Location.Line != 12 {
		t.Errorf("location = %#v, want line 12", c.Location)
	}
}

func TestMarkdownRealSchemeStillURL(t *testing.T) {
	ctx := testContext("[link](mailto:user@example.com)")
	candidates := markdownCandidates(ctx, 0)
	if len(candidates) == 0 {
		t.Fatal("expected candidate for mailto URL")
	}
	if candidates[0].Kind != entity.KindURL {
		t.Errorf("kind = %v, want KindURL", candidates[0].Kind)
	}
}

func TestUnescapeMarkdown(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, "hello"},
		{`hello\!world`, "hello!world"},
		{`path\/to\/file`, "path/to/file"},
		{`no\\escape`, `no\escape`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := unescapeMarkdown(tt.input); got != tt.want {
				t.Errorf("unescapeMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
