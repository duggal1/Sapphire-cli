package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/spf13/cobra"
)

func newSubAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sub-agents [limit]",
		Aliases: []string{"subagents", "subagent-limit"},
		Short:   "Get or set the maximum concurrent sub-agents per session",
		Long:    "Persist the maximum concurrent sub-agents the main agent may run at once for this Sapphire config.",
		Example: `
# Show the current sub-agent cap
sapphire sub-agents

# Persist a cap of 20 concurrent sub-agents
sapphire sub-agents 20
`,
		Args: cobra.MaximumNArgs(1),
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

			if len(args) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Sub-agent limit: %d\n", cfg.Options.AgentMaxThreads)
				fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", config.GlobalConfigData())
				return nil
			}

			limit, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				return fmt.Errorf("sub-agent limit must be a positive integer")
			}
			if err := cfg.SetAgentMaxThreads(limit); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved sub-agent limit: %d\n", limit)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", config.GlobalConfigData())
			return nil
		},
	}

	return cmd
}

func init() {
	rootCmd.AddCommand(newSubAgentsCmd())
}
