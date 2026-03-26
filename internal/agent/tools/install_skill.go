package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/skillsmp"
)

type InstallSkillParams struct {
	Query string `json:"query" description:"Short domain query for the best matching SkillsMP skill"`
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

			apiKey := strings.TrimSpace(os.Getenv("SKILLSMP_API_KEY"))
			if apiKey == "" {
				return fantasy.NewTextErrorResponse("SKILLSMP_API_KEY is required"), nil
			}

			client := skillsmp.NewClient(apiKey)
			skill, err := client.BestMatch(ctx, params.Query)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			body, err := client.FetchRawSkill(ctx, skill)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			dataDir := skillsmp.ResolveDataDir(GetWorkingDirFromContext(ctx))
			if err := skillsmp.NewInstaller(dataDir).Install(skill, body); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			source := skill.OwnerRepo()
			if source != "" {
				source = " from " + source
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Installed %q%s as a skill and plugin. Next: call `load_skill` with %q.", skill.Name, source, skill.Name)), nil
		},
	)
}
