// Package output renders trending results for non-interactive use.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jamshids/trnd/internal/bookmarks"
	"github.com/jamshids/trnd/internal/style"
	"github.com/jamshids/trnd/internal/trending"
)

// JSON writes v (repos or bookmarks) as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Pretty writes a colored, numbered list of trending repos sized to width.
func Pretty(w io.Writer, res *trending.Result, opts trending.Options, sortBy trending.SortBy, width int) {
	fmt.Fprintln(w, style.Title.Render(Header(opts, sortBy)))
	if res.Source == "search" {
		fmt.Fprintln(w, style.Dim.Render("(trending page unavailable; showing top new repos from the Search API)"))
	}
	if res.Warning != "" {
		fmt.Fprintln(w, style.Error.Render("Warning: "+res.Warning))
	}
	fmt.Fprintln(w)

	if len(res.Repos) == 0 {
		msg := "No trending repositories found."
		if len(opts.Topics) > 0 {
			msg = "No trending repositories match those topics. Try -s weekly or -s monthly for a bigger pool."
		}
		fmt.Fprintln(w, style.Dim.Render(msg))
		return
	}

	numW := len(fmt.Sprint(len(res.Repos)))
	for i, r := range res.Repos {
		var extra string
		if r.StarsPeriod > 0 {
			extra = style.Gained.Render(fmt.Sprintf("+%s %s", style.Count(r.StarsPeriod), opts.Since.PeriodLabel()))
		}
		writeRepo(w, i, numW, r, extra, width)
		if i < len(res.Repos)-1 {
			fmt.Fprintln(w)
		}
	}
}

// Saved writes the user's bookmarks.
func Saved(w io.Writer, saved []bookmarks.Bookmark, path string, width int) {
	fmt.Fprintln(w, style.Title.Render(fmt.Sprintf("Saved repositories (%d)", len(saved))))
	fmt.Fprintln(w)
	if len(saved) == 0 {
		fmt.Fprintln(w, style.Dim.Render("Nothing saved yet. Press b in the UI, or run: trnd saved add owner/repo"))
		return
	}
	numW := len(fmt.Sprint(len(saved)))
	for i, b := range saved {
		writeRepo(w, i, numW, b.Repo, style.Dim.Render("saved "+style.Ago(b.SavedAt)), width)
		if i < len(saved)-1 {
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, style.Dim.Render("Stored in "+path))
}

// Header describes the current view, e.g.
// "GitHub Trending · weekly · go · #cli · sorted by stars".
func Header(opts trending.Options, sortBy trending.SortBy) string {
	lang := opts.Language
	if lang == "" {
		lang = "all languages"
	}
	parts := []string{"GitHub Trending", string(opts.Since), lang}
	if len(opts.Topics) > 0 {
		parts = append(parts, style.Topics(opts.Topics))
	}
	if sortBy != trending.SortTrending && sortBy != "" {
		parts = append(parts, "sorted by "+string(sortBy))
	}
	return strings.Join(parts, " · ")
}

func writeRepo(w io.Writer, i, numW int, r trending.Repo, extra string, width int) {
	indent := strings.Repeat(" ", numW+2)
	num := style.Dim.Render(fmt.Sprintf("%*d.", numW, i+1))
	name := style.Owner.Render(r.Owner+"/") + style.Name.Render(r.Name)

	meta := []string{style.Stars.Render("★ " + style.Count(r.Stars))}
	if extra != "" {
		meta = append(meta, extra)
	}
	if l := style.Language(r.Language); l != "" {
		meta = append(meta, l)
	}

	fmt.Fprintf(w, "%s %s  %s\n", num, name, strings.Join(meta, style.Dim.Render("  ·  ")))
	if r.Description != "" {
		fmt.Fprintln(w, indent+style.Dim.Render(style.Truncate(r.Description, width-len(indent))))
	}
	if len(r.Topics) > 0 {
		fmt.Fprintln(w, indent+style.Topic.Render(style.Truncate(style.Topics(r.Topics), width-len(indent))))
	}
	fmt.Fprintln(w, indent+style.Dim.Render(r.URL))
}
