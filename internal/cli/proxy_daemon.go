package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch/internal/service"
)

func proxyDaemonCmd() *cobra.Command {
	var (
		port         int
		apiKey       string
		baseURL      string
		models       []string
		defaultModel string
	)

	cmd := &cobra.Command{
		Use:    "proxy-daemon",
		Short:  "Run the translation proxy in foreground (internal use)",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			if apiKey == "" || baseURL == "" {
				fmt.Fprintln(os.Stderr, "proxy-daemon: --api-key and --base-url are required")
				os.Exit(1)
			}

			ps := service.NewProxyDaemon(apiKey, baseURL, models, defaultModel, port)

			pidFile := service.ProxyPIDFile()

			// Graceful shutdown
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				<-sigCh
				ps.Shutdown()
				os.Remove(pidFile)
				os.Exit(0)
			}()

			// ListenAndServe picks the real port and writes the PID file
			if err := ps.ListenAndServe(); err != nil {
				fmt.Fprintf(os.Stderr, "[proxy-daemon] error: %v\n", err)
				os.Remove(pidFile)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Port to listen on (0 = auto)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for upstream")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Upstream base URL")
	cmd.Flags().StringSliceVar(&models, "models", nil, "Available models")
	cmd.Flags().StringVar(&defaultModel, "default-model", "", "Default model")

	return cmd
}
