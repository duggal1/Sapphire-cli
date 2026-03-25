package cmd

import (
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAPIKeyCmd() *cobra.Command {
	var providerID string
	var apiKey string

	cmd := &cobra.Command{
		Use:   "api-key [provider] [api-key]",
		Short: "Overwrite a provider API key",
		Long: `Overwrite the API key stored for a model provider.

If you pass a single argument, Sapphire treats it as the OpenRouter API key.
If you pass two arguments, Sapphire treats them as provider and API key.
`,
		Example: `
# Overwrite the OpenRouter key with the quick path
sapphire api-key sk-or-123456

# Overwrite the OpenRouter key explicitly
sapphire api-key openrouter sk-or-123456

# Overwrite another provider key
sapphire api-key anthropic sk-ant-123456

# Use flags instead of positional arguments
sapphire api-key --provider openrouter --api-key sk-or-123456
`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := ResolveCwd(cmd)
			if err != nil {
				return err
			}

			dataDir, _ := cmd.Flags().GetString("data-dir")
			debug, _ := cmd.Flags().GetBool("debug")

			cfg, err := config.Init(cwd, dataDir, debug)
			if err != nil {
				return err
			}

			providerID, apiKey, err := resolveAPIKeyOverride(providerID, apiKey, args)
			if err != nil {
				return err
			}

			if err := cfg.SetProviderAPIKey(providerID, apiKey); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s API key updated successfully.\n", humanizeProviderID(providerID))
			return nil
		},
	}

	cmd.Flags().StringVarP(&providerID, "provider", "p", "", "Provider ID to update")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key to store")

	return cmd
}

func resolveAPIKeyOverride(providerFlag, apiKeyFlag string, args []string) (string, string, error) {
	provider := strings.TrimSpace(providerFlag)
	apiKey := strings.TrimSpace(apiKeyFlag)

	switch len(args) {
	case 0:
		if apiKey == "" {
			return "", "", fmt.Errorf("no api key provided")
		}
	case 1:
		if apiKey != "" {
			return "", "", fmt.Errorf("use either a positional api key or --api-key, not both")
		}
		apiKey = strings.TrimSpace(args[0])
		if provider == "" {
			provider = "openrouter"
		}
	case 2:
		if provider != "" || apiKey != "" {
			return "", "", fmt.Errorf("use either positional arguments or flags, not both")
		}
		provider = strings.TrimSpace(args[0])
		apiKey = strings.TrimSpace(args[1])
	default:
		return "", "", fmt.Errorf("expected a provider and api key")
	}

	if provider == "" {
		provider = "openrouter"
	}
	if apiKey == "" {
		return "", "", fmt.Errorf("api key cannot be empty")
	}

	return strings.ToLower(provider), apiKey, nil
}

func humanizeProviderID(providerID string) string {
	if providerID == "" {
		return "Provider"
	}
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "openrouter":
		return "OpenRouter"
	case "github", "github-copilot", "copilot":
		return "GitHub Copilot"
	default:
		return strings.ToUpper(providerID[:1]) + providerID[1:]
	}
}

func init() {
	rootCmd.AddCommand(newAPIKeyCmd())
}
