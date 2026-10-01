// Package cli wires the commands together.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jamshids/trnd/internal/bookmarks"
	"github.com/jamshids/trnd/internal/cache"
	"github.com/jamshids/trnd/internal/github"
	"github.com/jamshids/trnd/internal/output"
	"github.com/jamshids/trnd/internal/style"
	"github.com/jamshids/trnd/internal/trending"
	"github.com/jamshids/trnd/internal/tui"
)

const (
	cacheTTL  = 15 * time.Minute
	topicsTTL = 7 * 24 * time.Hour // topics rarely change
)

type flags struct {
	lang    string
	since   string
	spoken  string
	topic   string
	sort    string
	noCache bool
	limit   int
	json    bool
}

// Execute runs the root command.
func Execute(version string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return newRoot(version).ExecuteContext(ctx)
}

func newRoot(version string) *cobra.Command {
	f := &flags{}

	root := &cobra.Command{
		Use:   "trnd",
		Short: "Browse GitHub trending repositories from your terminal",
		Long: `Browse GitHub trending repositories from your terminal.

Run without arguments for the interactive UI, or use "trnd list" for
plain output you can pipe into other tools.

A GitHub token is optional. It is read from GITHUB_TOKEN, GH_TOKEN or the
gh CLI, and gives higher API rate limits (useful with --topic) and lets
you star repos.`,
		Example: `  trnd                          # interactive UI
  trnd -l go -s weekly          # Go repos trending this week
  trnd list --topic llm,agents  # only repos with these topics
  trnd list --sort gained -n 10 # biggest gainers first
  trnd saved                    # your bookmarks`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, sortBy, err := f.options()
			if err != nil {
				return err
			}
			// Not a terminal (piped, CI): print the list instead of a UI.
			if !isTerminal() {
				return runList(cmd.Context(), f, opts, sortBy)
			}
			store, err := bookmarks.Open(bookmarks.DefaultPath())
			if err != nil {
				return err
			}
			token := github.Token()
			return tui.Run(tui.New(fetcher(token, f.noCache), github.NewClient(token), store, opts, sortBy))
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&f.lang, "lang", "l", "", "programming language, e.g. go, rust, python (default all)")
	pf.StringVarP(&f.since, "since", "s", "daily", "time range: daily, weekly or monthly")
	pf.StringVar(&f.spoken, "spoken", "", "spoken language code of the repo, e.g. en, zh")
	pf.StringVarP(&f.topic, "topic", "t", "", "only repos with any of these topics, comma-separated, e.g. llm,cli")
	pf.StringVar(&f.sort, "sort", "trending", "sort by: trending, gained, stars, forks or name")
	pf.BoolVar(&f.noCache, "no-cache", false, "always fetch fresh data")
	complete := func(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return values, cobra.ShellCompDirectiveNoFileComp
		}
	}
	_ = root.RegisterFlagCompletionFunc("since", complete("daily", "weekly", "monthly"))
	_ = root.RegisterFlagCompletionFunc("sort", complete("trending", "gained", "stars", "forks", "name"))

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Print trending repositories (non-interactive)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, sortBy, err := f.options()
			if err != nil {
				return err
			}
			return runList(cmd.Context(), f, opts, sortBy)
		},
	}
	list.Flags().IntVarP(&f.limit, "limit", "n", 25, "maximum number of repos to show")
	list.Flags().BoolVar(&f.json, "json", false, "output JSON")

	root.AddCommand(list, newSavedCmd(f))
	root.SetVersionTemplate("trnd {{.Version}}\n")
	return root
}

