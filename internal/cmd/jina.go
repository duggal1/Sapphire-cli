package cmd

import (
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	jinaAPIValue string
	jinaClearAPI bool
)

var jinaCmd = &cobra.Command{
	Use:   "jina",
	Short: "Manage Jina embeddings configuration",
	Long:  "Set, clear, or inspect the Jina API key used for codebase indexing.",
	Example: `
# Set the Jina API key
sapphire jina --api jina_xxx

# Clear the stored Jina API key
sapphire jina --clear-api

# Show current Jina key status
sapphire jina
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(jinaAPIValue) != "" && jinaClearAPI {
			return fmt.Errorf("use either --api or --clear-api, not both")
		}

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

		switch {
		case jinaClearAPI:
			if err := cfg.ClearJinaAPIKey(); err != nil {
				return err
			}
			cmd.Println("Cleared Jina API key.")
			cmd.Printf("Config: %s\n", config.GlobalConfigData())
			return nil
		case strings.TrimSpace(jinaAPIValue) != "":
			if err := cfg.SetJinaAPIKey(jinaAPIValue); err != nil {
				return err
			}
			cmd.Println("Saved Jina API key.")
			cmd.Printf("Config: %s\n", config.GlobalConfigData())
			return nil
		default:
			if strings.TrimSpace(cfg.ResolveJinaAPIKey()) == "" {
				cmd.Println("Jina API key: not configured")
			} else {
				cmd.Println("Jina API key: configured")
			}
			cmd.Printf("Config: %s\n", config.GlobalConfigData())
			return nil
		}
	},
}

func init() {
	jinaCmd.Flags().StringVar(&jinaAPIValue, "api", "", "Set the Jina API key used for codebase indexing")
	jinaCmd.Flags().BoolVar(&jinaClearAPI, "clear-api", false, "Clear the stored Jina API key")
}
