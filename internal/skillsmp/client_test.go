package skillsmp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientSearchAndList(t *testing.T) {
	t.Parallel()

	skillsCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("authorization header = %q, want %q", got, "Bearer token")
		}

		switch r.URL.Path {
		case "/api/v1/skills/ai-search":
			if got := r.URL.Query().Get("q"); got != "security" {
				t.Errorf("query = %q, want %q", got, "security")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{
						"name":        "security-audit",
						"owner":       "trail-of-bits",
						"repo":        "security-audit",
						"file_path":   "SKILL.md",
						"github_url":  "https://github.com/trail-of-bits/security-audit/blob/main/SKILL.md",
						"stars":       100,
						"description": "Security audit workflow",
						"installs":    999,
					},
					{
						"name":        "security-ops",
						"owner":       "trail-of-bits",
						"repo":        "security-ops",
						"file_path":   "SKILL.md",
						"github_url":  "https://github.com/trail-of-bits/security-ops/blob/main/SKILL.md",
						"stars":       80,
						"description": "Security operations workflow",
						"installs":    500,
					},
				},
			})
		case "/api/v1/skills":
			skillsCalls++
			wantLimit := "2000"
			if skillsCalls == 2 {
				wantLimit = "100"
			}
			if got := r.URL.Query().Get("limit"); got != wantLimit {
				t.Errorf("limit = %q, want %q", got, wantLimit)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{
						"name":        "popular-skills",
						"owner":       "vercel-labs",
						"repo":        "agent-skills",
						"file_path":   "SKILL.md",
						"github_url":  "https://github.com/vercel-labs/agent-skills/blob/main/SKILL.md",
						"stars":       200,
						"description": "Popular skill",
						"installs":    4000,
					},
					{
						"name":        "less-popular",
						"owner":       "vercel-labs",
						"repo":        "agent-skills",
						"file_path":   "SKILL.md",
						"github_url":  "https://github.com/vercel-labs/agent-skills/blob/main/SKILL.md",
						"stars":       250,
						"description": "Less popular skill",
						"installs":    100,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL+"/api/v1", "token", server.Client())

	search, err := client.Search(context.Background(), "security")
	require.NoError(t, err)
	require.Len(t, search, 2)
	require.Equal(t, "security-audit", search[0].Name)
	require.Equal(t, "trail-of-bits/security-audit", search[0].OwnerRepo())

	emptySearch, err := client.Search(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, emptySearch, 2)
	require.Equal(t, "popular-skills", emptySearch[0].Name)

	list, err := client.List(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "popular-skills", list[0].Name)
	require.Equal(t, "less-popular", list[1].Name)
}

func TestClientRawGitHubURL(t *testing.T) {
	t.Parallel()

	raw, err := githubRawURL("https://github.com/owner/repo/blob/main/path/SKILL.md")
	require.NoError(t, err)
	require.Equal(t, "https://raw.githubusercontent.com/owner/repo/main/path/SKILL.md", raw)
}
