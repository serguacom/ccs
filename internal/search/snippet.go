package search

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/serguacom/ccs/internal/session"
)

const (
	contextLines = 2
	maxLines     = 10
)

var marker = map[session.Role]string{session.User: "›", session.Assistant: "●"}

func Snippet(msgs []session.Message, terms []string, width int) []string {
	var out []string
	for i, m := range msgs {
		if len(out) == maxLines {
			return append(out, fmt.Sprintf("+%d more", len(msgs)-i))
		}
		lines := around(wrap(m.Text, width-2), terms)
		out = append(out, prefixed(m.Role, lines[:min(len(lines), maxLines-len(out))])...)
	}
	return out
}

func Transcript(msgs []session.Message, width int) []string {
	var out []string
	for i, m := range msgs {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, prefixed(m.Role, wrap(m.Text, width-2))...)
	}
	return out
}

func prefixed(role session.Role, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		prefix := "  "
		if i == 0 {
			prefix = marker[role] + " "
		}
		out[i] = prefix + l
	}
	return out
}

func around(lines, terms []string) []string {
	hit := slices.IndexFunc(lines, func(l string) bool {
		lower := strings.ToLower(l)
		return slices.ContainsFunc(terms, func(t string) bool { return strings.Contains(lower, t) })
	})
	if hit < 0 {
		return lines[:min(len(lines), 2*contextLines+1)]
	}
	return lines[max(0, hit-contextLines):min(len(lines), hit+contextLines+1)]
}

// wrap soft-wraps on whitespace by rune count, hard-breaks long words and drops blank lines.
func wrap(text string, width int) []string {
	width = max(width, 1)
	var out []string
	for para := range strings.SplitSeq(text, "\n") {
		var line []rune
		for word := range strings.FieldsSeq(para) {
			w := []rune(word)
			if len(line) > 0 && len(line)+1+len(w) <= width {
				line = append(append(line, ' '), w...)
				continue
			}
			if len(line) > 0 {
				out = append(out, string(line))
			}
			for len(w) > width {
				out = append(out, string(w[:width]))
				w = w[width:]
			}
			line = w
		}
		if len(line) > 0 {
			out = append(out, string(line))
		}
	}
	return out
}

func Highlight(line string, terms []string, mark func(string) string) string {
	runes := []rune(line)
	lower := make([]rune, len(runes))
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}
	var ranges [][2]int
	for _, t := range terms {
		tr := []rune(t)
		for i := 0; i+len(tr) <= len(lower); i++ {
			if slices.Equal(lower[i:i+len(tr)], tr) {
				ranges = append(ranges, [2]int{i, i + len(tr)})
			}
		}
	}
	slices.SortFunc(ranges, func(a, b [2]int) int { return cmp.Compare(a[0], b[0]) })
	var merged [][2]int
	for _, r := range ranges {
		if n := len(merged); n > 0 && r[0] <= merged[n-1][1] {
			merged[n-1][1] = max(merged[n-1][1], r[1])
			continue
		}
		merged = append(merged, r)
	}
	var b strings.Builder
	last := 0
	for _, r := range merged {
		b.WriteString(string(runes[last:r[0]]))
		b.WriteString(mark(string(runes[r[0]:r[1]])))
		last = r[1]
	}
	b.WriteString(string(runes[last:]))
	return b.String()
}
