package memory

import "strings"

const (
	constitutionCoreHeader    = "# Project Architecture Decisions (Core)"
	constitutionCoreSection   = "## Core Architecture Decisions"
	constitutionDurableHeader = "## Durable Cross-Session Decisions"
)

func mergeCoreConstitution(existing string, decisions []ArchitecturalDecision) string {
	existing = strings.TrimSpace(existing)
	var missing []string
	for _, decision := range decisions {
		line := formatConstitutionDecisionLine(decision)
		if line == "" || constitutionContainsLine(existing, line) {
			continue
		}
		missing = append(missing, line)
	}
	if len(missing) == 0 {
		return existing
	}
	if existing == "" {
		return strings.TrimSpace(constitutionCoreHeader + "\n\n" + strings.Join(missing, "\n"))
	}
	if strings.HasPrefix(existing, constitutionCoreHeader) || constitutionHasHeader(existing, constitutionCoreSection) {
		return strings.TrimSpace(existing + "\n" + strings.Join(missing, "\n"))
	}
	return strings.TrimSpace(existing + "\n\n" + constitutionCoreSection + "\n" + strings.Join(missing, "\n"))
}

func appendConstitutionDecision(existing, decision string) string {
	existing = strings.TrimSpace(existing)
	decision = strings.TrimSpace(decision)
	if decision == "" {
		return existing
	}
	line := "- " + decision
	if constitutionContainsLine(existing, line) {
		return existing
	}
	if existing == "" {
		return constitutionDurableHeader + "\n" + line
	}
	if constitutionHasHeader(existing, constitutionDurableHeader) {
		return strings.TrimSpace(existing + "\n" + line)
	}
	return strings.TrimSpace(existing + "\n\n" + constitutionDurableHeader + "\n" + line)
}

func formatConstitutionDecisionLine(decision ArchitecturalDecision) string {
	text := strings.TrimSpace(decision.Decision)
	if text == "" {
		return ""
	}
	rationale := strings.TrimSpace(decision.Rationale)
	if rationale == "" {
		return "- " + text
	}
	return "- " + text + ": " + rationale
}

func constitutionContainsLine(content, line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	for _, candidate := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(candidate) == line {
			return true
		}
	}
	return false
}

func constitutionHasHeader(content, header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.HasPrefix(content, header+"\n") {
		return true
	}
	return strings.Contains(content, "\n"+header+"\n")
}
