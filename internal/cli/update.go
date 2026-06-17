package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

// installerURL is the canonical install script used by `nexon update`.
const installerURL = "https://raw.githubusercontent.com/BX-Team/Nexon/main/scripts/install.sh"

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Nexon to the latest release (re-runs the installer)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("`nexon update` is only for systemd Linux installs; on %s rebuild from source", runtime.GOOS)
		}
		// Delegate to the install script (handles root check, download, and service restart).
		sh := exec.Command("bash", "-c", "curl -fsSL "+installerURL+" | bash -s update")
		sh.Stdin, sh.Stdout, sh.Stderr = os.Stdin, os.Stdout, os.Stderr
		return sh.Run()
	},
}
