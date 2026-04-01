package skills

import (
	"embed"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

const ProjectSkillsDirName = "skills"

//go:embed bundled
var bundledSkills embed.FS

func ProjectSkillsDir(dataDir string) string {
	return filepath.Join(dataDir, ProjectSkillsDirName)
}

func EnsureProjectSkills(dataDir string) error {
	targetRoot := ProjectSkillsDir(dataDir)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(targetRoot, "teach-impeccable")); err != nil {
		return err
	}

	return fs.WalkDir(bundledSkills, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := bundledSkills.ReadFile(path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetRoot, filepath.Dir(path[len("bundled/"):]), filepath.Base(path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		return os.WriteFile(targetPath, content, 0o644)
	})
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
	skill, err := parseSkillContent(string(content), skillPath)
	if err != nil {
		return nil, err
	}
	skill.IsInternal = true
	return skill, nil
}
