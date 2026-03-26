package planmode

import (
	"fmt"
	"regexp"
	"strings"
)

type StructuredBlock struct {
	Mode            SessionMode
	Title           string
	OpenTag         string
	CloseTag        string
	Content         string
	IsValid         bool
	ValidationError string
}

type structuredBlockSpec struct {
	mode  SessionMode
	title string
	tag   string
}

var structuredBlockSpecs = []structuredBlockSpec{
	{mode: PlanMode, title: "Plan", tag: "proposed_plan"},
	{mode: ArchitectureMode, title: "Architecture Spec", tag: "architecture_spec"},
	{mode: DebugMode, title: "Debug Report", tag: "debug_report"},
	{mode: ReviewMode, title: "Review Report", tag: "review_report"},
	{mode: SecurityMode, title: "Security Report", tag: "security_report"},
	{mode: OrchestratorMode, title: "Execution Orchestration", tag: "execution_orchestration"},
}

func blockSpecForMode(mode SessionMode) (structuredBlockSpec, bool) {
	mode = NormalizeMode(mode)
	for _, spec := range structuredBlockSpecs {
		if spec.mode == mode {
			return spec, true
		}
	}
	return structuredBlockSpec{}, false
}

func StructuredBlockTags(mode SessionMode) (string, string, bool) {
	spec, ok := blockSpecForMode(mode)
	if !ok {
		return "", "", false
	}
	return "<" + spec.tag + ">", "</" + spec.tag + ">", true
}

func blockRegex(tag string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`(?s)<%[1]s>\s*\n(.*?)\n\s*</%[1]s>`, regexp.QuoteMeta(tag)))
}

func extractStructuredBlockContent(spec structuredBlockSpec, content string) (string, bool) {
	openTag := "<" + spec.tag + ">"
	closeTag := "</" + spec.tag + ">"

	start := strings.Index(content, openTag)
	if start < 0 {
		return "", false
	}
	rest := content[start+len(openTag):]
	end := strings.Index(rest, closeTag)
	if end >= 0 {
		return rest[:end], true
	}
	return rest, true
}

func validateStructuredBlock(content string, tag string) *StructuredBlock {
	block := &StructuredBlock{
		Content: strings.TrimSpace(content),
		IsValid: true,
	}
	if block.Content == "" {
		block.IsValid = false
		block.ValidationError = "structured block content cannot be empty"
		return block
	}
	if strings.Contains(block.Content, "<"+tag+">") || strings.Contains(block.Content, "</"+tag+">") {
		block.IsValid = false
		block.ValidationError = "nested structured tags are not allowed"
		return block
	}
	return block
}

func ExtractStructuredBlock(content string) (*StructuredBlock, bool) {
	for _, spec := range structuredBlockSpecs {
		re := blockRegex(spec.tag)
		matches := re.FindStringSubmatch(content)
		if len(matches) < 2 {
			continue
		}
		block := validateStructuredBlock(matches[1], spec.tag)
		block.Mode = spec.mode
		block.Title = spec.title
		block.OpenTag = "<" + spec.tag + ">"
		block.CloseTag = "</" + spec.tag + ">"
		return block, true
	}
	return nil, false
}

func ExtractStructuredBlockForMode(mode SessionMode, content string) (*StructuredBlock, bool) {
	spec, ok := blockSpecForMode(mode)
	if !ok {
		return nil, false
	}
	re := blockRegex(spec.tag)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		block := validateStructuredBlock(matches[1], spec.tag)
		block.Mode = spec.mode
		block.Title = spec.title
		block.OpenTag = "<" + spec.tag + ">"
		block.CloseTag = "</" + spec.tag + ">"
		return block, true
	}

	raw, found := extractStructuredBlockContent(spec, content)
	if !found {
		return nil, false
	}
	block := validateStructuredBlock(raw, spec.tag)
	block.Mode = spec.mode
	block.Title = spec.title
	block.OpenTag = "<" + spec.tag + ">"
	block.CloseTag = "</" + spec.tag + ">"
	return block, true
}

func HasStructuredBlockForMode(mode SessionMode, content string) bool {
	_, ok := ExtractStructuredBlockForMode(mode, content)
	return ok
}

func RemoveStructuredBlocks(content string) string {
	out := content
	for _, spec := range structuredBlockSpecs {
		openTag := "<" + spec.tag + ">"
		closeTag := "</" + spec.tag + ">"
		for {
			start := strings.Index(out, openTag)
			if start < 0 {
				break
			}
			rest := out[start+len(openTag):]
			end := strings.Index(rest, closeTag)
			if end < 0 {
				out = out[:start]
				break
			}
			out = out[:start] + rest[end+len(closeTag):]
		}
	}
	return strings.TrimSpace(out)
}

func FormatStructuredBlock(mode SessionMode, content string) string {
	spec, ok := blockSpecForMode(mode)
	if !ok {
		return strings.TrimSpace(content)
	}
	body := strings.TrimSpace(content)
	if body == "" {
		return ""
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>", spec.tag, body, spec.tag)
}

// Legacy helpers retained for old call sites/tests, now mapped to Codex-style proposed_plan.
type PlanBlock = StructuredBlock

func ExtractPlanBlock(content string) (string, bool) {
	block, ok := ExtractStructuredBlockForMode(PlanMode, content)
	if !ok {
		return "", false
	}
	return block.Content, true
}

func ParsePlanBlock(content string) (*PlanBlock, bool) {
	block, ok := ExtractStructuredBlockForMode(PlanMode, content)
	if !ok {
		return nil, false
	}
	return block, true
}

func HasPlanBlock(content string) bool {
	return HasStructuredBlockForMode(PlanMode, content)
}

func CountPlanBlocks(content string) int {
	if HasPlanBlock(content) {
		return 1
	}
	return 0
}

func RemovePlanBlocks(content string) string {
	return RemoveStructuredBlocks(content)
}

func FormatPlanBlock(content string) string {
	return FormatStructuredBlock(PlanMode, content)
}
