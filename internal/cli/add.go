package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch-server/internal/preset"
)

func addCmd() *cobra.Command {
	var (
		name         string
		providerType string
		apiKey       string
		baseURL      string
		models       []string
		defaultModel string
		usePreset    string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new provider",
		Run: func(cobraCmd *cobra.Command, args []string) {
			// If a preset is specified, pre-fill from it
			if usePreset != "" {
				p := preset.FindPreset(usePreset)
				if p == nil {
					fmt.Fprintf(os.Stderr, "preset '%s' not found. Available presets:\n", usePreset)
					for _, pr := range preset.All() {
						fmt.Fprintf(os.Stderr, "  - %s (%s)\n", pr.Name, pr.Type)
					}
					os.Exit(1)
				}
				if name == "" {
					name = p.Name
				}
				if providerType == "" {
					providerType = string(p.Type)
				}
				if baseURL == "" {
					baseURL = p.BaseURL
				}
				if len(models) == 0 {
					models = p.Models
				}
				if defaultModel == "" {
					defaultModel = p.Default
				}
			}

			// Interactive mode if required fields are missing
			if name == "" || apiKey == "" || baseURL == "" {
				reader := bufio.NewReader(os.Stdin)

				if name == "" {
					fmt.Print("Provider name: ")
					name, _ = reader.ReadString('\n')
					name = strings.TrimSpace(name)
				}
				if apiKey == "" {
					fmt.Print("API Key: ")
					apiKey, _ = reader.ReadString('\n')
					apiKey = strings.TrimSpace(apiKey)
				}
				if baseURL == "" {
					fmt.Print("Base URL: ")
					baseURL, _ = reader.ReadString('\n')
					baseURL = strings.TrimSpace(baseURL)
				}
				if providerType == "" {
					fmt.Print("Type (anthropic/openai_compatible/openai): ")
					providerType, _ = reader.ReadString('\n')
					providerType = strings.TrimSpace(providerType)
					if providerType == "" {
						providerType = "openai_compatible"
					}
				}
			}

			p, err := provSvc.get().Add(name, providerType, apiKey, baseURL, models, defaultModel)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Provider added: %s (id: %s)\n", p.Name, p.ID)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Provider name")
	cmd.Flags().StringVar(&providerType, "type", "", "Provider type: anthropic, openai_compatible, openai")
	cmd.Flags().StringVar(&apiKey, "key", "", "API key")
	cmd.Flags().StringVar(&baseURL, "url", "", "Base URL")
	cmd.Flags().StringSliceVar(&models, "models", nil, "Available models (comma-separated)")
	cmd.Flags().StringVar(&defaultModel, "default-model", "", "Default model")
	cmd.Flags().StringVar(&usePreset, "preset", "", "Use a built-in preset")

	return cmd
}
