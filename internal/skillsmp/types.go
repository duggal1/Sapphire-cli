package skillsmp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultBaseURL = "https://skillsmp.com/api/v1"

// Skill describes a marketplace skill returned by SkillsMP.
type Skill struct {
	Name        string
	Owner       string
	Repo        string
	FilePath    string
	GithubURL   string
	Stars       int
	Description string
	Installs    int
}

type rawSkill struct {
	Name        string      `json:"name"`
	Owner       string      `json:"owner"`
	Repo        string      `json:"repo"`
	FilePath    string      `json:"file_path"`
	GithubURL   string      `json:"github_url"`
	Stars       flexibleInt `json:"stars"`
	Description string      `json:"description"`
	Installs    flexibleInt `json:"installs"`
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
	key := strings.TrimSpace(s.GithubURL)
	if key != "" {
		if filePath := strings.TrimSpace(s.FilePath); filePath != "" {
			return key + "|" + filePath
		}
		return key
	}
	return strings.Join([]string{
		strings.TrimSpace(s.Owner),
		strings.TrimSpace(s.Repo),
		strings.TrimSpace(s.FilePath),
		strings.TrimSpace(s.Name),
	}, "|")
}

func (s Skill) OwnerRepo() string {
	owner := strings.TrimSpace(s.Owner)
	repo := strings.TrimSpace(s.Repo)
	switch {
	case owner != "" && repo != "":
		return owner + "/" + repo
	case owner != "":
		return owner
	case repo != "":
		return repo
	default:
		return ""
	}
}

func (s Skill) LocalSkillPath(dataDir string) string {
	return filepath.Join(dataDir, "skills", s.Name, "SKILL.md")
}

func (s Skill) LocalPluginDir(dataDir string) string {
	return filepath.Join(dataDir, "plugins", s.Name)
}

func (s Skill) LocalPluginManifestPath(dataDir string) string {
	return filepath.Join(s.LocalPluginDir(dataDir), "plugin.json")
}

func (s Skill) LocalPluginSkillPath(dataDir string) string {
	return filepath.Join(s.LocalPluginDir(dataDir), "SKILL.md")
}

func (s Skill) RawGitHubURL() (string, error) {
	raw := strings.TrimSpace(s.GithubURL)
	if raw == "" {
		return "", fmt.Errorf("github_url is required")
	}
	return githubRawURL(raw)
}

func githubRawURL(githubURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(githubURL))
	if err != nil {
		return "", fmt.Errorf("parse github_url: %w", err)
	}

	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
	switch host {
	case "github.com":
	case "raw.githubusercontent.com":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("github_url must point to github.com")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 {
		return "", fmt.Errorf("github_url must include owner, repo, branch, and file path")
	}
	if parts[2] == "blob" {
		parts = append(parts[:2], parts[3:]...)
	}
	if len(parts) < 4 {
		return "", fmt.Errorf("github_url must include a file path")
	}

	return (&url.URL{
		Scheme: func() string {
			if parsed.Scheme != "" {
				return parsed.Scheme
			}
			return "https"
		}(),
		Host: "raw.githubusercontent.com",
		Path: "/" + strings.Join(parts, "/"),
	}).String(), nil
}
