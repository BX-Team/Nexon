package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/core"
)

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migratePasarguardCmd)
	migratePasarguardCmd.Flags().String("only", "", "import only these usernames (comma-separated)")
	migratePasarguardCmd.Flags().String("skip", "", "skip these usernames (comma-separated)")
	migratePasarguardCmd.Flags().Bool("reset-traffic", false, "zero used traffic instead of carrying it over")
	migratePasarguardCmd.Flags().String("keep-traffic", "", "usernames exempt from --reset-traffic (comma-separated)")
	migratePasarguardCmd.Flags().String("group", "", "node group id for imported users (default group if empty)")
	migratePasarguardCmd.Flags().Bool("dry-run", false, "show what would happen without writing")
}

var migrateCmd = &cobra.Command{Use: "migrate", Short: "Import data from other panels"}

var migratePasarguardCmd = &cobra.Command{
	Use:   "pasarguard <db.sqlite>",
	Short: "Import users and legacy subscription tokens from a PasarGuard database",
	Long: `Copies users (name, limits, traffic, expiry) from a PasarGuard SQLite database
and stores its token secret plus a user-id mapping, so old /sub/ links keep
resolving on Nexon without subscribers doing anything.

If sub.happ.providerid is set (nexon settings set sub.happ.providerid <id>,
from happ-proxy.com), legacy responses also carry a new-url header so Happ
clients silently switch to their native Nexon link.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		only, _ := cmd.Flags().GetString("only")
		skip, _ := cmd.Flags().GetString("skip")
		keep, _ := cmd.Flags().GetString("keep-traffic")
		reset, _ := cmd.Flags().GetBool("reset-traffic")
		group, _ := cmd.Flags().GetString("group")
		dry, _ := cmd.Flags().GetBool("dry-run")

		gid, err := parseGroupArg(group)
		if err != nil {
			return err
		}
		results, err := svc.ImportPasarguard(args[0], core.PasarguardImportOptions{
			Only:         nameSet(only),
			Skip:         nameSet(skip),
			ResetTraffic: reset,
			KeepTraffic:  nameSet(keep),
			GroupID:      gid,
			DryRun:       dry,
		})
		if err != nil {
			return err
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "USER\tLEGACY ID\tACTION\tLIMIT\tUSED\tNOTE")
		var imported, mapped, skipped int
		for _, r := range results {
			limit, used := "", ""
			switch r.Action {
			case "imported":
				imported++
				limit, used = limitStr(r.DataLimit), humanBytes(r.UsedTraffic)
			case "mapped":
				mapped++
			default:
				skipped++
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Username, strconv.FormatInt(r.LegacyID, 10), r.Action, limit, used, r.Detail)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Printf("\nимпортировано %d, привязано %d, пропущено %d", imported, mapped, skipped)
		if dry {
			fmt.Print(" (dry-run, ничего не записано)")
		}
		fmt.Println()
		return nil
	},
}

func nameSet(csv string) map[string]bool {
	set := map[string]bool{}
	for _, n := range strings.Split(csv, ",") {
		if n = strings.TrimSpace(n); n != "" {
			set[n] = true
		}
	}
	return set
}
