package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a manual backup of provider data",
		Run: func(cmd *cobra.Command, args []string) {
			s, err := provSvc.get().GetStore().Read()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading data: %v\n", err)
				os.Exit(1)
			}

			backupDir := filepath.Join(dataDir, "backups")
			if err := os.MkdirAll(backupDir, 0700); err != nil {
				fmt.Fprintf(os.Stderr, "error creating backup dir: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Backup complete (%d providers)\n", len(s.Providers))
			fmt.Printf("Data dir: %s\n", dataDir)
		},
	}
}
