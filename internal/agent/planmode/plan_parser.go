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
	// Content is the raw Markdown content inside the <plan> tags
	Content string
	
	// IsValid indicates if the plan block format is valid per Codex rules
	IsValid bool
	
	// ValidationError contains the error message if IsValid is false
	ValidationError string
}

// planBlockRegex matches <plan>...</plan> blocks
// - Opening tag must be on its own line
// - Closing tag must be on its own line
var planBlockRegex = regexp.MustCompile(`(?s)^\s*<plan>\s*\n(.*?)\n\s*</plan>\s*$`)

// ExtractPlanBlock extracts the first <plan> block from model response content
// Returns the plan content and a boolean indicating if a plan block was found
func ExtractPlanBlock(content string) (string, bool) {
	matches := planBlockRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

// ExtractAllPlanBlocks extracts all <plan> blocks from model response content
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

// ValidatePlanBlock validates a plan block against Codex format requirements
func ValidatePlanBlock(content string) *PlanBlock {
	plan := &PlanBlock{
		Content: content,
		IsValid: true,
	}

	// Check 1: Content is not empty
	if strings.TrimSpace(content) == "" {
		plan.IsValid = false
		plan.ValidationError = "plan block content cannot be empty"
		return plan
	}

	// Check 2: No nested <plan> tags
	if strings.Contains(content, "<plan>") || strings.Contains(content, "</plan>") {
		plan.IsValid = false
		plan.ValidationError = "nested <plan> tags are not allowed"
		return plan
	}

	// Check 3: Content should have some structure (at least a title or summary)
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		plan.IsValid = false
		plan.ValidationError = "plan block should have multiple lines with structure (title, summary, etc.)"
		return plan
	}

	return plan
}

// ParsePlanBlock extracts and validates a <plan> block from content
func ParsePlanBlock(content string) (*PlanBlock, bool) {
	planContent, found := ExtractPlanBlock(content)
	if !found {
		return nil, false
	}

	validation := ValidatePlanBlock(planContent)
	return validation, true
}

// HasPlanBlock checks if content contains a <plan> block
func HasPlanBlock(content string) bool {
	_, found := ExtractPlanBlock(content)
	return found
}

// CountPlanBlocks returns the number of <plan> blocks in content
// Codex rule: "Only produce at most one <plan> block per turn"
func CountPlanBlocks(content string) int {
	return len(ExtractAllPlanBlocks(content))
}

// RemovePlanBlocks removes all <plan> blocks from content
// Useful for getting the non-plan portion of a response
func RemovePlanBlocks(content string) string {
	return planBlockRegex.ReplaceAllString(content, "")
}

// FormatPlanBlock formats content as a valid <plan> block
func FormatPlanBlock(content string) string {
	return "<plan>\n" + strings.TrimSpace(content) + "\n</plan>"
}

// IsPlanModeOnlyPlanBlock checks if the content is ONLY a <plan> block (no other text)
// Codex rule: Final output should be plan-only, no "should I proceed?" questions
func IsPlanModeOnlyPlanBlock(content string) bool {
	trimmed := strings.TrimSpace(content)
	matches := planBlockRegex.FindStringSubmatch(trimmed)
	return len(matches) > 0 && strings.TrimSpace(matches[0]) == trimmed
}
