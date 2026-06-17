package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/BX-Team/Nexon/internal/subserver"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the subscription server + traffic poller",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// Subscription server.
		subSrv, err := subserver.New(svc, cfg.SubBaseURL)
		if err != nil {
			return err
		}
		httpSub := &http.Server{Addr: cfg.SubListen, Handler: subSrv.Handler()}
		go func() {
			slog.Info("subscription server listening", "addr", cfg.SubListen)
			if err := httpSub.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("sub server failed", "err", err)
				stop()
			}
		}()

		// Traffic poller.
		go svc.RunPoller(ctx, time.Duration(cfg.TrafficPollInterval)*time.Second)

		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSub.Shutdown(shutdownCtx)
		return nil
	},
}
