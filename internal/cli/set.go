package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch-server/internal/service"
	"github.com/Rookie629/cc-switch-server/internal/store"
)

func setCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <name>",
		Short: "Switch active provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			// Get the target provider
			p, err := provSvc.get().GetByName(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			// Backup current Claude Code settings
			backupPath, _ := configWr.Backup()
			if backupPath != "" {
				fmt.Printf("Backed up Claude Code settings to %s\n", backupPath)
			}

			// Handle proxy for non-anthropic providers
			baseURL := p.BaseURL
			if p.Type != store.TypeAnthropic {
				port, err := service.SpawnProxyDaemon(p.APIKey, p.BaseURL, p.Models, p.DefaultModel)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to start translation proxy: %v\n", err)
					fmt.Fprintf(os.Stderr, "provider set as active, but translation may not work\n")
				} else {
					baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
					fmt.Printf("Translation proxy started on port %d\n", port)
				}
			} else {
				service.KillDaemon()
			}

			// Write to Claude Code config
			extraEnv := make(map[string]string)
			if p.DefaultModel != "" {
				extraEnv["ANTHROPIC_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_SONNET_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_OPUS_MODEL"] = p.DefaultModel
			}

			if err := configWr.WriteProvider(p.APIKey, baseURL, extraEnv); err != nil {
				fmt.Fprintf(os.Stderr, "error writing claude config: %v\n", err)
				configWr.RestoreBackup()
				os.Exit(1)
			}

			// Mark as active in our store
			if _, err := provSvc.get().SetActive(name); err != nil {
				fmt.Fprintf(os.Stderr, "error updating active provider: %v\n", err)
				configWr.RestoreBackup()
				os.Exit(1)
			}

			fmt.Printf("Switched to provider: %s\n", p.Name)
			fmt.Printf("  Type:     %s\n", p.Type)
			fmt.Printf("  Base URL: %s\n", baseURL)
			fmt.Printf("  Model:    %s\n", p.DefaultModel)
		},
	}
}
