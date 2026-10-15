package output

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/jamshids/trnd/internal/history"
)

// History renders an offline report. Detailed reports also list each observation.
func History(w io.Writer, report history.Report, detailed bool) {
	lang := report.Language
	if lang == "" {
		lang = "all languages"
	}
	fmt.Fprintf(w, "Trend history: %s / %s", lang, report.Since)
	if report.Spoken != "" {
		fmt.Fprintf(w, " / spoken: %s", report.Spoken)
	}
	fmt.Fprintf(w, " (%d snapshots)\n", report.Snapshots)
	if len(report.Entries) == 0 {
		fmt.Fprintln(w, "No observations in this window. Browse this feed to build local history; refresh with R or --no-cache.")
		return
	}
	fmt.Fprintln(w, "Ranks compare the latest two snapshots. Days count distinct UTC dates.")
	fmt.Fprintln(w, "Stars show first-to-last observed change; sparklines use observation order, not elapsed time.")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "REPOSITORY\tRANK\tMOVE\tDAYS\tSTARS Δ\tSTAR HISTORY")
	for _, e := range report.Entries {
		rank, move, stars := "—", "—", "—"
		if e.Rank > 0 {
			rank = fmt.Sprint(e.Rank)
		}
		if e.RankChange != nil {
			move = fmt.Sprintf("%+d", *e.RankChange)
		}
		if len(e.Points) > 1 {
			stars = fmt.Sprintf("%+d", e.StarChange)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", e.Name, rank, move, e.DaysSeen, stars, history.Sparkline(e.Points))
	}
	_ = tw.Flush()
	fmt.Fprintln(w, "— means absent from the latest snapshot or insufficient comparison data; positive MOVE means up.")
	if detailed {
		for _, e := range report.Entries {
			fmt.Fprintf(w, "\n%s observations (UTC):\n", e.Name)
			for _, p := range e.Points {
				fmt.Fprintf(w, "%s  rank %d  stars %d\n", p.At.UTC().Format(time.RFC3339), p.Rank, p.Stars)
			}
		}
	}
}
