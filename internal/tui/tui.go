package tui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"ccs/internal/search"
	"ccs/internal/session"
)

type Result struct {
	Cwd string
	ID  string
}

const pageSize = 5

var (
	selectedStyle = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	matchStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
)

var (
	listKeys    = [][2]string{{"↑↓", "select"}, {"^A", "scope"}, {"Tab", "preview"}, {"^H", "hide"}, {"^S", "+hidden"}, {"Enter", "resume"}, {"Esc ^Q", "quit"}}
	previewKeys = [][2]string{{"↑↓ PgUp PgDn", "scroll"}, {"^H", "hide"}, {"Enter", "resume"}, {"Esc", "back"}}
)

type allLoadedMsg []session.Session

type model struct {
	input   textinput.Model
	project string
	current []session.Session
	all     []session.Session
	loadAll func() []session.Session
	showAll bool
	loading bool

	hidden     *session.Hidden
	showHidden bool

	previewing bool
	preview    viewport.Model

	terms    []string
	hits     []search.Hit
	selected int
	offset   int
	width    int
	height   int
	result   *Result
}

func newModel(current []session.Session, project string, loadAll func() []session.Session, hidden *session.Hidden) model {
	in := textinput.New()
	in.Prompt = "⌕ "
	in.Placeholder = "Search…"
	in.Focus()
	m := model{input: in, preview: viewport.New(), project: project, current: current, loadAll: loadAll, hidden: hidden}
	m.refresh()
	return m
}

func Run(current []session.Session, project string, loadAll func() []session.Session, hidden *session.Hidden) (*Result, error) {
	final, err := tea.NewProgram(newModel(current, project, loadAll, hidden), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return nil, err
	}
	return final.(model).result, nil
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fitInput()
		m.scroll()
		if m.previewing {
			atBottom := m.preview.AtBottom()
			m.fillPreview()
			if atBottom {
				m.preview.GotoBottom()
			}
		}
		return m, nil
	case allLoadedMsg:
		m.all, m.loading = msg, false
		if m.showAll {
			m.refresh()
		}
		m.fitInput()
		return m, nil
	case tea.PasteMsg:
		if m.previewing {
			return m, nil
		}
	case tea.KeyPressMsg:
		if m.previewing {
			return m.previewKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "ctrl+q", "esc":
			return m, tea.Quit
		case "enter":
			return m.resume()
		case "tab":
			if len(m.hits) > 0 {
				m.fillPreview()
				m.preview.GotoBottom()
				m.previewing = true
			}
			return m, nil
		case "ctrl+a":
			return m.toggleScope()
		case "ctrl+h":
			m.toggleHidden()
			return m, nil
		case "ctrl+s":
			m.showHidden = !m.showHidden
			m.refresh()
			m.fitInput()
			return m, nil
		case "up":
			m.move(-1)
			return m, nil
		case "down":
			m.move(1)
			return m, nil
		case "pgup":
			m.move(-pageSize)
			return m, nil
		case "pgdown":
			m.move(pageSize)
			return m, nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.refresh()
	}
	return m, cmd
}

func (m model) previewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit
	case "enter":
		return m.resume()
	case "esc", "tab":
		m.previewing = false
	case "ctrl+h":
		m.toggleHidden()
	case "up":
		m.preview.ScrollUp(1)
	case "down":
		m.preview.ScrollDown(1)
	case "pgup":
		m.preview.PageUp()
	case "pgdown":
		m.preview.PageDown()
	}
	return m, nil
}

func (m model) resume() (tea.Model, tea.Cmd) {
	if len(m.hits) == 0 {
		return m, nil
	}
	s := m.hits[m.selected].Session
	m.result = &Result{Cwd: s.Cwd, ID: s.ID}
	return m, tea.Quit
}

func (m *model) fillPreview() {
	lines := search.Transcript(m.hits[m.selected].Session.Messages, m.width)
	for i, l := range lines {
		lines[i] = highlight(l, m.terms)
	}
	m.preview.SetWidth(m.width)
	m.preview.SetHeight(max(m.height-3, 0))
	m.preview.SetContentLines(lines)
}

func (m model) toggleScope() (tea.Model, tea.Cmd) {
	m.showAll = !m.showAll
	m.refresh()
	var cmd tea.Cmd
	if m.showAll && m.all == nil && !m.loading {
		m.loading = true
		load := m.loadAll
		cmd = func() tea.Msg { return allLoadedMsg(load()) }
	}
	m.fitInput()
	return m, cmd
}

