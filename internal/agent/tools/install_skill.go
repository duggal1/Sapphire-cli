package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/skillsmp"
)

type InstallSkillParams struct {
	Query string `json:"query" description:"Short domain query for the best matching Sapphire extended skill"`
}

//go:embed install_skill.md
var installSkillDescription []byte

func NewInstallSkillTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		InstallSkillToolName,
		string(installSkillDescription),
		func(ctx context.Context, params InstallSkillParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.Query = strings.TrimSpace(params.Query)
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			apiKey := strings.TrimSpace(os.Getenv("SAPPHIRE_API_KEY"))
			if workingDir := strings.TrimSpace(GetWorkingDirFromContext(ctx)); workingDir != "" {
				if cfg, err := config.Load(workingDir, "", false); err == nil {
					apiKey = cfg.ResolveSapphireAPIKey()
				}
			}
			if apiKey == "" {
				return fantasy.NewTextErrorResponse("Sapphire API key is required"), nil
			}

			client := skillsmp.NewClient(apiKey)
			skill, err := client.BestMatch(ctx, params.Query)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			loaded, err := client.LoadSkill(ctx, skill.SkillID)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			dataDir := skillsmp.ResolveDataDir(GetWorkingDirFromContext(ctx))
			if err := skillsmp.NewInstaller(dataDir).Install(loaded.Skill, []byte(loaded.Markdown)); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			name := loaded.Skill.LocalName()
			return fantasy.NewTextResponse(fmt.Sprintf("Installed %q as a local extended skill and plugin. Next: call `load_skill` with %q.", name, name)), nil
		},
	)
}
