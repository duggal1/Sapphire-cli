package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/event"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var worktreesCmd = &cobra.Command{
	Use:     "worktrees",
	Aliases: []string{"worktree"},
	Short:   "Worktree orchestration utilities",
}

var worktreesOrchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Spawn sub-agents in isolated git worktrees from a spec file",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = os.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")
		event.SetNonInteractive(true)

		specPath, _ := cmd.Flags().GetString("spec")
		resumePath, _ := cmd.Flags().GetString("resume")

		sessionTitle, _ := cmd.Flags().GetString("session-title")
		if strings.TrimSpace(sessionTitle) == "" {
			sessionTitle = "Worktree Orchestration"
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		defer cancel()

		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if !app.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'sapphire' to set up a provider interactively")
		}

		event.AppInitialized()

		session, err := app.Sessions.Create(ctx, sessionTitle)
		if err != nil {
			return err
		}

		if strings.TrimSpace(resumePath) != "" {
			resumePrompt, _ := cmd.Flags().GetString("resume-prompt")
			agentKey, _ := cmd.Flags().GetString("agent")
			model, _ := cmd.Flags().GetString("model")
			reasoningEffort, _ := cmd.Flags().GetString("reasoning-effort")
			ref, err := app.AgentCoordinator.ResumeWorktree(ctx, session.ID, resumePath, resumePrompt, agentKey, model, reasoningEffort)
			if err != nil {
				return err
			}
			renderWorktreeGrid(os.Stdout, agent.OrchestrateWorktreesResult{Tasks: []agent.OrchestrationAgentRef{ref}})
		} else {
			if strings.TrimSpace(specPath) == "" {
				return fmt.Errorf("spec is required")
			}
			params, err := loadWorktreeSpec(specPath)
			if err != nil {
				return err
			}
			result, err := app.AgentCoordinator.OrchestrateWorktrees(ctx, session.ID, params)
			if err != nil {
				return err
			}
			renderWorktreeGrid(os.Stdout, result)
		}
		event.AppExited()
		return nil
	},
}

var worktreesCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove merged Sapphire worktrees from the local repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		mergedOnly, _ := cmd.Flags().GetBool("merged")
		if !mergedOnly {
			return fmt.Errorf("--merged is required")
		}

		root, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		removed, err := cleanMergedWorktrees(cmd.Context(), root)
		if err != nil {
			return err
		}
		for _, item := range removed {
			fmt.Fprintln(cmd.OutOrStdout(), item)
		}
		if len(removed) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no merged worktrees removed")
		}
		return nil
	},
}

func init() {
	worktreesOrchestrateCmd.Flags().StringP("spec", "s", "", "Path to JSON/YAML orchestration spec")
	worktreesOrchestrateCmd.Flags().String("resume", "", "Resume an orphaned worktree by path")
	worktreesOrchestrateCmd.Flags().String("resume-prompt", "", "Prompt for resumed worktree")
	worktreesOrchestrateCmd.Flags().String("agent", "", "Agent profile to use for resume")
	worktreesOrchestrateCmd.Flags().String("model", "", "Model override for resume")
	worktreesOrchestrateCmd.Flags().String("reasoning-effort", "", "Reasoning effort override for resume (low, medium, high)")
	worktreesOrchestrateCmd.Flags().String("session-title", "", "Parent session title")
	worktreesCleanCmd.Flags().Bool("merged", false, "Remove only worktrees whose branches are already merged into main")
	worktreesCmd.AddCommand(worktreesOrchestrateCmd)
	worktreesCmd.AddCommand(worktreesCleanCmd)
}

func loadWorktreeSpec(path string) (agent.OrchestrateWorktreesParams, error) {
	var params agent.OrchestrateWorktreesParams
	path = filepath.Clean(path)
	content, err := os.ReadFile(path)
	if err != nil {
		return params, fmt.Errorf("read spec: %w", err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &params); err != nil {
			return params, fmt.Errorf("parse yaml spec: %w", err)
		}
	default:
		if err := json.Unmarshal(content, &params); err != nil {
			return params, fmt.Errorf("parse json spec: %w", err)
		}
	}
	return params, nil
}

func renderWorktreeGrid(w io.Writer, result agent.OrchestrateWorktreesResult) {
	type row struct {
		role         string
		title        string
		agentID      string
		submissionID string
		branch       string
		worktree     string
	}

	rows := make([]row, 0, len(result.Tasks)+len(result.TestRunners)+1)
	for _, task := range result.Tasks {
		rows = append(rows, row{
			role:         "task",
			title:        task.Title,
			agentID:      task.AgentID,
			submissionID: task.SubmissionID,
			branch:       task.Branch,
			worktree:     task.WorktreePath,
		})
	}
	for _, runner := range result.TestRunners {
		rows = append(rows, row{
			role:         "test",
			title:        runner.Title,
			agentID:      runner.AgentID,
			submissionID: runner.SubmissionID,
			branch:       runner.Branch,
			worktree:     runner.WorktreePath,
		})
	}
	if result.IntegrationAgent != nil {
		rows = append(rows, row{
			role:         "integration",
			title:        result.IntegrationAgent.Title,
			agentID:      result.IntegrationAgent.AgentID,
			submissionID: result.IntegrationAgent.SubmissionID,
			branch:       result.IntegrationAgent.Branch,
			worktree:     result.IntegrationAgent.WorktreePath,
		})
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ROLE\tTITLE\tAGENT_ID\tSUBMISSION_ID\tBRANCH\tWORKTREE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.role, r.title, r.agentID, r.submissionID, r.branch, r.worktree)
	}
	_ = tw.Flush()
}

func cleanMergedWorktrees(ctx context.Context, root string) ([]string, error) {
	baseRef, err := worktreeCleanupBaseRef(ctx, root)
	if err != nil {
		return nil, err
	}
	worktreeRoot := filepath.Join(root, ".sapphire", "worktrees", "agent")
	removed := make([]string, 0)
	err = filepath.WalkDir(worktreeRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() || path == worktreeRoot {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			return nil
		}

		branch, err := gitOutput(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return nil
		}
		merged, err := isBranchMergedInto(ctx, root, strings.TrimSpace(branch), baseRef)
		if err != nil || !merged {
			return nil
		}
		if err := gitRun(ctx, root, "worktree", "remove", path); err != nil {
			return err
		}
		removed = append(removed, fmt.Sprintf("%s\t%s", branch, path))
		return filepath.SkipDir
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	slices.Sort(removed)
	return removed, nil
}

func worktreeCleanupBaseRef(ctx context.Context, root string) (string, error) {
	if ok, err := gitBranchExists(ctx, root, "main"); err == nil && ok {
		return "main", nil
	}
	if ok, err := gitBranchExists(ctx, root, "master"); err == nil && ok {
		return "master", nil
	}
	return "", fmt.Errorf("no cleanup base branch found; expected main or master")
}

func isBranchMergedInto(ctx context.Context, root, branch, baseRef string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "merge-base", "--is-ancestor", branch, baseRef)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("merge-base check failed: %s", strings.TrimSpace(string(out)))
}

func gitBranchExists(ctx context.Context, root, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("branch lookup failed: %s", strings.TrimSpace(string(out)))
}

func gitRun(ctx context.Context, root string, args ...string) error {
	_, err := gitOutput(ctx, root, args...)
	return err
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
