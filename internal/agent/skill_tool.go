package agent

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/tools"
)

//go:embed templates/load_skill.md
var loadSkillToolDescription []byte

type LoadSkillParams struct {
	Name string `json:"name" description:"The name of the skill to load (e.g., 'frontend', 'backend', 'debugging')"`
}

func (c *coordinator) loadSkillTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		tools.LoadSkillToolName,
		string(loadSkillToolDescription),
		func(ctx context.Context, params LoadSkillParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Name == "" {
				return fantasy.NewTextErrorResponse("skill name is required"), nil
			}

			// Discovery happens once and matches are retrieved from the cache.
			c.ensureSkillsDiscovered()

			target := strings.ToLower(params.Name)
			var matchedInstructions string
			var displayName string
			var isInternal bool
			var location string

			for _, s := range c.discoveredSkills {
				sName := strings.ToLower(s.Name)
				sFolder := strings.ToLower(filepath.Base(s.Path))

				if sName == target || sFolder == target || strings.Contains(sName, target) || strings.Contains(sFolder, target) {
					matchedInstructions = s.Instructions
					displayName = s.Name
					isInternal = s.IsInternal
					location = s.SkillFilePath
					if displayName == "" || displayName == "SKILLNAME" {
						displayName = filepath.Base(s.Path)
					}
					break
				}
			}

			if matchedInstructions == "" {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Skill %q not found. Check available skills in the project context.", params.Name)), nil
			}

			// Update the assistant message with the skill name for UI tracking.
			sessionID := tools.GetSessionFromContext(ctx)
			messageID := tools.GetMessageFromContext(ctx)
			if sessionID != "" && messageID != "" {
				msg, err := c.messages.Get(ctx, messageID)
				if err == nil {
					currSkills := []string{displayName}
					if sc := msg.SkillContext(); sc != nil {
						found := false
						for _, existing := range sc.Skills {
							if existing == displayName {
								found = true
								break
							}
						}
						if !found {
							currSkills = append(sc.Skills, displayName)
						} else {
							currSkills = sc.Skills
						}
					}
					msg.SetSkillContext(currSkills)
					_ = c.messages.Update(ctx, msg)
				}
			}

			msg := fmt.Sprintf("Successfully activated internal [System] skill: %s", displayName)
			if !isInternal {
				msg = fmt.Sprintf("Successfully loaded project skill: %s (from %s)", displayName, location)
			}

			return fantasy.ToolResponse{
				Content: fmt.Sprintf("%s\n\n<instructions>\n%s\n</instructions>", msg, matchedInstructions),
			}, nil
		}), nil
}
