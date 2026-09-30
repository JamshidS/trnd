# trnd

Browse GitHub trending repositories from your terminal. It's a single static binary with no runtime dependencies.

```
trnd                      # interactive UI
trnd -l go -s weekly      # Go repos trending this week
trnd list -n 10           # plain output, top 10 today
trnd list --topic llm,cli     # only repos with these topics
trnd list --sort gained       # biggest gainers first
trnd saved                    # your bookmarks
trnd list --json | jq -r '.[].url'
```

## Install

**Script (Linux / macOS):**
```sh
curl -sSfL https://raw.githubusercontent.com/jamshids/trnd/main/install.sh | sh
```

**Go:**
```sh
go install github.com/jamshids/trnd@latest
```
This installs into `~/go/bin`. If `trnd` isn't found afterwards, add that
folder to your PATH, e.g. `fish_add_path ~/go/bin` in fish or
`export PATH="$HOME/go/bin:$PATH"` in bash/zsh.

**Packages:** `.deb`, `.rpm`, `.apk` and Windows `.zip` files are attached to each [release](https://github.com/jamshids/trnd/releases).

## Interactive keys

| Key | Action |
|---|---|
| `↑`/`↓` or `j`/`k` | move |
| `enter` / `o` | open repo in browser |
| `r` | read the README in the terminal |
| `c` | copy clone URL |
| `s` | star the repo on GitHub (needs a token) |
| `b` | save / unsave the repo (bookmark) |
| `B` | show saved repos (`esc` to go back) |
| `tab` | cycle daily → weekly → monthly |
| `l` | change language |
| `t` | filter by topics, e.g. `llm, cli` |
| `S` | cycle sort: trending → gained → stars → forks → name |
| `/` | search the list |
| `R` | refresh (skip cache) |
| `?` | all keys |
| `q` | quit |

## Flags

| Flag | Description |
|---|---|
| `-l, --lang` | language, e.g. `go`, `rust`, `python` |
| `-s, --since` | `daily` (default), `weekly`, `monthly` |
| `--spoken` | spoken language of the repo, e.g. `en`, `zh` |
| `-t, --topic` | only repos with any of these topics, e.g. `llm,cli` |
| `--sort` | `trending` (default), `gained`, `stars`, `forks`, `name` |
| `--no-cache` | always fetch fresh data |
| `list -n, --limit` | max repos to print (default 25) |
| `list --json` | JSON output |

## Bookmarks

```sh
trnd saved                               # list saved repos (newest first)
trnd saved --sort stars --json           # sorted, as JSON
trnd saved add charmbracelet/bubbletea   # save any repo, not just trending ones
trnd saved rm charmbracelet/bubbletea
```

Bookmarks are stored in `~/.local/share/trnd/bookmarks.json` on Linux
(respects `XDG_DATA_HOME`) and in the user config directory on macOS/Windows.

Shell completions: `trnd completion bash|zsh|fish|powershell`.

## How it works

GitHub has no trending API, so `trnd` reads the public
[github.com/trending](https://github.com/trending) page. If that page can't be
parsed, it falls back to the official Search API. The fallback shows the
most-starred repos created in the time range, and says so on screen.

**Topics** aren't on the trending page, so with a topic filter `trnd` looks
each repo up via the GitHub API. Topics are cached for 7 days, so repeat
filters are instant. Without a token GitHub allows 60 lookups an hour, so
set one if you filter by topic a lot. Daily trending lists are short (~25
repos); use `-s weekly` or `-s monthly` for more matches.

Results are cached for 15 minutes in your user cache directory
(`~/.cache/trnd` on Linux).

**GitHub token (optional).** A token is read from `GITHUB_TOKEN` or `GH_TOKEN`,
or from the `gh` CLI if you are logged in. It's only needed to star repos
and for higher API rate limits (topic lookups, READMEs).

**Colors.** Light or dark mode is detected from your terminal. Override with
`TRND_THEME=light` or `TRND_THEME=dark`.

## Development

```sh
make build     # ./bin/trnd
make install   # copy to ~/.local/bin (or PREFIX=/usr/local)
make test
make snapshot  # cross-compile all release artifacts into dist/
```

Releasing: `git tag v0.1.0 && git push --tags`. GitHub Actions runs GoReleaser
and publishes binaries and packages.
