package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch/internal/api"
)

func serveCmd() *cobra.Command {
	var (
		port    int
		host    string
		release bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web management server",
		Run: func(cobraCmd *cobra.Command, args []string) {
			// Set release mode
			if release {
				gin.SetMode(gin.ReleaseMode)
			} else if os.Getenv("GIN_MODE") == "release" {
				gin.SetMode(gin.ReleaseMode)
			}

			router := api.NewRouter(provSvc.get(), proxySvc, configWr)

			addr := fmt.Sprintf("%s:%d", host, port)
			fmt.Printf("cc-switch-server web panel: http://%s\n", addr)

			// Graceful shutdown on SIGINT/SIGTERM
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				<-sigCh
				fmt.Println("\nshutting down...")
				proxySvc.Stop()
				os.Exit(0)
			}()

			if err := router.Run(addr); err != nil {
				fmt.Fprintf(os.Stderr, "server error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 9876, "Port to listen on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind to")
	cmd.Flags().BoolVar(&release, "release", false, "Run in production (release) mode")

	return cmd
}
