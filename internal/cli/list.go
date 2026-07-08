package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all providers",
		Run: func(cmd *cobra.Command, args []string) {
			providers, err := provSvc.get().List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			if len(providers) == 0 {
				fmt.Println("No providers configured. Use 'cc-switch add' to add one.")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "STATUS\tNAME\tTYPE\tBASE URL\tMODELS")
			for _, p := range providers {
				status := " "
				if p.IsActive {
					status = "*"
				}
				models := p.DefaultModel
				if len(p.Models) > 1 {
					models = fmt.Sprintf("%s (+%d more)", p.DefaultModel, len(p.Models)-1)
				}
				fmt.Fprintf(w, "[%s]\t%s\t%s\t%s\t%s\n", status, p.Name, p.Type, p.BaseURL, models)
			}
			w.Flush()

			// Show proxy status
			if proxySvc.IsRunning() {
				fmt.Printf("\n🔌 Translation proxy running on port %d\n", proxySvc.Port())
			}
		},
	}
}
