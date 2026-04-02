package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestKernelMutationBoundaryKeepsLearningOutOfKernel(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dataDir := filepath.Join(repoRoot, ".sapphire")
	require.NoError(t, os.MkdirAll(skills.ProjectSkillsDir(dataDir), 0o755))

	boundary := newKernelMutationBoundary(repoRoot, dataDir)

	learnedSkillPath := filepath.Join(skills.ProjectSkillsDir(dataDir), "autolearn-init", skills.SkillFileName)
	learnedPolicyPath := filepath.Join(dataDir, singularityDirName, singularityPolicyFileName)
	protectedPromptPath := filepath.Join(repoRoot, "internal", "agent", "templates", "initialize.md.tpl")
	protectedCatalogPath := filepath.Join(repoRoot, "internal", "agent", "tool_catalog_prompt.go")

	require.True(t, boundary.AllowsAutoWrite(learnedSkillPath))
	require.True(t, boundary.AllowsAutoWrite(learnedPolicyPath))
	require.False(t, boundary.IsKernelProtected(learnedSkillPath))
	require.False(t, boundary.IsKernelProtected(learnedPolicyPath))

	require.False(t, boundary.AllowsAutoWrite(protectedPromptPath))
	require.False(t, boundary.AllowsAutoWrite(protectedCatalogPath))
	require.True(t, boundary.IsKernelProtected(protectedPromptPath))
	require.True(t, boundary.IsKernelProtected(protectedCatalogPath))
}

