package config

import (
	"cmp"
	"slices"
	"strings"
)

type RegistryMCPCategory string

const (
	RegistryMCPCategoryCloudInfrastructure RegistryMCPCategory = "cloud_infrastructure"
	RegistryMCPCategoryDatabases           RegistryMCPCategory = "databases"
	RegistryMCPCategoryAIVectorSearch      RegistryMCPCategory = "ai_vector_search"
	RegistryMCPCategoryAuthentication      RegistryMCPCategory = "authentication"
	RegistryMCPCategoryPayments            RegistryMCPCategory = "payments"
	RegistryMCPCategoryDevelopmentInfra    RegistryMCPCategory = "development_infrastructure"
	RegistryMCPCategoryProductivity        RegistryMCPCategory = "productivity"
	RegistryMCPCategoryDesign              RegistryMCPCategory = "design"
	RegistryMCPCategoryTestingDebugging    RegistryMCPCategory = "testing_debugging"
)

type RegistryMCPInventoryEntry struct {
	Definition RegistryMCPDefinition
	Category   RegistryMCPCategory
	Tags       []string
	Priority   int
}

var registryCategoryInstructions = map[RegistryMCPCategory]string{
	RegistryMCPCategoryCloudInfrastructure: "Use for cloud infrastructure: deploy, manage environments, containers, and orchestration.",
	RegistryMCPCategoryDatabases:           "Use for database operations: provisioning, migrations, queries, and backups.",
	RegistryMCPCategoryAIVectorSearch:      "Use for AI retrieval: embeddings, vector indexing, and semantic search.",
	RegistryMCPCategoryAuthentication:      "Use for auth/identity: OAuth, SSO, credentials, and session management.",
	RegistryMCPCategoryPayments:            "Use for payments: billing, checkout, subscriptions, and invoicing.",
	RegistryMCPCategoryDevelopmentInfra:    "Use for engineering workflow: git, CI/CD, repo automation, and code intelligence.",
	RegistryMCPCategoryProductivity:        "Use for documentation and productivity workflows.",
	RegistryMCPCategoryDesign:              "Use for design workflows and design-system integration.",
	RegistryMCPCategoryTestingDebugging:    "Use for testing, debugging, monitoring, and observability workflows.",
}

// RegistryEntryInstructions returns a concise instructions string describing the MCP server.
func RegistryEntryInstructions(entry RegistryMCPInventoryEntry) string {
	desc := strings.TrimSpace(entry.Definition.Description)
	instruction := strings.TrimSpace(registryCategoryInstructions[entry.Category])
	tags := entry.Tags
	if len(tags) > 6 {
		tags = tags[:6]
	}
	tagText := strings.Join(tags, ", ")

	var parts []string
	if instruction != "" {
		parts = append(parts, instruction)
	}
	if desc != "" {
		parts = append(parts, desc)
	}
	if tagText != "" {
		parts = append(parts, "Tags: "+tagText+".")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// RegistryEntrySearchBlob builds a normalized search blob for matching MCP inventory entries.
func RegistryEntrySearchBlob(entry RegistryMCPInventoryEntry) string {
	parts := []string{
		entry.Definition.Name,
		entry.Definition.Description,
		string(entry.Category),
		strings.Join(entry.Tags, " "),
		RegistryEntryInstructions(entry),
	}
	return strings.ToLower(strings.Join(parts, " "))
}

type registryCategoryRule struct {
	Category RegistryMCPCategory
	Priority int
	Strong   []string
	Keywords []string
}

var registryCategoryRules = []registryCategoryRule{
	{
		Category: RegistryMCPCategoryCloudInfrastructure,
		Priority: 9,
		Strong: []string{
			"aws", "amazon ecs", "amazon eks", "google cloud", "gcp", "cloud run",
			"kubernetes", "k8s", "docker", "containerization", "cloudflare",
			"vercel", "netlify", "render", "railway", "terraform", "infracost",
		},
		Keywords: []string{
			"deploy", "deployment", "cluster", "container", "cloud", "infrastructure",
			"orchestration", "serverless", "lambda", "ecs", "eks", "ecr", "kube",
			"helm", "iac",
		},
	},
	{
		Category: RegistryMCPCategoryDatabases,
		Priority: 8,
		Strong: []string{
			"supabase", "neon", "postgres", "postgresql", "mysql", "sqlite", "redis",
			"mongodb", "dynamodb", "couchbase", "greptimedb", "arcadedb", "druid",
			"database",
		},
		Keywords: []string{
			"sql", "schema", "migration", "query", "table", "branch", "replica",
			"warehouse", "time-series",
		},
	},
	{
		Category: RegistryMCPCategoryAIVectorSearch,
		Priority: 7,
		Strong: []string{
			"pinecone", "qdrant", "weaviate", "milvus", "chroma", "pgvector",
			"vector database", "vector search", "semantic search",
		},
		Keywords: []string{
			"vector", "embedding", "retrieval", "rag", "rerank", "semantic", "indexing",
			"hybrid search",
		},
	},
	{
		Category: RegistryMCPCategoryAuthentication,
		Priority: 8,
		Strong: []string{
			"better auth", "betterauth", "auth0", "clerk", "okta", "fusionauth",
			"keycloak", "cognito", "1password", "vault",
		},
		Keywords: []string{
			"auth", "authentication", "oauth", "oidc", "identity", "sso", "credential",
			"secret", "access token", "login",
		},
	},
	{
		Category: RegistryMCPCategoryPayments,
		Priority: 8,
		Strong: []string{
			"stripe", "payment", "payments", "billing", "checkout", "subscription",
		},
		Keywords: []string{
			"invoice", "merchant", "tax", "fiat", "pricing", "refund",
		},
	},
	{
		Category: RegistryMCPCategoryDevelopmentInfra,
		Priority: 7,
		Strong: []string{
			"github", "gitlab", "bitbucket", "pull request", "worktree", "code review",
			"code search", "code indexing", "dependency graph", "call graph",
			"chrome devtools",
		},
		Keywords: []string{
			"git", "repository", "repo", "branch", "commit", "actions", "workflow",
			"release", "semantic code search", "code intelligence", "code analysis",
			"impact analysis", "static analysis",
		},
	},
	{
		Category: RegistryMCPCategoryProductivity,
		Priority: 5,
		Strong: []string{
			"notion", "knowledge base", "knowledge graph", "wiki", "documentation",
		},
		Keywords: []string{
			"knowledge", "docs", "spec", "task board", "project management",
		},
	},
	{
		Category: RegistryMCPCategoryDesign,
		Priority: 6,
		Strong: []string{
			"figma", "design system", "design tooling", "ui review",
		},
		Keywords: []string{
			"design", "prototype", "screenshot", "mockserver", "visual", "excalidraw",
		},
	},
	{
		Category: RegistryMCPCategoryTestingDebugging,
		Priority: 8,
		Strong: []string{
			"debugger", "debugging", "ci/cd", "github actions", "teamcity", "sentry",
			"datadog", "grafana", "prometheus", "megalinter", "uptime kuma",
			"log analyzer", "build tool", "test framework",
		},
		Keywords: []string{
			"test", "testing", "lint", "build", "diagnostic", "diagnostics", "error",
			"log", "logs", "trace", "profiling", "monitoring", "observability",
			"incident", "quality", "performance",
		},
	},
}

func CuratedRegistryDefinitions(defs []RegistryMCPDefinition) []RegistryMCPDefinition {
	catalog := CuratedRegistryCatalog(defs)
	out := make([]RegistryMCPDefinition, 0, len(catalog))
	for _, entry := range catalog {
		out = append(out, entry.Definition)
	}
	return out
}

func CuratedRegistryCatalog(defs []RegistryMCPDefinition) []RegistryMCPInventoryEntry {
	catalog := make([]RegistryMCPInventoryEntry, 0, len(defs))
	seen := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		entry, ok := CuratedRegistryEntry(def)
		if !ok {
			continue
		}
		if _, exists := seen[entry.Definition.Name]; exists {
			continue
		}
		seen[entry.Definition.Name] = struct{}{}
		catalog = append(catalog, entry)
	}
	slices.SortFunc(catalog, func(a, b RegistryMCPInventoryEntry) int {
		if c := strings.Compare(string(a.Category), string(b.Category)); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Priority, a.Priority); c != 0 {
			return c
		}
		return strings.Compare(a.Definition.Name, b.Definition.Name)
	})
	return catalog
}

