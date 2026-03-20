package formula

import (
	"fmt"
	"sort"
	"strings"
)

type SynthesisVerdict string

const (
	VerdictGo          SynthesisVerdict = "GO"
	VerdictGoWithFixes SynthesisVerdict = "GO_WITH_FIXES"
	VerdictNoGo        SynthesisVerdict = "NO_GO"
)

type FindingSeverity string

const (
	SeverityMustFix     FindingSeverity = "must_fix"
	SeverityShouldFix   FindingSeverity = "should_fix"
	SeverityObservation FindingSeverity = "observation"
)

type Finding struct {
	Text     string
	Severity FindingSeverity
	Sources  []string
}

type LegReport struct {
	LegType      string
	Verdict      string
	MustFix      []string
	ShouldFix    []string
	Observations []string
	Raw          string
}

type SynthesisResult struct {
	Verdict      SynthesisVerdict
	LegReports   []LegReport
	MustFix      []Finding
	ShouldFix    []Finding
	Observations []Finding
	Summary      string
}

func SynthesizeFindings(results []ExplorationResult) (SynthesisResult, error) {
	synthesis := SynthesisResult{
		Verdict:    VerdictGo,
		LegReports: make([]LegReport, 0, len(results)),
	}
	if len(results) == 0 {
		synthesis.Summary = "No exploration findings were available."
		return synthesis, nil
	}

	mustFix := make(map[string]*Finding)
	shouldFix := make(map[string]*Finding)
	observations := make(map[string]*Finding)
	hasFailedLeg := false

	for _, result := range results {
		report := parseLegReport(result)
		synthesis.LegReports = append(synthesis.LegReports, report)

		if reportVerdictIsFail(report.Verdict) || strings.TrimSpace(result.Error) != "" || strings.EqualFold(result.Status, "failed") {
			hasFailedLeg = true
		}
		mergeFindingMap(mustFix, report.LegType, report.MustFix, SeverityMustFix)
		mergeFindingMap(shouldFix, report.LegType, report.ShouldFix, SeverityShouldFix)
		mergeFindingMap(observations, report.LegType, report.Observations, SeverityObservation)
	}

	synthesis.MustFix = flattenFindingMap(mustFix)
	synthesis.ShouldFix = flattenFindingMap(shouldFix)
	synthesis.Observations = flattenFindingMap(observations)

	switch {
	case hasFailedLeg:
		synthesis.Verdict = VerdictNoGo
	case len(synthesis.MustFix) > 0:
		synthesis.Verdict = VerdictGoWithFixes
	default:
		synthesis.Verdict = VerdictGo
	}
	synthesis.Summary = buildSynthesisSummary(synthesis)
	return synthesis, nil
}

func parseLegReport(result ExplorationResult) LegReport {
	report := LegReport{
		LegType: strings.TrimSpace(result.LegType),
		Raw:     strings.TrimSpace(result.Result),
	}
	if report.LegType == "" {
		report.LegType = strings.TrimSpace(result.AgentID)
	}
	if parseTaggedLegReport(&report) {
		if strings.TrimSpace(result.Error) != "" {
			report.MustFix = append(report.MustFix, "Leg execution error: "+strings.TrimSpace(result.Error))
		}
		if report.Verdict == "" {
			report.Verdict = "PASS"
		}
		return report
	}

	currentSection := ""
	for _, rawLine := range strings.Split(report.Raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		switch {
		case strings.EqualFold(line, "## Verdict"):
			currentSection = "verdict"
		case strings.EqualFold(line, "## Must Fix"), strings.EqualFold(line, "## Critical Gaps / Questions"):
			currentSection = "must_fix"
		case strings.EqualFold(line, "## Should Fix"), strings.EqualFold(line, "## Important Considerations"):
			currentSection = "should_fix"
		case strings.EqualFold(line, "## Observations"):
			currentSection = "observations"
		case strings.HasPrefix(line, "## "):
			currentSection = ""
		default:
			switch currentSection {
			case "verdict":
				if report.Verdict == "" {
					report.Verdict = line
				}
			case "must_fix":
				if finding := trimListItem(line); finding != "" {
					report.MustFix = append(report.MustFix, finding)
				}
			case "should_fix":
				if finding := trimListItem(line); finding != "" {
					report.ShouldFix = append(report.ShouldFix, finding)
				}
			case "observations":
				if finding := trimListItem(line); finding != "" {
					report.Observations = append(report.Observations, finding)
				}
			}
		}
	}

	if report.Verdict == "" {
		switch {
		case strings.TrimSpace(result.Error) != "":
			report.Verdict = "FAIL"
		case len(report.MustFix) > 0:
			report.Verdict = "PASS WITH NOTES"
		default:
			report.Verdict = "PASS"
		}
	}
	if strings.TrimSpace(result.Error) != "" {
		report.MustFix = append(report.MustFix, "Leg execution error: "+strings.TrimSpace(result.Error))
	}
	return report
}

