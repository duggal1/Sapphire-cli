package formula

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

func ParseFile(path string) (*Formula, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read formula file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Formula, error) {
	parsed, err := parseTOMLWorkflow(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse formula TOML: %w", err)
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (f *Formula) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("formula field is required")
	}
	if f.Type == "" {
		f.Type = TypeWorkflow
	}
	if f.Type != TypeWorkflow {
		return fmt.Errorf("invalid formula type %q", f.Type)
	}
	if len(f.Steps) == 0 {
		return fmt.Errorf("workflow formula requires at least one step")
	}

	seen := make(map[string]struct{}, len(f.Steps))
	hasEntry := false
	for _, step := range f.Steps {
		if strings.TrimSpace(step.ID) == "" {
			return fmt.Errorf("step missing required id field")
		}
		if _, ok := seen[step.ID]; ok {
			return fmt.Errorf("duplicate step id: %s", step.ID)
		}
		seen[step.ID] = struct{}{}
		if len(step.Needs) == 0 {
			hasEntry = true
		}
	}
	if !hasEntry {
		return fmt.Errorf("workflow formula requires at least one entry step with no dependencies")
	}

	for _, step := range f.Steps {
		for _, need := range step.Needs {
			if _, ok := seen[need]; !ok {
				return fmt.Errorf("step %q needs unknown step: %s", step.ID, need)
			}
		}
	}

	return f.checkCycles()
}

func (f *Formula) checkCycles() error {
	visited := make(map[string]bool, len(f.Steps))
	active := make(map[string]bool, len(f.Steps))

	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fmt.Errorf("cycle detected involving: %s", id)
		}
		if visited[id] {
			return nil
		}
		visited[id] = true
		active[id] = true

		step, _ := f.StepByID(id)
		for _, need := range step.Needs {
			if err := visit(need); err != nil {
				return err
			}
		}

		active[id] = false
		return nil
	}

	for _, step := range f.Steps {
		if err := visit(step.ID); err != nil {
			return err
		}
	}
	return nil
}

func (f *Formula) TopologicalSort() ([]Step, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int, len(f.Steps))
	dependents := make(map[string][]string, len(f.Steps))
	orderIndex := make(map[string]int, len(f.Steps))
	for i, step := range f.Steps {
		orderIndex[step.ID] = i
		inDegree[step.ID] = len(step.Needs)
		for _, need := range step.Needs {
			dependents[need] = append(dependents[need], step.ID)
		}
	}

	queue := make([]string, 0, len(f.Steps))
	for _, step := range f.Steps {
		if inDegree[step.ID] == 0 {
			queue = append(queue, step.ID)
		}
	}

	sorted := make([]Step, 0, len(f.Steps))
	for len(queue) > 0 {
		slices.SortFunc(queue, func(a, b string) int {
			return orderIndex[a] - orderIndex[b]
		})
		id := queue[0]
		queue = queue[1:]
		step, _ := f.StepByID(id)
		sorted = append(sorted, step)
		for _, dependent := range dependents[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(f.Steps) {
		return nil, fmt.Errorf("formula contains dependency cycle")
	}
	return sorted, nil
}

func (f *Formula) ReadySteps(completed map[string]bool) []Step {
	if completed == nil {
		completed = map[string]bool{}
	}
	ready := make([]Step, 0, len(f.Steps))
	for _, step := range f.Steps {
		if completed[step.ID] {
			continue
		}
		allNeedsMet := true
		for _, need := range step.Needs {
			if !completed[need] {
				allNeedsMet = false
				break
			}
		}
		if allNeedsMet {
			ready = append(ready, step)
		}
	}
	return ready
}

func parseTOMLWorkflow(raw string) (*Formula, error) {
	lines := strings.Split(raw, "\n")
	formula := &Formula{Vars: make(map[string]Var)}
	var currentStep *Step
	currentVar := ""

	flushStep := func() {
		if currentStep != nil {
			formula.Steps = append(formula.Steps, *currentStep)
			currentStep = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case line == "[[steps]]":
			flushStep()
			currentVar = ""
			currentStep = &Step{}
			continue
		case strings.HasPrefix(line, "[vars.") && strings.HasSuffix(line, "]"):
			flushStep()
			currentVar = strings.TrimSuffix(strings.TrimPrefix(line, "[vars."), "]")
			if _, ok := formula.Vars[currentVar]; !ok {
				formula.Vars[currentVar] = Var{}
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid assignment %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if strings.HasPrefix(value, `"""`) {
			var multiline string
			var err error
			multiline, i, err = readMultilineValue(lines, i, value)
			if err != nil {
				return nil, err
			}
			value = multiline
		}

		switch {
		case currentStep != nil:
			if err := applyStepField(currentStep, key, value); err != nil {
				return nil, err
			}
		case currentVar != "":
			variable := formula.Vars[currentVar]
			if err := applyVarField(&variable, key, value); err != nil {
				return nil, err
			}
			formula.Vars[currentVar] = variable
		default:
			if err := applyFormulaField(formula, key, value); err != nil {
				return nil, err
			}
		}
	}

	flushStep()
	return formula, nil
}

func readMultilineValue(lines []string, start int, initial string) (string, int, error) {
	value := strings.TrimPrefix(initial, `"""`)
	if strings.HasSuffix(value, `"""`) {
		return strings.TrimSuffix(value, `"""`), start, nil
	}

	collected := []string{value}
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if idx := strings.Index(line, `"""`); idx >= 0 {
			collected = append(collected, line[:idx])
			return strings.Join(collected, "\n"), i, nil
		}
		collected = append(collected, line)
	}
	return "", start, fmt.Errorf("unterminated multiline string")
}

func applyFormulaField(formula *Formula, key, value string) error {
	switch key {
	case "formula":
		formula.Name = trimQuoted(value)
	case "description":
		formula.Description = trimQuoted(value)
	case "type":
		formula.Type = FormulaType(trimQuoted(value))
	case "version":
		version, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid version %q", value)
		}
		formula.Version = version
	default:
		return nil
	}
	return nil
}

func applyStepField(step *Step, key, value string) error {
	switch key {
	case "id":
		step.ID = trimQuoted(value)
	case "title":
		step.Title = trimQuoted(value)
	case "description":
		step.Description = trimQuoted(value)
	case "needs":
		needs, err := parseStringArray(value)
		if err != nil {
			return err
		}
		step.Needs = needs
	case "acceptance":
		step.Acceptance = trimQuoted(value)
	}
	return nil
}

func applyVarField(variable *Var, key, value string) error {
	switch key {
	case "description":
		variable.Description = trimQuoted(value)
	case "required":
		variable.Required = strings.EqualFold(value, "true")
	case "default":
		variable.Default = trimQuoted(value)
	}
	return nil
}

func parseStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "[]" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("invalid string array %q", raw)
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	if body == "" {
		return nil, nil
	}
	parts := strings.Split(body, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = trimQuoted(strings.TrimSpace(part))
		if part != "" {
			values = append(values, part)
		}
	}
	return values, nil
}

func trimQuoted(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	return value
}
