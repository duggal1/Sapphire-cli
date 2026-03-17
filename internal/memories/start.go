package memories

import (
	"context"
	"fmt"

	"github.com/charmbracelet/sapphire/internal/db"
)

func (s *Service) runStartup(ctx context.Context, opts StartOptions) error {
	if s == nil {
		return nil
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}
	if err := s.q.EnsureStage1JobForSession(ctx, db.EnsureStage1JobForSessionParams{
		SessionID:   opts.SessionID,
		RolloutPath: "",
		Cwd:         opts.WorkingDir,
	}); err != nil {
		return fmt.Errorf("ensure stage1 job: %w", err)
	}
	if err := s.q.EnsureGlobalPhase2Job(ctx); err != nil {
		return fmt.Errorf("ensure phase2 job: %w", err)
	}
	if _, err := s.q.PruneStage1OutputsForRetention(ctx, db.PruneStage1OutputsForRetentionParams{
		LastUsage: nullableInt64(nowUnix() - int64(retentionWindow.Seconds())),
		Limit:     200,
	}); err != nil {
		return fmt.Errorf("prune stage1 outputs: %w", err)
	}
	if err := s.runPhase1(ctx); err != nil {
		return err
	}
	if err := s.runPhase2(ctx, opts.SessionID); err != nil {
		return err
	}
	return nil
}
