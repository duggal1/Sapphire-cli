package cmd

import "testing"

func TestAgentRunDefaults(t *testing.T) {
	t.Parallel()

	if got := agentRunCmd.Flags().Lookup("model").DefValue; got != defaultAgentModel {
		t.Fatalf("unexpected default model: %q", got)
	}
	if got := agentRunCmd.Flags().Lookup("small-model").DefValue; got != defaultAgentModel {
		t.Fatalf("unexpected default small model: %q", got)
	}
	if got := agentRunCmd.Flags().Lookup("reasoning-effort").DefValue; got != defaultAgentReasoning {
		t.Fatalf("unexpected default reasoning effort: %q", got)
	}
}
