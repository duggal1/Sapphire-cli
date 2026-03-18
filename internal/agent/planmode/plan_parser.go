// Codex-style <plan> block parser
// Reference: codex-rs/core/templates/collaboration_mode/plan.md
//
// CODEX FORMAT REQUIREMENTS:
// 1. Opening tag <plan> must be on its own line
// 2. Plan content starts on the next line (no text on same line as tag)
// 3. Closing tag </plan> must be on its own line
// 4. Use Markdown inside the block
// 5. Tags must be exactly <plan> and </plan>

package planmode

import (
	"regexp"
	"strings"
)

// PlanBlock represents a parsed <plan> block from model response
type PlanBlock struct {
	Content           string
	IsValid           bool
	ValidationError   string
}

// planBlockRegex matches <plan>...</plan> blocks
// Opening and closing tags must be on their own lines
var planBlockRegex = regexp.MustCompile(`(?s)^\s*<plan>\s*\n(.*?)\n\s*</plan>\s*$`)

// ExtractPlanBlock extracts the first <plan> block from model response
// Returns the plan content and a boolean indicating if found
func ExtractPlanBlock(content string) (string, bool) {
	matches := planBlockRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

// ExtractAllPlanBlocks extracts all <plan> blocks
// Codex rule: "Only produce at most one <plan> block per turn"
func ExtractAllPlanBlocks(content string) []string {
	allMatches := planBlockRegex.FindAllStringSubmatch(content, -1)
	var plans []string
	for _, matches := range allMatches {
		if len(matches) >= 2 {
			plans = append(plans, strings.TrimSpace(matches[1]))
		}
	}
	return plans
}

// ValidatePlanBlock validates against Codex format requirements
func ValidatePlanBlock(content string) *PlanBlock {
	plan := &PlanBlock{Content: content, IsValid: true}

	if strings.TrimSpace(content) == "" {
		plan.IsValid = false
		plan.ValidationError = "plan block content cannot be empty"
		return plan
	}

	if strings.Contains(content, "<plan>") || strings.Contains(content, "</plan>") {
		plan.IsValid = false
		plan.ValidationError = "nested <plan> tags are not allowed"
		return plan
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		plan.IsValid = false
		plan.ValidationError = "plan block should have multiple lines with structure"
		return plan
	}

	return plan
}

// ParsePlanBlock extracts and validates a <plan> block
func ParsePlanBlock(content string) (*PlanBlock, bool) {
	planContent, found := ExtractPlanBlock(content)
	if !found {
		return nil, false
	}
	return ValidatePlanBlock(planContent), true
}

// HasPlanBlock checks if content contains a <plan> block
func HasPlanBlock(content string) bool {
	_, found := ExtractPlanBlock(content)
	return found
}

// CountPlanBlocks returns the number of <plan> blocks
func CountPlanBlocks(content string) int {
	return len(ExtractAllPlanBlocks(content))
}

// RemovePlanBlocks removes all <plan> blocks from content
func RemovePlanBlocks(content string) string {
	return planBlockRegex.ReplaceAllString(content, "")
}

// FormatPlanBlock formats content as a valid <plan> block
func FormatPlanBlock(content string) string {
	return "<plan>\n" + strings.TrimSpace(content) + "\n</plan>"
}
