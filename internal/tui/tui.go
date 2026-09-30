// Package tui is the interactive terminal UI.
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/pkg/browser"

	"github.com/jamshids/trnd/internal/bookmarks"
	"github.com/jamshids/trnd/internal/github"
	"github.com/jamshids/trnd/internal/output"
	"github.com/jamshids/trnd/internal/style"
	"github.com/jamshids/trnd/internal/trending"
)

// FetchFunc loads trending repos; cached reports whether the result came
// from the local cache. refresh bypasses the cache.
type FetchFunc func(ctx context.Context, opts trending.Options, refresh bool) (res *trending.Result, cached bool, err error)

type mode int

const (
	modeList mode = iota
	modeReadme
	modePrompt
)

type promptKind int

const (
	promptLanguage promptKind = iota
	promptTopic
)

type (
	fetchedMsg struct {
		seq    int
		res    *trending.Result
		cached bool
		err    error
	}
	readmeMsg struct {
		name, content string
		err           error
	}
	starMsg struct {
		name string
		err  error
	}
)

type keyMap struct {
	Open, Copy, Readme, Star, Since, Lang, Topic, Sort, Bookmark, Saved, Refresh, Back key.Binding
}

var keys = keyMap{
	Open:     key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter", "open")),
	Copy:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy clone url")),
	Readme:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "readme")),
	Star:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star on GitHub")),
	Since:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "day/week/month")),
	Lang:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "language")),
	Topic:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "topic")),
	Sort:     key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort")),
	Bookmark: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "save")),
	Saved:    key.NewBinding(key.WithKeys("B"), key.WithHelp("B", "saved")),
	Refresh:  key.NewBinding(key.WithKeys("R", "ctrl+r"), key.WithHelp("R", "refresh")),
	Back:     key.NewBinding(key.WithKeys("esc", "q", "backspace"), key.WithHelp("esc", "back")),
}

// Model is the root Bubble Tea model.
type Model struct {
	fetch  FetchFunc
	gh     *github.Client
	store  *bookmarks.Store
	opts   trending.Options
	sortBy trending.SortBy

	mode      mode
	prompt    promptKind
	savedView bool
	list      list.Model
	viewport  viewport.Model
	input     textinput.Model

	repos       []trending.Repo // last fetched trending list, unsorted
	seq         int
	loading     bool
	err         error
	readmeName  string
	readmeRaw   string
	glamourTone string
	width       int
	height      int
}

// New builds the TUI model.
func New(fetch FetchFunc, gh *github.Client, store *bookmarks.Store, opts trending.Options, sortBy trending.SortBy) Model {
	l := list.New(nil, delegate{}, 0, 0)
	l.SetStatusBarItemName("repo", "repos")
	l.StatusMessageLifetime = 3 * time.Second
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).
		Background(style.Accent).Padding(0, 1)
	// Free up "l" (language) and "b" (bookmark) from the default paging keys.
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("right", "pgdown", "f"), key.WithHelp("→/pgdn", "next page"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("left", "pgup", "u"), key.WithHelp("←/pgup", "prev page"))
	short := []key.Binding{keys.Open, keys.Bookmark, keys.Since, keys.Topic, keys.Sort}
	l.AdditionalShortHelpKeys = func() []key.Binding { return short }
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Open, keys.Readme, keys.Copy, keys.Star, keys.Bookmark, keys.Saved,
			keys.Since, keys.Lang, keys.Topic, keys.Sort, keys.Refresh}
	}

	in := textinput.New()
	in.CharLimit = 80

	tone := style.InitTheme()

	// pkg/browser prints to stdout, which would corrupt the UI.
	browser.Stdout, browser.Stderr = io.Discard, io.Discard

	if sortBy == "" {
		sortBy = trending.SortTrending
	}
	m := Model{fetch: fetch, gh: gh, store: store, opts: opts, sortBy: sortBy,
		list: l, input: in, glamourTone: tone, seq: 1, loading: true}
	m.updateTitle()
	return m
}

