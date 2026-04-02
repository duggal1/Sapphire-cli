package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/session"
)

func TestBuildTurnPolicyRequiresHighContextForAutoMemoryInjection(t *testing.T) {
	t.Parallel()

	lowContext := buildTurnPolicy(SessionAgentCall{Prompt: "fix the handler"}, session.Session{
		PromptTokens:     10000,
		CompletionTokens: 0,
	}, 100000, false, false)
	if lowContext.AllowAutoMemoryInjection {
		t.Fatalf("expected auto memory injection to stay off below 50%% context usage")
	}

	highContext := buildTurnPolicy(SessionAgentCall{Prompt: "fix the handler"}, session.Session{
		PromptTokens:     50000,
		CompletionTokens: 0,
	}, 100000, false, false)
	if !highContext.AllowAutoMemoryInjection {
		t.Fatalf("expected auto memory injection at 50%% context usage")
	}
}

func TestBuildTurnPolicyAllowsMemoryReadsForExplicitContinuity(t *testing.T) {
	t.Parallel()

	policy := buildTurnPolicy(SessionAgentCall{Prompt: "resume the earlier session and recover the prior decision"}, session.Session{}, 100000, false, false)
	if !policy.AllowMemoryRead {
		t.Fatalf("expected memory reads for explicit continuity request")
	}
	if !policy.AllowAutoMemoryInjection {
		t.Fatalf("expected auto memory injection for continuity request")
	}
}

func TestBuildTurnPolicyKeepsMemoryOffOnNormalShortHorizonTurn(t *testing.T) {
	t.Parallel()

	policy := buildTurnPolicy(SessionAgentCall{Prompt: "fix the handler"}, session.Session{}, 100000, false, false)
	if policy.AllowMemoryRead || policy.AllowMemoryWrite || policy.AllowAutoMemoryInjection {
		t.Fatalf("expected durable memory to stay off on a normal short-horizon turn, got %+v", policy)
	}
}
