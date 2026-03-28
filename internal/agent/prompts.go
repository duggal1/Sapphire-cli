package agent

import (
	"context"
	_ "embed"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/Personality/soul/SOUL.md
var soulPromptSection []byte

//go:embed templates/skills_policy.md
var skillsPolicyPromptSection []byte

//go:embed templates/extended_skills.md
var extendedSkillsPromptSection []byte

//go:embed templates/codebase_indexing.md
var codebaseIndexingPromptSection []byte

//go:embed templates/mcp_policy.md
var mcpPolicyPromptSection []byte

//go:embed templates/modes/plan.md
var planModePromptSection []byte

//go:embed templates/modes/architect-mode.md
var architectModePromptSection []byte

//go:embed templates/modes/debug-mode.md
var debugModePromptSection []byte

//go:embed templates/modes/security-mode.md
var securityModePromptSection []byte

//go:embed templates/modes/review-mode.md
var reviewModePromptSection []byte

//go:embed templates/modes/orchestrator-mode.md
var orchestratorModePromptSection []byte

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
	return coderPromptForMode(planmode.DefaultSessionMode, opts...)
}

func coderPromptForMode(mode planmode.SessionMode, opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	sections := [][]byte{
		soulPromptSection,
		coderPromptTmpl,
		skillsPolicyPromptSection,
		extendedSkillsPromptSection,
		codebaseIndexingPromptSection,
		mcpPolicyPromptSection,
	}
	if modeSection := modePromptSection(planmode.NormalizeMode(mode)); modeSection != nil {
		sections = append(sections, modeSection)
	}
	sections = append(sections, mainAgentOrchestrationOverlay)
	systemPrompt, err := prompt.NewPrompt(
		"coder",
		string(composePromptSections(sections...)),
		opts...,
	)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	return taskPromptForMode(planmode.DefaultSessionMode, opts...)
}

func taskPromptForMode(mode planmode.SessionMode, opts ...prompt.Option) (*prompt.Prompt, error) {
	opts = append(opts, prompt.WithPlanToolPrompt(string(planToolPromptTmpl)))
	sections := [][]byte{
		soulPromptSection,
		taskPromptTmpl,
		skillsPolicyPromptSection,
		extendedSkillsPromptSection,
		codebaseIndexingPromptSection,
		mcpPolicyPromptSection,
	}
	if modeSection := modePromptSection(planmode.NormalizeMode(mode)); modeSection != nil {
		sections = append(sections, modeSection)
	}
	sections = append(sections, mainAgentOrchestrationOverlay)
	systemPrompt, err := prompt.NewPrompt(
		"task",
		string(composePromptSections(sections...)),
		opts...,
	)
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

func modePromptSection(mode planmode.SessionMode) []byte {
	switch planmode.NormalizeMode(mode) {
	case planmode.PlanMode:
		return planModePromptSection
	case planmode.ArchitectureMode:
		return architectModePromptSection
	case planmode.DebugMode:
		return debugModePromptSection
	case planmode.SecurityMode:
		return securityModePromptSection
	case planmode.ReviewMode:
		return reviewModePromptSection
	case planmode.OrchestratorMode:
		return orchestratorModePromptSection
	default:
		return nil
	}
}