func CuratedRegistryEntry(def RegistryMCPDefinition) (RegistryMCPInventoryEntry, bool) {
	blob := normalizeRegistryBlob(def)
	if blob == "" {
		return RegistryMCPInventoryEntry{}, false
	}

	bestScore := 0
	bestRule := registryCategoryRule{}
	bestTags := []string{}

	for _, rule := range registryCategoryRules {
		score, tags := scoreRegistryRule(blob, rule)
		if score <= bestScore {
			continue
		}
		bestScore = score
		bestRule = rule
		bestTags = tags
	}

	if bestScore == 0 {
		return RegistryMCPInventoryEntry{}, false
	}

	return RegistryMCPInventoryEntry{
		Definition: def,
		Category:   bestRule.Category,
		Tags:       bestTags,
		Priority:   bestRule.Priority + bestScore,
	}, true
}

func normalizeRegistryBlob(def RegistryMCPDefinition) string {
	parts := []string{
		strings.Join(registryNameTokens(def.Name), " "),
		strings.ToLower(strings.TrimSpace(def.Description)),
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func registryNameTokens(name string) []string {
	segments := strings.Split(strings.ToLower(strings.TrimSpace(name)), "/")
	noise := map[string]struct{}{
		"ai":     {},
		"api":    {},
		"com":    {},
		"dev":    {},
		"eu":     {},
		"io":     {},
		"mcp":    {},
		"org":    {},
		"server": {},
	}

	replacer := strings.NewReplacer(".", " ", "-", " ", "_", " ")
	tokens := make([]string, 0, len(segments)*2)
	for i, segment := range segments {
		if i < len(segments)-1 {
			for _, prefix := range []string{"io.github.", "com.", "dev.", "eu.", "org."} {
				if strings.HasPrefix(segment, prefix) {
					segment = strings.TrimPrefix(segment, prefix)
					break
				}
			}
		}
		for _, token := range strings.Fields(replacer.Replace(segment)) {
			if _, skip := noise[token]; skip {
				continue
			}
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func scoreRegistryRule(blob string, rule registryCategoryRule) (int, []string) {
	score := 0
	tags := make([]string, 0, len(rule.Strong)+len(rule.Keywords))
	seen := map[string]struct{}{}
	addTag := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	for _, phrase := range rule.Strong {
		if strings.Contains(blob, phrase) {
			score += 6
			addTag(phrase)
		}
	}
	for _, phrase := range rule.Keywords {
		if strings.Contains(blob, phrase) {
			score += 2
			addTag(phrase)
		}
	}

	if score == 0 {
		return 0, nil
	}
	slices.Sort(tags)
	return score, tags
}
