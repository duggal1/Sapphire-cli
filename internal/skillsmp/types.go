package skillsmp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultBaseURL = "https://skills.trysapphire.today"

// Skill describes a skill summary returned by the Sapphire Skills API.
type Skill struct {
	SkillID      string
	FolderName   string
	SkillName    string
	RelativePath string
	MarkdownPath string
	SizeBytes    int
	IsNested     bool
	Category     string
}

// SkillFile is an optional file payload returned by the load endpoint.
type SkillFile struct {
	Path      string
	SizeBytes int
	MimeType  string
	SHA256    string
	Encoding  string
	Text      string
	Base64    *string
}

// LoadedSkill is the full payload returned by the load endpoint.
type LoadedSkill struct {
	Skill    Skill
	Markdown string
	Files    []SkillFile
}

type rawSkill struct {
	SkillID      string      `json:"skill_id"`
	FolderName   string      `json:"folder_name"`
	SkillName    string      `json:"skill_name"`
	RelativePath string      `json:"relative_path"`
	MarkdownPath string      `json:"markdown_path"`
	SizeBytes    flexibleInt `json:"size_bytes"`
	IsNested     bool        `json:"is_nested"`
	Category     *string     `json:"category"`
}

type rawSkillFile struct {
	Path      string      `json:"path"`
	SizeBytes flexibleInt `json:"size_bytes"`
	MimeType  string      `json:"mime_type"`
	SHA256    string      `json:"sha256"`
	Encoding  string      `json:"encoding"`
	Text      string      `json:"text"`
	Base64    *string     `json:"base64"`
}

type flexibleInt int

func (i *flexibleInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*i = 0
		return nil
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			*i = 0
			return nil
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			return err
		}
		*i = flexibleInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*i = flexibleInt(n)
	return nil
}

func (i flexibleInt) Int() int {
	return int(i)
}

func (s Skill) Key() string {
	if key := strings.TrimSpace(s.SkillID); key != "" {
		return key
	}
	if key := strings.TrimSpace(s.FolderName); key != "" {
		return key
	}
	return strings.TrimSpace(s.SkillName)
}

func (s Skill) DisplayName() string {
	if name := strings.TrimSpace(s.SkillName); name != "" {
		return name
	}
	if name := strings.TrimSpace(s.FolderName); name != "" {
		return name
	}
	return strings.TrimSpace(s.SkillID)
}

func (s Skill) LocalName() string {
	if name := strings.TrimSpace(s.FolderName); name != "" {
		return name
	}
	return s.DisplayName()
}

func (s Skill) LocalSkillPath(dataDir string) string {
	return filepath.Join(dataDir, "skills", s.LocalName(), "SKILL.md")
}

func (s Skill) LocalPluginDir(dataDir string) string {
	return filepath.Join(dataDir, "plugins", s.LocalName())
}

func (s Skill) LocalPluginManifestPath(dataDir string) string {
	return filepath.Join(s.LocalPluginDir(dataDir), "plugin.json")
}

func (s Skill) LocalPluginSkillPath(dataDir string) string {
	return filepath.Join(s.LocalPluginDir(dataDir), "SKILL.md")
}
