package cmd

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/ui/skillsbrowser"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Browse and install SkillsMP skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = args
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		dataDir, err := cmd.Flags().GetString("data-dir")
		if err != nil {
			return err
		}
		cfg, err := config.Load(cwd, dataDir, false)
		if err != nil {
			return err
		}
		apiKey := cfg.ResolveSkillsMPAPIKey()

		styleSet := styles.DefaultStyles(false)
		model := skillsbrowser.New(&styleSet, apiKey, cfg.Options.DataDirectory)
		program := tea.NewProgram(
			model,
			tea.WithContext(cmd.Context()),
		)
		if _, err := program.Run(); err != nil {
			slog.Error("skills browser failed", "error", err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
}
