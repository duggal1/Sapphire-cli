package chat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

var proposedPlanPattern = regexp.MustCompile(`(?s)<proposed_plan>\s*(.*?)\s*</proposed_plan>`)

type proposedPlanParts struct {
	Before string
	Plan   string
	After  string
}

func extractProposedPlanParts(content string) (proposedPlanParts, bool) {
	loc := proposedPlanPattern.FindStringSubmatchIndex(content)
	if loc == nil || len(loc) < 4 {
		return proposedPlanParts{}, false
	}

	return proposedPlanParts{
		Before: strings.TrimSpace(content[:loc[0]]),
		Plan:   strings.TrimSpace(content[loc[2]:loc[3]]),
		After:  strings.TrimSpace(content[loc[1]:]),
	}, true
}

func ExtractProposedPlanParts(content string) (before string, plan string, after string, ok bool) {
	parts, ok := extractProposedPlanParts(content)
	if !ok {
		return "", "", "", false
	}
	return parts.Before, parts.Plan, parts.After, true
}

func stripProposedPlanBlocks(content string) string {
	return strings.TrimSpace(proposedPlanPattern.ReplaceAllString(content, ""))
}

func ProposedPlanID(messageID string) string {
	return fmt.Sprintf("%s:proposed-plan", messageID)
}

func NormalizeProposedPlanMessage(msg *message.Message) (*message.Message, string, bool) {
	if msg == nil {
		return msg, "", false
	}
	before, plan, after, ok := ExtractProposedPlanParts(msg.Content().Text)
	if !ok {
		return msg, "", false
	}
	parts := make([]string, 0, 2)
	if before != "" {
		parts = append(parts, before)
	}
	if after != "" {
		parts = append(parts, after)
	}
	stripped := strings.TrimSpace(strings.Join(parts, "\n\n"))

	clone := msg.Clone()
	updated := false
	for i, part := range clone.Parts {
		if _, ok := part.(message.TextContent); ok {
			clone.Parts[i] = message.TextContent{Text: stripped}
			updated = true
			break
		}
	}
	if !updated {
		clone.Parts = append(clone.Parts, message.TextContent{Text: stripped})
	}
	return &clone, plan, true
}

type ProposedPlanItem struct {
	*cachedMessageItem

	id   string
	sty  *styles.Styles
	plan string
}

func NewProposedPlanItem(sty *styles.Styles, messageID string, plan string) MessageItem {
	return &ProposedPlanItem{
		cachedMessageItem: &cachedMessageItem{},
		id:                ProposedPlanID(messageID),
		sty:               sty,
		plan:              strings.TrimSpace(plan),
	}
}

func (p *ProposedPlanItem) ID() string { return p.id }

func (p *ProposedPlanItem) SetPlan(plan string) {
	p.plan = strings.TrimSpace(plan)
	p.clearCache()
}

func (p *ProposedPlanItem) RawRender(width int) string {
	innerWidth := cappedMessageWidth(width)
	if rendered, _, ok := p.getCachedRender(innerWidth); ok {
		return rendered
	}

	title := p.sty.Base.Bold(true).Render("• Proposed Plan")
	renderer := common.MarkdownRenderer(p.sty, max(1, innerWidth-2))
	body, err := renderer.Render(p.plan)
	if err != nil {
		body = p.plan
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = p.sty.Subtle.Italic(true).Render("(empty)")
	}

	lines := []string{title, ""}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, "  "+line)
	}
	rendered := strings.Join(lines, "\n")
	p.setCachedRender(rendered, innerWidth, strings.Count(rendered, "\n")+1)
	return rendered
}

func (p *ProposedPlanItem) Render(width int) string {
	return p.RawRender(width)
}
