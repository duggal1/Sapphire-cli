package agent

import "testing"

func TestShouldActivateLongHorizonRequiresStrongerSignals(t *testing.T) {
	agent := &sessionAgent{}

	longButOrdinary := SessionAgentCall{
		Prompt: "Investigate the coordinator, review the prompt system, inspect the tool registry, trace the background task paths, and summarize the architecture in detail without turning this into a persistent multi-turn workflow.",
	}
	if agent.shouldActivateLongHorizon(longButOrdinary) {
		t.Fatal("unexpected long-horizon activation for ordinary broad prompt")
	}

	explicitLongHorizon := SessionAgentCall{
		Prompt: "Create a long-horizon roadmap with milestones and resume later across sessions.",
	}
	if !agent.shouldActivateLongHorizon(explicitLongHorizon) {
		t.Fatal("expected long-horizon activation for explicit milestone workflow")
	}

	deepPlanningComplex := SessionAgentCall{
		Prompt:             "Deep planning request: investigate the entire codebase architecture across the whole project, coordinate multiple packages, trace cross-cutting integration paths, and produce a comprehensive implementation strategy for a large refactor that touches the architecture, migration shape, and broad integration boundaries across many components. This needs sustained execution across a very large task surface with many interacting components, explicit planning depth, broad codebase coverage, careful dependency tracing, and detailed reasoning before execution begins in earnest because the work spans multiple files, multiple packages, and broad architectural surfaces.",
		DeepPlanningActive: true,
	}
	if !agent.shouldActivateLongHorizon(deepPlanningComplex) {
		t.Fatal("expected long-horizon activation for very large deep-planning task")
	}
}
