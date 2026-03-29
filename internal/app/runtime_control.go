package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/runtimecontrol"
	"github.com/duggal1/Sapphire-cli/internal/shell"
)

const runtimeControlPollInterval = 500 * time.Millisecond

func (app *App) startRuntimeControlLoop() {
	if app == nil || app.config == nil {
		return
	}
	dataDir := strings.TrimSpace(app.config.Options.DataDirectory)
	if dataDir == "" {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	startedAt := time.Now().UTC()

	go func() {
		defer close(done)
		ticker := time.NewTicker(runtimeControlPollInterval)
		defer ticker.Stop()

		var lastHandledRequestID string
		writeHeartbeat := func(now time.Time) {
			_ = runtimecontrol.WriteRuntimeStatus(dataDir, runtimecontrol.RuntimeStatus{
				PID:        os.Getpid(),
				StartedAt:  startedAt,
				UpdatedAt:  now,
				WorkingDir: app.config.WorkingDir(),
			})
		}

		writeHeartbeat(startedAt)
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				writeHeartbeat(now.UTC())
				req, err := runtimecontrol.ReadRequest(dataDir)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						continue
					}
					continue
				}
				if req.ID == "" || req.ID == lastHandledRequestID {
					continue
				}
				if !req.RequestedAt.IsZero() && req.RequestedAt.Before(startedAt) {
					_ = runtimecontrol.RemoveRequest(dataDir)
					continue
				}
				resp := app.handleRuntimeControlRequest(req)
				_ = runtimecontrol.WriteResponse(dataDir, resp)
				_ = runtimecontrol.RemoveRequest(dataDir)
				lastHandledRequestID = req.ID
			}
		}
	}()

	app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
		cancel()
		<-done
		return runtimecontrol.RemoveRuntimeStatus(dataDir)
	})
}

func (app *App) handleRuntimeControlRequest(req runtimecontrol.Request) runtimecontrol.Response {
	resp := runtimecontrol.Response{
		ID:        req.ID,
		Action:    req.Action,
		Status:    "ok",
		HandledAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(app.globalCtx), 10*time.Second)
	defer cancel()

	switch req.Action {
	case runtimecontrol.ActionStopBackground:
		summary, err := app.StopBackgroundActivity(ctx)
		resp.Summary = summary.ToMap()
		if err != nil {
			resp.Status = "error"
			resp.Message = err.Error()
		}
	default:
		resp.Status = "error"
		resp.Message = fmt.Sprintf("unsupported runtime control action %q", req.Action)
	}

	return resp
}

func (app *App) StopBackgroundActivity(ctx context.Context) (agent.BackgroundStopSummary, error) {
	var (
		summary agent.BackgroundStopSummary
		errs    []error
	)

	if stopper, ok := app.AgentCoordinator.(interface {
		StopBackgroundActivity(context.Context) (agent.BackgroundStopSummary, error)
	}); ok {
		agentSummary, err := stopper.StopBackgroundActivity(ctx)
		summary = agentSummary
		if err != nil {
			errs = append(errs, err)
		}
	}

	bgManager := shell.GetBackgroundShellManager()
	summary.KilledBackgroundShells = bgManager.RunningCount()
	bgManager.KillAll(ctx)

	fastManager := shell.GetFastBackgroundShellManager()
	summary.KilledFastBackgroundShells = fastManager.ActiveCount()
	if err := fastManager.KillAll(ctx); err != nil {
		errs = append(errs, err)
	}

	return summary, errors.Join(errs...)
}
