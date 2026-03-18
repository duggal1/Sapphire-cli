package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/sapphire/internal/agent"
	"github.com/charmbracelet/sapphire/internal/event"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var worktreesCmd = &cobra.Command{
	Use:   "worktrees",
	Short: "Worktree orchestration utilities",
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

func init() {
	worktreesOrchestrateCmd.Flags().StringP("spec", "s", "", "Path to JSON/YAML orchestration spec")
	worktreesOrchestrateCmd.Flags().String("resume", "", "Resume an orphaned worktree by path")
	worktreesOrchestrateCmd.Flags().String("resume-prompt", "", "Prompt for resumed worktree")
	worktreesOrchestrateCmd.Flags().String("agent", "", "Agent profile to use for resume")
	worktreesOrchestrateCmd.Flags().String("model", "", "Model override for resume")
	worktreesOrchestrateCmd.Flags().String("reasoning-effort", "", "Reasoning effort override for resume (low, medium, high)")
	worktreesOrchestrateCmd.Flags().String("session-title", "", "Parent session title")
	worktreesCmd.AddCommand(worktreesOrchestrateCmd)
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
