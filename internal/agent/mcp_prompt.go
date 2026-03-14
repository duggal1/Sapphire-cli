package agent

import (
	"strings"
)

// sanitizeMCPPrompt removes file/path-like tokens so MCP discovery doesn't trigger
// on filenames (e.g., "tool_test_01.txt") or paths.
func sanitizeMCPPrompt(prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return ""
	}
	tokens := strings.Fields(prompt)
	kept := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if looksLikePathToken(tok) {
			continue
		}
		kept = append(kept, tok)
	}
	return normalizePrompt(strings.Join(kept, " "))
}

func looksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	if strings.ContainsAny(token, `/\`) {
		return true
	}
	if dot := strings.LastIndex(token, "."); dot > 0 && dot < len(token)-1 {
		ext := token[dot+1:]
		if isAlphaNum(ext) && len(ext) <= 6 {
			return true
		}
	}
	return false
}

func isAlphaNum(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func tokenSet(tokens []string) map[string]struct{} {
	if len(tokens) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		set[token] = struct{}{}
	}
	return set
}

func containsKeyword(prompt string, tokens map[string]struct{}, keyword string) bool {
	if keyword == "" || prompt == "" {
		return false
	}
	if strings.Contains(keyword, " ") {
		return strings.Contains(prompt, keyword)
	}
	_, ok := tokens[keyword]
	return ok
}

func isMCPInventoryPrompt(prompt string) bool {
	prompt = sanitizeMCPPrompt(prompt)
	if prompt == "" {
		return false
	}
	tokens := tokenSet(strings.Fields(prompt))
	if containsKeyword(prompt, tokens, "mcp") || containsKeyword(prompt, tokens, "mcps") {
		if containsKeyword(prompt, tokens, "list") ||
			containsKeyword(prompt, tokens, "available") ||
			containsKeyword(prompt, tokens, "inventory") ||
			containsKeyword(prompt, tokens, "show") ||
			containsKeyword(prompt, tokens, "which") {
			return true
		}
	}
	return false
}
