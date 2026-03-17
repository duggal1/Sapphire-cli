package contextmanager

import "github.com/charmbracelet/sapphire/internal/message"

func SliceFromSummaryCheckpoint(msgs []message.Message, summaryMessageID string) []message.Message {
	if summaryMessageID == "" {
		return msgs
	}
	for i, msg := range msgs {
		if msg.ID != summaryMessageID {
			continue
		}
		sliced := append([]message.Message(nil), msgs[i:]...)
		if len(sliced) > 0 {
			sliced[0].Role = message.User
		}
		return sliced
	}
	return msgs
}
