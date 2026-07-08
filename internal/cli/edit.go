package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func editCmd() *cobra.Command {
	var (
		newName      string
		providerType string
		apiKey       string
		baseURL      string
		models       []string
		defaultModel string
	)

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cobraCmd *cobra.Command, args []string) {
			updates := make(map[string]interface{})
			if newName != "" {
				updates["name"] = newName
			}
			if providerType != "" {
				updates["type"] = providerType
			}
			if apiKey != "" {
				updates["api_key"] = apiKey
			}
			if baseURL != "" {
				updates["base_url"] = baseURL
			}
			if len(models) > 0 {
				updates["models"] = models
			}
			if defaultModel != "" {
				updates["default_model"] = defaultModel
			}

			if len(updates) == 0 {
				fmt.Println("No updates specified. Use flags to specify what to change.")
				os.Exit(1)
			}

			p, err := provSvc.get().Edit(args[0], updates)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Provider '%s' updated.\n", p.Name)
		},
	}

	cmd.Flags().StringVar(&newName, "name", "", "New name")
	cmd.Flags().StringVar(&providerType, "type", "", "New type")
	cmd.Flags().StringVar(&apiKey, "key", "", "New API key")
	cmd.Flags().StringVar(&baseURL, "url", "", "New base URL")
	cmd.Flags().StringSliceVar(&models, "models", nil, "New model list (comma-separated)")
	cmd.Flags().StringVar(&defaultModel, "default-model", "", "New default model")

	return cmd
}
