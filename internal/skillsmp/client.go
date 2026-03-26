package skillsmp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const defaultBrowseLimit = 2000

// Client talks to the SkillsMP API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return NewClientWithBaseURL(DefaultBaseURL, apiKey, nil)
}

func NewClientWithBaseURL(baseURL, apiKey string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     strings.TrimSpace(apiKey),
		HTTPClient: httpClient,
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]Skill, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return c.List(ctx, defaultBrowseLimit)
	}
	return c.fetchSkills(ctx, "/skills/ai-search", url.Values{"q": []string{query}})
}

func (c *Client) List(ctx context.Context, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = 100
	}
	skills, err := c.fetchSkills(ctx, "/skills", url.Values{"limit": []string{fmt.Sprintf("%d", limit)}})
	if err != nil {
		return nil, err
	}
	sortSkillsByPopularity(skills)
	return skills, nil
}

func (c *Client) FetchRawSkill(ctx context.Context, skill Skill) ([]byte, error) {
	rawURL, err := skill.RawGitHubURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build raw GitHub request: %w", err)
	}
	req.Header.Set("User-Agent", "Sapphire-cli/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch raw skill markdown: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read raw skill markdown: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, errors.New("raw skill markdown is empty")
	}
	return body, nil
}

func (c *Client) fetchSkills(ctx context.Context, endpoint string, query url.Values) ([]Skill, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("SKILLSMP_API_KEY is required")
	}

	reqURL, err := url.JoinPath(c.BaseURL, strings.TrimPrefix(endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("build skillsmp url: %w", err)
	}
	if encoded := query.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build skillsmp request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Sapphire-cli/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call SkillsMP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read SkillsMP response: %w", err)
	}
	skills, err := decodeSkillPayload(body)
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func decodeSkillPayload(data []byte) ([]Skill, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("empty SkillsMP response")
	}

	if skills, ok := findSkillArray(data, 0); ok {
		return skills, nil
	}

	return nil, errors.New("unexpected SkillsMP response shape")
}

func findSkillArray(data []byte, depth int) ([]Skill, bool) {
	if depth > 3 {
		return nil, false
	}

	var rawSkills []rawSkill
	if err := json.Unmarshal(data, &rawSkills); err != nil {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, false
		}
		for _, key := range []string{"skills", "results", "items", "data"} {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			if skills, ok := findSkillArray(raw, depth+1); ok {
				return skills, true
			}
		}
		for _, raw := range obj {
			if skills, ok := findSkillArray(raw, depth+1); ok {
				return skills, true
			}
		}
		return nil, false
	}
	skills := make([]Skill, 0, len(rawSkills))
	for _, raw := range rawSkills {
		skill := Skill{
			Name:        strings.TrimSpace(raw.Name),
			Owner:       strings.TrimSpace(raw.Owner),
			Repo:        strings.TrimSpace(raw.Repo),
			FilePath:    strings.TrimSpace(raw.FilePath),
			GithubURL:   strings.TrimSpace(raw.GithubURL),
			Stars:       raw.Stars.Int(),
			Description: strings.TrimSpace(raw.Description),
			Installs:    raw.Installs.Int(),
		}
		if skill.Name == "" {
			continue
		}
		skills = append(skills, skill)
	}
	return skills, true
}

func sortSkillsByPopularity(skills []Skill) {
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].Installs != skills[j].Installs {
			return skills[i].Installs > skills[j].Installs
		}
		if skills[i].Stars != skills[j].Stars {
			return skills[i].Stars > skills[j].Stars
		}
		return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name)
	})
}

func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = resp.Status
	}
	return fmt.Errorf("SkillsMP request failed: %s", text)
}
