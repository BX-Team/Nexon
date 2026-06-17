package cli

import (
	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/tui"
)

func init() {
	rootCmd.AddCommand(tuiCmd)
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive terminal cockpit (dashboard, users, nodes, groups, clients, templates, settings)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(svc, cfg.SubBaseURL)
	},
}
