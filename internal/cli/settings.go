package cli

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(settingsCmd)
	settingsCmd.AddCommand(settingsSetCmd, settingsGetCmd, ruleCmd)
	ruleCmd.AddCommand(ruleAddCmd, ruleListCmd)
}

var settingsCmd = &cobra.Command{Use: "settings", Short: "Manage settings and detection rules"}

var settingsSetCmd = &cobra.Command{
	Use: "set <key> <value>", Short: "Set a setting", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.Store().SetSetting(args[0], args[1]), args[0], "set")
	},
}

var settingsGetCmd = &cobra.Command{
	Use: "get <key>", Short: "Get a setting", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := svc.Store().GetSetting(args[0])
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}

var ruleCmd = &cobra.Command{Use: "rule", Short: "Manage UA→format detection rules"}

var ruleAddCmd = &cobra.Command{
	Use:   "add <regex> <format>",
	Short: "Add a detection rule (lower priority runs first)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		prio, _ := cmd.Flags().GetInt("priority")
		return statusMsg(svc.Store().AddSubRule(prio, args[0], args[1]), args[0], "rule added")
	},
}

var ruleListCmd = &cobra.Command{
	Use: "list", Short: "List detection rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		rules, err := svc.Store().ListSubRules()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "PRIORITY\tFORMAT\tREGEX")
		for _, r := range rules {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", strconv.Itoa(r.Priority), r.Format, r.Regex)
		}
		return tw.Flush()
	},
}

func init() {
	ruleAddCmd.Flags().Int("priority", 50, "rule priority (lower runs first)")
}
