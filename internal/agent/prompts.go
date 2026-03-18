package agent

import (
	"context"
	_ "embed"

	"github.com/charmbracelet/sapphire/internal/agent/prompt"
	"github.com/charmbracelet/sapphire/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/plan_tool.md.tpl
var planToolPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

//go:embed templates/structured_summary.md
var structuredSummaryPromptTmpl []byte

//go:embed templates/orchestrator.md.tpl
var orchestratorPrompt []byte

//go:embed templates/subagent_orchestrator.md.tpl
var subAgentOrchestratorPrompt []byte

//go:embed templates/memories/read_path.md
var memoryReadPrompt []byte

//go:embed templates/memories/stage_one_system.md
var memoryExtractionPrompt []byte

//go:embed templates/memories/stage_one_input.md
var memoryExtractionInputPrompt []byte

//go:embed templates/memories/consolidation.md
var memoryConsolidationPrompt []byte

// coderPrompt creates a new prompt specifically tailored for the coding agent.
func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	systemPrompt, err := prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func InitializePrompt(cfg config.Config) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
