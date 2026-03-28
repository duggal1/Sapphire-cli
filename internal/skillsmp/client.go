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
	"sort"
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
		skills, fallbackErr := c.searchViaManifest(ctx, query, defaultSearchLimit)
		if fallbackErr == nil {
			return skills, nil
		}
		return nil, err
	}

	skills, err := decodeSkillPayload(respBody)
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func (c *Client) searchViaManifest(ctx context.Context, query string, limit int) ([]Skill, error) {
	skills, err := c.List(ctx, defaultBrowseLimit)
	if err != nil {
		return nil, err
	}
	return filterSkillsLocally(skills, query, limit), nil
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

		page, hasNext, err := decodeManifestPayload(respBody)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		skills = append(skills, page...)
		if !hasNext {
			break
		}
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

	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read sapphire api request body: %w", err)
		}
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

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		reqBody := io.Reader(nil)
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("build sapphire api request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Sapphire-cli/1.0")
		req.Header.Set("x-api-key", c.APIKey)
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call Sapphire API: %w", err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read Sapphire API response: %w", readErr)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode == http.StatusOK {
			return respBody, nil
		}

		lastErr = apiErrorStatus(resp.StatusCode, respBody)
		if resp.StatusCode < http.StatusInternalServerError || attempt == 2 {
			return nil, lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("Sapphire API request failed")
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

func decodeManifestPayload(data []byte) ([]Skill, bool, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, false, errors.New("empty Sapphire API response")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		skills, err := decodeSkillPayload(data)
		if err != nil {
			return nil, false, err
		}
		rawNext, ok := obj["next_cursor"]
		if !ok {
			return skills, true, nil
		}
		rawNext = bytes.TrimSpace(rawNext)
		if bytes.Equal(rawNext, []byte("null")) || len(rawNext) == 0 || bytes.Equal(rawNext, []byte(`""`)) {
			return skills, false, nil
		}
		return skills, true, nil
	}

	skills, err := decodeSkillPayload(data)
	if err != nil {
		return nil, false, err
	}
	return skills, true, nil
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

func filterSkillsLocally(skills []Skill, query string, limit int) []Skill {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return skills
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	tokens := strings.Fields(query)
	type ranked struct {
		skill Skill
		score int
	}
	rankedSkills := make([]ranked, 0, len(skills))

	for _, skill := range skills {
		name := strings.ToLower(skill.DisplayName())
		folder := strings.ToLower(skill.FolderName)
		skillID := strings.ToLower(skill.SkillID)
		category := strings.ToLower(skill.Category)
		path := strings.ToLower(skill.RelativePath + " " + skill.MarkdownPath)

		score := 0
		matchedAll := true
		for _, token := range tokens {
			tokenScore := 0
			switch {
			case name == token:
				tokenScore = 12
			case strings.Contains(name, token):
				tokenScore = 8
			case strings.Contains(folder, token) || strings.Contains(skillID, token):
				tokenScore = 6
			case category != "" && strings.Contains(category, token):
				tokenScore = 4
			case strings.Contains(path, token):
				tokenScore = 2
			}
			if tokenScore == 0 {
				matchedAll = false
				break
			}
			score += tokenScore
		}
		if !matchedAll || score == 0 {
			continue
		}
		rankedSkills = append(rankedSkills, ranked{skill: skill, score: score})
	}

	sort.SliceStable(rankedSkills, func(i, j int) bool {
		if rankedSkills[i].score != rankedSkills[j].score {
			return rankedSkills[i].score > rankedSkills[j].score
		}
		return strings.ToLower(rankedSkills[i].skill.DisplayName()) < strings.ToLower(rankedSkills[j].skill.DisplayName())
	})

	if len(rankedSkills) > limit {
		rankedSkills = rankedSkills[:limit]
	}

	filtered := make([]Skill, 0, len(rankedSkills))
	for _, item := range rankedSkills {
		filtered = append(filtered, item.skill)
	}
	return filtered
}

func apiErrorStatus(statusCode int, body []byte) error {
	body = bytes.TrimSpace(body)
	if len(body) > 4096 {
		body = body[:4096]
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = http.StatusText(statusCode)
	}
	return fmt.Errorf("Sapphire API request failed: %s", text)
}
