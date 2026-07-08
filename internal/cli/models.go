package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models <name>",
		Short: "List models for a provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			p, err := provSvc.get().GetByName(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Models for %s:\n", p.Name)
			for _, m := range p.Models {
				marker := " "
				if m == p.DefaultModel {
					marker = "*"
				}
				fmt.Printf("  [%s] %s\n", marker, m)
			}
		},
	}
}