func newSavedCmd(f *flags) *cobra.Command {
	var asJSON bool
	saved := &cobra.Command{
		Use:     "saved",
		Aliases: []string{"bookmarks", "bm"},
		Short:   "List your saved repositories",
		Long: `List your saved repositories.

Save repos with b in the interactive UI, or with "trnd saved add".
Use --sort to reorder (the default is most recently saved first).`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			sortBy, err := trending.ParseSort(f.sort)
			if err != nil {
				return err
			}
			store, err := bookmarks.Open(bookmarks.DefaultPath())
			if err != nil {
				return err
			}
			all := sortBookmarks(store.All(), sortBy)
			if asJSON {
				return output.JSON(os.Stdout, all)
			}
			output.Saved(os.Stdout, all, store.Path(), termWidth())
			return nil
		},
	}
	saved.Flags().BoolVar(&asJSON, "json", false, "output JSON")

	add := &cobra.Command{
		Use:     "add owner/repo...",
		Short:   "Save repositories",
		Example: "  trnd saved add charmbracelet/bubbletea\n  trnd saved add https://github.com/spf13/cobra",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := bookmarks.Open(bookmarks.DefaultPath())
			if err != nil {
				return err
			}
			gh := github.NewClient(github.Token())
			for _, arg := range args {
				repo, err := gh.Repo(cmd.Context(), arg)
				if err != nil {
					return err
				}
				if err := store.Add(repo); err != nil {
					return err
				}
				fmt.Println(style.Gained.Render("Saved ") + repo.FullName())
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use:     "rm owner/repo...",
		Aliases: []string{"remove"},
		Short:   "Remove saved repositories",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := bookmarks.Open(bookmarks.DefaultPath())
			if err != nil {
				return err
			}
			for _, arg := range args {
				name := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(arg, "https://"), "github.com/"), "/")
				ok, err := store.Remove(name)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("%s is not saved", name)
				}
				fmt.Println(style.Dim.Render("Removed ") + name)
			}
			return nil
		},
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			store, err := bookmarks.Open(bookmarks.DefaultPath())
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var names []string
			for _, b := range store.All() {
				names = append(names, b.FullName())
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
	}

	saved.AddCommand(add, rm)
	return saved
}

// sortBookmarks keeps saved order for "trending" and sorts otherwise.
func sortBookmarks(all []bookmarks.Bookmark, by trending.SortBy) []bookmarks.Bookmark {
	repos := make([]trending.Repo, len(all))
	byName := make(map[string]bookmarks.Bookmark, len(all))
	for i, b := range all {
		repos[i] = b.Repo
		byName[b.FullName()] = b
	}
	out := make([]bookmarks.Bookmark, 0, len(all))
	for _, r := range trending.Sort(repos, by) {
		out = append(out, byName[r.FullName()])
	}
	return out
}

func (f *flags) options() (trending.Options, trending.SortBy, error) {
	since, err := trending.ParseSince(f.since)
	if err != nil {
		return trending.Options{}, "", err
	}
	sortBy, err := trending.ParseSort(f.sort)
	if err != nil {
		return trending.Options{}, "", err
	}
	return trending.Options{
		Language:       strings.TrimSpace(f.lang),
		Since:          since,
		SpokenLanguage: strings.TrimSpace(f.spoken),
		Topics:         trending.ParseTopics(f.topic),
	}, sortBy, nil
}

func runList(ctx context.Context, f *flags, opts trending.Options, sortBy trending.SortBy) error {
	res, _, err := fetcher(github.Token(), f.noCache)(ctx, opts, false)
	if err != nil {
		return err
	}
	res.Repos = trending.Sort(res.Repos, sortBy)
	if f.limit > 0 && len(res.Repos) > f.limit {
		res.Repos = res.Repos[:f.limit]
	}
	if f.json {
		if res.Warning != "" {
			fmt.Fprintln(os.Stderr, "Warning: "+res.Warning)
		}
		return output.JSON(os.Stdout, res.Repos)
	}
	output.Pretty(os.Stdout, res, opts, sortBy, termWidth())
	return nil
}

// fetcher returns a cached fetch function shared by the CLI and TUI. When
// opts.Topics is set it also looks up each repo's topics and filters by them.
func fetcher(token string, noCache bool) tui.FetchFunc {
	client := trending.NewClient(token)
	gh := github.NewClient(token)
	listCache := cache.New(cacheTTL)
	topicCache := cache.New(topicsTTL)
	return func(ctx context.Context, opts trending.Options, refresh bool) (*trending.Result, bool, error) {
		key := fmt.Sprintf("v1|%s|%s|%s", strings.ToLower(opts.Language), opts.Since, opts.SpokenLanguage)
		var (
			res    *trending.Result
			cached bool
		)
		var hit trending.Result
		if !noCache && !refresh && listCache.Get(key, &hit) {
			res, cached = &hit, true
		} else {
			fresh, err := client.Fetch(ctx, opts)
			if err != nil {
				return nil, false, err
			}
			listCache.Set(key, fresh)
			res = fresh
		}

		if len(opts.Topics) == 0 {
			return res, cached, nil
		}
		enriched, err := gh.EnrichTopics(ctx, res.Repos, topicCache)
		if err != nil {
			known := 0
			for _, r := range enriched {
				if r.Topics != nil {
					known++
				}
			}
			if known == 0 {
				return nil, false, fmt.Errorf("could not load topics: %w", err)
			}
			res.Warning = fmt.Sprintf("topics unknown for %d of %d repos (%v)", len(enriched)-known, len(enriched), err)
		}
		res.Repos = trending.FilterTopics(enriched, opts.Topics)
		return res, cached, nil
	}
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 100
}
