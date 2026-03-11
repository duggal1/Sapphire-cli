package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:generate env SAPPHIRE_MCP_GENERATE=1 go test ./internal/config -run TestNonexistent -count=1

const (
	mcpRegistryURL            = "https://registry.modelcontextprotocol.io/v0/servers"
	mcpRegistryMetaKey        = "io.modelcontextprotocol.registry/official"
	mcpRegistryGenerateEnv    = "SAPPHIRE_MCP_GENERATE"
	mcpRegistryGeneratedFile  = "internal/config/mcp_registry_gen.go"
	mcpRegistryTimeout        = 20 * time.Second
	mcpRegistryMinDefinitions = 100
	mcpRegistryMaxPages       = 200
)

// RegistryMCPDefinition is a normalized MCP server definition derived from the registry.
type RegistryMCPDefinition struct {
	Name        string
	Description string
	Type        MCPType
	Command     string
	Args        []string
	EnvKeys     []string
}

type registryResponse struct {
	Metadata registryMetadata        `json:"metadata"`
	Servers  []registryServerWrapper `json:"servers"`
}

type registryMetadata struct {
	Count      int    `json:"count"`
	NextCursor string `json:"nextCursor"`
}

type registryServerWrapper struct {
	Server registryServer                `json:"server"`
	Meta   map[string]registryServerMeta `json:"_meta"`
}

type registryServerMeta struct {
	IsLatest bool `json:"isLatest"`
}

type registryServer struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Packages    []registryPackage `json:"packages"`
}

type registryPackage struct {
	RegistryType         string            `json:"registryType"`
	Identifier           string            `json:"identifier"`
	Transport            registryTransport `json:"transport"`
	EnvironmentVariables []registryEnvVar  `json:"environmentVariables"`
}

type registryTransport struct {
	Type string `json:"type"`
}

type registryEnvVar struct {
	Name       string `json:"name"`
	IsRequired bool   `json:"isRequired"`
}

