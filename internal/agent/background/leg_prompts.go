package background

import (
	"fmt"
	"regexp"
	"strings"
)

type LegType string

const (
	LegRequirements LegType = "requirements"
	LegGaps         LegType = "gaps"
	LegAmbiguity    LegType = "ambiguity"
	LegFeasibility  LegType = "feasibility"
	LegScope        LegType = "scope"
)

var legPromptTag = regexp.MustCompile(`(?is)^\s*LEG_TYPE\s*=\s*([a-z_]+)\s*\n+`)

func OrderedLegTypes() []LegType {
	return []LegType{
		LegRequirements,
		LegGaps,
		LegAmbiguity,
		LegFeasibility,
		LegScope,
	}
}

func ParseLegType(raw string) (LegType, bool) {
	switch LegType(strings.ToLower(strings.TrimSpace(raw))) {
	case LegRequirements, LegGaps, LegAmbiguity, LegFeasibility, LegScope:
		return LegType(strings.ToLower(strings.TrimSpace(raw))), true
	default:
		return "", false
	}
}

func ExtractLegRequest(raw string) (LegType, string) {
	raw = strings.TrimSpace(raw)
	match := legPromptTag.FindStringSubmatch(raw)
	if len(match) != 2 {
		return "", raw
	}
	legType, ok := ParseLegType(match[1])
	if !ok {
		return "", raw
	}
	cleaned := strings.TrimSpace(legPromptTag.ReplaceAllString(raw, ""))
	return legType, cleaned
}

func BuildLegPrompt(legType LegType, taskContext string) string {
	taskContext = strings.TrimSpace(taskContext)
	var builder strings.Builder
	builder.WriteString("Task context:\n")
	if taskContext == "" {
		builder.WriteString("- No extra context provided. Inspect the repo directly.\n\n")
	} else {
		builder.WriteString(taskContext)
		builder.WriteString("\n\n")
	}
	builder.WriteString(legPromptBody(legType))
	return builder.String()
}

func legPromptBody(legType LegType) string {
	switch legType {
	case LegRequirements:
		return `Check requirements coverage only. Look for missing behaviors, acceptance criteria, failure cases, rollout checks, and testability gaps. Use repo evidence. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`
	case LegGaps:
		return `Check missing requirements only. Look for blind spots such as auth, migration, compatibility, operations, support tooling, concurrency, rollout, observability, and tenant-specific behavior. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`
	case LegAmbiguity:
		return `Check ambiguity only. Find vague, contradictory, or underspecified statements. Call out unclear scope boundaries, missing ordering, conflicting assumptions, and wording that could produce different implementations. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`
	case LegFeasibility:
		return `Check feasibility only. Find prerequisites, hard constraints, coupling, migration cost, performance risk, and capabilities the repo may not have. Flag where effort or risk is understated. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`
	case LegScope:
		return `Check scope only. Define the smallest viable implementation. Find scope creep, unnecessary refactors, premature polish, and work that belongs in later phases or follow-ups. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`
	default:
		return fmt.Sprintf(`Check only the "%s" dimension. Keep findings short and concrete. Return exactly this format and no extra sections:

<verdict>PASS|PASS_WITH_NOTES|FAIL</verdict>
<must_fix>
- item
</must_fix>
<should_fix>
- item
</should_fix>
<observations>
- item
</observations>`, legType)
	}
}
