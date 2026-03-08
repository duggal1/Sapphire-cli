package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

const ProjectSkillsDirName = "skills"

//go:embed bundled/*/SKILL.md
var bundledSkills embed.FS

func ProjectSkillsDir(dataDir string) string {
	return filepath.Join(dataDir, ProjectSkillsDirName)
}

func EnsureProjectSkills(dataDir string) error {
	targetRoot := ProjectSkillsDir(dataDir)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
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