// Run starts the TUI in the alternate screen.
func Run(m Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// Init starts the first fetch. New has already set seq and loading, since
// Init has a value receiver and cannot update the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.list.StartSpinner(), m.fetchCmd(false))
}

// load starts a fetch that supersedes any in-flight one.
func (m *Model) load(refresh bool) tea.Cmd {
	m.seq++
	m.loading = true
	return tea.Batch(m.list.StartSpinner(), m.fetchCmd(refresh))
}

func (m Model) fetchCmd(refresh bool) tea.Cmd {
	seq, opts, fetch := m.seq, m.opts, m.fetch
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		res, cached, err := fetch(ctx, opts, refresh)
		return fetchedMsg{seq: seq, res: res, cached: cached, err: err}
	}
}

func (m *Model) updateTitle() {
	if m.savedView {
		title := "Saved repositories"
		if m.sortBy != trending.SortTrending {
			title += " · sorted by " + string(m.sortBy)
		}
		m.list.Title = title
		return
	}
	m.list.Title = output.Header(m.opts, m.sortBy)
}

// refreshItems rebuilds the list from the current source (trending or saved)
// and sort order, keeping the cursor on the same repo when possible.
func (m *Model) refreshItems() tea.Cmd {
	current, _ := m.selected()
	oldIndex := m.list.Index()

	var items []list.Item
	if m.savedView {
		saved := m.store.All()
		savedAt := make(map[string]time.Time, len(saved))
		repos := make([]trending.Repo, len(saved))
		for i, b := range saved {
			repos[i] = b.Repo
			savedAt[b.FullName()] = b.SavedAt
		}
		for _, r := range trending.Sort(repos, m.sortBy) {
			items = append(items, item{repo: r, since: m.opts.Since, saved: true, savedAt: savedAt[r.FullName()]})
		}
	} else {
		for _, r := range trending.Sort(m.repos, m.sortBy) {
			items = append(items, item{repo: r, since: m.opts.Since, saved: m.store.Has(r.FullName())})
		}
	}

	cmd := m.list.SetItems(items)
	// Stay on the same repo; if it's gone (unsaved in the saved view),
	// stay at the same position instead.
	idx := min(oldIndex, max(len(items)-1, 0))
	for i, it := range items {
		if it.(item).repo.FullName() == current.FullName() {
			idx = i
			break
		}
	}
	m.list.Select(idx)
	return cmd
}

func (m *Model) resize() {
	h := m.height
	if m.mode == modePrompt {
		h -= 2
	}
	m.list.SetSize(m.width, h)
	m.viewport.Width = m.width
	m.viewport.Height = m.height - 2
}

func (m Model) selected() (trending.Repo, bool) {
	it, ok := m.list.SelectedItem().(item)
	return it.repo, ok
}

