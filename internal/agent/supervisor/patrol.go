package supervisor

import "context"

func (s *Service) RunPatrolCycle(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.runPatrolCycle(ctx)
	return nil
}
