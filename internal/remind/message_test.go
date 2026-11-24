package remind

import "testing"

func TestMessageNameAndTagsOnly(t *testing.T) {
	got := Message("  Phở  ", []string{"food", " ", "soup"})
	want := "<blockquote>🌿  <b>Phở</b>\n<i>food  ·  soup</i></blockquote>"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestMessageEscapesName(t *testing.T) {
	got := Message("A < B & C", []string{"hot soup"})
	want := "<blockquote>🌿  <b>A &lt; B &amp; C</b>\n<i>hot soup</i></blockquote>"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestMessageNameWithoutTags(t *testing.T) {
	got := Message("Walk", nil)
	if got != "<blockquote>🌿  <b>Walk</b></blockquote>" {
		t.Fatalf("got %q", got)
	}
}

func TestMessageEmpty(t *testing.T) {
	if got := Message("  ", []string{" ", ""}); got != "" {
		t.Fatalf("got %q", got)
	}
}