func init() {
	if os.Getenv(mcpRegistryGenerateEnv) == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := GenerateRegistryDefinitionsFile(ctx, mcpRegistryGeneratedFile); err != nil {
			fmt.Fprintf(os.Stderr, "mcp registry generation failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// SeedMCPOnFirstLaunch seeds MCP config when no MCP configuration exists.
// It never overwrites existing user configuration.
func SeedMCPOnFirstLaunch(ctx context.Context, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if len(cfg.MCP) != 0 {
		return nil
	}

	defs, err := FetchRegistryDefinitions(ctx)
	if err != nil || len(defs) == 0 {
		defs = RegistryMCPDefinitions
	}
	if len(defs) == 0 {
		return nil
	}

	_, err = applyRegistryDefinitions(cfg, defs, true)
	return err
}

// SyncFromRegistry fetches the MCP registry and adds new entries without
// overwriting existing user configuration.
func SyncFromRegistry(ctx context.Context, cfg *Config) (int, error) {
	if cfg == nil {
		return 0, errors.New("config is nil")
	}
	defs, err := FetchRegistryDefinitions(ctx)
	if err != nil {
		return 0, err
	}
	return applyRegistryDefinitions(cfg, defs, true)
}

// FetchRegistryDefinitions fetches the MCP registry and derives MCP definitions.
func FetchRegistryDefinitions(ctx context.Context) ([]RegistryMCPDefinition, error) {
	defs, err := fetchRegistryDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return nil, errors.New("no supported MCP servers found in registry")
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs, nil
}

func fetchRegistryDefinitions(ctx context.Context) ([]RegistryMCPDefinition, error) {
	client := &http.Client{Timeout: mcpRegistryTimeout}
	cursor := ""
	defs := make([]RegistryMCPDefinition, 0, mcpRegistryMinDefinitions)
	seen := make(map[string]struct{})

	for page := 0; page < mcpRegistryMaxPages; page++ {
		reqURL := mcpRegistryURL
		if cursor != "" {
			reqURL = reqURL + "?cursor=" + url.QueryEscape(cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("registry request failed: %s", strings.TrimSpace(string(body)))
		}
		var payload registryResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, wrapped := range payload.Servers {
			def, ok := definitionFromServer(wrapped)
			if !ok {
				continue
			}
			if _, exists := seen[def.Name]; exists {
				continue
			}
			seen[def.Name] = struct{}{}
			defs = append(defs, def)
			if len(defs) >= mcpRegistryMinDefinitions {
				return defs, nil
			}
		}

		if payload.Metadata.NextCursor == "" {
			break
		}
		cursor = payload.Metadata.NextCursor
	}

	return defs, nil
}

func definitionFromServer(wrapped registryServerWrapper) (RegistryMCPDefinition, bool) {
	if meta, ok := wrapped.Meta[mcpRegistryMetaKey]; ok && !meta.IsLatest {
		return RegistryMCPDefinition{}, false
	}
	server := wrapped.Server
	name := strings.TrimSpace(server.Name)
	if name == "" {
		return RegistryMCPDefinition{}, false
	}
	description := strings.TrimSpace(server.Description)
	if isWebSearchServer(name, description) {
		return RegistryMCPDefinition{}, false
	}

	for _, pkg := range server.Packages {
		mcpType, ok := mcpTypeFromTransport(pkg.Transport.Type)
		if !ok {
			continue
		}
		cmd, args, ok := spawnFromRegistry(pkg.RegistryType, pkg.Identifier)
		if !ok {
			continue
		}
		return RegistryMCPDefinition{
			Name:        name,
			Description: description,
			Type:        mcpType,
			Command:     cmd,
			Args:        args,
			EnvKeys:     requiredEnvKeys(pkg.EnvironmentVariables),
		}, true
	}

	return RegistryMCPDefinition{}, false
}

func requiredEnvKeys(vars []registryEnvVar) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, v := range vars {
		if !v.IsRequired {
			continue
		}
		name := strings.TrimSpace(v.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func mcpTypeFromTransport(transport string) (MCPType, bool) {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "stdio":
		return MCPStdio, true
	case "streamable-http", "http":
		return MCPHttp, true
	case "sse":
		return MCPSSE, true
	default:
		return "", false
	}
}

func spawnFromRegistry(registryType, identifier string) (string, []string, bool) {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return "", nil, false
	}
	switch strings.ToLower(strings.TrimSpace(registryType)) {
	case "npm":
		return "npx", []string{"-y", id}, true
	case "pypi":
		return "uvx", []string{id}, true
	case "oci", "docker":
		return "docker", []string{"run", "-i", "--rm", id}, true
	default:
		return "", nil, false
	}
}

func isWebSearchServer(name, description string) bool {
	blob := strings.ToLower(strings.TrimSpace(name + " " + description))
	return strings.Contains(blob, "web search") ||
		strings.Contains(blob, "search engine") ||
		strings.Contains(blob, "serp") ||
		strings.Contains(blob, "search api")
}

func applyRegistryDefinitions(cfg *Config, defs []RegistryMCPDefinition, onlyNew bool) (int, error) {
	if cfg.MCP == nil {
		cfg.MCP = make(map[string]MCPConfig)
	}

	added := 0
	for _, def := range defs {
		if def.Name == "" {
			continue
		}
		if onlyNew {
			if _, exists := cfg.MCP[def.Name]; exists {
				continue
			}
		}

		envMap, missing := buildEnvMap(def.EnvKeys)
		disabled := len(def.EnvKeys) > 0 && len(missing) > 0
		cfg.MCP[def.Name] = MCPConfig{
			Type:     def.Type,
			Command:  def.Command,
			Args:     append([]string{}, def.Args...),
			Env:      envMap,
			Disabled: disabled,
		}
		added++
	}

	if added == 0 {
		return 0, nil
	}

	if err := cfg.SaveMCPConfigs(); err != nil {
		return 0, err
	}
	return added, nil
}

func buildEnvMap(keys []string) (map[string]string, []string) {
	if len(keys) == 0 {
		return nil, nil
	}
	missing := []string{}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = "$" + key
		if _, ok := os.LookupEnv(key); !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return out, missing
}

// GenerateRegistryDefinitionsFile fetches registry data and writes a Go file
// containing RegistryMCPDefinitions.
func GenerateRegistryDefinitionsFile(ctx context.Context, outputPath string) error {
	defs, err := FetchRegistryDefinitions(ctx)
	if err != nil {
		return err
	}
	if len(defs) == 0 {
		return errors.New("no MCP definitions derived from registry")
	}

	var sb strings.Builder
	sb.WriteString("// Code generated by Sapphire MCP registry generator. DO NOT EDIT.\n")
	sb.WriteString("package config\n\n")
	sb.WriteString("var RegistryMCPDefinitions = []RegistryMCPDefinition{\n")
	for _, def := range defs {
		sb.WriteString("\t{")
		sb.WriteString(fmt.Sprintf("Name: %q, ", def.Name))
		sb.WriteString(fmt.Sprintf("Description: %q, ", def.Description))
		sb.WriteString(fmt.Sprintf("Type: %q, ", def.Type))
		sb.WriteString(fmt.Sprintf("Command: %q, ", def.Command))
		sb.WriteString("Args: []string{")
		for i, arg := range def.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q", arg))
		}
		sb.WriteString("}, ")
		sb.WriteString("EnvKeys: []string{")
		for i, key := range def.EnvKeys {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q", key))
		}
		sb.WriteString("},")
		sb.WriteString("},\n")
	}
	sb.WriteString("}\n")

	formatted, err := format.Source([]byte(sb.String()))
	if err != nil {
		return fmt.Errorf("format generated file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, formatted, 0o600); err != nil {
		return err
	}

	if len(defs) < 100 {
		slog.Warn("Registry returned fewer than 100 MCP definitions", "count", len(defs))
	}

	return nil
}
