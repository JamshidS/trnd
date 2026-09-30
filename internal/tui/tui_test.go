package tui

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jamshids/trnd/internal/bookmarks"
	"github.com/jamshids/trnd/internal/github"
	"github.com/jamshids/trnd/internal/trending"
)

func fakeFetch(calls *[]trending.Options) FetchFunc {
	return func(_ context.Context, opts trending.Options, _ bool) (*trending.Result, bool, error) {
		*calls = append(*calls, opts)
		repos := []trending.Repo{
			{Owner: "acme", Name: "rocket", URL: "https://github.com/acme/rocket", Description: "Fast things", Language: "Go", Stars: 1234, StarsPeriod: 56},
			{Owner: "acme", Name: "turtle", URL: "https://github.com/acme/turtle", Language: "Rust", Stars: 9000, StarsPeriod: 3},
		}
		if len(opts.Topics) > 0 { // pretend only rocket has the topic
			repos = repos[:1]
			repos[0].Topics = opts.Topics
		}
		return &trending.Result{Source: "trending", FetchedAt: time.Now(), Repos: repos}, false, nil
	}
}

// run executes a command and feeds fetch results back into the model.
// Commands that don't return quickly (spinner ticks, status-message timers,
// cursor blinks) are timers the test doesn't care about, so they're dropped.
func run(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(50 * time.Millisecond):
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = run(t, m, c)
		}
		return m
	}
	if f, ok := msg.(fetchedMsg); ok {
		next, _ := m.Update(f)
		return next.(Model)
	}
	return m
}

func press(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// send presses keys in order, running any resulting commands.
func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(press(k))
		m = run(t, next.(Model), cmd)
	}
	return m
}

func setup(t *testing.T) (Model, *[]trending.Options, *bookmarks.Store) {
	t.Helper()
	var calls []trending.Options
	store, err := bookmarks.Open(filepath.Join(t.TempDir(), "bookmarks.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(fakeFetch(&calls), github.NewClient(""), store, trending.Options{Since: trending.Daily}, trending.SortTrending)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return run(t, next.(Model), m.Init()), &calls, store
}

func visibleNames(m Model) []string {
	var out []string
	for _, li := range m.list.Items() {
		out = append(out, li.(item).repo.Name)
	}
	return out
}

func TestListAndControls(t *testing.T) {
	m, calls, _ := setup(t)

	view := m.View()
	for _, want := range []string{"acme/", "rocket", "Fast things", "+56 today", "turtle"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}

	// tab cycles the time range and refetches.
	m = send(t, m, "tab")
	if got := (*calls)[len(*calls)-1].Since; got != trending.Weekly {
		t.Errorf("after tab since = %q, want weekly", got)
	}
	if !strings.Contains(m.View(), "+56 this week") {
		t.Errorf("expected weekly label in view")
	}

	// l opens the language prompt; enter applies it.
	m = send(t, m, "l")
	if m.mode != modePrompt {
		t.Fatalf("mode = %v, want prompt", m.mode)
	}
	m = send(t, m, "go", "enter")
	if got := (*calls)[len(*calls)-1].Language; got != "go" {
		t.Errorf("language = %q, want go", got)
	}
	if !strings.Contains(m.View(), "weekly · go") {
		t.Errorf("title not updated:\n%s", m.View())
	}
}

func TestSort(t *testing.T) {
	m, _, _ := setup(t)
	if got := visibleNames(m); !slices.Equal(got, []string{"rocket", "turtle"}) {
		t.Fatalf("initial order = %v", got)
	}
	m = send(t, m, "S") // gained: rocket (56) before turtle (3)
	if m.sortBy != trending.SortGained {
		t.Fatalf("sortBy = %q, want gained", m.sortBy)
	}
	m = send(t, m, "S") // stars: turtle (9000) first
	if got := visibleNames(m); !slices.Equal(got, []string{"turtle", "rocket"}) {
		t.Errorf("stars order = %v", got)
	}
	if !strings.Contains(m.View(), "sorted by stars") {
		t.Error("title should mention sort")
	}
}

func TestTopicPrompt(t *testing.T) {
	m, calls, _ := setup(t)
	m = send(t, m, "t", "LLM, cli", "enter")
	if got := (*calls)[len(*calls)-1].Topics; !slices.Equal(got, []string{"llm", "cli"}) {
		t.Errorf("topics = %v, want [llm cli]", got)
	}
	if got := visibleNames(m); !slices.Equal(got, []string{"rocket"}) {
		t.Errorf("filtered = %v, want [rocket]", got)
	}
	view := m.View()
	if !strings.Contains(view, "#llm #cli") {
		t.Errorf("view should show topics:\n%s", view)
	}
}

func TestBookmarks(t *testing.T) {
	m, _, store := setup(t)

	// Save the second repo.
	m = send(t, m, "down", "b")
	if !store.Has("acme/turtle") {
		t.Fatal("b should save the selected repo")
	}
	if !strings.Contains(m.View(), "turtle ⚑") {
		t.Errorf("saved repo should be marked:\n%s", m.View())
	}

	// B opens the saved view with only the saved repo.
	m = send(t, m, "B")
	if !m.savedView || !slices.Equal(visibleNames(m), []string{"turtle"}) {
		t.Fatalf("saved view = %v, %v", m.savedView, visibleNames(m))
	}
	if !strings.Contains(m.View(), "saved just now") {
		t.Errorf("saved view should show when it was saved:\n%s", m.View())
	}

	// b in the saved view unsaves; esc goes back instead of quitting.
	m = send(t, m, "b")
	if store.Has("acme/turtle") || len(m.list.Items()) != 0 {
		t.Error("b in saved view should remove the repo")
	}
	if !strings.Contains(m.View(), "Nothing saved yet") {
		t.Errorf("expected empty state:\n%s", m.View())
	}
	next, cmd := m.Update(press("esc"))
	m = next.(Model)
	if m.savedView {
		t.Error("esc should leave the saved view")
	}
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Error("esc in saved view must not quit")
		}
	}
	if !slices.Equal(visibleNames(m), []string{"rocket", "turtle"}) {
		t.Errorf("back in trending view, got %v", visibleNames(m))
	}
}
