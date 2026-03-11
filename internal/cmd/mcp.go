package cmd

import (
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers",
}

var mcpSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync MCP servers from the registry",
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

		added, err := config.SyncFromRegistry(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		if added == 0 {
			cmd.Println("No new MCP servers found.")
			return nil
		}

		cmd.Printf("Added %d MCP servers.\n", added)
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpSyncCmd)
	rootCmd.AddCommand(mcpCmd)
}
