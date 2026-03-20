package formula

import "fmt"

type FormulaType string

const (
	TypeWorkflow FormulaType = "workflow"
)

type Formula struct {
	Name        string         `toml:"formula"`
	Description string         `toml:"description"`
	Type        FormulaType    `toml:"type"`
	Version     int            `toml:"version"`
	Steps       []Step         `toml:"steps"`
	Vars        map[string]Var `toml:"vars"`
}

type Step struct {
	ID          string   `toml:"id"`
	Title       string   `toml:"title"`
	Description string   `toml:"description"`
	Needs       []string `toml:"needs"`
	Acceptance  string   `toml:"acceptance"`
}

type Var struct {
	Description string `toml:"description"`
	Required    bool   `toml:"required"`
	Default     string `toml:"default"`
}

func (v *Var) UnmarshalTOML(data any) error {
	switch value := data.(type) {
	case string:
		v.Default = value
		return nil
	case map[string]any:
		if description, ok := value["description"].(string); ok {
			v.Description = description
		}
		if required, ok := value["required"].(bool); ok {
			v.Required = required
		}
		if def, ok := value["default"].(string); ok {
			v.Default = def
		}
		return nil
	default:
		return fmt.Errorf("expected string or table for var, got %T", data)
	}
}

func (f *Formula) StepByID(id string) (Step, bool) {
	for _, step := range f.Steps {
		if step.ID == id {
			return step, true
		}
	}
	return Step{}, false
}
