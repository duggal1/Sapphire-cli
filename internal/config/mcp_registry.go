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
	URL         string
	Headers     map[string]string
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
	Name        string              `json:"name"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Packages    []registryPackage   `json:"packages"`
	Remotes     []registryTransport `json:"remotes"`
}

type registryPackage struct {
	RegistryType         string             `json:"registryType"`
	Identifier           string             `json:"identifier"`
	RuntimeHint          string             `json:"runtimeHint"`
	Transport            registryTransport  `json:"transport"`
	RuntimeArguments     []registryArgument `json:"runtimeArguments"`
	PackageArguments     []registryArgument `json:"packageArguments"`
	EnvironmentVariables []registryInput    `json:"environmentVariables"`
}

type registryTransport struct {
	Type      string                   `json:"type"`
	URL       string                   `json:"url"`
	Headers   []registryInput          `json:"headers"`
	Variables map[string]registryInput `json:"variables"`
}

type registryInput struct {
	Name       string `json:"name"`
	Default    string `json:"default"`
	Value      string `json:"value"`
	Format     string `json:"format"`
	IsRequired bool   `json:"isRequired"`
	IsSecret   bool   `json:"isSecret"`
}

type registryArgument struct {
	Type       string                   `json:"type"`
	Name       string                   `json:"name"`
	Default    string                   `json:"default"`
	Value      string                   `json:"value"`
	Format     string                   `json:"format"`
	IsRequired bool                     `json:"isRequired"`
	Variables  map[string]registryInput `json:"variables"`
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

	defs := DefaultRegistryDefinitions(ctx)
	if len(defs) == 0 {
		return nil
	}

	_, err := applyRegistryDefinitions(cfg, defs, true)
	return err
}

// SyncFromRegistry fetches the MCP registry and adds new entries without
// overwriting existing user configuration.
func SyncFromRegistry(ctx context.Context, cfg *Config) (int, error) {
	if cfg == nil {
		return 0, errors.New("config is nil")
	}
	return applyRegistryDefinitions(cfg, DefaultRegistryDefinitions(ctx), true)
}

func DefaultRegistryDefinitions(ctx context.Context) []RegistryMCPDefinition {
	if ctx == nil {
		return CuratedRegistryDefinitions(RegistryMCPDefinitions)
	}
	if _, ok := ctx.Deadline(); !ok {
		return CuratedRegistryDefinitions(RegistryMCPDefinitions)
	}
	defs, err := FetchRegistryDefinitions(ctx)
	if err == nil && len(defs) > 0 {
		return CuratedRegistryDefinitions(defs)
	}
	return CuratedRegistryDefinitions(RegistryMCPDefinitions)
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

	type candidate struct {
		def   RegistryMCPDefinition
		score int
	}

	candidates := make([]candidate, 0, len(server.Remotes)+len(server.Packages))
	for _, remote := range server.Remotes {
		def, score, ok := definitionFromRemote(name, description, remote)
		if ok {
			candidates = append(candidates, candidate{def: def, score: score})
		}
	}
	for _, pkg := range server.Packages {
		def, score, ok := definitionFromPackage(name, description, pkg)
		if ok {
			candidates = append(candidates, candidate{def: def, score: score})
		}
	}
	if len(candidates) == 0 {
		return RegistryMCPDefinition{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].def, true
}

func requiredEnvKeys(vars []registryInput) []string {
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

func definitionFromRemote(name, description string, transport registryTransport) (RegistryMCPDefinition, int, bool) {
	mcpType, ok := mcpTypeFromTransport(transport.Type)
	if !ok {
		return RegistryMCPDefinition{}, 0, false
	}
	endpoint, headers, ok := resolveTransport(transport)
	if !ok {
		return RegistryMCPDefinition{}, 0, false
	}
	score := 90
	if endpointIsLocal(endpoint) {
		score = 55
	}
	return RegistryMCPDefinition{
		Name:        name,
		Description: description,
		Type:        mcpType,
		URL:         endpoint,
		Headers:     headers,
	}, score, true
}

func definitionFromPackage(name, description string, pkg registryPackage) (RegistryMCPDefinition, int, bool) {
	mcpType, ok := mcpTypeFromTransport(pkg.Transport.Type)
	if !ok {
		return RegistryMCPDefinition{}, 0, false
	}
	command, args, ok := spawnFromRegistry(pkg)
	if !ok {
		return RegistryMCPDefinition{}, 0, false
	}

	def := RegistryMCPDefinition{
		Name:        name,
		Description: description,
		Type:        mcpType,
		Command:     command,
		Args:        args,
		EnvKeys:     requiredEnvKeys(pkg.EnvironmentVariables),
	}

	score := 80
	if command == "docker" {
		score -= 10
	}
	if mcpType != MCPStdio {
		endpoint, headers, ok := resolveTransport(pkg.Transport)
		if !ok {
			return RegistryMCPDefinition{}, 0, false
		}
		def.URL = endpoint
		def.Headers = headers
		score = 75
		if endpointIsLocal(endpoint) {
			score = 72
		}
	}

	return def, score, true
}

func spawnFromRegistry(pkg registryPackage) (string, []string, bool) {
	id := strings.TrimSpace(pkg.Identifier)
	if id == "" {
		return "", nil, false
	}
	runtime := strings.ToLower(strings.TrimSpace(pkg.RuntimeHint))
	if runtime == "" {
		runtime = strings.ToLower(strings.TrimSpace(pkg.RegistryType))
	}
	switch runtime {
	case "npm", "npx":
		runtimeArgs, ok := buildRegistryArgs(pkg.RuntimeArguments)
		if !ok {
			return "", nil, false
		}
		packageArgs, ok := buildRegistryArgs(pkg.PackageArguments)
		if !ok {
			return "", nil, false
		}
		args := append([]string{}, runtimeArgs...)
		if !slicesContains(args, "-y") && !slicesContains(args, "--yes") {
			args = append(args, "-y")
		}
		args = append(args, id)
		args = append(args, packageArgs...)
		return "npx", args, true
	case "pypi", "uvx":
		runtimeArgs, ok := buildRegistryArgs(pkg.RuntimeArguments)
		if !ok {
			return "", nil, false
		}
		executable, usedPackageArg := guessUVXExecutable(id, pkg.PackageArguments)
		packageArgs, ok := buildRegistryArgs(skipFirstArgument(pkg.PackageArguments, usedPackageArg))
		if !ok {
			return "", nil, false
		}
		if executable == "" {
			args := append([]string{}, runtimeArgs...)
			args = append(args, id)
			args = append(args, packageArgs...)
			return "uvx", args, true
		}
		args := append([]string{}, runtimeArgs...)
		args = append(args, "--from", id, executable)
		args = append(args, packageArgs...)
		return "uvx", args, true
	case "oci", "docker":
		runtimeArgs, ok := buildRegistryArgs(pkg.RuntimeArguments)
		if !ok {
			return "", nil, false
		}
		packageArgs, ok := buildRegistryArgs(pkg.PackageArguments)
		if !ok {
			return "", nil, false
		}
		args := []string{"run", "-i", "--rm"}
		args = append(args, runtimeArgs...)
		args = append(args, id)
		args = append(args, packageArgs...)
		return "docker", args, true
	default:
		return "", nil, false
	}
}

func guessUVXExecutable(identifier string, args []registryArgument) (string, bool) {
	if len(args) > 0 && strings.EqualFold(strings.TrimSpace(args[0].Type), "positional") {
		if value, ok := resolveArgumentValue(args[0]); ok && value != "" && !strings.HasPrefix(value, "-") {
			return value, true
		}
	}
	guess := strings.TrimSpace(identifier)
	if guess == "" {
		return "", false
	}
	if idx := strings.LastIndexAny(guess, "/:"); idx >= 0 && idx < len(guess)-1 {
		guess = guess[idx+1:]
	}
	return guess, false
}

func skipFirstArgument(args []registryArgument, skip bool) []registryArgument {
	if !skip || len(args) == 0 {
		return args
	}
	return args[1:]
}

func buildRegistryArgs(args []registryArgument) ([]string, bool) {
	out := make([]string, 0, len(args)*2)
	for _, arg := range args {
		value, ok := resolveArgumentValue(arg)
		if !ok {
			return nil, false
		}
		switch strings.ToLower(strings.TrimSpace(arg.Type)) {
		case "named":
			name := strings.TrimSpace(arg.Name)
			if name == "" || value == "" {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(arg.Format), "boolean") {
				switch strings.ToLower(strings.TrimSpace(value)) {
				case "true":
					out = append(out, name)
				case "false":
				default:
					out = append(out, name, value)
				}
				continue
			}
			out = append(out, name, value)
		case "positional":
			if value != "" {
				out = append(out, value)
			}
		}
	}
	return out, true
}

func resolveArgumentValue(arg registryArgument) (string, bool) {
	values := resolveVariableValues(arg.Variables)
	if value := substituteVariables(strings.TrimSpace(arg.Value), values); value != "" {
		return value, true
	}
	if value := substituteVariables(strings.TrimSpace(arg.Default), values); value != "" {
		return value, true
	}
	return "", !arg.IsRequired
}

func resolveTransport(transport registryTransport) (string, map[string]string, bool) {
	values := resolveVariableValues(transport.Variables)
	endpoint := substituteVariables(strings.TrimSpace(transport.URL), values)
	if endpoint == "" || strings.Contains(endpoint, "{") || strings.Contains(endpoint, "}") {
		return "", nil, false
	}

	headers := make(map[string]string)
	for _, header := range transport.Headers {
		name := strings.TrimSpace(header.Name)
		if name == "" {
			continue
		}
		value, ok := resolveInputValue(header)
		if !ok || value == "" {
			continue
		}
		headers[name] = substituteVariables(value, values)
	}
	if len(headers) == 0 {
		headers = nil
	}
	return endpoint, headers, true
}

func resolveVariableValues(inputs map[string]registryInput) map[string]string {
	if len(inputs) == 0 {
		return nil
	}
	values := make(map[string]string, len(inputs))
	for key, input := range inputs {
		value, ok := resolveInputValue(input)
		if !ok || value == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func resolveInputValue(input registryInput) (string, bool) {
	if value := strings.TrimSpace(input.Value); value != "" {
		return value, true
	}
	if value := strings.TrimSpace(input.Default); value != "" {
		return value, true
	}
	return "", !input.IsRequired
}

func substituteVariables(value string, values map[string]string) string {
	if value == "" || len(values) == 0 {
		return value
	}
	for key, replacement := range values {
		value = strings.ReplaceAll(value, "{"+key+"}", replacement)
	}
	return value
}

func endpointIsLocal(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

		cfg.MCP[def.Name] = RegistryDefinitionToMCPConfig(def, false)
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

func RegistryDefinitionToMCPConfig(def RegistryMCPDefinition, autoStart bool) MCPConfig {
	envMap, missing := buildEnvMap(def.EnvKeys)
	disabled := len(def.EnvKeys) > 0 && len(missing) > 0
	return MCPConfig{
		Type:      def.Type,
		Command:   def.Command,
		Args:      append([]string{}, def.Args...),
		URL:       def.URL,
		Headers:   cloneStringMap(def.Headers),
		Env:       envMap,
		Disabled:  disabled,
		AutoStart: boolPtr(autoStart),
	}
}

func DisableSeededMCPAutoStart(ctx context.Context, cfg *Config) error {
	if cfg == nil || len(cfg.MCP) < mcpRegistryMinDefinitions/2 {
		return nil
	}
	defs := DefaultRegistryDefinitions(ctx)
	if len(defs) == 0 {
		return nil
	}
	defMap := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		defMap[def.Name] = struct{}{}
	}

	changed := false
	for name, mcpCfg := range cfg.MCP {
		if mcpCfg.AutoStart != nil {
			continue
		}
		if _, ok := defMap[name]; !ok {
			continue
		}
		mcpCfg.AutoStart = boolPtr(false)
		cfg.MCP[name] = mcpCfg
		changed = true
	}
	if !changed {
		return nil
	}
	return cfg.SaveMCPConfigs()
}

func PruneSeededMCPInventory(ctx context.Context, cfg *Config) error {
	if cfg == nil || len(cfg.MCP) < mcpRegistryMinDefinitions/2 {
		return nil
	}

	allDefs := RegistryMCPDefinitions
	if len(allDefs) == 0 {
		return nil
	}
	allNames := make(map[string]struct{}, len(allDefs))
	for _, def := range allDefs {
		allNames[def.Name] = struct{}{}
	}

	curatedNames := make(map[string]struct{})
	for _, def := range DefaultRegistryDefinitions(ctx) {
		curatedNames[def.Name] = struct{}{}
	}
	if len(curatedNames) == 0 {
		return nil
	}

	changed := false
	for name := range cfg.MCP {
		if _, known := allNames[name]; !known {
			continue
		}
		if _, keep := curatedNames[name]; keep {
			continue
		}
		delete(cfg.MCP, name)
		changed = true
	}
	if !changed {
		return nil
	}
	return cfg.SaveMCPConfigs()
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func boolPtr(value bool) *bool {
	return &value
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
		sb.WriteString(fmt.Sprintf("URL: %q, ", def.URL))
		sb.WriteString("Headers: map[string]string{")
		headerKeys := make([]string, 0, len(def.Headers))
		for key := range def.Headers {
			headerKeys = append(headerKeys, key)
		}
		sort.Strings(headerKeys)
		for i, key := range headerKeys {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q: %q", key, def.Headers[key]))
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
