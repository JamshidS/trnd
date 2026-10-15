package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamshids/trnd/internal/trending"
)

func TestHistoryComparisons(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	opts := trending.Options{Language: "go", Since: trending.Daily}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	repo := func(name string, stars int) trending.Repo {
		return trending.Repo{Owner: "test", Name: name, Stars: stars}
	}
	for i, repos := range [][]trending.Repo{
		{repo("a", 100), repo("b", 200), repo("gone", 50)},
		{repo("a", 105), repo("b", 210)},
		{repo("b", 220), repo("a", 103), repo("new", 10)},
	} {
		at := start.Add(time.Duration(i) * 12 * time.Hour)
		result := &trending.Result{Source: "trending", FetchedAt: at, Repos: repos}
		if err := s.Record(opts, result); err != nil {
			t.Fatal(err)
		}
		// Re-recording a timestamp must not create another observation.
		if err := s.Record(opts, result); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Read(opts, start, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Snapshots != 3 || len(r.Entries) != 4 {
		t.Fatalf("unexpected report: %+v", r)
	}
	b, a, newRepo, gone := r.Entries[0], r.Entries[1], r.Entries[2], r.Entries[3]
	if b.Name != "test/b" || b.RankChange == nil || *b.RankChange != 1 || b.DaysSeen != 2 || b.StarChange != 20 {
		t.Fatalf("bad b: %+v", b)
	}
	if a.RankChange == nil || *a.RankChange != -1 || a.StarChange != 3 {
		t.Fatalf("bad a: %+v", a)
	}
	if newRepo.RankChange != nil || len(newRepo.Points) != 1 {
		t.Fatalf("bad new repo: %+v", newRepo)
	}
	if gone.Rank != 0 || gone.RankChange != nil {
		t.Fatalf("bad absent repo: %+v", gone)
	}
	r, err = s.Read(opts, start.Add(24*time.Hour), "TEST/A")
	if err != nil || r.Snapshots != 1 || len(r.Entries) != 1 || r.Entries[0].RankChange != nil {
		t.Fatalf("cutoff/name: %+v, %v", r, err)
	}
}

func TestFeedIsolationAndFallback(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	opts := trending.Options{Language: "Go", Since: trending.Daily, Topics: []string{"cli"}}
	res := &trending.Result{Source: "trending", FetchedAt: time.Now(), Repos: []trending.Repo{{Owner: "a", Name: "b", Stars: 10}}}
	if err := s.Record(opts, res); err != nil {
		t.Fatal(err)
	}
	opts.Language, opts.Topics = "go", nil
	r, err := s.Read(opts, time.Time{}, "")
	if err != nil || r.Snapshots != 1 {
		t.Fatalf("normalized feed: %+v %v", r, err)
	}
	for _, other := range []trending.Options{
		{Language: "rust", Since: trending.Daily},
		{Language: "go", Since: trending.Weekly},
		{Language: "go", Since: trending.Daily, SpokenLanguage: "en"},
	} {
		r, err := s.Read(other, time.Time{}, "")
		if err != nil || r.Snapshots != 0 {
			t.Fatalf("mixed feeds: %+v %v", r, err)
		}
	}
	res.Source, res.FetchedAt = "search", time.Now().Add(time.Hour)
	if err := s.Record(opts, res); err != nil {
		t.Fatal(err)
	}
	r, err = s.Read(opts, time.Time{}, "")
	if err != nil || r.Snapshots != 1 {
		t.Fatalf("fallback recorded: %+v %v", r, err)
	}
}

func TestCorruptHistory(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	opts := trending.Options{Since: trending.Daily}
	if err := os.MkdirAll(s.feedDir(opts), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.feedDir(opts), "broken.json"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(opts, time.Time{}, ""); err == nil {
		t.Fatal("expected corruption error")
	}
}

func TestSparkline(t *testing.T) {
	for _, tc := range []struct {
		values []int
		want   string
	}{
		{nil, ""}, {[]int{7}, "▁"}, {[]int{7, 7}, "▁▁"}, {[]int{0, 7, 3}, "▁█▄"}, {[]int{7, 0}, "█▁"},
	} {
		var points []Point
		for _, stars := range tc.values {
			points = append(points, Point{Stars: stars})
		}
		if got := Sparkline(points); got != tc.want {
			t.Errorf("%v: %q != %q", tc.values, got, tc.want)
		}
	}
	points := make([]Point, 100)
	for i := range points {
		points[i].Stars = i
	}
	if got := []rune(Sparkline(points)); len(got) != 24 || got[0] != '▁' || got[23] != '█' {
		t.Fatalf("bad downsampling: %s", string(got))
	}
}
