package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const ProjectSkillsDirName = "skills"

//go:embed bundled
var bundledSkills embed.FS

var (
	bundledCatalogOnce sync.Once
	bundledCatalog     []*Skill
)

func ProjectSkillsDir(dataDir string) string {
	return filepath.Join(dataDir, ProjectSkillsDirName)
}

func EnsureProjectSkills(dataDir string) error {
	targetRoot := ProjectSkillsDir(dataDir)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return err
	}
	return pruneLegacyBundledProjectSkills(targetRoot)
}

func LoadBundledSkill(name string) (*Skill, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, os.ErrNotExist
	}
	skillPath := pathpkg.Join("bundled", name, SkillFileName)
	content, err := bundledSkills.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}
	return parseBundledSkillContent(skillPath, string(content))
}

func DiscoverBundledSkills() []*Skill {
	bundledCatalogOnce.Do(func() {
		var discovered []*Skill
		_ = fs.WalkDir(bundledSkills, "bundled", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}

			content, readErr := bundledSkills.ReadFile(path)
			if readErr != nil {
				return nil
			}

			skill, parseErr := parseBundledSkillContent(path, string(content))
			if parseErr != nil {
				return nil
			}
			discovered = append(discovered, skill)
			return nil
		})

		slices.SortFunc(discovered, func(a, b *Skill) int {
			return strings.Compare(a.SkillFilePath, b.SkillFilePath)
		})
		bundledCatalog = discovered
	})

	out := make([]*Skill, 0, len(bundledCatalog))
	for _, skill := range bundledCatalog {
		copied := *skill
		out = append(out, &copied)
	}
	return out
}

func pruneLegacyBundledProjectSkills(targetRoot string) error {
	for _, name := range bundledSkillNames() {
		embeddedPath := pathpkg.Join("bundled", name, SkillFileName)
		projectPath := filepath.Join(targetRoot, name, SkillFileName)

		projectContent, err := os.ReadFile(projectPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		embeddedContent, err := bundledSkills.ReadFile(embeddedPath)
		if err != nil {
			continue
		}

		if string(projectContent) != string(embeddedContent) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(targetRoot, name)); err != nil {
			return err
		}
	}
	return nil
}

func bundledSkillNames() []string {
	entries, err := fs.ReadDir(bundledSkills, "bundled")
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names
}

func parseBundledSkillContent(sourcePath, content string) (*Skill, error) {
	skill, err := parseSkillContent(content, sourcePath)
	if err == nil {
		return normalizeBundledSkill(skill, sourcePath), nil
	}

	frontmatter, body, hasFrontmatter := bundledFrontmatterAndBody(content)
	meta := bundledSkillFrontmatter{}
	if hasFrontmatter {
		if unmarshalErr := yaml.Unmarshal([]byte(frontmatter), &meta); unmarshalErr != nil {
			return nil, fmt.Errorf("parse bundled skill %s: %w", sourcePath, unmarshalErr)
		}
	} else {
		body = content
	}

	skill = &Skill{
		Name:          strings.TrimSpace(meta.Name),
		Description:   strings.TrimSpace(meta.Description),
		License:       strings.TrimSpace(meta.License),
		Compatibility: strings.TrimSpace(meta.Compatibility),
		Instructions:  strings.TrimSpace(body),
		Path:          pathpkg.Dir(sourcePath),
		SkillFilePath: sourcePath,
		IsInternal:    true,
	}
	return normalizeBundledSkill(skill, sourcePath), nil
}

type bundledSkillFrontmatter struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	License       string `yaml:"license"`
	Compatibility string `yaml:"compatibility"`
}

func bundledFrontmatterAndBody(content string) (frontmatter, body string, ok bool) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return "", strings.TrimSpace(content), false
	}
	return frontmatter, body, true
}

func normalizeBundledSkill(skill *Skill, sourcePath string) *Skill {
	if skill == nil {
		return nil
	}

	dirName := pathpkg.Base(pathpkg.Dir(sourcePath))
	if strings.TrimSpace(skill.Name) == "" {
		skill.Name = dirName
	}
	if strings.TrimSpace(skill.Description) == "" {
		skill.Description = deriveBundledDescription(skill.Instructions, dirName)
	}
	skill.Path = pathpkg.Dir(sourcePath)
	skill.SkillFilePath = sourcePath
	skill.IsInternal = true
	return skill
}

func deriveBundledDescription(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
			continue
		}
		if len(line) > MaxDescriptionLength {
			return line[:MaxDescriptionLength]
		}
		return line
	}
	return "Internal bundled skill: " + fallback
}
