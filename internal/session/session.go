package session

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type Role int

const (
	User Role = iota
	Assistant
)

type Message struct {
	Role    Role
	Text    string
	Command bool
}

type Session struct {
	ID         string
	Cwd        string
	Project    string
	Branch     string
	Title      string
	LastPrompt string
	Modified   time.Time
	Messages   []Message
	CwdMissing bool
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// EncodeDir mirrors Claude Code's project dir naming under ~/.claude/projects.
func EncodeDir(path string) string {
	return nonAlnum.ReplaceAllString(path, "-")
}

type record struct {
	Type             string `json:"type"`
	Cwd              string `json:"cwd"`
	GitBranch        string `json:"gitBranch"`
	IsMeta           bool   `json:"isMeta"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	AgentName        string `json:"agentName"`
	CustomTitle      string `json:"customTitle"`
	AITitle          string `json:"aiTitle"`
	Message          struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var skippedUserPrefixes = []string{"<task-notification>", "<local-command-stdout>", "[Request interrupted by user"}

func Parse(path string) (Session, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Session{}, false
	}

	s := Session{ID: strings.TrimSuffix(filepath.Base(path), ".jsonl"), Modified: info.ModTime()}
	var agentName, customTitle, aiTitle string
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		var rec record
		if len(line) > 0 && json.Unmarshal(line, &rec) == nil {
			if s.Cwd == "" {
				s.Cwd = rec.Cwd
			}
			s.Branch = cmp.Or(rec.GitBranch, s.Branch)
			agentName = cmp.Or(rec.AgentName, agentName)
			customTitle = cmp.Or(rec.CustomTitle, customTitle)
			aiTitle = cmp.Or(rec.AITitle, aiTitle)
			s.Messages = append(s.Messages, dialogue(rec)...)
		}
		if err != nil {
			break
		}
	}

	firstPrompt := pickUserText(slices.All(s.Messages))
	if firstPrompt == "" {
		return Session{}, false
	}
	s.Project = filepath.Base(s.Cwd)
	_, err = os.Stat(s.Cwd)
	s.CwdMissing = errors.Is(err, fs.ErrNotExist)
	s.Title = cmp.Or(agentName, customTitle, aiTitle, firstPrompt)
	s.LastPrompt = pickUserText(slices.Backward(s.Messages))
	return s, true
}

func dialogue(rec record) []Message {
	switch {
	case rec.Type == "assistant":
		var out []Message
		for _, t := range texts(rec.Message.Content) {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, Message{Role: Assistant, Text: t})
			}
		}
		return out
	case rec.Type == "user" && !rec.IsMeta && !rec.IsCompactSummary:
		var out []Message
		for _, t := range texts(rec.Message.Content) {
			if m, ok := userMessage(t); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func texts(raw json.RawMessage) []string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []string{s}
	}
	var blocks []block
	_ = json.Unmarshal(raw, &blocks)
	var out []string
	for _, b := range blocks {
		if b.Type == "text" {
			out = append(out, b.Text)
		}
	}
	return out
}

func userMessage(text string) (Message, bool) {
	text = strings.TrimSpace(text)
	if strings.Contains(text, "<command-name>") {
		cmd := strings.TrimSpace(tagValue(text, "command-name") + " " + tagValue(text, "command-args"))
		return Message{Role: User, Text: cmd, Command: true}, cmd != ""
	}
	for _, p := range skippedUserPrefixes {
		if strings.HasPrefix(text, p) {
			return Message{}, false
		}
	}
	return Message{Role: User, Text: text}, text != ""
}

func tagValue(s, tag string) string {
	_, after, ok := strings.Cut(s, "<"+tag+">")
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(after, "</"+tag+">")
	return strings.TrimSpace(v)
}

// pickUserText returns the first non-command user text in iteration order,
// else the first command text.
func pickUserText(msgs iter.Seq2[int, Message]) string {
	var command string
	for _, m := range msgs {
		if m.Role != User {
			continue
		}
		if !m.Command {
			return m.Text
		}
		if command == "" {
			command = m.Text
		}
	}
	return command
}

func Load(dirs []string) []Session {
	var paths []string
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, "*.jsonl"))
		paths = append(paths, matches...)
	}

	sessions := make([]Session, len(paths))
	ok := make([]bool, len(paths))
	var g errgroup.Group
	g.SetLimit(runtime.NumCPU())
	for i, p := range paths {
		g.Go(func() error {
			sessions[i], ok[i] = Parse(p)
			return nil
		})
	}
	g.Wait()

	var out []Session
	for i, s := range sessions {
		if ok[i] {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b Session) int {
		return cmp.Or(b.Modified.Compare(a.Modified), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func AllDirs(root string) []string {
	entries, _ := os.ReadDir(root)
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs
}
