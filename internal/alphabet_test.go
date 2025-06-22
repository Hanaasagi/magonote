package internal

import (
	"reflect"
	"testing"
)

func TestSimpleMatches(t *testing.T) {
	alphabet := NewAlphabet("abcd")
	got := alphabet.Hints(3)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimpleMatches = %v; want %v", got, want)
	}
}

func TestComposedMatches(t *testing.T) {
	alphabet := NewAlphabet("abcd")
	got := alphabet.Hints(6)
	want := []string{"a", "b", "c", "da", "db", "dc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ComposedMatches = %v; want %v", got, want)
	}
}

func TestComposedMatchesMultiple(t *testing.T) {
	alphabet := NewAlphabet("abcd")
	got := alphabet.Hints(8)
	want := []string{"a", "b", "ca", "cb", "da", "db", "dc", "dd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ComposedMatchesMultiple = %v; want %v", got, want)
	}
}

func TestComposedMatchesMax(t *testing.T) {
	alphabet := NewAlphabet("ab")
	got := alphabet.Hints(8)
	want := []string{"aa", "ab", "ba", "bb"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ComposedMatchesMax = %v; want %v", got, want)
	}
}
