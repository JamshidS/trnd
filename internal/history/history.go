// Package history stores local observations of GitHub's trending feeds.
package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jamshids/trnd/internal/bookmarks"
	"github.com/jamshids/trnd/internal/trending"
)

type Snapshot struct {
	At    time.Time       `json:"at"`
	Repos []trending.Repo `json:"repos"`
}

type Point struct {
	At    time.Time `json:"at"`
	Rank  int       `json:"rank"`
	Stars int       `json:"stars"`
}

type Entry struct {
	Name       string  `json:"name"`
	Rank       int     `json:"rank"`                  // zero means absent from the latest snapshot
	RankChange *int    `json:"rank_change,omitempty"` // positive means moved up
	DaysSeen   int     `json:"days_seen"`
	StarChange int     `json:"star_change"`
	Points     []Point `json:"points"`
}

type Report struct {
	Language  string         `json:"language"`
	Since     trending.Since `json:"since"`
	Spoken    string         `json:"spoken"`
	Snapshots int            `json:"snapshots"`
	Entries   []Entry        `json:"entries"`
}

type Store struct{ Dir string }

func Default() Store {
	return Store{Dir: filepath.Join(filepath.Dir(bookmarks.DefaultPath()), "history")}
}

func (s Store) feedDir(opts trending.Options) string {
	key := strings.ToLower(strings.TrimSpace(opts.Language)) + "|" + string(opts.Since) + "|" + strings.ToLower(strings.TrimSpace(opts.SpokenLanguage))
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.Dir, hex.EncodeToString(sum[:]))
}

// Record saves the unfiltered, unsorted feed. Each fetch has its own file, so
// simultaneous processes cannot overwrite each other's observations.
func (s Store) Record(opts trending.Options, res *trending.Result) error {
	if res.Source != "trending" || len(res.Repos) == 0 {
		return nil
	}
	if res.FetchedAt.IsZero() {
		return fmt.Errorf("history snapshot has no fetch time")
	}
	dir := s.feedDir(opts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(Snapshot{At: res.FetchedAt.UTC(), Repos: res.Repos})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(dir, res.FetchedAt.UTC().Format("20060102T150405.000000000")+".json"))
}

// Read compares only observations of the same feed within the requested window.
// Topic filtering is deliberately excluded: ranks refer to the original feed.
func (s Store) Read(opts trending.Options, after time.Time, name string) (Report, error) {
	report := Report{Language: opts.Language, Since: opts.Since, Spoken: opts.SpokenLanguage, Entries: []Entry{}}
	files, err := os.ReadDir(s.feedDir(opts))
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	var snapshots []Snapshot
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.feedDir(opts), file.Name()))
		if err != nil {
			return report, err
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			return report, fmt.Errorf("reading history %s: %w", file.Name(), err)
		}
		if !snap.At.Before(after) {
			snapshots = append(snapshots, snap)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].At.Before(snapshots[j].At) })
	report.Snapshots = len(snapshots)
	entries := map[string]*Entry{}
	days := map[string]map[string]bool{}
	previous := map[string]int{}
	for i, snap := range snapshots {
		for rank, repo := range snap.Repos {
			key := strings.ToLower(repo.FullName())
			if name != "" && !strings.EqualFold(name, key) {
				continue
			}
			e := entries[key]
			if e == nil {
				e = &Entry{Name: repo.FullName()}
				entries[key] = e
				days[key] = map[string]bool{}
			}
			e.Points = append(e.Points, Point{At: snap.At, Rank: rank + 1, Stars: repo.Stars})
			days[key][snap.At.UTC().Format("2006-01-02")] = true
			if i == len(snapshots)-2 {
				previous[key] = rank + 1
			}
			if i == len(snapshots)-1 {
				e.Rank = rank + 1
				if p, ok := previous[key]; ok {
					change := p - e.Rank
					e.RankChange = &change
				}
			}
		}
	}
	for key, e := range entries {
		e.DaysSeen = len(days[key])
		e.StarChange = e.Points[len(e.Points)-1].Stars - e.Points[0].Stars
		report.Entries = append(report.Entries, *e)
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		a, b := report.Entries[i], report.Entries[j]
		if a.Rank != b.Rank {
			if a.Rank == 0 {
				return false
			}
			if b.Rank == 0 {
				return true
			}
			return a.Rank < b.Rank
		}
		return a.Name < b.Name
	})
	return report, nil
}

// Sparkline plots at most 24 evenly spaced star observations, oldest first.
func Sparkline(points []Point) string {
	if len(points) == 0 {
		return ""
	}
	lo, hi := points[0].Stars, points[0].Stars
	for _, p := range points {
		lo = min(lo, p.Stars)
		hi = max(hi, p.Stars)
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	n := min(24, len(points))
	var out strings.Builder
	for i := 0; i < n; i++ {
		index := 0
		if n > 1 {
			index = i * (len(points) - 1) / (n - 1)
		}
		level := 0
		if hi > lo {
			level = int(float64(points[index].Stars-lo) * 7 / float64(hi-lo))
		}
		out.WriteRune(bars[level])
	}
	return out.String()
}
