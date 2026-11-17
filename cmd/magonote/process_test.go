package main

import (
	"testing"

	"github.com/Hanaasagi/magonote/internal"
	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

func TestProcessResultsProtocol(t *testing.T) {
	selected := []internal.ChosenMatch{
		{Text: "https://example.com", Kind: "url", Group: "open"},
		{Text: "nginx", Kind: "word", Group: "copy"},
	}
	got, err := processResults(selected, "%G:%K:%H")
	if err != nil {
		t.Fatal(err)
	}
	want := "open:url:https://example.com\ncopy:word:nginx"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestProcessResultsTextWithColon(t *testing.T) {
	selected := []internal.ChosenMatch{{Text: "https://example.com:8080/x", Kind: "url", Group: "open"}}
	got, _ := processResults(selected, "%G:%K:%H")
	want := "open:url:https://example.com:8080/x"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestProcessResultsJSONWithLocation(t *testing.T) {
	selected := []internal.ChosenMatch{
		{Text: "src/main.go:12:4", Kind: "path", Group: "open",
			Location: &entity.FileLocation{Path: "src/main.go", Line: 12, Column: 4}},
	}
	got, err := processResults(selected, "json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"group":"open","kind":"path","text":"src/main.go:12:4","path":"src/main.go","line":12,"column":4}`
	if got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}
