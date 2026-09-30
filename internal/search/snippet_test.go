package search

import (
	"slices"
	"testing"

	"ccs/internal/session"
)

func TestWrap(t *testing.T) {
	got := wrap("aaa bbb ccc\n\n  dddddddd", 7)
	want := []string{"aaa bbb", "ccc", "ddddddd", "d"}
	if !slices.Equal(got, want) {
		t.Errorf("wrap = %q, want %q", got, want)
	}
}

func TestSnippetContextAroundHit(t *testing.T) {
	// One long line wraps to w1..w9 (width 2+2); the hit is on "w6".
	text := "w1 w2 w3 w4 w5 w6 w7 w8 w9"
	got := Snippet([]session.Message{msg(session.User, text)}, Terms("W6"), 4)
	want := []string{"› w4", "  w5", "  w6", "  w7", "  w8"}
	if !slices.Equal(got, want) {
		t.Errorf("Snippet = %q, want %q", got, want)
	}
}

func TestSnippetClipsAtMessageBounds(t *testing.T) {
	got := Snippet([]session.Message{msg(session.Assistant, "hit\nb\nc\nd")}, Terms("hit"), 20)
	want := []string{"● hit", "  b", "  c"}
	if !slices.Equal(got, want) {
		t.Errorf("Snippet = %q, want %q", got, want)
	}
}

func TestSnippetCapAndMore(t *testing.T) {
	five := "a\nb\nhit\nc\nd"
	msgs := []session.Message{
		msg(session.User, five),
		msg(session.User, five),
		msg(session.User, five),
		msg(session.User, five),
	}
	got := Snippet(msgs, Terms("hit"), 20)
	if len(got) != 11 {
		t.Fatalf("len = %d, want 10 lines + more: %q", len(got), got)
	}
	if got[10] != "+2 more" {
		t.Errorf("last line = %q", got[10])
	}

	short := msg(session.User, "hit\nb\nc")
	cut := Snippet([]session.Message{short, msgs[0], msgs[0]}, Terms("hit"), 20)
	if len(cut) != 10 || cut[9] != "  b" {
		t.Errorf("third snippet must be cut to 2 lines, no more line: %q", cut)
	}
}

func TestSnippetFallbackWhenHitSpansWrap(t *testing.T) {
	// "bc" is split by the hard break between "ab" and "cd".
	got := Snippet([]session.Message{msg(session.User, "abcdefghijkl")}, Terms("bc"), 4)
	if len(got) != 5 || got[0] != "› ab" || got[4] != "  ij" {
		t.Errorf("Snippet = %q", got)
	}
}

func TestHighlight(t *testing.T) {
	cases := []struct {
		line  string
		terms []string
		want  string
	}{
		{"Таблиця в ТАБЛИЦІ", Terms("таблиц"), "[Таблиц]я в [ТАБЛИЦ]І"},
		{"set PGOPTIONS=x", Terms("pgoptions"), "set [PGOPTIONS]=x"},
		{"abcdef", Terms("abc cde"), "[abcde]f"},
		{"nothing", Terms("x"), "nothing"},
	}
	bracket := func(s string) string { return "[" + s + "]" }
	for _, c := range cases {
		if got := Highlight(c.line, c.terms, bracket); got != c.want {
			t.Errorf("Highlight(%q, %q) = %q, want %q", c.line, c.terms, got, c.want)
		}
	}
}

func TestTranscript(t *testing.T) {
	msgs := []session.Message{msg(session.User, "aa bb cc"), msg(session.Assistant, "dd")}
	got := Transcript(msgs, 7)
	want := []string{"› aa bb", "  cc", "", "● dd"}
	if !slices.Equal(got, want) {
		t.Errorf("Transcript = %q, want %q", got, want)
	}
}
