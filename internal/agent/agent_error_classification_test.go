package agent

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestClassifyProviderTransportError_OpenRouterPrivacyBlocked(t *testing.T) {
	t.Parallel()

	title, details, ok := classifyProviderTransportError(
		errors.New(`Post "https://openrouter.ai/api/v1/chat/completions": No endpoints available matching your guardrail restrictions and data policy. Configure: https://openrouter.ai/settings/privacy`),
		lipgloss.NewStyle(),
	)
	if !ok {
		t.Fatal("expected openrouter privacy error to be classified")
	}
	if title != "OpenRouter privacy settings blocked this model" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(details, "openrouter.ai/settings/privacy") {
		t.Fatalf("expected privacy settings link in details, got %q", details)
	}
}

func TestClassifyProviderTransportError_OpenRouterConnectionReset(t *testing.T) {
	t.Parallel()

	title, details, ok := classifyProviderTransportError(
		errors.New(`Post "https://openrouter.ai/api/v1/chat/completions": read tcp 192.168.1.9:59121->104.18.3.115:443: read: connection reset by peer`),
		lipgloss.NewStyle(),
	)
	if !ok {
		t.Fatal("expected openrouter connection reset to be classified")
	}
	if title != "OpenRouter connection reset" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(strings.ToLower(details), "retry") {
		t.Fatalf("expected retry guidance, got %q", details)
	}
}