func (m *Model) status(s string) tea.Cmd { return m.list.NewStatusMessage(s) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		if m.mode == modeReadme {
			m.viewport.SetContent(m.renderMarkdown(m.readmeRaw))
		}
		return m, nil

	case fetchedMsg:
		if msg.seq != m.seq {
			return m, nil // stale response from a superseded request
		}
		m.loading = false
		m.list.StopSpinner()
		if msg.err != nil {
			m.err = msg.err
			m.repos = nil
			if !m.savedView {
				m.list.SetItems(nil)
			}
			return m, nil
		}
		m.err = nil
		m.repos = msg.res.Repos
		if m.savedView {
			return m, nil // keep showing bookmarks; trending list is ready for later
		}
		m.list.ResetFilter()
		m.list.ResetSelected()
		cmd := m.refreshItems()
		m.list.ResetSelected()

		status := style.Dim.Render(fmt.Sprintf("Loaded %d repos", len(m.repos)))
		switch {
		case msg.res.Warning != "":
			status = style.Error.Render(msg.res.Warning)
		case msg.res.Source == "search":
			status = style.Dim.Render("Trending page unavailable; showing top new repos from the Search API")
		case len(m.repos) == 0 && len(m.opts.Topics) > 0:
			status = style.Dim.Render("No repos match those topics; try tab for weekly/monthly")
		case msg.cached:
			status = style.Dim.Render(fmt.Sprintf("Loaded %d repos (cached %s, R to refresh)", len(m.repos), style.Ago(msg.res.FetchedAt)))
		}
		return m, tea.Batch(cmd, m.status(status))

	case readmeMsg:
		if msg.err != nil {
			return m, m.status(style.Error.Render(msg.err.Error()))
		}
		m.mode = modeReadme
		m.readmeName, m.readmeRaw = msg.name, msg.content
		m.resize()
		m.viewport.SetContent(m.renderMarkdown(msg.content))
		m.viewport.GotoTop()
		return m, nil

	case starMsg:
		if msg.err != nil {
			return m, m.status(style.Error.Render(msg.err.Error()))
		}
		return m, m.status(style.Stars.Render("★ Starred " + msg.name + " on GitHub"))

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeReadme:
			return m.updateReadme(msg)
		case modePrompt:
			return m.updatePrompt(msg)
		}
		if m.list.FilterState() != list.Filtering {
			if model, cmd, handled := m.handleListKey(msg); handled {
				return model, cmd
			}
		}
	}

	var cmd tea.Cmd
	switch m.mode {
	case modeReadme:
		m.viewport, cmd = m.viewport.Update(msg)
	case modePrompt:
		m.input, cmd = m.input.Update(msg)
	default:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// In the saved view, esc/q/B return to trending instead of quitting
	// (unless esc is clearing an active / filter).
	if m.savedView && (key.Matches(msg, keys.Saved) ||
		(key.Matches(msg, keys.Back) && m.list.FilterState() == list.Unfiltered)) {
		return m.toggleSavedView(), nil, true
	}

	switch {
	case key.Matches(msg, keys.Saved):
		return m.toggleSavedView(), nil, true

	case key.Matches(msg, keys.Sort):
		m.sortBy = m.sortBy.Next()
		m.updateTitle()
		cmd := m.refreshItems()
		m.list.ResetSelected()
		return m, tea.Batch(cmd, m.status(style.Dim.Render("Sorted by "+string(m.sortBy)))), true

	case key.Matches(msg, keys.Since), key.Matches(msg, keys.Refresh),
		key.Matches(msg, keys.Lang), key.Matches(msg, keys.Topic):
		if m.savedView {
			return m, m.status(style.Dim.Render("Not available in saved view (B to go back)")), true
		}
		switch {
		case key.Matches(msg, keys.Since):
			m.opts.Since = m.opts.Since.Next()
			m.updateTitle()
			return m, m.load(false), true
		case key.Matches(msg, keys.Refresh):
			return m, m.load(true), true
		case key.Matches(msg, keys.Lang):
			return m.openPrompt(promptLanguage), textinput.Blink, true
		default:
			return m.openPrompt(promptTopic), textinput.Blink, true
		}
	}

	repo, ok := m.selected()
	if !ok {
		return m, nil, false
	}
	switch {
	case key.Matches(msg, keys.Bookmark):
		saved, err := m.store.Toggle(repo)
		if err != nil {
			return m, m.status(style.Error.Render("Could not save: " + err.Error())), true
		}
		cmd := m.refreshItems()
		text := style.Gained.Render("⚑ Saved " + repo.FullName())
		if !saved {
			text = style.Dim.Render("Removed " + repo.FullName() + " from saved")
		}
		return m, tea.Batch(cmd, m.status(text)), true

	case key.Matches(msg, keys.Open):
		if err := browser.OpenURL(repo.URL); err != nil {
			return m, m.status(style.Error.Render("Could not open browser: " + repo.URL)), true
		}
		return m, m.status(style.Dim.Render("Opened " + repo.FullName())), true

	case key.Matches(msg, keys.Copy):
		if err := clipboard.WriteAll(repo.CloneURL()); err != nil {
			termenv.Copy(repo.CloneURL()) // OSC 52 fallback for SSH / no xclip
		}
		return m, m.status(style.Gained.Render("Copied " + repo.CloneURL())), true

	case key.Matches(msg, keys.Readme):
		gh, name := m.gh, repo.FullName()
		return m, tea.Batch(m.status(style.Dim.Render("Loading README…")), func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			content, err := gh.Readme(ctx, name)
			return readmeMsg{name: name, content: content, err: err}
		}), true

	case key.Matches(msg, keys.Star):
		gh, name := m.gh, repo.FullName()
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return starMsg{name: name, err: gh.Star(ctx, name)}
		}, true
	}
	return m, nil, false
}

