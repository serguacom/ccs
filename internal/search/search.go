package search

import (
	"slices"
	"strings"

	"ccs/internal/session"
)

type Hit struct {
	Session  session.Session
	Messages []session.Message
}

func Terms(query string) []string {
	return strings.Fields(strings.ToLower(query))
}

func Match(sessions []session.Session, terms []string) []Hit {
	var hits []Hit
	for _, s := range sessions {
		if len(terms) == 0 {
			hits = append(hits, Hit{Session: s})
			continue
		}
		var matched []session.Message
		for _, m := range slices.Backward(s.Messages) {
			if containsAll(strings.ToLower(m.Text), terms) {
				matched = append(matched, m)
			}
		}
		if matched != nil {
			hits = append(hits, Hit{Session: s, Messages: matched})
		}
	}
	return hits
}

func containsAll(lower string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(lower, t) {
			return false
		}
	}
	return true
}
