package search

import (
	"slices"
	"testing"

	"ccs/internal/session"
)

func msg(role session.Role, text string) session.Message {
	return session.Message{Role: role, Text: text}
}

func TestTerms(t *testing.T) {
	if got := Terms("  PGOptions  Таблиця "); !slices.Equal(got, []string{"pgoptions", "таблиця"}) {
		t.Errorf("Terms = %q", got)
	}
	if got := Terms("   "); len(got) != 0 {
		t.Errorf("Terms(blank) = %q", got)
	}
}

func TestMatch(t *testing.T) {
	sessions := []session.Session{
		{ID: "s1", Messages: []session.Message{
			msg(session.User, "Set PGOPTIONS here"),
			msg(session.Assistant, "unrelated"),
			msg(session.Assistant, "pgoptions and GUC together"),
		}},
		{ID: "s2", Messages: []session.Message{msg(session.User, "nothing")}},
		{ID: "s3", Messages: []session.Message{msg(session.User, "Таблиця в різних енвах")}},
	}

	hits := Match(sessions, Terms("pgoptions"))
	if len(hits) != 1 || hits[0].Session.ID != "s1" {
		t.Fatalf("hits = %+v", hits)
	}
	want := []session.Message{sessions[0].Messages[2], sessions[0].Messages[0]}
	if !slices.Equal(hits[0].Messages, want) {
		t.Errorf("Messages = %+v, want newest first %+v", hits[0].Messages, want)
	}

	if hits := Match(sessions, Terms("guc PGOPTIONS")); len(hits) != 1 || len(hits[0].Messages) != 1 {
		t.Errorf("AND within one message: %+v", hits)
	}
	if hits := Match(sessions, Terms("guc nothing")); len(hits) != 0 {
		t.Errorf("terms split across messages must not match: %+v", hits)
	}
	if hits := Match(sessions, Terms("ТАБЛИЦЯ")); len(hits) != 1 || hits[0].Session.ID != "s3" {
		t.Errorf("Cyrillic case-insensitive: %+v", hits)
	}

	all := Match(sessions, nil)
	if len(all) != 3 || all[0].Session.ID != "s1" || all[2].Session.ID != "s3" || all[0].Messages != nil {
		t.Errorf("empty query = %+v", all)
	}
}
