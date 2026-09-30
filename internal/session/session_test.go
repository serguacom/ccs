package session

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func writeSession(t *testing.T, dir, id string, modified time.Time, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEncodeDir(t *testing.T) {
	cases := map[string]string{
		"/Users/l/code/st/tshs":       "-Users-l-code-st-tshs",
		"/Users/l/code/st/aa_service": "-Users-l-code-st-aa-service",
		"/a/b.c":                      "-a-b-c",
	}
	for in, want := range cases {
		if got := EncodeDir(in); got != want {
			t.Errorf("EncodeDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDialogue(t *testing.T) {
	path := writeSession(t, t.TempDir(), "abc-123", time.Now(),
		`{"type":"last-prompt","lastPrompt":"x"}`,
		`{"type":"user","cwd":"/w/proj","gitBranch":"main","message":{"content":"first question"}}`,
		`{"type":"assistant","cwd":"/w/proj/sub","message":{"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"answer one"},{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"tool out"}]}}`,
		`{"type":"user","isMeta":true,"message":{"content":"meta text"}}`,
		`{"type":"user","isCompactSummary":true,"message":{"content":"This session is being continued"}}`,
		`{"type":"user","message":{"content":"<task-notification>done</task-notification>"}}`,
		`{"type":"user","message":{"content":"<local-command-stdout>out</local-command-stdout>"}}`,
		`{"type":"user","message":{"content":"[Request interrupted by user]"}}`,
		`{"type":"user","message":{"content":"<command-message>critic</command-message>\n<command-name>/critic</command-name>\n<command-args>check item</command-args>"}}`,
		`not json {`,
		`{"type":"user","gitBranch":"feature","message":{"content":[{"type":"text","text":"last question"}]}}`,
		`{"type":"user","message":{"content":"<command-name>/context</command-name>\n<command-args></command-args>"}}`,
	)

	s, ok := Parse(path)
	if !ok {
		t.Fatal("Parse returned !ok")
	}
	want := []Message{
		{Role: User, Text: "first question"},
		{Role: Assistant, Text: "answer one"},
		{Role: User, Text: "/critic check item", Command: true},
		{Role: User, Text: "last question"},
		{Role: User, Text: "/context", Command: true},
	}
	if !slices.Equal(s.Messages, want) {
		t.Errorf("Messages = %+v\nwant %+v", s.Messages, want)
	}
	if s.ID != "abc-123" || s.Cwd != "/w/proj" || s.Project != "proj" || s.Branch != "feature" {
		t.Errorf("metadata = %q %q %q %q", s.ID, s.Cwd, s.Project, s.Branch)
	}
	if s.LastPrompt != "last question" {
		t.Errorf("LastPrompt = %q", s.LastPrompt)
	}
	if s.Title != "first question" {
		t.Errorf("Title = %q", s.Title)
	}
}

func TestParseCwdMissing(t *testing.T) {
	cases := map[string]bool{"/w/gone": true, t.TempDir(): false}
	for cwd, want := range cases {
		line := fmt.Sprintf(`{"type":"user","cwd":%q,"message":{"content":"q"}}`, cwd)
		s, ok := Parse(writeSession(t, t.TempDir(), "id", time.Now(), line))
		if !ok || s.CwdMissing != want {
			t.Errorf("cwd %q: CwdMissing = %v (ok=%v), want %v", cwd, s.CwdMissing, ok, want)
		}
	}
}

func TestParseTitle(t *testing.T) {
	const (
		cmd    = `{"type":"user","message":{"content":"<command-name>/model</command-name>"}}`
		prompt = `{"type":"user","message":{"content":"real prompt"}}`
	)
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"last ai-title wins", []string{prompt, `{"type":"ai-title","aiTitle":"old"}`, `{"type":"ai-title","aiTitle":"new"}`}, "new"},
		{"custom over ai", []string{prompt, `{"type":"custom-title","customTitle":"mine"}`, `{"type":"ai-title","aiTitle":"ai"}`}, "mine"},
		{"agent over custom", []string{prompt, `{"type":"agent-name","agentName":"agent"}`, `{"type":"custom-title","customTitle":"mine"}`}, "agent"},
		{"first non-command prompt", []string{cmd, prompt}, "real prompt"},
		{"only commands", []string{cmd}, "/model"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, ok := Parse(writeSession(t, t.TempDir(), "id", time.Now(), c.lines...))
			if !ok || s.Title != c.want {
				t.Errorf("Title = %q (ok=%v), want %q", s.Title, ok, c.want)
			}
		})
	}
}

func TestParseLastPromptFallsBackToCommand(t *testing.T) {
	s, _ := Parse(writeSession(t, t.TempDir(), "id", time.Now(),
		`{"type":"user","message":{"content":"<command-name>/clear</command-name>"}}`))
	if s.LastPrompt != "/clear" {
		t.Errorf("LastPrompt = %q", s.LastPrompt)
	}
}

func TestParseDropsSessionWithoutUserMessages(t *testing.T) {
	path := writeSession(t, t.TempDir(), "id", time.Now(),
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`,
		`{"type":"user","isMeta":true,"message":{"content":"meta"}}`)
	if _, ok := Parse(path); ok {
		t.Error("Parse returned ok for session without user messages")
	}
	if _, ok := Parse(filepath.Join(t.TempDir(), "missing.jsonl")); ok {
		t.Error("Parse returned ok for missing file")
	}
}

func TestLoadOrdersNewestFirst(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	prompt := `{"type":"user","message":{"content":"q"}}`
	writeSession(t, a, "old", base, prompt)
	writeSession(t, b, "new", base.Add(2*time.Minute), prompt)
	writeSession(t, a, "tie-b", base.Add(time.Minute), prompt)
	writeSession(t, b, "tie-a", base.Add(time.Minute), prompt)
	writeSession(t, a, "empty", base.Add(3*time.Minute), `{"type":"mode"}`)
	if err := os.MkdirAll(filepath.Join(a, "old", "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSession(t, filepath.Join(a, "old", "subagents"), "agent-x", base, prompt)

	dirs := AllDirs(root)
	if !slices.Equal(dirs, []string{a, b}) {
		t.Fatalf("AllDirs = %v", dirs)
	}
	var ids []string
	for _, s := range Load(dirs) {
		ids = append(ids, s.ID)
	}
	if want := []string{"new", "tie-a", "tie-b", "old"}; !slices.Equal(ids, want) {
		t.Errorf("Load ids = %v, want %v", ids, want)
	}
}
