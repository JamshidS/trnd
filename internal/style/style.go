// Package style holds colors and formatting shared by the CLI and TUI.
package style

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	Accent = lipgloss.AdaptiveColor{Light: "#6f42c1", Dark: "#b392f0"}
	Muted  = lipgloss.AdaptiveColor{Light: "#6a737d", Dark: "#8b949e"}
	Star   = lipgloss.AdaptiveColor{Light: "#b08800", Dark: "#e3b341"}
	Green  = lipgloss.AdaptiveColor{Light: "#22863a", Dark: "#56d364"}
	Red    = lipgloss.AdaptiveColor{Light: "#cb2431", Dark: "#f85149"}

	Title    = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Name     = lipgloss.NewStyle().Bold(true)
	Owner    = lipgloss.NewStyle().Foreground(Muted)
	Dim      = lipgloss.NewStyle().Foreground(Muted)
	Stars    = lipgloss.NewStyle().Foreground(Star)
	Gained   = lipgloss.NewStyle().Foreground(Green).Bold(true)
	Error    = lipgloss.NewStyle().Foreground(Red)
	Selected = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	Topic    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"})
)

// languageColors mirrors GitHub's linguist colors for common languages.
var languageColors = map[string]string{
	"Go": "#00ADD8", "Rust": "#dea584", "Python": "#3572A5", "TypeScript": "#3178c6",
	"JavaScript": "#f1e05a", "Java": "#b07219", "C": "#555555", "C++": "#f34b7d",
	"C#": "#178600", "Ruby": "#701516", "PHP": "#4F5D95", "Swift": "#F05138",
	"Kotlin": "#A97BFF", "Dart": "#00B4AB", "Shell": "#89e051", "Lua": "#000080",
	"Zig": "#ec915c", "Elixir": "#6e4a7e", "Haskell": "#5e5086", "Scala": "#c22d40",
	"HTML": "#e34c26", "CSS": "#663399", "Vue": "#41b883", "Svelte": "#ff3e00",
	"Jupyter Notebook": "#DA5B0B", "Nix": "#7e7eff", "OCaml": "#ef7a08", "Julia": "#a270ba",
	"MDX": "#fcb32c", "Astro": "#ff5a03", "Clojure": "#db5855", "Erlang": "#B83998",
}

// Language renders "● Go" in the language's GitHub color.
func Language(lang string) string {
	if lang == "" {
		return ""
	}
	c, ok := languageColors[lang]
	if !ok {
		c = "#8b949e"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("●") + " " + lang
}

// Count formats 12422 as "12.4k".
func Count(n int) string {
	switch {
	case n >= 1_000_000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "m"
	case n >= 1_000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1_000)) + "k"
	}
	return fmt.Sprint(n)
}

func trimZero(s string) string { return strings.TrimSuffix(s, ".0") }

// Truncate shortens s to at most w display cells, adding an ellipsis.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// InitTheme settles light vs dark once and returns "dark" or "light" for
// glamour. TRND_THEME=light|dark overrides detection; otherwise the
// terminal's answer is used (Bubble Tea already queries it at init, so this
// costs nothing extra).
func InitTheme() string {
	var dark bool
	switch strings.ToLower(os.Getenv("TRND_THEME")) {
	case "light":
		dark = false
	case "dark":
		dark = true
	default:
		dark = lipgloss.HasDarkBackground()
	}
	lipgloss.SetHasDarkBackground(dark)
	if dark {
		return "dark"
	}
	return "light"
}

// Topics renders ["go", "cli"] as "#go #cli".
func Topics(topics []string) string {
	if len(topics) == 0 {
		return ""
	}
	return "#" + strings.Join(topics, " #")
}

// Ago formats how long ago t was, e.g. "just now", "5m ago", "3d ago".
func Ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("Jan 2006")
}
