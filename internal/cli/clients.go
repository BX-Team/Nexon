package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/store"
)

func init() {
	rootCmd.AddCommand(clientsCmd)
	clientsCmd.AddCommand(clientsListCmd, clientsAddCmd, clientsRmCmd, clientsFormatCmd)

	clientsAddCmd.Flags().IntP("sort", "s", 100, "display order (lower runs first)")
	clientsAddCmd.Flags().Bool("disabled", false, "create the client disabled")
	clientsAddCmd.Flags().StringArray("header", nil, "custom response header KEY:VALUE (repeatable)")
	clientsAddCmd.Flags().String("format", "", "pinned output format: links|base64|clash|clash-meta|singbox|xray (empty = auto)")
}

var clientsCmd = &cobra.Command{Use: "clients", Short: "Manage VPN client apps (User-Agent → custom sub headers)"}

var clientsListCmd = &cobra.Command{
	Use: "list", Short: "List managed client apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := svc.ListClientApps()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tSORT\tON\tNAME\tUA-PATTERN\tFORMAT\tHEADERS")
		for _, a := range apps {
			on := "✓"
			if !a.Enabled {
				on = "✗"
			}
			format := a.Format
			if format == "" {
				format = "auto"
			}
			fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%s\t%d\n", a.ID, a.Sort, on, a.Name, a.UAPattern, format, len(a.Headers))
		}
		return tw.Flush()
	},
}

var clientsAddCmd = &cobra.Command{
	Use:   "add <name> <ua_pattern>",
	Short: "Add a managed client app",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sort, _ := cmd.Flags().GetInt("sort")
		disabled, _ := cmd.Flags().GetBool("disabled")
		rawHeaders, _ := cmd.Flags().GetStringArray("header")
		headers, err := parseHeaderFlags(rawHeaders)
		if err != nil {
			return err
		}
		format, _ := cmd.Flags().GetString("format")
		c := &store.ClientApp{
			Name:      args[0],
			UAPattern: args[1],
			Headers:   headers,
			Enabled:   !disabled,
			Sort:      sort,
			Format:    format,
		}
		if err := svc.CreateClientApp(c); err != nil {
			return err
		}
		fmt.Printf("added client %q (#%d)\n", c.Name, c.ID)
		return nil
	},
}

var clientsFormatCmd = &cobra.Command{
	Use:   "format <id> <format>",
	Short: "Set a client's pinned output format (use 'auto' to clear)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q", args[0])
		}
		apps, err := svc.ListClientApps()
		if err != nil {
			return err
		}
		var app *store.ClientApp
		for _, a := range apps {
			if a.ID == id {
				app = a
				break
			}
		}
		if app == nil {
			return fmt.Errorf("client #%d not found", id)
		}
		format := args[1]
		if format == "auto" {
			format = ""
		}
		app.Format = format
		if err := svc.UpdateClientApp(app); err != nil {
			return err
		}
		fmt.Printf("client %q format → %s\n", app.Name, args[1])
		return nil
	},
}

var clientsRmCmd = &cobra.Command{
	Use: "rm <id>", Short: "Remove a managed client app", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q", args[0])
		}
		return statusMsg(svc.DeleteClientApp(id), args[0], "removed")
	},
}

// parseHeaderFlags turns repeated KEY:VALUE flags into a header map.
func parseHeaderFlags(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	headers := make(map[string]string, len(raw))
	for _, h := range raw {
		k, v, ok := strings.Cut(h, ":")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --header %q (want KEY:VALUE)", h)
		}
		headers[k] = strings.TrimSpace(v)
	}
	return headers, nil
}
