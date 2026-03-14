package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
)

const mcpPolicyBlock = `<mcp_policy>
1. Sapphire has built-in MCP support.
2. The MCP capability map only lists CONNECTED servers. It is not the full inventory.
3. Use list_available_mcps with a query to discover available MCP servers before claiming coverage or inventory.
4. Prefer direct connected MCP tools when they are already available for the selected server.
5. Always use a tool to get information before using that information. Never assume or invent.
6. Chain MCP tools sequentially. The output of one tool call is the input to the next.
7. Execute autonomously. Do not ask for confirmation between steps unless blocked.
8. When a task needs multiple MCP servers, plan the full sequence before tool calls.
9. Do not stop after listing MCPs or tools if an execution path is available.
10. Treat every tool response as ground truth for this session.
</mcp_policy>`

type activeToolSet struct {
	mu    sync.Mutex
	names map[string]struct{}
}

func newActiveToolSet(initial []string) *activeToolSet {
	set := &activeToolSet{names: make(map[string]struct{}, len(initial))}
	for _, name := range initial {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		set.names[name] = struct{}{}
	}
	return set
}

func (s *activeToolSet) Add(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	s.mu.Lock()
	s.names[name] = struct{}{}
	s.mu.Unlock()
}

func (s *activeToolSet) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.names))
	for name := range s.names {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

type mcpCapability struct {
	Name      string
	Domain    string
	ToolCount int
}

func buildMCPCapabilityMap() string {
	states := mcp.GetStates()
	if len(states) == 0 {
		return ""
	}

	toolNames := make(map[string][]string)
	for name, tools := range mcp.Tools() {
		for _, tool := range tools {
			toolNames[name] = append(toolNames[name], tool.Name)
		}
	}

	caps := make([]mcpCapability, 0, len(states))
	for name, state := range states {
		if state.State != mcp.StateConnected {
			continue
		}
		list := toolNames[name]
		slices.Sort(list)
		count := state.Counts.Tools
		if count == 0 {
			count = len(list)
		}
		caps = append(caps, mcpCapability{
			Name:      name,
			Domain:    describeMCPDomain(list, count),
			ToolCount: count,
		})
	}
	if len(caps) == 0 {
		return ""
	}
	slices.SortFunc(caps, func(a, b mcpCapability) int {
		return strings.Compare(a.Name, b.Name)
	})

	var sb strings.Builder
	sb.WriteString("<mcp_capability_map>\n")
	sb.WriteString(fmt.Sprintf("Connected MCP servers only: %d\n", len(caps)))
	for _, cap := range caps {
		sb.WriteString(fmt.Sprintf("- %s: %s (tools: %d)\n", cap.Name, cap.Domain, cap.ToolCount))
	}
	sb.WriteString("</mcp_capability_map>")
	return sb.String()
}

func connectedMCPNames() []string {
	states := mcp.GetStates()
	if len(states) == 0 {
		return nil
	}
	names := make([]string, 0, len(states))
	for name, state := range states {
		if state.State != mcp.StateConnected {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func describeMCPDomain(tools []string, count int) string {
	if len(tools) == 0 {
		return fmt.Sprintf("Connected MCP server with %d tools.", count)
	}
	limit := min(6, len(tools))
	return fmt.Sprintf("Exposes MCP tools such as %s.", strings.Join(tools[:limit], ", "))
}

var (
	urlRegex     = regexp.MustCompile(`https?://[^\s"'<>]+`)
	uuidRegex    = regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	statusCodeRe = regexp.MustCompile(`\b[1-5][0-9]{2}\b`)
	keyValueRe   = regexp.MustCompile(`(?i)\b(id|url|status|token|secret|key|name|endpoint)\b[:=]\s*([\w\-./:]+)`)
)

func buildToolGrounding(toolName, content string) string {
	facts := extractFacts(content)
	if len(facts) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<tool_grounding tool=\"%s\">\n", toolName))
	for _, fact := range facts {
		sb.WriteString("- " + fact + "\n")
	}
	sb.WriteString("</tool_grounding>")
	return sb.String()
}

func extractFacts(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	facts := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		if _, ok := seen[val]; ok {
			return
		}
		seen[val] = struct{}{}
		facts = append(facts, val)
	}

	for _, match := range urlRegex.FindAllString(content, 6) {
		add("url=" + match)
	}
	for _, match := range uuidRegex.FindAllString(content, 6) {
		add("id=" + match)
	}
	for _, match := range keyValueRe.FindAllStringSubmatch(content, 6) {
		if len(match) == 3 {
			add(strings.ToLower(match[1]) + "=" + match[2])
		}
	}
	for _, match := range statusCodeRe.FindAllString(content, 3) {
		add("status=" + match)
	}

	var payload any
	if json.Unmarshal([]byte(content), &payload) == nil {
		collectJSONFacts(payload, "", add)
	}

	if len(facts) > 12 {
		facts = facts[:12]
	}
	return facts
}

func collectJSONFacts(data any, prefix string, add func(string)) {
	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			keyLower := strings.ToLower(key)
			if shouldCaptureKey(keyLower) {
				switch t := val.(type) {
				case string:
					add(keyLower + "=" + t)
				case float64:
					add(keyLower + "=" + strconv.FormatFloat(t, 'f', -1, 64))
				case bool:
					add(keyLower + "=" + strconv.FormatBool(t))
				}
			}
			collectJSONFacts(val, prefix+keyLower+".", add)
		}
	case []any:
		for i, item := range v {
			if i >= 6 {
				break
			}
			collectJSONFacts(item, prefix, add)
		}
	}
}

func shouldCaptureKey(key string) bool {
	switch key {
	case "id", "url", "status", "status_code", "name", "token", "secret", "key", "endpoint", "project", "repo", "database", "table", "bucket":
		return true
	default:
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
