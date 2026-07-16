package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/core"
)

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeAddCmd, nodeListCmd, nodeShowCmd, nodeRmCmd, nodeSyncCmd, nodeInboundsCmd, inboundCmd)

	inboundCmd.AddCommand(inboundAddCmd, inboundRmCmd, inboundHideCmd, inboundShowCmd)
	inboundAddCmd.Flags().String("tag", "", "inbound tag (must match the tag on the node)")
	inboundAddCmd.Flags().String("protocol", "", "vmess|vless|trojan|shadowsocks")
	inboundAddCmd.Flags().Int("port", 443, "inbound port")
	inboundAddCmd.Flags().String("network", "tcp", "transport: tcp|ws|grpc")
	inboundAddCmd.Flags().String("tls", "", "security: tls|reality (empty = none)")
	inboundAddCmd.Flags().String("settings", "{}", "JSON: sni, host, path, pbk, sid, fp…")
	inboundAddCmd.Flags().String("remark", "", "subscription display name (empty = <node>-<tag>)")
	inboundAddCmd.Flags().Bool("hidden", false, "provision users but keep this inbound out of subscriptions")
	_ = inboundAddCmd.MarkFlagRequired("tag")
	_ = inboundAddCmd.MarkFlagRequired("protocol")

	nodeAddCmd.Flags().String("address", "", "node address (host/IP)")
	nodeAddCmd.Flags().Int("api-port", 8443, "Xray API port")
	_ = nodeAddCmd.MarkFlagRequired("address")
}

var nodeCmd = &cobra.Command{Use: "node", Short: "Manage nodes"}

var nodeAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("address")
		port, _ := cmd.Flags().GetInt("api-port")
		n, err := svc.AddNode(core.AddNodeParams{
			Name: args[0], Address: addr, APIPort: port,
		})
		if err != nil {
			return err
		}
		fmt.Printf("added node %q (%s:%d) status=%s\n", n.Name, n.Address, n.APIPort, n.Status)
		return nil
	},
}

var nodeListCmd = &cobra.Command{
	Use: "list", Short: "List nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes, err := svc.ListNodes()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tADDRESS\tPORT\tSTATUS\tXRAY\tLAST-SEEN")
		for _, n := range nodes {
			ls := "-"
			if n.LastSeen != nil {
				ls = n.LastSeen.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n", n.Name, n.Address, n.APIPort, n.Status, orDash(n.XrayVersion), ls)
		}
		return tw.Flush()
	},
}

var nodeShowCmd = &cobra.Command{
	Use: "show <name>", Short: "Show node + inbounds", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := svc.GetNode(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("name:    %s\naddress: %s:%d\nstatus:  %s\nxray:    %s\n", n.Name, n.Address, n.APIPort, n.Status, orDash(n.XrayVersion))
		ins, _ := svc.Store().ListInbounds(n.ID)
		fmt.Printf("inbounds: %d\n", len(ins))
		for _, in := range ins {
			fmt.Printf("  - %s [%s] :%d %s/%s%s\n", in.Tag, in.Protocol, in.Port, in.Network, in.TLS, hiddenSuffix(in.Hidden))
		}
		return nil
	},
}

var nodeRmCmd = &cobra.Command{
	Use: "rm <name>", Short: "Remove a node", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.DeleteNode(args[0]), args[0], "removed")
	},
}

var nodeSyncCmd = &cobra.Command{
	Use: "sync <name>", Short: "Resync all users to a node", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.SyncNode(args[0]), args[0], "synced")
	},
}

var nodeInboundsCmd = &cobra.Command{
	Use: "inbounds <name>", Short: "List inbounds a node exposes", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := svc.GetNode(args[0])
		if err != nil {
			return err
		}
		ins, err := svc.Store().ListInbounds(n.ID)
		if err != nil {
			return err
		}
		for _, in := range ins {
			fmt.Printf("%s\t%s\t:%d\t%s/%s%s\n", in.Tag, in.Protocol, in.Port, in.Network, in.TLS, hiddenSuffix(in.Hidden))
		}
		return nil
	},
}

func hiddenSuffix(hidden bool) string {
	if hidden {
		return "\t(hidden)"
	}
	return ""
}

var inboundCmd = &cobra.Command{Use: "inbound", Short: "Manage a node's inbounds"}

var inboundAddCmd = &cobra.Command{
	Use:   "add <node>",
	Short: "Add/update an inbound on a node and resync users",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tag, _ := cmd.Flags().GetString("tag")
		proto, _ := cmd.Flags().GetString("protocol")
		port, _ := cmd.Flags().GetInt("port")
		network, _ := cmd.Flags().GetString("network")
		tls, _ := cmd.Flags().GetString("tls")
		settings, _ := cmd.Flags().GetString("settings")
		remark, _ := cmd.Flags().GetString("remark")
		hidden, _ := cmd.Flags().GetBool("hidden")
		in, err := svc.AddInbound(core.AddInboundParams{
			NodeName: args[0], Tag: tag, Protocol: proto, Port: port,
			Network: network, TLS: tls, SettingsJSON: settings, Remark: remark, Hidden: hidden,
		})
		if err != nil {
			return err
		}
		fmt.Printf("inbound %q [%s] added to %s\n", in.Tag, in.Protocol, args[0])
		return nil
	},
}

var inboundRmCmd = &cobra.Command{
	Use:   "rm <node> <tag>",
	Short: "Remove an inbound from a node",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.RemoveInbound(args[0], args[1]), args[1], "inbound removed")
	},
}

var inboundHideCmd = &cobra.Command{
	Use:   "hide <node> <tag>",
	Short: "Hide an inbound from subscriptions (still provisioned)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.SetInboundHidden(args[0], args[1], true), args[1], "inbound hidden")
	},
}

var inboundShowCmd = &cobra.Command{
	Use:   "show <node> <tag>",
	Short: "Show an inbound in subscriptions again",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.SetInboundHidden(args[0], args[1], false), args[1], "inbound shown")
	},
}
