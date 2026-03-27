package skillsmp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBrowseLimit  = 2000
	manifestPageLimit   = 200
	defaultSearchLimit  = 200
	defaultManifestKind = "all"
)

// Client talks to the Sapphire Skills API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	baseURL := strings.TrimSpace(os.Getenv("SAPPHIRE_API_BASE_URL"))
	return NewClientWithBaseURL(baseURL, apiKey, nil)
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

	payload := map[string]any{
		"query": query,
		"limit": defaultSearchLimit,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sapphire search request: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, []string{"v1", "skills", "search"}, bytes.NewReader(body), map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}

	skills, err := decodeSkillPayload(respBody)
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func (c *Client) List(ctx context.Context, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = manifestPageLimit
	}

	skills := make([]Skill, 0, min(limit, manifestPageLimit))
	for cursor := 0; len(skills) < limit; cursor += manifestPageLimit {
		pageLimit := min(manifestPageLimit, limit-len(skills))
		values := url.Values{}
		values.Set("limit", strconv.Itoa(pageLimit))
		values.Set("cursor", strconv.Itoa(cursor))
		values.Set("category", defaultManifestKind)

		respBody, err := c.do(ctx, http.MethodGet, []string{"v1", "skills", "manifest"}, nil, nil, values)
		if err != nil {
			return nil, err
		}

		page, err := decodeSkillPayload(respBody)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		skills = append(skills, page...)
	}

	return skills, nil
}

func (c *Client) LoadSkill(ctx context.Context, skillID string) (LoadedSkill, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return LoadedSkill{}, errors.New("skill_id is required")
	}

	respBody, err := c.do(ctx, http.MethodGet, []string{"v1", "skills", skillID}, nil, nil)
	if err != nil {
		return LoadedSkill{}, err
	}

	var payload struct {
		Skill    rawSkill       `json:"skill"`
		Markdown string         `json:"markdown"`
		Files    []rawSkillFile `json:"files"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return LoadedSkill{}, fmt.Errorf("decode sapphire load response: %w", err)
	}
	if strings.TrimSpace(payload.Markdown) == "" {
		return LoadedSkill{}, errors.New("loaded skill markdown is empty")
	}

	files := make([]SkillFile, 0, len(payload.Files))
	for _, file := range payload.Files {
		files = append(files, SkillFile{
			Path:      strings.TrimSpace(file.Path),
			SizeBytes: file.SizeBytes.Int(),
			MimeType:  strings.TrimSpace(file.MimeType),
			SHA256:    strings.TrimSpace(file.SHA256),
			Encoding:  strings.TrimSpace(file.Encoding),
			Text:      file.Text,
			Base64:    file.Base64,
		})
	}

	return LoadedSkill{
		Skill:    normalizeSkill(payload.Skill),
		Markdown: payload.Markdown,
		Files:    files,
	}, nil
}

func (c *Client) do(ctx context.Context, method string, pathParts []string, body io.Reader, headers map[string]string, query ...url.Values) ([]byte, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("Sapphire API key is required")
	}

	reqURL, err := url.JoinPath(c.BaseURL, pathParts...)
	if err != nil {
		return nil, fmt.Errorf("build sapphire api url: %w", err)
	}
	if len(query) > 0 && query[0] != nil {
		encoded := query[0].Encode()
		if encoded != "" {
			reqURL += "?" + encoded
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("build sapphire api request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Sapphire-cli/1.0")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Sapphire API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Sapphire API response: %w", err)
	}
	return respBody, nil
}

func decodeSkillPayload(data []byte) ([]Skill, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("empty Sapphire API response")
	}

	if skills, ok := findSkillArray(data, 0); ok {
		return skills, nil
	}

	return nil, errors.New("unexpected Sapphire API response shape")
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
		for _, key := range []string{"results", "items", "skills", "data"} {
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
		skill := normalizeSkill(raw)
		if skill.Key() == "" {
			continue
		}
		skills = append(skills, skill)
	}
	return skills, true
}

func normalizeSkill(raw rawSkill) Skill {
	category := ""
	if raw.Category != nil {
		category = strings.TrimSpace(*raw.Category)
	}
	return Skill{
		SkillID:      strings.TrimSpace(raw.SkillID),
		FolderName:   strings.TrimSpace(raw.FolderName),
		SkillName:    strings.TrimSpace(raw.SkillName),
		RelativePath: strings.TrimSpace(raw.RelativePath),
		MarkdownPath: strings.TrimSpace(raw.MarkdownPath),
		SizeBytes:    raw.SizeBytes.Int(),
		IsNested:     raw.IsNested,
		Category:     category,
	}
}

func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = resp.Status
	}
	return fmt.Errorf("Sapphire API request failed: %s", text)
}
