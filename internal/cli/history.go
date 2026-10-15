package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamshids/trnd/internal/history"
	"github.com/jamshids/trnd/internal/output"
	"github.com/spf13/cobra"
)

func newHistoryCmd(f *flags) *cobra.Command {
	var days int
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "history [owner/repo]",
		Short:   "Show locally recorded trend history (no network requests)",
		Long:    "Show local observations for the selected language, period and spoken-language feed.\nHistory starts with fresh trending fetches; cached and Search API results are excluded.\nRanks always refer to the complete feed, before topic filtering or sorting.",
		Example: "  trnd history -l go -s weekly\n  trnd history charmbracelet/bubbletea --days 90\n  trnd history --json",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if days < 1 || days > 36500 {
				return fmt.Errorf("--days must be between 1 and 36500")
			}
			opts, _, err := f.options()
			if err != nil {
				return err
			}
			if len(opts.Topics) > 0 {
				return fmt.Errorf("history uses complete feeds; select --lang, --since or --spoken instead of --topic")
			}
			if cmd.Flags().Changed("sort") {
				return fmt.Errorf("history is ordered by latest rank; --sort is not supported")
			}
			name := ""
			if len(args) > 0 {
				name = strings.Trim(args[0], "/")
				parts := strings.Split(name, "/")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					return fmt.Errorf("repository must be in owner/repo form")
				}
			}
			report, err := history.Default().Read(opts, time.Now().Add(-time.Duration(days)*24*time.Hour), name)
			if err != nil {
				return err
			}
			if asJSON {
				return output.JSON(cmd.OutOrStdout(), report)
			}
			output.History(cmd.OutOrStdout(), report, name != "")
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "look back this many days")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON including all observation points")
	return cmd
}
