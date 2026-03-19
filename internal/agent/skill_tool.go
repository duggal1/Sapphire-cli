package agent

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

//go:embed templates/list_skills.md
var listSkillsToolDescription []byte

//go:embed templates/load_skill.md
var loadSkillToolDescription []byte

type LoadSkillParams struct {
	Name string `json:"name" description:"The name of the skill to load (e.g., 'frontend', 'backend', 'debugging')"`
}

type ListSkillsResponse struct {
	Skills []SkillInfo `json:"skills"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsInternal  bool   `json:"is_internal"`
	Location    string `json:"location,omitempty"`
}

// listSkillsTool returns a tool that lists all available skills with O(1) cache lookup.
func (c *coordinator) listSkillsTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		"list_skills",
		string(listSkillsToolDescription),
		func(ctx context.Context, _ struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Use fast skill loader with caching
			c.ensureSkillsDiscovered()

			if len(c.discoveredSkills) == 0 {
				return fantasy.NewTextResponse("No skills available. Check that skills are installed in the project or system skills directory."), nil
			}

			skillInfos := make([]SkillInfo, 0, len(c.discoveredSkills))
			for _, s := range c.discoveredSkills {
				displayName := s.Name
				if displayName == "" || displayName == "SKILLNAME" {
					displayName = filepath.Base(s.Path)
				}
				skillInfos = append(skillInfos, SkillInfo{
					Name:        displayName,
					Description: s.Description,
					IsInternal:  s.IsInternal,
					Location:    s.SkillFilePath,
				})
			}

			// Sort by name for consistent output
			sort.Slice(skillInfos, func(i, j int) bool {
				return skillInfos[i].Name < skillInfos[j].Name
			})

			var sb strings.Builder
			sb.WriteString("## Available Skills\n\n")
			for _, info := range skillInfos {
				source := "System"
				if !info.IsInternal {
					source = fmt.Sprintf("Project (%s)", info.Location)
				}
				sb.WriteString(fmt.Sprintf("- **%s**: %s [%s]\n", info.Name, info.Description, source))
			}

			return fantasy.NewTextResponse(sb.String()), nil
		}), nil
}

// loadSkillTool returns a tool that loads a specific skill's instructions with O(1) cache lookup.
func (c *coordinator) loadSkillTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		tools.LoadSkillToolName,
		string(loadSkillToolDescription),
		func(ctx context.Context, params LoadSkillParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Name == "" {
				return fantasy.NewTextErrorResponse("skill name is required"), nil
			}

			// Fast O(1) cache lookup - skills already discovered
			c.ensureSkillsDiscovered()

			target := strings.ToLower(params.Name)
			var matchedInstructions string
			var displayName string
			var isInternal bool
			var location string

			// Optimized linear search with early exit
			// Skills are pre-cached, so this is memory-only (no I/O)
			for _, s := range c.discoveredSkills {
				sName := strings.ToLower(s.Name)
				sFolder := strings.ToLower(filepath.Base(s.Path))

				// Exact match first (fast path)
				if sName == target || sFolder == target {
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

			// Fuzzy match if no exact match (still fast, memory-only)
			if matchedInstructions == "" {
				for _, s := range c.discoveredSkills {
					sName := strings.ToLower(s.Name)
					sFolder := strings.ToLower(filepath.Base(s.Path))

					if strings.Contains(sName, target) || strings.Contains(sFolder, target) {
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
			}

			if matchedInstructions == "" {
				availableSkills := make([]string, 0, len(c.discoveredSkills))
				for _, s := range c.discoveredSkills {
					name := s.Name
					if name == "" || name == "SKILLNAME" {
						name = filepath.Base(s.Path)
					}
					availableSkills = append(availableSkills, name)
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Skill %q not found. Available skills: %s", params.Name, strings.Join(availableSkills, ", "))), nil
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
