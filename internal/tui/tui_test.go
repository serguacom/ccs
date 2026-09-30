package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/serguacom/ccs/internal/session"
)

func sess(id, text string) session.Session {
	return session.Session{ID: id, Cwd: "/w/" + id, Project: id, Title: id, LastPrompt: text,
		Messages: []session.Message{{Role: session.User, Text: text}}}
}

func emptyHidden(t *testing.T) *session.Hidden {
	h, err := session.LoadHidden(filepath.Join(t.TempDir(), "hidden"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

var keys = map[string]tea.KeyPressMsg{
	"enter":  {Code: tea.KeyEnter},
	"esc":    {Code: tea.KeyEscape},
	"up":     {Code: tea.KeyUp},
	"down":   {Code: tea.KeyDown},
	"tab":    {Code: tea.KeyTab},
	"pgup":   {Code: tea.KeyPgUp},
	"pgdown": {Code: tea.KeyPgDown},
	"ctrl+a": {Code: 'a', Mod: tea.ModCtrl},
	"ctrl+h": {Code: 'h', Mod: tea.ModCtrl},
	"ctrl+q": {Code: 'q', Mod: tea.ModCtrl},
	"ctrl+s": {Code: 's', Mod: tea.ModCtrl},
}

func press(m model, key string) (model, tea.Cmd) {
	k, ok := keys[key]
	if !ok {
		k = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(k)
	return next.(model), cmd
}

func newTestModel(t *testing.T, ss ...session.Session) model {
	t.Helper()
	return sized(newModel(ss, "proj", nil, emptyHidden(t)))
}

func sized(m model) model { return resized(m, 80, 24) }

func resized(m model, width, height int) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(model)
}

func longSess(id string, n int) session.Session {
	s := sess(id, "msg0")
	for i := 1; i < n; i++ {
		s.Messages = append(s.Messages, session.Message{Role: session.Assistant, Text: fmt.Sprintf("msg%d", i)})
	}
	return s
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

func containsPlain(s, sub string) bool {
	return strings.Contains(ansi.Strip(s), sub)
}

func TestTypingFiltersAndEnterSelects(t *testing.T) {
	m := newTestModel(t, sess("a", "alpha"), sess("b", "beta"))
	if len(m.hits) != 2 {
		t.Fatalf("initial hits = %d", len(m.hits))
	}
	for _, c := range "bet" {
		m, _ = press(m, string(c))
	}
	if len(m.hits) != 1 || m.hits[0].Session.ID != "b" {
		t.Fatalf("hits after typing = %+v", m.hits)
	}
	m, cmd := press(m, "enter")
	if m.result == nil || *m.result != (Result{Cwd: "/w/b", ID: "b"}) || cmd == nil {
		t.Errorf("result = %+v, cmd nil = %v", m.result, cmd == nil)
	}
}

func TestSelectionClamps(t *testing.T) {
	m := newTestModel(t, sess("a", "x"), sess("b", "y"))
	m, _ = press(m, "up")
	if m.selected != 0 {
		t.Errorf("selected after up = %d", m.selected)
	}
	m, _ = press(m, "down")
	m, _ = press(m, "down")
	if m.selected != 1 {
		t.Errorf("selected after down x2 = %d", m.selected)
	}
}

func TestEscQuitsWithoutResult(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	m, cmd := press(m, "esc")
	if m.result != nil || cmd == nil {
		t.Errorf("result = %+v, cmd nil = %v", m.result, cmd == nil)
	}
}

func TestCtrlATogglesScope(t *testing.T) {
	loads := 0
	loadAll := func() []session.Session {
		loads++
		return []session.Session{sess("a", "x"), sess("z", "other project")}
	}
	m := sized(newModel([]session.Session{sess("a", "x")}, "proj", loadAll, emptyHidden(t)))
	m, _ = press(m, "o")

	m, cmd := press(m, "ctrl+a")
	if !m.showAll || !m.loading || cmd == nil {
		t.Fatalf("after ctrl+a: showAll=%v loading=%v cmd nil=%v", m.showAll, m.loading, cmd == nil)
	}
	if m.input.Value() != "o" {
		t.Errorf("ctrl+a reached textinput: value %q", m.input.Value())
	}
	next, _ := m.Update(cmd())
	m = next.(model)
	if m.loading || len(m.hits) != 1 || m.hits[0].Session.ID != "z" {
		t.Fatalf("after load: loading=%v hits=%+v", m.loading, m.hits)
	}

	m, _ = press(m, "ctrl+a")
	m, cmd = press(m, "ctrl+a")
	if cmd != nil || loads != 1 || !m.showAll {
		t.Errorf("second toggle to all must reuse cache: loads=%d cmd nil=%v", loads, cmd == nil)
	}
}

func TestViewRendersHeaderAndSnippet(t *testing.T) {
	m := newTestModel(t, sess("a", "find the needle here"))
	for _, c := range "needle" {
		m, _ = press(m, string(c))
	}
	out := m.View().Content
	for _, want := range []string{"[proj]", "a · a", "needle"} {
		if !containsPlain(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}

func TestScopeLabelFitsAfterToggle(t *testing.T) {
	loadAll := func() []session.Session { return nil }
	m := sized(newModel([]session.Session{sess("a", "x")}, "ccs", loadAll, emptyHidden(t)))
	m, _ = press(m, "ctrl+a")
	top, _, _ := strings.Cut(m.View().Content, "\n")
	if !containsPlain(top, "[all projects, loading…]") {
		t.Errorf("top row = %q", ansi.Strip(top))
	}
}

func TestLoadingAllShowsNoStaleHits(t *testing.T) {
	loadAll := func() []session.Session { return nil }
	m := sized(newModel([]session.Session{sess("a", "x")}, "proj", loadAll, emptyHidden(t)))
	m, _ = press(m, "ctrl+a")
	if len(m.hits) != 0 {
		t.Fatalf("hits while loading = %+v", m.hits)
	}
	m, cmd := press(m, "enter")
	if cmd != nil || m.result != nil {
		t.Errorf("enter while loading: cmd=%v result=%+v", cmd != nil, m.result)
	}
}

func TestNoMatchesShownInCurrentScopeDuringLoad(t *testing.T) {
	loadAll := func() []session.Session { return nil }
	m := sized(newModel([]session.Session{sess("a", "x")}, "proj", loadAll, emptyHidden(t)))
	m, _ = press(m, "ctrl+a")
	m, _ = press(m, "ctrl+a")
	m, _ = press(m, "q")
	if !m.loading || !containsPlain(m.View().Content, "no matches") {
		t.Errorf("loading=%v view:\n%s", m.loading, ansi.Strip(m.View().Content))
	}
}

func TestTabOpensPreviewAtBottom(t *testing.T) {
	m := newTestModel(t, longSess("a", 40))
	m, _ = press(m, "tab")
	out := m.View().Content
	if !m.previewing || !containsPlain(out, "msg39") || containsPlain(out, "msg0") {
		t.Errorf("previewing=%v view:\n%s", m.previewing, ansi.Strip(out))
	}
}

func TestTabWithoutHitsIsNoop(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	m, _ = press(m, "q")
	m, cmd := press(m, "tab")
	if m.previewing || cmd != nil {
		t.Errorf("previewing=%v cmd nil=%v", m.previewing, cmd == nil)
	}
}

func TestPreviewScrolls(t *testing.T) {
	m := newTestModel(t, longSess("a", 40))
	m, _ = press(m, "tab")
	m, _ = press(m, "pgup")
	m, _ = press(m, "pgup")
	m, _ = press(m, "up")
	if m.preview.AtBottom() {
		t.Fatal("pgup/up did not scroll")
	}
	m, _ = press(m, "down")
	m, _ = press(m, "pgdown")
	m, _ = press(m, "pgdown")
	if !m.preview.AtBottom() {
		t.Error("down/pgdown did not scroll back")
	}
}

func TestEscLeavesPreviewKeepingState(t *testing.T) {
	m := newTestModel(t, sess("a", "x1"), sess("b", "x2"))
	m, _ = press(m, "x")
	m, _ = press(m, "down")
	m, _ = press(m, "tab")
	m, _ = press(m, "z")
	m, cmd := press(m, "esc")
	if m.previewing || cmd != nil || m.input.Value() != "x" || m.selected != 1 {
		t.Errorf("previewing=%v cmd nil=%v query=%q selected=%d", m.previewing, cmd == nil, m.input.Value(), m.selected)
	}
}

func TestEnterInPreviewResumes(t *testing.T) {
	m := newTestModel(t, sess("a", "x"), sess("b", "y"))
	m, _ = press(m, "down")
	m, _ = press(m, "tab")
	m, cmd := press(m, "enter")
	if m.result == nil || *m.result != (Result{Cwd: "/w/b", ID: "b"}) || cmd == nil {
		t.Errorf("result = %+v, cmd nil = %v", m.result, cmd == nil)
	}
}

func TestFooterOnLastLine(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	if l := lastLine(m.View().Content); !containsPlain(l, "Tab preview") || !containsPlain(l, "^H hide") {
		t.Errorf("list footer = %q", ansi.Strip(l))
	}
	m, _ = press(m, "tab")
	if l := lastLine(m.View().Content); !containsPlain(l, "Esc back") || !containsPlain(l, "^H hide") {
		t.Errorf("preview footer = %q", ansi.Strip(l))
	}
}

func TestViewFitsHeight(t *testing.T) {
	for _, h := range []int{24, 3, 1} {
		m := resized(newModel([]session.Session{longSess("a", 40), longSess("b", 40)}, "proj", nil, emptyHidden(t)), 80, h)
		for _, mode := range []string{"list", "preview"} {
			if n := len(strings.Split(m.View().Content, "\n")); n > h {
				t.Errorf("height %d %s: %d lines", h, mode, n)
			}
			m, _ = press(m, "tab")
		}
	}
}

func TestResizeInPreviewKeepsBottom(t *testing.T) {
	m := newTestModel(t, longSess("a", 40))
	m, _ = press(m, "tab")
	m = resized(m, 60, 12)
	if out := m.View().Content; !containsPlain(out, "msg39") {
		t.Errorf("view after resize:\n%s", ansi.Strip(out))
	}
}

func TestPasteInPreviewIgnored(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	m, _ = press(m, "tab")
	next, _ := m.Update(tea.PasteMsg{Content: "nomatch"})
	m = next.(model)
	if m.input.Value() != "" || len(m.hits) != 1 {
		t.Errorf("query=%q hits=%d", m.input.Value(), len(m.hits))
	}
}

func TestRefreshLeavesPreview(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	m, _ = press(m, "tab")
	m.input.SetValue("nomatch")
	m.refresh()
	if m.previewing || len(m.View().Content) == 0 {
		t.Errorf("previewing=%v after hits emptied", m.previewing)
	}
}

func hitIDs(m model) string {
	ids := make([]string, len(m.hits))
	for i, h := range m.hits {
		ids[i] = h.Session.ID
	}
	return strings.Join(ids, ",")
}

func TestCtrlHHidesAndKeepsIndex(t *testing.T) {
	hidden := emptyHidden(t)
	m := sized(newModel([]session.Session{sess("a", "x"), sess("b", "y"), sess("c", "z")}, "proj", nil, hidden))
	m, _ = press(m, "down")
	m, _ = press(m, "ctrl+h")
	if hitIDs(m) != "a,c" || m.selected != 1 || m.hits[m.selected].Session.ID != "c" {
		t.Fatalf("hits=%s selected=%d", hitIDs(m), m.selected)
	}
	if !hidden.Has("b") {
		t.Error("b not hidden")
	}
	if containsPlain(m.View().Content, "b · b") {
		t.Errorf("hidden session in view:\n%s", ansi.Strip(m.View().Content))
	}
	if m.input.Value() != "" {
		t.Errorf("ctrl+h reached textinput: value %q", m.input.Value())
	}
}

func TestCtrlHClampsAndEmptiesList(t *testing.T) {
	m := resized(newModel([]session.Session{sess("a", "x"), sess("b", "y"), sess("c", "z")}, "proj", nil, emptyHidden(t)), 80, 8)
	m, _ = press(m, "down")
	m, _ = press(m, "down")
	m, _ = press(m, "ctrl+h")
	if hitIDs(m) != "a,b" || m.selected != 1 || m.offset > m.selected {
		t.Fatalf("hits=%s selected=%d offset=%d", hitIDs(m), m.selected, m.offset)
	}
	m, _ = press(m, "ctrl+h")
	m, _ = press(m, "ctrl+h")
	if len(m.hits) != 0 || m.selected != 0 || m.offset != 0 || !containsPlain(m.View().Content, "no matches") {
		t.Fatalf("hits=%s selected=%d offset=%d", hitIDs(m), m.selected, m.offset)
	}
	m, _ = press(m, "ctrl+h")
	m, _ = press(m, "tab")
	if m.previewing || len(m.hits) != 0 {
		t.Errorf("previewing=%v hits=%d", m.previewing, len(m.hits))
	}
}

func TestCtrlHWithoutHitsIsNoop(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	m, _ = press(m, "q")
	m, cmd := press(m, "ctrl+h")
	if len(m.hits) != 0 || cmd != nil {
		t.Errorf("hits=%d cmd nil=%v", len(m.hits), cmd == nil)
	}
}

func TestCtrlSShowsHiddenWithMarker(t *testing.T) {
	m := newTestModel(t, sess("a", "x"), sess("b", "y"), sess("c", "z"))
	m, _ = press(m, "down")
	m, _ = press(m, "ctrl+h")
	m, _ = press(m, "ctrl+s")
	out := m.View().Content
	if hitIDs(m) != "a,b,c" || !containsPlain(out, "[hidden] ") || !containsPlain(out, "[proj +hidden]") {
		t.Fatalf("hits=%s view:\n%s", hitIDs(m), ansi.Strip(out))
	}
	if m.input.Value() != "" {
		t.Errorf("ctrl+s reached textinput: value %q", m.input.Value())
	}
	m, _ = press(m, "ctrl+s")
	out = m.View().Content
	if hitIDs(m) != "a,c" || !containsPlain(out, "[proj]") {
		t.Errorf("hits=%s view:\n%s", hitIDs(m), ansi.Strip(out))
	}
}

func TestCtrlHOnShownHiddenUnhides(t *testing.T) {
	hidden := emptyHidden(t)
	m := sized(newModel([]session.Session{sess("a", "x"), sess("b", "y"), sess("c", "z")}, "proj", nil, hidden))
	m, _ = press(m, "down")
	m, _ = press(m, "ctrl+h")
	m, _ = press(m, "ctrl+s")
	m, _ = press(m, "down")
	m, _ = press(m, "ctrl+h")
	if hidden.Has("b") || len(m.hits) != 3 || containsPlain(m.View().Content, "[hidden]") {
		t.Errorf("Has(b)=%v hits=%s view:\n%s", hidden.Has("b"), hitIDs(m), ansi.Strip(m.View().Content))
	}
}

func TestCtrlHInPreviewReturnsToList(t *testing.T) {
	hidden := emptyHidden(t)
	m := sized(newModel([]session.Session{sess("a", "x"), sess("b", "y")}, "proj", nil, hidden))
	m, _ = press(m, "tab")
	m, _ = press(m, "ctrl+h")
	if m.previewing || len(m.hits) != 1 || !hidden.Has("a") {
		t.Errorf("previewing=%v hits=%s Has(a)=%v", m.previewing, hitIDs(m), hidden.Has("a"))
	}
}

func TestCtrlQQuitsFromListAndPreview(t *testing.T) {
	m := newTestModel(t, sess("a", "x"))
	if _, cmd := press(m, "ctrl+q"); cmd == nil || cmd() != tea.Quit() {
		t.Error("list: ctrl+q did not quit")
	}
	m, _ = press(m, "tab")
	if _, cmd := press(m, "ctrl+q"); cmd == nil || cmd() != tea.Quit() {
		t.Error("preview: ctrl+q did not quit")
	}
}

func TestLabelMarksMissingCwd(t *testing.T) {
	gone := sess("a", "x")
	gone.CwdMissing = true
	m := newTestModel(t, gone)
	if !containsPlain(m.View().Content, "[no dir] ") {
		t.Fatalf("view:\n%s", ansi.Strip(m.View().Content))
	}
	m, _ = press(m, "ctrl+h")
	m, _ = press(m, "ctrl+s")
	if !containsPlain(m.View().Content, "[hidden] [no dir] ") {
		t.Errorf("view:\n%s", ansi.Strip(m.View().Content))
	}
}

func TestScrollKeepsSelectedVisible(t *testing.T) {
	var ss []session.Session
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		ss = append(ss, sess(id, "x"))
	}
	m := resized(newModel(ss, "proj", nil, emptyHidden(t)), 80, 12)
	for _, step := range []struct {
		key              string
		selected, offset int
	}{
		{"down", 1, 0}, {"down", 2, 0}, {"down", 3, 1}, {"down", 4, 2},
		{"up", 3, 2}, {"up", 2, 2}, {"up", 1, 1}, {"down", 2, 1},
	} {
		m, _ = press(m, step.key)
		if m.selected != step.selected || m.offset != step.offset {
			t.Fatalf("%s: selected=%d offset=%d, want %d %d", step.key, m.selected, m.offset, step.selected, step.offset)
		}
	}
	m = resized(m, 80, 4)
	if m.offset != m.selected {
		t.Errorf("item taller than list: selected=%d offset=%d", m.selected, m.offset)
	}
}
