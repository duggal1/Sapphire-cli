package skillsmp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientSearchListAndLoad(t *testing.T) {
	t.Parallel()

	searchCalls := 0
	manifestCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "token", r.Header.Get("x-api-key"))
		require.Empty(t, r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/search":
			searchCalls++
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "security", payload["query"])
			require.EqualValues(t, defaultSearchLimit, payload["limit"])

			_ = json.NewEncoder(w).Encode(map[string]any{
				"query":    "security",
				"returned": 1,
				"results": []map[string]any{
					{
						"skill_id":      "security-audit",
						"folder_name":   "security-audit",
						"skill_name":    "security-audit",
						"relative_path": "security/SKILL.md",
						"markdown_path": "security/SKILL.md",
						"size_bytes":    1536,
						"is_nested":     false,
						"category":      "security",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/manifest":
			manifestCalls++
			switch manifestCalls {
			case 1:
				require.Equal(t, "200", r.URL.Query().Get("limit"))
				require.Equal(t, "0", r.URL.Query().Get("cursor"))
				require.Equal(t, "all", r.URL.Query().Get("category"))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{
						{
							"skill_id":      "alpha",
							"folder_name":   "alpha",
							"skill_name":    "alpha",
							"relative_path": "alpha/SKILL.md",
							"markdown_path": "alpha/SKILL.md",
							"size_bytes":    100,
							"is_nested":     false,
							"category":      "core",
						},
						{
							"skill_id":      "beta",
							"folder_name":   "beta",
							"skill_name":    "beta",
							"relative_path": "beta/SKILL.md",
							"markdown_path": "beta/SKILL.md",
							"size_bytes":    200,
							"is_nested":     true,
							"category":      "integrations",
						},
					},
				})
			case 2:
				require.Equal(t, "200", r.URL.Query().Get("limit"))
				require.Equal(t, "200", r.URL.Query().Get("cursor"))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{
						{
							"skill_id":      "gamma",
							"folder_name":   "gamma",
							"skill_name":    "gamma",
							"relative_path": "gamma/SKILL.md",
							"markdown_path": "gamma/SKILL.md",
							"size_bytes":    300,
							"is_nested":     false,
							"category":      nil,
						},
					},
				})
			case 3:
				require.Equal(t, "200", r.URL.Query().Get("limit"))
				require.Equal(t, "400", r.URL.Query().Get("cursor"))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{},
				})
			default:
				t.Fatalf("unexpected manifest request %d", manifestCalls)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/security-audit":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"skill": map[string]any{
					"skill_id":      "security-audit",
					"folder_name":   "security-audit",
					"skill_name":    "security-audit",
					"relative_path": "security/SKILL.md",
					"markdown_path": "security/SKILL.md",
					"size_bytes":    1536,
					"is_nested":     false,
					"category":      "security",
				},
				"markdown": "# Security Audit\n",
				"files": []map[string]any{
					{
						"path":       "SKILL.md",
						"size_bytes": 18,
						"mime_type":  "text/markdown",
						"sha256":     "abc",
						"encoding":   "text",
						"text":       "# Security Audit\n",
						"base64":     nil,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, "token", server.Client())

	search, err := client.Search(context.Background(), "security")
	require.NoError(t, err)
	require.Len(t, search, 1)
	require.Equal(t, "security-audit", search[0].SkillID)
	require.Equal(t, "security", search[0].Category)
	require.Equal(t, "security-audit", search[0].DisplayName())

	list, err := client.List(context.Background(), 450)
	require.NoError(t, err)
	require.Len(t, list, 3)
	require.Equal(t, "alpha", list[0].SkillID)
	require.Equal(t, "gamma", list[2].SkillID)

	loaded, err := client.LoadSkill(context.Background(), "security-audit")
	require.NoError(t, err)
	require.Equal(t, "security-audit", loaded.Skill.LocalName())
	require.Equal(t, "# Security Audit\n", loaded.Markdown)
	require.Len(t, loaded.Files, 1)
}

func TestClientSearchRetriesTransientServerErrors(t *testing.T) {
	t.Parallel()

	searchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/skills/search", r.URL.Path)
		searchCalls++
		if searchCalls < 3 {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "retry-me",
			"results": []map[string]any{
				{
					"skill_id":      "retry-skill",
					"folder_name":   "retry-skill",
					"skill_name":    "retry-skill",
					"relative_path": "retry-skill/SKILL.md",
					"markdown_path": "retry-skill/SKILL.md",
					"size_bytes":    128,
					"is_nested":     false,
					"category":      "backend",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, "token", server.Client())
	results, err := client.Search(context.Background(), "retry-me")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 3, searchCalls)
}

func TestClientSearchFallsBackToManifestOnServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "token", r.Header.Get("x-api-key"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/search":
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/manifest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"next_cursor": nil,
				"items": []map[string]any{
					{
						"skill_id":      "java-pro",
						"folder_name":   "java-pro",
						"skill_name":    "java-pro",
						"relative_path": "java-pro/SKILL.md",
						"markdown_path": "java-pro/SKILL.md",
						"size_bytes":    100,
						"is_nested":     false,
						"category":      "backend",
					},
					{
						"skill_id":      "python-pro",
						"folder_name":   "python-pro",
						"skill_name":    "python-pro",
						"relative_path": "python-pro/SKILL.md",
						"markdown_path": "python-pro/SKILL.md",
						"size_bytes":    100,
						"is_nested":     false,
						"category":      "backend",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, "token", server.Client())
	results, err := client.Search(context.Background(), "java")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "java-pro", results[0].SkillID)
}

func TestClientOnlySendsXAPIKeyHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "token", r.Header.Get("x-api-key"))
		require.Empty(t, r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []map[string]any{},
			"next_cursor": nil,
		})
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, "token", server.Client())
	_, err := client.List(context.Background(), 1)
	require.NoError(t, err)
}