func (m Model) toggleSavedView() Model {
	m.savedView = !m.savedView
	m.list.ResetFilter()
	m.updateTitle()
	m.refreshItems()
	m.list.ResetSelected()
	return m
}

func (m Model) openPrompt(kind promptKind) Model {
	m.mode, m.prompt = modePrompt, kind
	if kind == promptLanguage {
		m.input.Prompt = "Language: "
		m.input.Placeholder = "go, rust, python… (empty for all)"
		m.input.SetValue(m.opts.Language)
	} else {
		m.input.Prompt = "Topics: "
		m.input.Placeholder = "llm, cli, machine-learning… (comma-separated, empty for all)"
		m.input.SetValue(strings.Join(m.opts.Topics, ", "))
	}
	m.input.CursorEnd()
	m.input.Focus()
	m.resize()
	return m
}

func (m Model) updateReadme(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.mode = modeList
		m.resize()
		return m, nil
	case key.Matches(msg, keys.Open):
		_ = browser.OpenURL("https://github.com/" + m.readmeName)
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		m.resize()
		return m, nil
	case "enter":
		m.mode = modeList
		m.input.Blur()
		m.resize()
		value := m.input.Value()
		if m.prompt == promptLanguage {
			m.opts.Language = strings.TrimSpace(value)
		} else {
			m.opts.Topics = trending.ParseTopics(value)
		}
		m.updateTitle()
		cmd := m.load(false)
		if len(m.opts.Topics) > 0 {
			cmd = tea.Batch(cmd, m.status(style.Dim.Render("Looking up topics…")))
		}
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) renderMarkdown(md string) string {
	w := m.width - 4
	if w > 120 {
		w = 120
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.glamourTone), glamour.WithWordWrap(w))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}

func (m Model) View() string {
	switch m.mode {
	case modeReadme:
		header := style.Title.Render(m.readmeName) + style.Dim.Render("  README")
		pct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
		footer := style.Dim.Render("↑/↓ scroll · o open in browser · esc back  " + pct)
		return header + "\n" + m.viewport.View() + "\n" + footer
	case modePrompt:
		return m.list.View() + "\n\n" + m.input.View()
	}

	if m.savedView && len(m.list.Items()) == 0 {
		return "\n  " + style.Title.Render(m.list.Title) + "\n\n  " +
			style.Dim.Render("Nothing saved yet. Press b on a repo to save it.") + "\n\n  " +
			style.Dim.Render("B back · q back")
	}
	if !m.savedView && m.err != nil {
		return "\n  " + style.Title.Render(m.list.Title) + "\n\n  " + style.Error.Render("Error: "+m.err.Error()) +
			"\n\n  " + style.Dim.Render("R retry · tab day/week/month · l language · t topic · B saved · q quit")
	}
	if !m.savedView && m.loading && len(m.list.Items()) == 0 {
		return "\n  " + style.Title.Render(m.list.Title) + "\n\n  " + style.Dim.Render("Fetching trending repositories…")
	}
	return m.list.View()
}
