package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/config"
	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

var (
	cfg     config.Config
	svc     *core.Service
	st      *store.Store
	dbPath  string
	verbose bool
)

// Execute is the CLI entrypoint.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:           "nexon",
	Short:         "Nexon — Xray control-plane (subscriptions, nodes, traffic)",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		cfg = config.Default()
		if dbPath != "" {
			cfg.DBPath = dbPath
		}
		var err error
		st, err = store.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		if err := st.SeedDefaults(); err != nil {
			return err
		}
		// NEXON_NODE_MODE=stub uses logging-only connectors (no live node).
		var factory node.Factory
		if os.Getenv("NEXON_NODE_MODE") == "stub" {
			factory = node.StubFactory
		}
		svc = core.New(st, factory)
		if err := svc.SeedTemplates(); err != nil {
			return err
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if st != nil {
			_ = st.Close()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "path to SQLite database (default from NEXON_DB / data dir)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose (debug) logging")
}
