package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch-server/internal/service"
)

func proxyStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proxy-start",
		Short: "Manually start the translation proxy",
		Run: func(cmd *cobra.Command, args []string) {
			if proxySvc.IsRunning() {
				fmt.Printf("Proxy already running on port %d\n", proxySvc.Port())
				return
			}

			p, err := provSvc.get().GetActive()
			if err != nil || p == nil {
				fmt.Fprintln(os.Stderr, "error: no active provider. Set one first with 'cc-switch set <name>'")
				os.Exit(1)
			}

			port, err := service.SpawnProxyDaemon(p.APIKey, p.BaseURL, p.Models, p.DefaultModel)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Proxy started on port %d, forwarding to %s\n", port, p.BaseURL)
		},
	}
}

func proxyStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proxy-stop",
		Short: "Stop the translation proxy",
		Run: func(cmd *cobra.Command, args []string) {
			if !proxySvc.IsRunning() {
				fmt.Println("Proxy is not running.")
				return
			}

			if err := service.KillDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "error stopping proxy: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Proxy stopped.")
		},
	}
}

func proxyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proxy-status",
		Short: "Check proxy status",
		Run: func(cmd *cobra.Command, args []string) {
			if proxySvc.IsRunning() {
				fmt.Printf("Proxy: running on port %d\n", proxySvc.Port())
			} else {
				fmt.Println("Proxy: not running")
			}
		},
	}
}
