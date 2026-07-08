package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Rookie629/cc-switch/internal/service"
	"github.com/Rookie629/cc-switch/internal/store"
)

var (
	dataDir   string
	provSvc   *providerServiceHolder
	configWr  *service.ConfigWriter
	proxySvc  *service.ProxyService
)

type providerServiceHolder struct {
	svc *service.ProviderService
}

func (h *providerServiceHolder) get() *service.ProviderService {
	if h.svc == nil {
		s, err := store.New(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to initialize store: %v\n", err)
			os.Exit(1)
		}
		h.svc = service.NewProviderService(s)
	}
	return h.svc
}

func init() {
	provSvc = &providerServiceHolder{}
	configWr = service.NewConfigWriter()
	proxySvc = service.NewProxyService()
}

// Execute runs the root command.
func Execute() {
	rootCmd := &cobra.Command{
		Use:   "cc-switch",
		Short: "A CLI tool to manage Claude Code AI providers on servers",
		Long: `cc-switch-server is a lightweight CLI + Web tool for managing
Claude Code's AI provider configuration on remote servers.

It supports Anthropic official API, OpenAI-compatible APIs (DeepSeek, etc.),
and includes a built-in format translation proxy.`,
	}

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", store.DefaultDataDir(), "Data directory for providers and backups")

	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(setCmd())
	rootCmd.AddCommand(addCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(editCmd())
	rootCmd.AddCommand(modelsCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(proxyStartCmd())
	rootCmd.AddCommand(proxyStopCmd())
	rootCmd.AddCommand(proxyStatusCmd())
	rootCmd.AddCommand(configPathCmd())
	rootCmd.AddCommand(backupCmd())
	rootCmd.AddCommand(proxyDaemonCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
