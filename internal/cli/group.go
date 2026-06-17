package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(groupCmd)
	groupCmd.AddCommand(groupListCmd, groupAddCmd, groupRmCmd, groupAssignUserCmd, groupAssignNodeCmd)
}

var groupCmd = &cobra.Command{Use: "group", Short: "Manage node groups (route users to a subset of nodes)"}

var groupListCmd = &cobra.Command{
	Use: "list", Short: "List node groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		groups, err := svc.ListNodeGroups()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tDEFAULT")
		for _, g := range groups {
			fmt.Fprintf(tw, "%d\t%s\t%v\n", g.ID, g.Name, g.IsDefault)
		}
		return tw.Flush()
	},
}

var groupAddCmd = &cobra.Command{
	Use: "add <name>", Short: "Create a node group", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := svc.CreateNodeGroup(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("created group %q (#%d)\n", g.Name, g.ID)
		return nil
	},
}

var groupRmCmd = &cobra.Command{
	Use: "rm <id>", Short: "Delete a node group (its nodes/users revert to default)", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q", args[0])
		}
		return statusMsg(svc.DeleteNodeGroup(id), args[0], "group deleted")
	},
}

var groupAssignUserCmd = &cobra.Command{
	Use: "assign-user <username> <group_id|default>", Short: "Move a user into a node group", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := svc.GetUser(args[0])
		if err != nil {
			return err
		}
		gid, err := parseGroupArg(args[1])
		if err != nil {
			return err
		}
		if err := svc.SetUserGroup(u.ID, gid); err != nil {
			return err
		}
		fmt.Printf("user %s → group %s\n", u.Username, args[1])
		return nil
	},
}

var groupAssignNodeCmd = &cobra.Command{
	Use: "assign-node <node> <group_id|default>", Short: "Move a node into a group", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := svc.GetNode(args[0])
		if err != nil {
			return err
		}
		gid, err := parseGroupArg(args[1])
		if err != nil {
			return err
		}
		if err := svc.SetNodeGroup(n.ID, gid); err != nil {
			return err
		}
		fmt.Printf("node %s → group %s\n", n.Name, args[1])
		return nil
	},
}

// parseGroupArg turns "default" / "0" into nil (default group) or a group id.
func parseGroupArg(s string) (*int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "default" || s == "0" {
		return nil, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid group %q (use an id or 'default')", s)
	}
	return &id, nil
}
