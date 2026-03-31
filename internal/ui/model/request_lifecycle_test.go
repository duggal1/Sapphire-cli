package model

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/ui/util"
)

func TestCancelPromptInfoMsgUsesImmediateErrorFeedback(t *testing.T) {
	t.Parallel()

	msg := cancelPromptInfoMsg()
	if msg.Type != util.InfoTypeError {
		t.Fatalf("expected error info type, got %v", msg.Type)
	}
	if msg.Msg != "Interrupted. Tell the model what to do differently." {
		t.Fatalf("unexpected cancel feedback message %q", msg.Msg)
	}
}