func parseTaggedLegReport(report *LegReport) bool {
	if report == nil {
		return false
	}
	raw := strings.TrimSpace(report.Raw)
	if raw == "" {
		return false
	}
	verdict := strings.TrimSpace(parseTaggedBlock(raw, "verdict"))
	mustFix := parseTaggedItems(raw, "must_fix")
	shouldFix := parseTaggedItems(raw, "should_fix")
	observations := parseTaggedItems(raw, "observations")
	if verdict == "" && len(mustFix) == 0 && len(shouldFix) == 0 && len(observations) == 0 {
		return false
	}
	report.Verdict = verdict
	report.MustFix = mustFix
	report.ShouldFix = shouldFix
	report.Observations = observations
	return true
}

func trimListItem(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	return strings.TrimSpace(line)
}

func parseTaggedItems(raw, tag string) []string {
	block := parseTaggedBlock(raw, tag)
	if block == "" {
		return nil
	}
	lines := strings.Split(block, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		if item := trimListItem(line); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func mergeFindingMap(target map[string]*Finding, source string, entries []string, severity FindingSeverity) {
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key := normalizeFindingKey(entry)
		if existing, ok := target[key]; ok {
			if !containsString(existing.Sources, source) {
				existing.Sources = append(existing.Sources, source)
			}
			continue
		}
		target[key] = &Finding{
			Text:     entry,
			Severity: severity,
			Sources:  []string{source},
		}
	}
}

func flattenFindingMap(items map[string]*Finding) []Finding {
	flattened := make([]Finding, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		sort.Strings(item.Sources)
		flattened = append(flattened, *item)
	}
	sort.Slice(flattened, func(i, j int) bool {
		if flattened[i].Text == flattened[j].Text {
			return strings.Join(flattened[i].Sources, ",") < strings.Join(flattened[j].Sources, ",")
		}
		return flattened[i].Text < flattened[j].Text
	})
	return flattened
}

func normalizeFindingKey(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer("`", "", "*", "", "_", "", ":", " ", ";", " ", ",", " ", ".", " ", "  ", " ")
	key = replacer.Replace(key)
	return strings.Join(strings.Fields(key), " ")
}

func reportVerdictIsFail(raw string) bool {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	return strings.HasPrefix(raw, "FAIL") || strings.Contains(raw, "NO-GO") || strings.Contains(raw, "NO_GO")
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func buildSynthesisSummary(result SynthesisResult) string {
	return fmt.Sprintf("%s with %d must-fix, %d should-fix, %d observations across %d legs.",
		result.Verdict,
		len(result.MustFix),
		len(result.ShouldFix),
		len(result.Observations),
		len(result.LegReports),
	)
}

func RenderSynthesisMarkdown(task string, result SynthesisResult) string {
	var builder strings.Builder
	builder.WriteString("<synthesis>\n")
	builder.WriteString("<overall_verdict>")
	builder.WriteString(string(result.Verdict))
	builder.WriteString("</overall_verdict>\n")
	builder.WriteString(fmt.Sprintf("<must_fix_count>%d</must_fix_count>\n", len(result.MustFix)))
	builder.WriteString(fmt.Sprintf("<should_fix_count>%d</should_fix_count>\n", len(result.ShouldFix)))
	builder.WriteString(fmt.Sprintf("<observation_count>%d</observation_count>\n", len(result.Observations)))
	builder.WriteString("</synthesis>\n\n")
	builder.WriteString("# Synthesis: ")
	builder.WriteString(strings.TrimSpace(task))
	builder.WriteString("\n\n## Overall Verdict\n")
	builder.WriteString("**")
	builder.WriteString(string(result.Verdict))
	builder.WriteString("**")
	if result.Summary != "" {
		builder.WriteString(" - ")
		builder.WriteString(strings.TrimSpace(result.Summary))
	}
	builder.WriteString("\n\n## Leg Verdicts\n")
	for _, report := range result.LegReports {
		builder.WriteString("- ")
		builder.WriteString(report.LegType)
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(report.Verdict))
		builder.WriteString("\n")
	}
	renderFindingSection(&builder, "Must Fix", result.MustFix)
	renderFindingSection(&builder, "Should Fix", result.ShouldFix)
	renderFindingSection(&builder, "Observations", result.Observations)
	return strings.TrimSpace(builder.String()) + "\n"
}

func renderFindingSection(builder *strings.Builder, title string, findings []Finding) {
	builder.WriteString("\n## ")
	builder.WriteString(title)
	builder.WriteString("\n")
	if len(findings) == 0 {
		builder.WriteString("- None.\n")
		return
	}
	for _, finding := range findings {
		builder.WriteString("- ")
		builder.WriteString(finding.Text)
		if len(finding.Sources) > 0 {
			builder.WriteString(" [")
			builder.WriteString(strings.Join(finding.Sources, ", "))
			builder.WriteString("]")
		}
		builder.WriteString("\n")
	}
}
