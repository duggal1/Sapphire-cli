package skillsmp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/fsext"
	"github.com/duggal1/Sapphire-cli/internal/skills"
)

// Installer writes Sapphire API skill results into the local Sapphire extended-skill/plugin layout.
type Installer struct {
	DataDir string
}

func NewInstaller(dataDir string) *Installer {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = ResolveDataDir("")
	}
	return &Installer{DataDir: dataDir}
}

func SearchInstall(query, apiKey string) error {
	ctx := context.Background()
	client := NewClient(apiKey)
	installer := NewInstaller(ResolveDataDir(""))
	skill, err := client.BestMatch(ctx, query)
	if err != nil {
		return err
	}
	loaded, err := client.LoadSkill(ctx, skill.SkillID)
	if err != nil {
		return err
	}
	return installer.Install(loaded.Skill, []byte(loaded.Markdown))
}

func ResolveDataDir(workingDir string) string {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			workingDir = cwd
		}
	}
	if workingDir == "" {
		return filepath.Join(".", ".sapphire")
	}
	if path, ok := fsext.LookupClosest(workingDir, ".sapphire"); ok {
		return path
	}
	return filepath.Join(workingDir, ".sapphire")
}

func (c *Client) BestMatch(ctx context.Context, query string) (Skill, error) {
	skills, err := c.Search(ctx, query)
	if err != nil {
		return Skill{}, err
	}
	if len(skills) == 0 {
		return Skill{}, errors.New("no matching extended skills found")
	}
	return skills[0], nil
}

func (i *Installer) Install(skill Skill, skillMarkdown []byte) error {
	if i == nil {
		return errors.New("installer is nil")
	}
	if strings.TrimSpace(i.DataDir) == "" {
		return errors.New("data directory is required")
	}
	localName := skill.LocalName()
	if strings.TrimSpace(localName) == "" {
		return errors.New("skill name is required")
	}
	description := deriveDescription(skill, skillMarkdown)

	skillDir := filepath.Join(i.DataDir, "skills", localName)
	pluginDir := filepath.Join(i.DataDir, "plugins", localName)

	if err := validateLocalSkill(skillDir, localName, description); err != nil {
		return err
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	pluginSkillPath := filepath.Join(pluginDir, "SKILL.md")
	manifestPath := filepath.Join(pluginDir, "plugin.json")

	if err := os.WriteFile(skillPath, skillMarkdown, 0o644); err != nil {
		return fmt.Errorf("write skill markdown: %w", err)
	}
	if err := os.WriteFile(pluginSkillPath, skillMarkdown, 0o644); err != nil {
		return fmt.Errorf("write plugin skill markdown: %w", err)
	}

	manifest := map[string]any{
		"name":        localName,
		"version":     "1.0.0",
		"description": description,
		"skills": []map[string]string{
			{"path": "./SKILL.md"},
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write plugin manifest: %w", err)
	}

	return nil
}

func validateLocalSkill(skillDir, name, description string) error {
	skill := &skills.Skill{
		Name:        name,
		Description: description,
		Path:        skillDir,
	}
	if err := skill.Validate(); err != nil {
		return fmt.Errorf("invalid skill metadata: %w", err)
	}
	return nil
}

func deriveDescription(skill Skill, skillMarkdown []byte) string {
	if description := frontmatterDescription(skillMarkdown); description != "" {
		return description
	}
	if category := strings.TrimSpace(skill.Category); category != "" {
		return "Sapphire extended skill for " + category
	}
	if name := strings.TrimSpace(skill.DisplayName()); name != "" {
		return name + " extended skill"
	}
	return "Installed from Sapphire Extended Skills API"
}

func frontmatterDescription(skillMarkdown []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(skillMarkdown)))
	if !scanner.Scan() {
		return ""
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return ""
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		const prefix = "description:"
		if !strings.HasPrefix(strings.ToLower(line), prefix) {
			continue
		}
		value := strings.TrimSpace(line[len(prefix):])
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}
