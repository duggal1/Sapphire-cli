package mailbox

import "context"

func (s *Service) Nudge(ctx context.Context, recipient string) error {
	if s == nil || s.nudge == nil {
		return nil
	}
	return s.nudge(ctx, recipient)
}
