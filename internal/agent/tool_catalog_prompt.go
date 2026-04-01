package agent

import (
	"sort"
	"strings"

	"charm.land/fantasy"
)

var runtimeToolSummaries = map[string]string{
	"agent":                   "delegate one bounded task to a worker agent",
	"agent_directory":         "inspect the active agent and work-item directory with routeable aliases",
	"agent_mail_ack":          "durably acknowledge handled coordination mail",
	"agent_mail_inbox":        "read durable coordination mail from other agents",
	"agent_mail_send":         "send durable coordination mail to another agent",
	"agentic_edit":            "edit multiple files in one structured operation",
	"agentic_fetch":           "fetch and analyze external web sources",
	"agentic_view":            "default repository read tool; it can read one file or many, and should be used aggressively for repo exploration",
	"apply_patch":             "apply precise unified-diff patches",
	"bash":                    "last-resort terminal escape hatch for build/test/process work when structured tools do not fit",
	"call_mcp_tool":           "invoke an MCP tool dynamically",
	"check_hook":              "inspect durable work-item hook state",
	"close_agent":             "close a sub-agent and release its resources",
	"collect_result":          "collect final result payloads from finished sub-agents",
	"connect_mcp":             "connect an installed MCP server",
	"index_codebase":          "warm the durable codebase graph so future turns get richer boot-packet context",
	"install_mcp":             "install and configure an MCP server by exact registry name",
	"install_skill":           "search the Sapphire skills API and install the best matching extended skill into the local skill store",
	"download":                "download remote content to a file",
	"edit":                    "legacy single-file edit tool",
	"fetch":                   "fetch remote content for inspection",
	"glob":                    "find files by filename pattern when the path shape is already known",
	"google_search":           "run grounded Google search when available",
	"grep":                    "search file contents by pattern or literal text when you explicitly want grep behavior",
	"job_kill":                "stop a background bash job",
	"job_list":                "list active background bash jobs",
	"job_output":              "read live output from a background bash job",
	"list_available_mcps":     "list registry MCP inventory plus local install and connection state",
	"list_mcp_resources":      "list resources exposed by an MCP server",
	"list_mcp_tools":          "list tools exposed by an MCP server",
	"list_skills":             "list available built-in and installed local skills",
	"list_tools":              "list tools available to this agent right now",
	"load_skill":              "activate a built-in or installed local skill workflow",
	"ls":                      "list directory trees and verify exact paths or top-level layout",
	"lsp_diagnostics":         "read full current diagnostics for a file",
	"lsp_references":          "find symbol references via LSP",
	"lsp_restart":             "restart language servers",
	"orchestrate_worktrees":   "run pre-scoped worktree batches",
	"python":                  "run structured computation, parsing, or verification",
	"read_mcp_resource":       "read a specific MCP resource",
	"refresh_memory":          "long-horizon maintenance of durable memory state",
	"report_agent_job_result": "report a CSV batch-worker result row",
	"rg":                      "run real ripgrep content search when the exact text or symbol string is already known",
	"rg_files":                "run ripgrep --files style filename/path search when the path shape is already known",
	"run_harness":             "classify the turn, route complex work through the harness, and return a strict JSON execution contract",
	"request_user_input":      "ask structured clarifying questions when mode permits",
	"resume_agent":            "resume a previously spawned sub-agent",
	"search_skills":           "search built-in and installed local skills by focused domain query",
	"search_tools":            "search tools by name, purpose, or parameters",
	"send_input":              "send more context or instructions to a sub-agent",
	"set_mode":                "switch execution mode",
	"single_edit":             "edit exactly one file",
	"single_view":             "extreme fallback for an explicitly narrow, guaranteed-trivial one-file read",
	"sourcegraph":             "query Sourcegraph code search when configured",
	"spawn_agent":             "spawn a real sub-agent with explicit lifecycle control",
	"spawn_agents_on_csv":     "launch CSV-driven batch worker jobs only",
	"tool_search":             "primary bounded repo locator for unknown file, symbol, or code-region lookup before reading",
	"tool_suggest":            "suggest MCP capabilities when local tools are insufficient",
	"update_plan":             "update the live execution plan",
	"view":                    "legacy file read tool",
	"view_memory":             "read durable session memory only for resume, compaction, or long-horizon continuity",
	"wait":                    "wait for sub-agents to make progress or finish",
	"web_fetch":               "fetch a single web page",
	"web_search":              "search the web",
	"wc":                      "measure file size and density before choosing how much to read or edit",
	"wc_l":                    "measure exact file line counts before full or segmented reads",
	"write":                   "create or replace a file explicitly",
}

func appendToolCatalogToPrompt(base string, agentTools []fantasy.AgentTool) string {
	catalog := renderToolCatalogPrompt(agentTools)
	if catalog == "" {
		return base
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return catalog
	}
	return base + "\n\n" + catalog
}

func renderToolCatalogPrompt(agentTools []fantasy.AgentTool) string {
	if len(agentTools) == 0 {
		return ""
	}
	infos := make([]fantasy.ToolInfo, 0, len(agentTools))
	seen := make(map[string]struct{}, len(agentTools))
	for _, tool := range agentTools {
		if tool == nil {
			continue
		}
		info := tool.Info()
		if info.Name == "" {
			continue
		}
		if _, ok := seen[info.Name]; ok {
			continue
		}
		seen[info.Name] = struct{}{}
		infos = append(infos, info)
	}
	if len(infos) == 0 {
		return ""
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	var sb strings.Builder
	sb.WriteString("<runtime_tool_catalog>\n")
	sb.WriteString("Authoritative tool surface for this agent. Prefer the narrowest structured tool. If a structured tool exists, do not fall back to bash.\n")
	for _, info := range infos {
		sb.WriteString("- `")
		sb.WriteString(info.Name)
		sb.WriteString("`: ")
		sb.WriteString(conciseToolSummary(info))
		sb.WriteString("\n")
	}
	sb.WriteString("</runtime_tool_catalog>")
	return sb.String()
}

func conciseToolSummary(info fantasy.ToolInfo) string {
	if summary, ok := runtimeToolSummaries[info.Name]; ok {
		return summary
	}
	text := strings.TrimSpace(info.Description)
	if text == "" {
		return "tool available; inspect with list_tools or search_tools"
	}
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, ". "); idx >= 0 {
		text = text[:idx+1]
	}
	if len(text) > 120 {
		text = strings.TrimSpace(text[:120]) + "..."
	}
	return text
}
