package tools

import (
	"path/filepath"
	"strings"
)

type WriteScope struct {
	root  string
	rules []writeScopeRule
}

type writeScopeRule struct {
	pattern  string
	isPrefix bool
	isGlob   bool
}

func NewWriteScope(root string, allowed []string) *WriteScope {
	if allowed == nil {
		return nil
	}
	root = filepath.Clean(root)
	if root == "" {
		return nil
	}
	rules := make([]writeScopeRule, 0, len(allowed))
	for _, raw := range allowed {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		rule := normalizeWriteScopeRule(root, raw)
		rules = append(rules, rule)
	}
	return &WriteScope{root: root, rules: rules}
}

func (s *WriteScope) Allows(path string) bool {
	if s == nil {
		return true
	}
	if path == "" {
		return false
	}
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(s.root, absPath)
	}
	absPath = filepath.Clean(absPath)
	if !withinRoot(s.root, absPath) {
		return false
	}
	if len(s.rules) == 0 {
		return false
	}
	for _, rule := range s.rules {
		if rule.isPrefix {
			if withinRoot(rule.pattern, absPath) {
				return true
			}
			continue
		}
		if rule.isGlob {
			if ok, _ := filepath.Match(rule.pattern, absPath); ok {
				return true
			}
			continue
		}
		if samePath(rule.pattern, absPath) {
			return true
		}
	}
	return false
}

func (s *WriteScope) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *WriteScope) Patterns() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.rules))
	for _, rule := range s.rules {
		out = append(out, rule.pattern)
	}
	return out
}

func normalizeWriteScopeRule(root, raw string) writeScopeRule {
	raw = filepath.Clean(raw)
	containsGlob := strings.ContainsAny(raw, "*?[]")
	if raw == "." || raw == string(filepath.Separator) {
		return writeScopeRule{pattern: root, isPrefix: true}
	}
	if strings.Contains(raw, "**") {
		prefix := strings.Split(raw, "**")[0]
		prefix = strings.TrimSuffix(prefix, string(filepath.Separator))
		prefix = strings.TrimSuffix(prefix, "/")
		if !filepath.IsAbs(prefix) {
			prefix = filepath.Join(root, prefix)
		}
		return writeScopeRule{pattern: filepath.Clean(prefix), isPrefix: true}
	}
	if containsGlob {
		pattern := raw
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, pattern)
		}
		return writeScopeRule{pattern: filepath.Clean(pattern), isGlob: true}
	}
	pattern := raw
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(root, pattern)
	}
	return writeScopeRule{pattern: filepath.Clean(pattern)}
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func withinRoot(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	rootWithSep := root + string(filepath.Separator)
	return strings.HasPrefix(path, rootWithSep)
}
