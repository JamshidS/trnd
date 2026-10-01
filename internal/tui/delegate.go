package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jamshids/trnd/internal/style"
	"github.com/jamshids/trnd/internal/trending"
)

// item adapts a trending.Repo to list.Item.
type item struct {
	repo    trending.Repo
	since   trending.Since
	saved   bool      // bookmarked
	savedAt time.Time // set in the saved view
}

func (i item) FilterValue() string {
	return i.repo.FullName() + " " + i.repo.Language + " " + i.repo.Description + " " + strings.Join(i.repo.Topics, " ")
}

var (
	bar       = lipgloss.NewStyle().Foreground(style.Accent).Render("│")
	selName   = lipgloss.NewStyle().Bold(true).Foreground(style.Accent)
	savedMark = lipgloss.NewStyle().Foreground(style.Accent).Render(" ⚑")
)

// delegate renders each repo as two lines: name + stats, then description
// (prefixed by topics when known).
type delegate struct{}

func (delegate) Height() int                         { return 2 }
func (delegate) Spacing() int                        { return 1 }
func (delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}
	r := it.repo
	selected := index == m.Index()

	prefix := "  "
	name := style.Owner.Render(r.Owner+"/") + style.Name.Render(r.Name)
	if selected {
		prefix = bar + " "
		name = style.Owner.Render(r.Owner+"/") + selName.Render(r.Name)
	}
	if it.saved && it.savedAt.IsZero() {
		name += savedMark // mark saved repos in the trending view
	}

	meta := []string{style.Stars.Render("★ " + style.Count(r.Stars))}
	switch {
	case !it.savedAt.IsZero():
		meta = append(meta, style.Dim.Render("saved "+style.Ago(it.savedAt)))
	case r.StarsPeriod > 0:
		meta = append(meta, style.Gained.Render(fmt.Sprintf("+%s %s", style.Count(r.StarsPeriod), it.since.PeriodLabel())))
	}
	if l := style.Language(r.Language); l != "" {
		meta = append(meta, l)
	}

	width := m.Width() - 4
	desc := r.Description
	if desc == "" {
		desc = "No description"
	}
	second := style.Dim.Render(style.Truncate(desc, width))
	if len(r.Topics) > 0 {
		topics := style.Truncate(style.Topics(r.Topics), width/2)
		second = style.Topic.Render(topics) + "  " +
			style.Dim.Render(style.Truncate(desc, width-lipgloss.Width(topics)-2))
	}

	fmt.Fprintf(w, "%s%s  %s\n%s%s",
		prefix, name, strings.Join(meta, style.Dim.Render(" · ")),
		prefix, second)
}
