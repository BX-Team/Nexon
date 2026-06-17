package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userAddCmd, userListCmd, userShowCmd, userSetCmd,
		userDisableCmd, userEnableCmd, userResetCmd, userRmCmd, userSubCmd, userDevicesCmd)

	userAddCmd.Flags().String("data-limit", "", "data limit, e.g. 100G (0/empty = unlimited)")
	userAddCmd.Flags().String("expire", "", "expiry, e.g. 30d or 720h")
	userAddCmd.Flags().Int("hwid-limit", 0, "device (HWID) limit (0 = unlimited)")
	userAddCmd.Flags().String("reset", "no_reset", "traffic reset strategy: no_reset|day|week|month")

	userListCmd.Flags().String("status", "", "filter by status: active|disabled|limited|expired")
	userListCmd.Flags().Bool("json", false, "output JSON")

	userSetCmd.Flags().String("data-limit", "", "new data limit, e.g. 200G")
	userSetCmd.Flags().String("expire", "", "new expiry, e.g. +15d")
	userSetCmd.Flags().Int("hwid-limit", -1, "new device limit (-1 = leave unchanged)")
}

var userCmd = &cobra.Command{Use: "user", Short: "Manage users"}

var userAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dl, _ := cmd.Flags().GetString("data-limit")
		bytes, err := parseSize(dl)
		if err != nil {
			return err
		}
		exp, _ := cmd.Flags().GetString("expire")
		var expireAt *time.Time
		if exp != "" {
			expireAt, err = core.ParseDuration(exp, time.Now())
			if err != nil {
				return err
			}
		}
		hwid, _ := cmd.Flags().GetInt("hwid-limit")
		reset, _ := cmd.Flags().GetString("reset")
		u, err := svc.AddUser(core.CreateUserParams{
			Username: args[0], DataLimit: bytes, ExpireAt: expireAt, HWIDLimit: hwid, ResetStrategy: reset,
		})
		if err != nil {
			return err
		}
		fmt.Printf("created user %q\n", u.Username)
		fmt.Printf("sub: %s/sub/%s\n", strings.TrimRight(cfg.SubBaseURL, "/"), u.SubToken)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		users, err := svc.ListUsers(status)
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return json.NewEncoder(os.Stdout).Encode(users)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "USER\tSTATUS\tUSED\tLIMIT\tEXPIRES")
		for _, u := range users {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", u.Username, u.Status,
				humanBytes(u.UsedTraffic), limitStr(u.DataLimit), expireStr(u.ExpireAt))
		}
		return tw.Flush()
	},
}

var userShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := svc.GetUser(args[0])
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(u, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var userSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Update a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var p core.SetUserParams
		if cmd.Flags().Changed("data-limit") {
			dl, _ := cmd.Flags().GetString("data-limit")
			b, err := parseSize(dl)
			if err != nil {
				return err
			}
			p.DataLimit = &b
		}
		if cmd.Flags().Changed("expire") {
			exp, _ := cmd.Flags().GetString("expire")
			t, err := core.ParseDuration(exp, time.Now())
			if err != nil {
				return err
			}
			p.ExpireAt = t
		}
		if h, _ := cmd.Flags().GetInt("hwid-limit"); h >= 0 {
			p.HWIDLimit = &h
		}
		if _, err := svc.SetUser(args[0], p); err != nil {
			return err
		}
		fmt.Printf("updated %q\n", args[0])
		return nil
	},
}

var userDisableCmd = &cobra.Command{
	Use: "disable <name>", Short: "Disable a user", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.SetStatus(args[0], store.StatusDisabled), args[0], "disabled")
	},
}

var userEnableCmd = &cobra.Command{
	Use: "enable <name>", Short: "Enable a user", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.SetStatus(args[0], store.StatusActive), args[0], "enabled")
	},
}

var userResetCmd = &cobra.Command{
	Use: "reset-traffic <name>", Short: "Reset a user's traffic", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.ResetTraffic(args[0]), args[0], "traffic reset")
	},
}

var userRmCmd = &cobra.Command{
	Use: "rm <name>", Short: "Delete a user", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.DeleteUser(args[0]), args[0], "deleted")
	},
}

var userSubCmd = &cobra.Command{
	Use: "sub <name>", Short: "Print subscription link + QR", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := svc.GetUser(args[0])
		if err != nil {
			return err
		}
		url := fmt.Sprintf("%s/sub/%s", strings.TrimRight(cfg.SubBaseURL, "/"), u.SubToken)
		fmt.Println(url)
		qr, err := qrcode.New(url, qrcode.Low)
		if err == nil {
			fmt.Println(qr.ToSmallString(false))
		}
		return nil
	},
}

var userDevicesCmd = &cobra.Command{
	Use: "devices <name>", Short: "List a user's devices", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := svc.Devices(args[0])
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tHWID\tUSER-AGENT\tLAST-SEEN\tREVOKED")
		for _, d := range devices {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%v\n", d.ID, orDash(d.HWID), truncate(d.UserAgent, 30),
				d.LastSeen.Format("2006-01-02 15:04"), d.Revoked)
		}
		return tw.Flush()
	},
}

func statusMsg(err error, name, action string) error {
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", name, action)
	return nil
}
