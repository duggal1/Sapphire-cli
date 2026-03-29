package agent

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/skills"
)

//go:embed templates/list_skills.md
var listSkillsToolDescription []byte

//go:embed templates/search_skills.md
var searchSkillsToolDescription []byte

//go:embed templates/load_skill.md
var loadSkillToolDescription []byte

type LoadSkillParams struct {
	Name string `json:"name" description:"The name of the skill to load (e.g., 'frontend', 'backend', 'debugging')"`
}

type SearchSkillsParams struct {
	Query string `json:"query" description:"Short domain query for skill discovery, e.g. 'frontend react ui accessibility motion'"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of matches to return"`
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

type rankedSkill struct {
	info  SkillInfo
	score int
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

func (c *coordinator) searchSkillsTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		"search_skills",
		string(searchSkillsToolDescription),
		func(ctx context.Context, params SearchSkillsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			c.ensureSkillsDiscovered()

			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}
			if len(c.discoveredSkills) == 0 {
				return fantasy.NewTextResponse("No skills available. Check that skills are installed in the project or system skills directory."), nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 8
			}
			if limit > 20 {
				limit = 20
			}

			matches := rankSkills(c.discoveredSkills, query, limit)
			if len(matches) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("No local skills matched query %q. Local search covers bundled and already-installed skills. Next: call `install_skill(query: %q)` or refine the query.", query, query)), nil
			}

			var sb strings.Builder
			sb.WriteString("## Matching Local Skills\n\n")
			sb.WriteString("These results are local skills already available in this environment. Load from here before considering `install_skill`.\n\n")
			for _, match := range matches {
				source := "System"
				if !match.info.IsInternal {
					source = "Project"
				}
				sb.WriteString("- **")
				sb.WriteString(match.info.Name)
				sb.WriteString("**")
				sb.WriteString(" (score ")
				sb.WriteString(strconv.Itoa(match.score))
				sb.WriteString("): ")
				sb.WriteString(match.info.Description)
				sb.WriteString(" [")
				sb.WriteString(source)
				sb.WriteString(": ")
				sb.WriteString(match.info.Location)
				sb.WriteString("]\n")
			}
			sb.WriteString("\nNext: call `load_skill` with the exact local skill name you want to activate. Use `install_skill` only if these local matches are insufficient.")
			return fantasy.NewTextResponse(sb.String()), nil
		},
	), nil
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
				msg = fmt.Sprintf("Successfully loaded local skill: %s (from %s)", displayName, location)
			}

			return fantasy.ToolResponse{
				Content: fmt.Sprintf("%s\n\n<instructions>\n%s\n</instructions>", msg, matchedInstructions),
			}, nil
		}), nil
}

func rankSkills(discovered []*skills.Skill, query string, limit int) []rankedSkill {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	tokens := uniqueTokens(query)
	matches := make([]rankedSkill, 0, len(discovered))

	for _, skill := range discovered {
		if skill == nil {
			continue
		}
		info := skillInfoFromSkill(skill)
		name := strings.ToLower(info.Name)
		description := strings.ToLower(info.Description)
		location := strings.ToLower(info.Location)
		instructions := strings.ToLower(skill.Instructions)

		score := 0
		if name == query {
			score += 300
		}
		if strings.Contains(name, query) {
			score += 180
		}
		if strings.Contains(description, query) {
			score += 120
		}
		if strings.Contains(location, query) {
			score += 80
		}
		if strings.Contains(instructions, query) {
			score += 30
		}

		matchedTokens := 0
		for _, token := range tokens {
			tokenMatched := false
			if strings.Contains(name, token) {
				score += 45
				tokenMatched = true
			}
			if strings.Contains(description, token) {
				score += 28
				tokenMatched = true
			}
			if strings.Contains(location, token) {
				score += 18
				tokenMatched = true
			}
			if strings.Contains(instructions, token) {
				score += 8
				tokenMatched = true
			}
			if tokenMatched {
				matchedTokens++
			}
		}
		if matchedTokens == len(tokens) && len(tokens) > 0 {
			score += 40
		}
		if score == 0 {
			continue
		}
		matches = append(matches, rankedSkill{
			info:  info,
			score: score,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].info.Name < matches[j].info.Name
		}
		return matches[i].score > matches[j].score
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func skillInfoFromSkill(s *skills.Skill) SkillInfo {
	displayName := s.Name
	if displayName == "" || displayName == "SKILLNAME" {
		displayName = filepath.Base(s.Path)
	}
	return SkillInfo{
		Name:        displayName,
		Description: s.Description,
		IsInternal:  s.IsInternal,
		Location:    s.SkillFilePath,
	}
}

func uniqueTokens(query string) []string {
	parts := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	seen := make(map[string]struct{}, len(parts))
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if len(part) < 2 {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		tokens = append(tokens, part)
	}
	return tokens
}
