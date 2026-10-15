package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jamshids/trnd/internal/history"
	"github.com/jamshids/trnd/internal/trending"
)

func TestHistoryOutput(t *testing.T) {
	var out bytes.Buffer
	report := history.Report{Since: trending.Daily}
	History(&out, report, false)
	if !strings.Contains(out.String(), "No observations") {
		t.Fatal(out.String())
	}
	out.Reset()
	change := 2
	report.Snapshots = 2
	report.Entries = []history.Entry{{Name: "a/b", Rank: 1, RankChange: &change, DaysSeen: 2, StarChange: -5, Points: []history.Point{
		{At: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Rank: 3, Stars: 100},
		{At: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Rank: 1, Stars: 95},
	}}}
	History(&out, report, true)
	for _, want := range []string{"a/b", "+2", "-5", "█▁", "2026-10-01T00:00:00Z  rank 3  stars 100"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q: %s", want, out.String())
		}
	}
}
