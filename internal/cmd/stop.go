package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/runtimecontrol"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const backgroundStopReason = "background activity stopped by user"

var stopCmd = &cobra.Command{
	Use:     "stop [allbg]",
	Aliases: []string{"kill"},
	Short:   "Stop or kill live background activity in a running Sapphire instance",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "allbg") {
			return nil
		}
		return fmt.Errorf("usage: %s [allbg]", cmd.CommandPath())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		allbg := len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "allbg")
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		debug, _ := cmd.Flags().GetBool("debug")
		dataDir, _ := cmd.Flags().GetString("data-dir")
		cfg, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return err
		}
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if timeout <= 0 {
			timeout = 10 * time.Second
		}

		status, err := runtimecontrol.ReadRuntimeStatus(cfg.Options.DataDirectory)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory)
			}
			return err
		}

		if runtimecontrol.IsLive(status, time.Now().UTC()) {
			if err := requestBackgroundStop(cmd, cfg.Options.DataDirectory, timeout); err == nil {
				if allbg && status.PID > 0 {
					if forceErr := forceStopRuntime(status.PID, timeout); forceErr != nil {
						return forceErr
					}
					cleanupRuntimeControlFiles(cfg.Options.DataDirectory)
					if cleanupErr := cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory); cleanupErr != nil {
						return cleanupErr
					}
					fmt.Fprintf(
						cmd.OutOrStdout(),
						"force-stopped Sapphire runtime pid=%d\n",
						status.PID,
					)
				}
				return nil
			} else if status.PID <= 0 {
				if cleanupErr := cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory); cleanupErr != nil {
					return errors.Join(err, cleanupErr)
				}
				return nil
			} else {
				if forceErr := forceStopRuntime(status.PID, timeout); forceErr != nil {
					return errors.Join(err, forceErr)
				}
				cleanupRuntimeControlFiles(cfg.Options.DataDirectory)
				if cleanupErr := cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory); cleanupErr != nil {
					return errors.Join(err, cleanupErr)
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"force-stopped Sapphire runtime pid=%d after stop request failed: %s\n",
					status.PID,
					strings.TrimSpace(err.Error()),
				)
				return nil
			}
		}

		if status.PID <= 0 {
			cleanupRuntimeControlFiles(cfg.Options.DataDirectory)
			return cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory)
		}
		if err := forceStopRuntime(status.PID, timeout); err != nil {
			return err
		}
		cleanupRuntimeControlFiles(cfg.Options.DataDirectory)
		if err := cleanupDurableBackgroundState(cmd, cfg.Options.DataDirectory); err != nil {
			return err
		}
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"force-stopped stale Sapphire runtime pid=%d\n",
			status.PID,
		)
		return nil
	},
}

func requestBackgroundStop(cmd *cobra.Command, dataDir string, timeout time.Duration) error {
	req := runtimecontrol.Request{
		ID:          uuid.NewString(),
		Action:      runtimecontrol.ActionStopBackground,
		RequestedAt: time.Now().UTC(),
	}
	if err := runtimecontrol.WriteRequest(dataDir, req); err != nil {
		return err
	}
	handled := false
	defer func() {
		if !handled {
			_ = runtimecontrol.RemoveRequest(dataDir)
		}
	}()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		resp, err := runtimecontrol.ReadResponse(dataDir)
		if err == nil && resp.ID == req.ID {
			if resp.Status != "ok" {
				return errors.New(strings.TrimSpace(resp.Message))
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"stopped background activity: sub_agents=%d background_tasks=%d dispatches=%d work_items=%d agent_states=%d mail=%d shells=%d fast_shells=%d indexes=%d\n",
				resp.Summary["closed_sub_agents"],
				resp.Summary["stopped_background_tasks"],
				resp.Summary["stopped_dispatches"],
				resp.Summary["blocked_work_items"],
				resp.Summary["blocked_agent_states"],
				resp.Summary["dead_lettered_mail"],
				resp.Summary["killed_background_shells"],
				resp.Summary["killed_fast_background_shells"],
				resp.Summary["cancelled_codebase_indexes"],
			)
			handled = true
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for Sapphire to stop background activity")
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-ticker.C:
		}
	}
}

func forceStopRuntime(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("runtime pid is required")
	}
	if !processExists(pid) {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	if err := signalRuntime(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	if waitForProcessExit(pid, minDuration(timeout, 2*time.Second)) {
		return nil
	}
	if err := signalRuntime(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	if !waitForProcessExit(pid, minDuration(timeout, 2*time.Second)) {
		return fmt.Errorf("runtime pid %d did not exit", pid)
	}
	return nil
}

func signalRuntime(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("runtime pid is required")
	}
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid == pid {
		if killErr := syscall.Kill(-pid, sig); killErr == nil || errors.Is(killErr, syscall.ESRCH) {
			return killErr
		} else if !errors.Is(killErr, syscall.EPERM) {
			return killErr
		}
	}
	return syscall.Kill(pid, sig)
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !processExists(pid)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func cleanupRuntimeControlFiles(dataDir string) {
	_ = runtimecontrol.RemoveRequest(dataDir)
	_ = os.Remove(runtimecontrol.ResponsePath(dataDir))
	_ = runtimecontrol.RemoveRuntimeStatus(dataDir)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func cleanupDurableBackgroundState(cmd *cobra.Command, dataDir string) error {
	summary, err := stopDurableBackgroundState(cmd.Context(), dataDir, backgroundStopReason)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		cmd.OutOrStdout(),
		"stopped durable background state: dispatches=%d work_items=%d agent_states=%d mail=%d\n",
		summary.StoppedDispatches,
		summary.BlockedWorkItems,
		summary.BlockedAgentState,
		summary.DeadLetteredMail,
	)
	return nil
}

func stopDurableBackgroundState(ctx context.Context, dataDir, reason string) (orchestrationdb.BackgroundCleanupSummary, error) {
	store, err := orchestrationdb.Open(ctx, dataDir)
	if err != nil {
		return orchestrationdb.BackgroundCleanupSummary{}, err
	}
	defer store.Close()
	return store.StopBackgroundActivity(ctx, reason)
}

func init() {
	stopCmd.Flags().Duration("timeout", 10*time.Second, "How long to wait for the running Sapphire instance to stop background activity")
}