func (m *model) fitInput() {
	m.input.SetWidth(max(m.width-lipgloss.Width(m.scopeLabel())-6, 10))
}

// filterHits is the only writer of m.hits; leaving preview keeps View from indexing stale hits.
func (m *model) filterHits() {
	sessions := m.current
	if m.showAll {
		sessions = m.all
	}
	m.terms = search.Terms(m.input.Value())
	m.hits = search.Match(sessions, m.terms)
	if !m.showHidden {
		m.hits = slices.DeleteFunc(m.hits, func(h search.Hit) bool { return m.hidden.Has(h.Session.ID) })
	}
	m.previewing = false
}

func (m *model) refresh() {
	m.filterHits()
	m.selected, m.offset = 0, 0
}

func (m *model) toggleHidden() {
	if len(m.hits) == 0 {
		return
	}
	m.hidden.Toggle(m.hits[m.selected].Session.ID)
	m.filterHits()
	m.move(0)
}

func (m *model) move(delta int) {
	m.selected = max(0, min(len(m.hits)-1, m.selected+delta))
	m.scroll()
}

// scroll keeps the selected item fully visible; items have variable heights.
func (m *model) scroll() {
	if m.selected < m.offset {
		m.offset = m.selected
		return
	}
	for m.offset < m.selected && m.linesBetween(m.offset, m.selected) > m.listHeight() {
		m.offset++
	}
}

func (m model) linesBetween(from, to int) int {
	n := 0
	for i := from; i <= to; i++ {
		n += len(m.item(i))
	}
	return n
}

func (m model) listHeight() int { return m.height - 3 }

func (m model) scopeLabel() string {
	scope := m.project
	switch {
	case m.showAll && m.loading:
		scope = "all projects, loading…"
	case m.showAll:
		scope = "all projects"
	}
	if m.showHidden {
		scope += " +hidden"
	}
	return "[" + scope + "]"
}

func (m model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	if m.previewing {
		lines := []string{selectedStyle.Render(m.label(m.hits[m.selected].Session)), ""}
		lines = append(lines, strings.Split(m.preview.View(), "\n")...)
		return m.frame(lines, previewKeys)
	}
	lines := []string{m.input.View() + "  " + dimStyle.Render(m.scopeLabel()), ""}
	if len(m.hits) == 0 && !(m.showAll && m.loading) {
		lines = append(lines, dimStyle.Render("  no matches"))
	}
	for i := m.offset; i < len(m.hits) && len(lines) < m.height-1; i++ {
		lines = append(lines, m.item(i)...)
	}
	return m.frame(lines, listKeys)
}

func (m model) frame(body []string, keys [][2]string) tea.View {
	rows := max(m.height-1, 0)
	lines := body[:min(len(body), rows)]
	for len(lines) < rows {
		lines = append(lines, "")
	}
	lines = append(lines, footer(keys))
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width, "…")
	}
	v := tea.NewView(strings.Join(lines[:min(len(lines), m.height)], "\n"))
	v.AltScreen = true
	return v
}

func footer(keys [][2]string) string {
	entries := make([]string, len(keys))
	for i, k := range keys {
		entries[i] = k[0] + " " + dimStyle.Render(k[1])
	}
	return strings.Join(entries, "  ")
}

func (m model) item(i int) []string {
	h := m.hits[i]
	s := h.Session
	hd := m.label(s)
	lines := []string{"  " + hd}
	if i == m.selected {
		lines[0] = selectedStyle.Render("▸ " + hd)
	}
	if len(m.terms) == 0 {
		lines = append(lines, "    "+dimStyle.Render(oneLine(s.LastPrompt)))
	} else {
		for _, l := range search.Snippet(h.Messages, m.terms, m.width-4) {
			lines = append(lines, "    "+highlight(l, m.terms))
		}
	}
	return append(lines, "")
}

func (m model) label(s session.Session) string {
	var tags string
	if m.hidden.Has(s.ID) {
		tags += "[hidden] "
	}
	if s.CwdMissing {
		tags += "[no dir] "
	}
	return tags + header(s)
}

func header(s session.Session) string {
	parts := []string{age(s.Modified), s.Project}
	if s.Branch != "" {
		parts = append(parts, s.Branch)
	}
	return strings.Join(append(parts, oneLine(s.Title)), " · ")
}

func highlight(line string, terms []string) string {
	return search.Highlight(line, terms, func(s string) string { return matchStyle.Render(s) })
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func age(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
