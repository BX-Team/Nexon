package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X .../cli.Version=...".
var Version = "dev"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the Nexon version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nexon %s\n", Version)
		},
	})
	rootCmd.Version = Version
}
