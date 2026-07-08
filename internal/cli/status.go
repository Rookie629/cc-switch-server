package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current active provider",
		Run: func(cmd *cobra.Command, args []string) {
			p, err := provSvc.get().GetActive()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			if p == nil {
				fmt.Println("No active provider. Use 'cc-switch set <name>' to activate one.")
				return
			}

			fmt.Printf("Active Provider: %s\n", p.Name)
			fmt.Printf("  Type:       %s\n", p.Type)
			fmt.Printf("  Base URL:   %s\n", p.BaseURL)
			fmt.Printf("  API Key:    %s****%s\n", p.APIKey[:3], last4(p.APIKey))
			fmt.Printf("  Models:     %v\n", p.Models)
			fmt.Printf("  Default:    %s\n", p.DefaultModel)

			if p.Type != "anthropic" && proxySvc.IsRunning() {
				fmt.Printf("  Proxy:      running on port %d\n", proxySvc.Port())
			}
		},
	}
}

func last4(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[len(s)-4:]
}
