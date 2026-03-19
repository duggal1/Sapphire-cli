package agent

import (
	"context"
	_ "embed"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/config"
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
var orchestratorPromptHeader []byte

//go:embed templates/subagent_orchestrator.md.tpl
var subAgentOrchestratorPromptHeader []byte

//go:embed templates/orchestration/00_shared_principles.md
var orchestrationSharedPrinciples []byte

//go:embed templates/orchestration/10_startup_and_recovery.md
var orchestrationStartupAndRecovery []byte

//go:embed templates/orchestration/20_worktree_and_git.md
var orchestrationWorktreeAndGit []byte

//go:embed templates/orchestration/30_mail_protocol.md
var orchestrationMailProtocol []byte

//go:embed templates/orchestration/40_health_and_stall.md
var orchestrationHealthAndStall []byte

//go:embed templates/orchestration/50_reporting_and_validation.md
var orchestrationReportingAndValidation []byte

//go:embed templates/orchestration/60_orchestrator_role.md
var orchestrationOrchestratorRole []byte

//go:embed templates/orchestration/70_subagent_role.md
var orchestrationSubagentRole []byte

//go:embed templates/orchestration/80_handoff_and_long_horizon.md
var orchestrationHandoffAndLongHorizon []byte

//go:embed templates/memories/read_path.md
var memoryReadPrompt []byte

//go:embed templates/memories/stage_one_system.md
var memoryExtractionPrompt []byte

//go:embed templates/memories/stage_one_input.md
var memoryExtractionInputPrompt []byte

//go:embed templates/memories/consolidation.md
var memoryConsolidationPrompt []byte

var orchestratorPrompt = composePromptSections(
	orchestratorPromptHeader,
	orchestrationSharedPrinciples,
	orchestrationStartupAndRecovery,
	orchestrationWorktreeAndGit,
	orchestrationMailProtocol,
	orchestrationHealthAndStall,
	orchestrationReportingAndValidation,
	orchestrationOrchestratorRole,
	orchestrationHandoffAndLongHorizon,
)

var subAgentOrchestratorPrompt = composePromptSections(
	subAgentOrchestratorPromptHeader,
	orchestrationSharedPrinciples,
	orchestrationStartupAndRecovery,
	orchestrationWorktreeAndGit,
	orchestrationMailProtocol,
	orchestrationHealthAndStall,
	orchestrationReportingAndValidation,
	orchestrationSubagentRole,
	orchestrationHandoffAndLongHorizon,
)

var mainAgentOrchestrationOverlay = composePromptSections(
	orchestrationSharedPrinciples,
	orchestrationStartupAndRecovery,
	orchestrationWorktreeAndGit,
	orchestrationMailProtocol,
	orchestrationHealthAndStall,
	orchestrationReportingAndValidation,
	orchestrationOrchestratorRole,
	orchestrationHandoffAndLongHorizon,
)

// coderPrompt creates a new prompt specifically tailored for the coding agent.
func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	systemPrompt, err := prompt.NewPrompt("coder", appendPromptSections(string(coderPromptTmpl), string(mainAgentOrchestrationOverlay)), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	systemPrompt, err := prompt.NewPrompt("task", appendPromptSections(string(taskPromptTmpl), string(mainAgentOrchestrationOverlay)), opts...)
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

func composePromptSections(parts ...[]byte) []byte {
	sections := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(string(part)); text != "" {
			sections = append(sections, text)
		}
	}
	if len(sections) == 0 {
		return nil
	}
	return []byte(strings.Join(sections, "\n\n"))
}

func appendPromptSections(base string, extras ...string) string {
	sections := []string{strings.TrimSpace(base)}
	for _, extra := range extras {
		if text := strings.TrimSpace(extra); text != "" {
			sections = append(sections, text)
		}
	}
	return strings.Join(sections, "\n\n")
}
