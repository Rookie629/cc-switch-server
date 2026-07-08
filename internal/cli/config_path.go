package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-path",
		Short: "Print the data directory path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(filepath.Join(dataDir, "providers.json"))
		},
	}
}
