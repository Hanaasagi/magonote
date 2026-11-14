package detect

import "testing"

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