func TestSingularityManagerCompilesRecurringRoutePolicyAndGeneratesSkill(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Initialize the codebase, map architecture across the repo, trace dependencies in parallel, and generate AGENTS.md."

	family := manager.StartTurn("session-init", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	require.Equal(t, "initialize", family.GoalType)

	manager.RecordToolCall("session-init", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("session-init", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("session-init", agenttools.ToolSearchToolName, "")
	manager.RecordToolResult("session-init", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("session-init", agenttools.RGFilesToolName, "")
	manager.RecordToolResult("session-init", agenttools.RGFilesToolName, "", "", false)
	manager.RecordToolCall("session-init", agenttools.AgenticViewToolName, "")
	manager.RecordToolResult("session-init", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("session-init", agenttools.IndexCodebaseToolName, "")
	manager.RecordToolResult("session-init", agenttools.IndexCodebaseToolName, "", "", false)
	manager.RecordToolCall("session-init", agenttools.DiagnosticsToolName, "")
	manager.RecordToolResult("session-init", agenttools.DiagnosticsToolName, "", "", false)
	manager.RecordToolCall("session-init", SpawnAgentToolName, "")
	manager.RecordToolResult("session-init", SpawnAgentToolName, "", "", false)
	manager.RecordToolCall("session-init", CollectResultToolName, "")
	manager.RecordToolResult("session-init", CollectResultToolName, "", "", false)

	trace := manager.FinishTurn("session-init", "completed", "Mapped the architecture and wrote the initialization guide.")
	require.NotNil(t, trace)

	manager.CompileTurn(context.Background(), trace)

	policy, lookedUpFamily, ok := manager.LookupPolicy(prompt)
	require.True(t, ok)
	require.Equal(t, family.ID, lookedUpFamily.ID)
	require.Equal(t, "initialize/broad/codebase", family.ID)
	require.Equal(t, family.ID, policy.TaskFamily)
	require.True(t, policy.RequireHarness)
	require.True(t, policy.PreferParallel)
	require.True(t, policy.PreferIndexCodebase)
	require.True(t, policy.ForbidBashDiscovery)
	require.True(t, policy.RequireContextRead)
	require.GreaterOrEqual(t, policy.Confidence, minPolicyConfidenceForInjection)
	require.Contains(t, []string{learnedPolicyStateCandidate, learnedPolicyStatePromoted}, policy.PromotionState)
	require.Equal(t, []string{
		agenttools.ToolSearchToolName,
		agenttools.AgenticViewToolName,
		agenttools.RGFilesToolName,
		agenttools.RGToolName,
		agenttools.LSToolName,
	}, policy.PreferredDiscovery)
	require.NotEmpty(t, policy.PreferredVerification)
	require.NotEmpty(t, policy.SkillName)
	require.NotEmpty(t, policy.SkillFilePath)
	require.Equal(t, []string{policy.SkillName}, policy.PreferredSkills)
	require.FileExists(t, policy.SkillFilePath)
	require.FileExists(t, manager.policyPath)
	require.True(t, strings.HasPrefix(policy.SkillFilePath, skills.ProjectSkillsDir(manager.dataDir)))

	skillContent, err := os.ReadFile(policy.SkillFilePath)
	require.NoError(t, err)
	require.Contains(t, string(skillContent), "This skill is procedural only. It must not edit Sapphire's kernel prompt, tool schema, or preflight layers.")

	hints, activeSkills := manager.RenderPromptHints(policy)
	require.Contains(t, hints, "<learned_route_policy")
	require.Contains(t, hints, "Run `run_harness` before editing, execution, or delegation.")
	require.Contains(t, hints, "Preferred discovery order:")
	require.Contains(t, hints, "Preferred verification tools:")
	require.Contains(t, hints, "<learned_project_skill>")
	require.Contains(t, activeSkills, policy.SkillName)

	learnedToolPolicy := manager.LearnedToolPolicy(policy)
	require.Equal(t, policy.TaskFamily, learnedToolPolicy.TaskFamily)
	require.True(t, learnedToolPolicy.ForbidBashDiscovery)
	require.True(t, learnedToolPolicy.RequireContextRead)
	require.True(t, learnedToolPolicy.RequirePostWriteVerification)

	learnedHarness := manager.LearnedHarnessRequirement(prompt, policy)
	require.NotNil(t, learnedHarness)
	require.True(t, learnedHarness.Required)
	require.True(t, learnedHarness.RequireBeforeDiscovery)

	reloaded := newTestSingularityManagerAt(t, manager.repoRoot, manager.dataDir)
	reloadedPolicy, _, ok := reloaded.LookupPolicy(prompt)
	require.True(t, ok)
	require.Equal(t, policy.TaskFamily, reloadedPolicy.TaskFamily)
	require.Equal(t, policy.SkillName, reloadedPolicy.SkillName)
}

func TestClassifyLearnedTaskFamilyUsesStableInitializationFamily(t *testing.T) {
	t.Parallel()

	family := classifyLearnedTaskFamily("Initialize this codebase thoroughly, inspect architecture across auth and billing, and write AGENTS.md for future agents.")
	require.Equal(t, "initialize/broad/codebase", family.ID)
	require.Equal(t, "initialize", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
	require.Equal(t, []string{"codebase"}, family.Domains)
}

func TestClassifyLearnedTaskFamilyTreatsArchitecturePromptsAsDesign(t *testing.T) {
	t.Parallel()

	family := classifyLearnedTaskFamily("Architecture task only. Read the repository and compare two designs for evolving cmd/api into a real HTTP server.")
	require.Equal(t, "design", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
}

func TestClassifyLearnedTaskFamilyTreatsArchitectureOnlyBenchmarkPromptAsDesign(t *testing.T) {
	t.Parallel()

	family := classifyLearnedTaskFamily("Architecture-only task: design a champion versus challenger benchmark lane for Sapphire singularity policy promotion. Ground the answer in the current repo only. Compare at least two concrete designs, explain repo fit, migration cost, verifier strategy, rollback strategy, and which files should change first. Do not implement code and do not assume nonexistent APIs or symbols.")
	require.Equal(t, "design", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
}

func TestClassifyLearnedTaskFamilyTreatsResearchPromptsAsResearch(t *testing.T) {
	t.Parallel()

	family := classifyLearnedTaskFamily("Research task only. Investigate the repository, compare options for auth and billing orchestration, and produce a recommendation.")
	require.Equal(t, "research", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
}

func TestClassifyLearnedTaskFamilyTreatsBroadImplementationPromptsAsImplementation(t *testing.T) {
	t.Parallel()

	family := classifyLearnedTaskFamily("Implement a safe multi-file change to wire platform.RuntimeConfig into cmd/api without changing auth or billing signatures. Inspect the repository broadly, plan the work before editing, then make the minimal code changes and verify the build.")
	require.Equal(t, "implementation", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
}

func TestBuildHarnessRequirementRequiresHarnessBeforeBroadDesignDiscovery(t *testing.T) {
	t.Parallel()

	requirement := buildHarnessRequirement("Architecture task only. Read the repository and compare two designs for evolving cmd/api into a real HTTP server.")
	require.True(t, requirement.Required)
	require.True(t, requirement.RequireBeforeDiscovery)
	require.Contains(t, requirement.Reason, "design")
}

func TestBuildHarnessRequirementRequiresHarnessBeforeBroadResearchDiscovery(t *testing.T) {
	t.Parallel()

	requirement := buildHarnessRequirement("Research the repository broadly, investigate auth and billing integration points, and compare the best implementation approaches.")
	require.True(t, requirement.Required)
	require.True(t, requirement.RequireBeforeDiscovery)
	require.Contains(t, requirement.Reason, "research")
}

func TestBuildHarnessRequirementTreatsBroadImplementationPromptsAsImplementation(t *testing.T) {
	t.Parallel()

	requirement := buildHarnessRequirement("Implement a safe multi-file change to wire platform.RuntimeConfig into cmd/api without changing auth or billing signatures. Inspect the repository broadly, plan the work before editing, then make the minimal code changes and verify the build.")
	require.True(t, requirement.Required)
	require.GreaterOrEqual(t, requirement.ComplexityScore, 3)
}

func TestColdStartRoutePolicyInjectsBroadInitializationDefaults(t *testing.T) {
	t.Parallel()

	policy, family, ok := coldStartRoutePolicy("Initialize this codebase thoroughly, inspect architecture across auth and billing, and write AGENTS.md for future agents.")
	require.True(t, ok)
	require.Equal(t, "initialize/broad/codebase", family.ID)
	require.Equal(t, family.ID, policy.TaskFamily)
	require.True(t, policy.RequireHarness)
	require.True(t, policy.PreferParallel)
	require.True(t, policy.ForbidBashDiscovery)
	require.True(t, policy.RequirePostWriteVerification)
	require.Equal(t, learnedPolicyStateCandidate, policy.PromotionState)
	require.Equal(t, []string{
		agenttools.ToolSearchToolName,
		agenttools.RGFilesToolName,
		agenttools.AgenticViewToolName,
		agenttools.RGToolName,
		agenttools.LSToolName,
	}, policy.PreferredDiscovery)
}

func TestColdStartRoutePolicyInjectsBroadDesignDefaults(t *testing.T) {
	t.Parallel()

	policy, family, ok := coldStartRoutePolicy("Architecture task only. Read the repository broadly, compare two backend designs, and recommend the best fit.")
	require.True(t, ok)
	require.Equal(t, "design", family.GoalType)
	require.True(t, policy.RequireHarness)
	require.True(t, policy.ForbidBashDiscovery)
	require.True(t, policy.RequireContextRead)
	require.True(t, policy.RequireExplicitPlan)
	require.False(t, policy.RequirePostWriteVerification)
}

func TestColdStartRoutePolicyInjectsBroadResearchDefaults(t *testing.T) {
	t.Parallel()

	policy, family, ok := coldStartRoutePolicy("Research the repository broadly, investigate auth and billing integration points, and compare the best implementation approaches.")
	require.True(t, ok)
	require.Equal(t, "research", family.GoalType)
	require.True(t, policy.RequireHarness)
	require.True(t, policy.ForbidBashDiscovery)
	require.True(t, policy.RequireContextRead)
	require.True(t, policy.RequireExplicitPlan)
}

func TestColdStartRoutePolicyInjectsBroadImplementationDefaults(t *testing.T) {
	t.Parallel()

	policy, family, ok := coldStartRoutePolicy("Implement a broad multi-file change across the repository, compare the safest integration path, and update the codebase in parallel.")
	require.True(t, ok)
	require.Equal(t, "implementation", family.GoalType)
	require.Equal(t, "broad", family.Breadth)
	require.True(t, policy.RequireHarness)
	require.True(t, policy.ForbidBashDiscovery)
	require.True(t, policy.RequireContextRead)
	require.True(t, policy.RequireExplicitPlan)
}

func TestCompileSingularityExperienceRequiresRealStructuredDiscovery(t *testing.T) {
	t.Parallel()

	trace := &completedTurnTrace{
		Prompt: "Architecture task only. Read the repository broadly, compare two backend designs, and recommend the best fit.",
		Family: learnedTaskFamily{
			ID:       "design/broad/backend",
			GoalType: "design",
			Breadth:  "broad",
			Domains:  []string{"backend"},
		},
		OrderedTools: []string{agenttools.RunHarnessToolName, agenttools.AgenticViewToolName},
		ToolCalls: map[string]int{
			agenttools.RunHarnessToolName:  1,
			agenttools.AgenticViewToolName: 1,
		},
		ReadEvidence: map[string]int{"internal/platform/runtime.go": 1},
	}

	experience := compileSingularityExperience(trace)
	require.Equal(t, "weak", experience.Context.Discipline)
	require.Equal(t, 0, experience.Context.StructuredEvidenceCount)
	require.Equal(t, 1, experience.Context.ReadEvidenceCount)
}

func TestAssessTracePlanQualityUsesStructuredPlanInPlanMode(t *testing.T) {
	t.Parallel()

	trace := &completedTurnTrace{
		Mode:   string(planmode.PlanMode),
		Prompt: "Plan the safest architecture migration across the repository and compare the implementation options.",
		Family: learnedTaskFamily{
			ID:       "design/broad/backend+infra",
			GoalType: "design",
			Breadth:  "broad",
		},
		ResultText: `<proposed_plan>
## Current Reality
- The repository currently routes runtime setup through internal/platform and cmd/api.

## Key Changes
1. First read the runtime and API wiring paths deeply enough to confirm the current boundary.
2. Then compare an incremental adapter versus a direct constructor migration.
3. Next choose the lower-blast-radius path and list the file-level change sequence.

## Risks
- Migration compatibility risk if cmd/api is switched before the adapter exists.
- Open question: whether auth and billing both depend on the current runtime shape.

## Test Plan
- Verify the recommendation against the current package structure and the existing runtime symbols.
</proposed_plan>`,
	}

	assessment := assessTracePlanQuality(trace)
	require.Equal(t, "strong", assessment.Discipline)
	require.Empty(t, assessment.Blockers)
}

func TestAssessTracePlanQualityUsesExecutionEvidenceWhenFinalTextIsBrief(t *testing.T) {
	t.Parallel()

	trace := &completedTurnTrace{
		Prompt: "Architecture task only. Read the repository broadly, compare backend designs, and recommend the best fit.",
		Family: learnedTaskFamily{
			ID:       "design/broad/backend",
			GoalType: "design",
			Breadth:  "broad",
		},
		OrderedTools: []string{
			agenttools.RunHarnessToolName,
			agenttools.ToolSearchToolName,
			agenttools.AgenticViewToolName,
			agenttools.UpdatePlanToolName,
		},
		ToolCalls: map[string]int{
			agenttools.RunHarnessToolName:  1,
			agenttools.ToolSearchToolName:  1,
			agenttools.AgenticViewToolName: 1,
			agenttools.UpdatePlanToolName:  1,
		},
		StructuredEvidence: map[string]int{"backend": 1},
		ReadEvidence:       map[string]int{"internal/platform/runtime.go": 1},
		ResultText:         "Validated the recommendation against the current package structure and migration risks.",
	}

	assessment := assessTracePlanQuality(trace)
	require.Equal(t, "strong", assessment.Discipline)
	require.NotContains(t, assessment.Blockers, "execution_sequence")
}

func TestAssessTraceArchitectureQualityRequiresRepoFitAndMigrationCost(t *testing.T) {
	t.Parallel()

	trace := &completedTurnTrace{
		Prompt: "Architecture task only. Read the repository broadly, compare two backend designs, and recommend the best fit.",
		Family: learnedTaskFamily{
			ID:       "design/broad/backend+infra",
			GoalType: "design",
			Breadth:  "broad",
		},
		ResultText: "Option A keeps cmd/api thin by adding an adapter around internal/platform.RuntimeConfig, while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it reuses the existing boundaries and supports a gradual migration with lower blast radius, better compatibility, and lower rollout cost.",
	}

	assessment := assessTraceArchitectureQuality(trace)
	require.Equal(t, "strong", assessment.Discipline)
	require.Empty(t, assessment.Blockers)
}

func TestSingularityManagerPenalizesWeakValidationDiscipline(t *testing.T) {
	t.Parallel()

	prompt := "Initialize the codebase, inspect architecture across the repo, and refresh AGENTS.md."

	strong := newTestSingularityManager(t)
	strong.StartTurn("strong-validation", prompt, strong.repoRoot, nil, learnedRoutePolicy{})
	strong.RecordToolCall("strong-validation", agenttools.RunHarnessToolName, "")
	strong.RecordToolResult("strong-validation", agenttools.RunHarnessToolName, "", "", false)
	strong.RecordToolCall("strong-validation", agenttools.ToolSearchToolName, "")
	strong.RecordToolResult("strong-validation", agenttools.ToolSearchToolName, "", "", false)
	strong.RecordToolCall("strong-validation", agenttools.AgenticViewToolName, "")
	strong.RecordToolResult("strong-validation", agenttools.AgenticViewToolName, "", "", false)
	strong.RecordToolCall("strong-validation", agenttools.DiagnosticsToolName, "")
	strong.RecordToolResult("strong-validation", agenttools.DiagnosticsToolName, "", "", false)
	strongTrace := strong.FinishTurn("strong-validation", "completed", "validated the generated initialization guide against diagnostics")
	require.NotNil(t, strongTrace)
	strong.CompileTurn(context.Background(), strongTrace)
	strongPolicy, _, ok := strong.LookupPolicy(prompt)
	require.True(t, ok)

	weak := newTestSingularityManager(t)
	weak.StartTurn("weak-validation", prompt, weak.repoRoot, nil, learnedRoutePolicy{})
	weak.RecordToolCall("weak-validation", agenttools.RunHarnessToolName, "")
	weak.RecordToolResult("weak-validation", agenttools.RunHarnessToolName, "", "", false)
	weak.RecordToolCall("weak-validation", agenttools.ToolSearchToolName, "")
	weak.RecordToolResult("weak-validation", agenttools.ToolSearchToolName, "", "", false)
	weak.RecordToolCall("weak-validation", agenttools.AgenticViewToolName, "")
	weak.RecordToolResult("weak-validation", agenttools.AgenticViewToolName, "", "", false)
	weakTrace := weak.FinishTurn("weak-validation", "completed", "wrote the initialization guide after reading the repository")
	require.NotNil(t, weakTrace)
	weak.CompileTurn(context.Background(), weakTrace)
	weakPolicy := weak.store.Policies[classifyLearnedTaskFamily(prompt).ID]
	require.NotEmpty(t, weakPolicy.TaskFamily)

	require.Less(t, weakPolicy.Confidence, strongPolicy.Confidence)
	require.Greater(t, weakPolicy.ValidationFailures, 0)
	require.Greater(t, weakPolicy.RecentValidationPenalty, 0.0)
	require.Equal(t, learnedPolicyStateQuarantined, weakPolicy.PromotionState)
}

func TestAssessSingularityCognitionRecognizesTradeoffsAndRepoGroundedValidation(t *testing.T) {
	t.Parallel()

	trace := &completedTurnTrace{
		Prompt: "Architecture task only. Read the repository and compare two approaches for evolving cmd/api into a real HTTP server.",
		Family: learnedTaskFamily{
			ID:       "design/broad/backend+infra",
			GoalType: "design",
			Breadth:  "broad",
		},
		OrderedTools: []string{agenttools.AgenticViewToolName, agenttools.ToolSearchToolName},
		ToolCalls: map[string]int{
			agenttools.AgenticViewToolName: 1,
			agenttools.ToolSearchToolName:  1,
		},
		ResultSummary: "Based on a review of the repository, I compared two approaches, listed trade-offs with pros and cons, and validated the recommendation against the current package structure.",
	}

	assessment := assessSingularityCognition(trace)
	require.Equal(t, "strong", assessment.ContextDiscipline)
	require.Equal(t, "strong", assessment.TradeoffDiscipline)
	require.Equal(t, "strong", assessment.ValidationDiscipline)
}

func TestSingularityManagerPenalizesWeakDecompositionDiscipline(t *testing.T) {
	t.Parallel()

	prompt := "Architecture task only. Read the repository broadly, compare two backend designs, and recommend the best fit."

	strong := newTestSingularityManager(t)
	strong.StartTurn("strong-decomp", prompt, strong.repoRoot, nil, learnedRoutePolicy{})
	strong.RecordToolCall("strong-decomp", agenttools.RunHarnessToolName, "")
	strong.RecordToolResult("strong-decomp", agenttools.RunHarnessToolName, "", "", false)
	strong.RecordToolCall("strong-decomp", agenttools.ToolSearchToolName, `{"query":"backend designs"}`)
	strong.RecordToolResult("strong-decomp", agenttools.ToolSearchToolName, "", "", false)
	strong.RecordToolCall("strong-decomp", agenttools.AgenticViewToolName, `{"paths":["internal/agent/coordinator.go"]}`)
	strong.RecordToolResult("strong-decomp", agenttools.AgenticViewToolName, "", "", false)
	strong.RecordToolCall("strong-decomp", agenttools.UpdatePlanToolName, `{"plan":[{"step":"compare designs","status":"in_progress"}]}`)
	strong.RecordToolResult("strong-decomp", agenttools.UpdatePlanToolName, "", "", false)
	strong.RecordToolCall("strong-decomp", SpawnAgentToolName, "")
	strong.RecordToolResult("strong-decomp", SpawnAgentToolName, "", "", false)
	strongTrace := strong.FinishTurn("strong-decomp", "completed", "Option A reuses the current repository structure, while Option B rewrites the boundary directly. I compared the trade-offs, explained why Option A is the better repo fit, and validated the recommendation against the current repository structure with lower migration cost and blast radius.")
	require.NotNil(t, strongTrace)
	strong.CompileTurn(context.Background(), strongTrace)
	strongPolicy := strong.store.Policies[classifyLearnedTaskFamily(prompt).ID]
	require.NotEmpty(t, strongPolicy.TaskFamily)

	weak := newTestSingularityManager(t)
	weak.StartTurn("weak-decomp", prompt, weak.repoRoot, nil, learnedRoutePolicy{})
	weak.RecordToolCall("weak-decomp", agenttools.ToolSearchToolName, `{"query":"backend designs"}`)
	weak.RecordToolResult("weak-decomp", agenttools.ToolSearchToolName, "", "", false)
	weak.RecordToolCall("weak-decomp", agenttools.AgenticViewToolName, `{"paths":["internal/agent/coordinator.go"]}`)
	weak.RecordToolResult("weak-decomp", agenttools.AgenticViewToolName, "", "", false)
	weakTrace := weak.FinishTurn("weak-decomp", "completed", "Compared two backend approaches and recommended one.")
	require.NotNil(t, weakTrace)
	weak.CompileTurn(context.Background(), weakTrace)
	weakPolicy := weak.store.Policies[classifyLearnedTaskFamily(prompt).ID]
	require.NotEmpty(t, weakPolicy.TaskFamily)

	require.Less(t, weakPolicy.Confidence, strongPolicy.Confidence)
	require.Greater(t, weakPolicy.DecompositionFailures, 0)
	require.Greater(t, weakPolicy.RecentDecompositionPenalty, 0.0)
}

func TestSingularityManagerQuarantinesWeakBroadDesignLearning(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Architecture task only. Read the repository broadly, compare backend designs, and recommend the best fit."

	manager.StartTurn("weak-design", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("weak-design", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("weak-design", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("weak-design", agenttools.ToolSearchToolName, `{"query":"backend design"}`)
	manager.RecordToolResult("weak-design", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("weak-design", agenttools.AgenticViewToolName, `{"paths":["internal/platform/runtime.go"]}`)
	manager.RecordToolResult("weak-design", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("weak-design", agenttools.UpdatePlanToolName, `{"plan":[{"step":"compare the repository approaches","status":"in_progress"}]}`)
	manager.RecordToolResult("weak-design", agenttools.UpdatePlanToolName, "", "", false)

	trace := manager.FinishTurn("weak-design", "completed", "Recommended the cleanest architecture.")
	require.NotNil(t, trace)
	manager.CompileTurn(context.Background(), trace)

	family := classifyLearnedTaskFamily(prompt)
	policy := manager.store.Policies[family.ID]
	require.Equal(t, learnedPolicyStateQuarantined, policy.PromotionState)
	require.Equal(t, singularityLearningVerdictQuarantined, policy.LastLearningVerdict)
	require.Equal(t, 1, policy.QuarantineCount)
	require.Equal(t, 0, policy.SuccessCount)
	require.Greater(t, policy.RecentQualityGatePenalty, 0.0)

	_, _, ok := manager.LookupPolicy(prompt)
	require.False(t, ok)
	require.Empty(t, policy.SkillFilePath)
}

func TestSingularityManagerAllowsStrongBroadDesignLearning(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Architecture task only. Read the repository broadly, compare backend designs, and recommend the best fit."

	manager.StartTurn("strong-design", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("strong-design", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("strong-design", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("strong-design", agenttools.ToolSearchToolName, `{"query":"backend design"}`)
	manager.RecordToolResult("strong-design", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("strong-design", agenttools.RGFilesToolName, `{"pattern":"*runtime*.go"}`)
	manager.RecordToolResult("strong-design", agenttools.RGFilesToolName, "", "", false)
	manager.RecordToolCall("strong-design", agenttools.AgenticViewToolName, `{"paths":["internal/platform/runtime.go"]}`)
	manager.RecordToolResult("strong-design", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("strong-design", agenttools.UpdatePlanToolName, `{"plan":[{"step":"compare the repository approaches","status":"in_progress"}]}`)
	manager.RecordToolResult("strong-design", agenttools.UpdatePlanToolName, "", "", false)
	manager.RecordToolCall("strong-design", agenttools.DiagnosticsToolName, "")
	manager.RecordToolResult("strong-design", agenttools.DiagnosticsToolName, "", "", false)

	trace := manager.FinishTurn("strong-design", "completed", "Option A keeps cmd/api thin by adding an adapter around internal/platform.RuntimeConfig, while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it reuses the existing boundaries and supports a gradual migration with lower blast radius, better compatibility, and lower rollout cost. I validated the recommendation against the current package structure and listed the trade-offs with pros and cons.")
	require.NotNil(t, trace)
	manager.CompileTurn(context.Background(), trace)

	policy := manager.store.Policies[classifyLearnedTaskFamily(prompt).ID]
	require.Equal(t, singularityLearningVerdictAccepted, policy.LastLearningVerdict)
	require.Zero(t, policy.QuarantineCount)
	require.Equal(t, learnedPolicyStateObserver, policy.PromotionState)
	require.Equal(t, 1, policy.ChallengerWins)
	require.GreaterOrEqual(t, policy.LastPlanQualityConfidence, 70)
	require.Equal(t, 95, policy.LastArchitectureConfidence)
}

func TestSingularityManagerRequiresMultipleStrongArchitectureWinsBeforePromotion(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Architecture task only. Read the repository broadly, compare backend designs, and recommend the best fit."
	family := classifyLearnedTaskFamily(prompt)

	runStrongDesignTurn := func(sessionID string) learnedRoutePolicy {
		manager.StartTurn(sessionID, prompt, manager.repoRoot, nil, learnedRoutePolicy{})
		manager.RecordToolCall(sessionID, agenttools.RunHarnessToolName, "")
		manager.RecordToolResult(sessionID, agenttools.RunHarnessToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.ToolSearchToolName, `{"query":"runtime boundary"}`)
		manager.RecordToolResult(sessionID, agenttools.ToolSearchToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.RGFilesToolName, `{"pattern":"*runtime*.go"}`)
		manager.RecordToolResult(sessionID, agenttools.RGFilesToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.AgenticViewToolName, `{"paths":["internal/platform/runtime.go","cmd/api/main.go"]}`)
		manager.RecordToolResult(sessionID, agenttools.AgenticViewToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.UpdatePlanToolName, `{"plan":[{"step":"compare the adapter and rewrite options against the current repo boundaries","status":"in_progress"}]}`)
		manager.RecordToolResult(sessionID, agenttools.UpdatePlanToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.DiagnosticsToolName, "")
		manager.RecordToolResult(sessionID, agenttools.DiagnosticsToolName, "", "", false)

		trace := manager.FinishTurn(sessionID, "completed", "Option A keeps cmd/api thin by adding an adapter around internal/platform.RuntimeConfig, while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it reuses the existing boundaries and supports a gradual migration with lower blast radius, better compatibility, and lower rollout cost. I validated the recommendation against the current package structure and listed the trade-offs with pros and cons.")
		require.NotNil(t, trace)
		manager.CompileTurn(context.Background(), trace)
		return manager.store.Policies[family.ID]
	}

	first := runStrongDesignTurn("design-one")
	require.Equal(t, singularityLearningVerdictAccepted, first.LastLearningVerdict)
	require.Equal(t, 1, first.ChallengerWins)
	require.Equal(t, learnedPolicyStateObserver, first.PromotionState)
	_, _, ok := manager.LookupPolicy(prompt)
	require.False(t, ok)

	second := runStrongDesignTurn("design-two")
	require.Equal(t, 2, second.ChallengerWins)
	require.Equal(t, learnedPolicyStateCandidate, second.PromotionState)
	_, _, ok = manager.LookupPolicy(prompt)
	require.True(t, ok)

	third := runStrongDesignTurn("design-three")
	require.Equal(t, 3, third.ChallengerWins)
	require.Equal(t, learnedPolicyStatePromoted, third.PromotionState)
	require.GreaterOrEqual(t, third.LastPlanQualityConfidence, 70)
	require.Equal(t, 95, third.LastArchitectureConfidence)
}

func TestSingularityManagerBansDiscoveryBashAfterRepeatedStructuredWins(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Initialize the codebase, map architecture across the repo, trace dependencies in parallel, and generate AGENTS.md."

	manager.StartTurn("session-bash-fail", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("session-bash-fail", agenttools.BashToolName, `{"command":"find . -name '*.go'"}`)
	manager.RecordToolResult("session-bash-fail", agenttools.BashToolName, "learned route blocked bash", `{"tool_error":{"tool_name":"bash","code":"learned_route_policy","ui_message":"Use structured discovery tools."}}`, true)
	failedTrace := manager.FinishTurn("session-bash-fail", "error", "bash discovery looped without enough context")
	require.NotNil(t, failedTrace)
	manager.CompileTurn(context.Background(), failedTrace)

	for i := 0; i < 2; i++ {
		sessionID := "session-structured-" + string(rune('a'+i))
		manager.StartTurn(sessionID, prompt, manager.repoRoot, nil, learnedRoutePolicy{})
		manager.RecordToolCall(sessionID, agenttools.RunHarnessToolName, "")
		manager.RecordToolResult(sessionID, agenttools.RunHarnessToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.ToolSearchToolName, "")
		manager.RecordToolResult(sessionID, agenttools.ToolSearchToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.RGFilesToolName, "")
		manager.RecordToolResult(sessionID, agenttools.RGFilesToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.AgenticViewToolName, "")
		manager.RecordToolResult(sessionID, agenttools.AgenticViewToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.IndexCodebaseToolName, "")
		manager.RecordToolResult(sessionID, agenttools.IndexCodebaseToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.DiagnosticsToolName, "")
		manager.RecordToolResult(sessionID, agenttools.DiagnosticsToolName, "", "", false)
		manager.RecordToolCall(sessionID, SpawnAgentToolName, "")
		manager.RecordToolResult(sessionID, SpawnAgentToolName, "", "", false)
		successTrace := manager.FinishTurn(sessionID, "completed", "Mapped the repository, validated the initialization guide against the current package structure, and completed the structured initialization route.")
		require.NotNil(t, successTrace)
		manager.CompileTurn(context.Background(), successTrace)
	}

	policy, _, ok := manager.LookupPolicy(prompt)
	require.True(t, ok)
	require.True(t, policy.ForbidBashDiscovery)
	require.GreaterOrEqual(t, policy.Confidence, minPolicyConfidenceForInjection)
	require.GreaterOrEqual(t, policy.BashDiscoveryFailures, 1)

	learnedToolPolicy := manager.LearnedToolPolicy(policy)
	require.True(t, learnedToolPolicy.ForbidBashDiscovery)

	hints, _ := manager.RenderPromptHints(policy)
	require.Contains(t, hints, "Do not use bash for repository discovery")
}

func newTestSingularityManager(t *testing.T) *singularityManager {
	t.Helper()
	repoRoot := t.TempDir()
	dataDir := filepath.Join(repoRoot, ".sapphire")
	return newTestSingularityManagerAt(t, repoRoot, dataDir)
}

func newTestSingularityManagerAt(t *testing.T, repoRoot, dataDir string) *singularityManager {
	t.Helper()
	require.NoError(t, os.MkdirAll(skills.ProjectSkillsDir(dataDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, singularityDirName), 0o755))

	manager := &singularityManager{
		repoRoot:   filepath.Clean(repoRoot),
		dataDir:    filepath.Clean(dataDir),
		policyDir:  filepath.Join(filepath.Clean(dataDir), singularityDirName),
		historyDir: filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityHistoryDirName),
		policyPath: filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityPolicyFileName),
		auditPath:  filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityAuditFileName),
		skillRoot:  skills.ProjectSkillsDir(dataDir),
		boundary:   newKernelMutationBoundary(repoRoot, dataDir),
		store: learnedPolicyStore{
			Version:  singularityStoreVersion,
			Policies: map[string]learnedRoutePolicy{},
		},
		active: map[string]*turnLearningTrace{},
	}
	manager.load()
	return manager
}
