package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/spf13/cobra"
)

func newSingularityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "singularity",
		Short: "Inspect and manage learned singularity policies",
		Long:  "Inspect, diff, and reset Sapphire's learned route policies without touching the protected tool-calling kernel.",
	}

	policiesCmd := &cobra.Command{
		Use:   "policies",
		Short: "List learned route policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveSingularityConfig(cmd)
			if err != nil {
				return err
			}
			info, err := agent.ListSingularityPolicies(cfg)
			if err != nil {
				return err
			}
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				return writeSingularityJSON(cmd, info)
			}
			return printSingularityPolicyList(cmd, info)
		},
	}
	policiesCmd.Flags().Bool("json", false, "Output as JSON")

	showCmd := &cobra.Command{
		Use:   "show <task-family>",
		Short: "Show a learned route policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveSingularityConfig(cmd)
			if err != nil {
				return err
			}
			info, err := agent.GetSingularityPolicy(cfg, args[0])
			if err != nil {
				return err
			}
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				return writeSingularityJSON(cmd, info)
			}
			return printSingularityPolicy(cmd, info)
		},
	}
	showCmd.Flags().Bool("json", false, "Output as JSON")

	diffCmd := &cobra.Command{
		Use:   "diff [task-family]",
		Short: "Diff current learned policies against the previous snapshot",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveSingularityConfig(cmd)
			if err != nil {
				return err
			}
			selector := ""
			if len(args) == 1 {
				selector = args[0]
			}
			diff, err := agent.DiffSingularityPolicies(cfg, selector)
			if err != nil {
				return err
			}
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				return writeSingularityJSON(cmd, diff)
			}
			return printSingularityDiff(cmd, diff)
		},
	}
	diffCmd.Flags().Bool("json", false, "Output as JSON")

	auditCmd := &cobra.Command{
		Use:   "audit [task-family]",
		Short: "Show recent singularity turn audit records",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveSingularityConfig(cmd)
			if err != nil {
				return err
			}
			selector := ""
			if len(args) == 1 {
				selector = args[0]
			}
			limit, _ := cmd.Flags().GetInt("last")
			info, err := agent.ListSingularityAudit(cfg, selector, limit)
			if err != nil {
				return err
			}
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				return writeSingularityJSON(cmd, info)
			}
			return printSingularityAudit(cmd, info)
		},
	}
	auditCmd.Flags().Int("last", 10, "Maximum number of recent audit records to show")
	auditCmd.Flags().Bool("json", false, "Output as JSON")

	resetCmd := &cobra.Command{
		Use:   "reset [task-family]",
		Short: "Reset one learned policy or all learned policies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resetAll, _ := cmd.Flags().GetBool("all")
			if !resetAll && len(args) == 0 {
				return fmt.Errorf("provide a task family or use --all")
			}
			cfg, err := resolveSingularityConfig(cmd)
			if err != nil {
				return err
			}
			selector := ""
			if len(args) == 1 {
				selector = args[0]
			}
			result, err := agent.ResetSingularityPolicies(cfg, selector, resetAll)
			if err != nil {
				return err
			}
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				return writeSingularityJSON(cmd, result)
			}
			return printSingularityReset(cmd, result)
		},
	}
	resetCmd.Flags().Bool("all", false, "Reset every learned policy")
	resetCmd.Flags().Bool("json", false, "Output as JSON")

	cmd.AddCommand(policiesCmd, showCmd, diffCmd, auditCmd, resetCmd)
	return cmd
}

func init() {
	rootCmd.AddCommand(newSingularityCmd())
}

func resolveSingularityConfig(cmd *cobra.Command) (*config.Config, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}
	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")
	return config.Init(cwd, dataDir, debug)
}

func writeSingularityJSON(cmd *cobra.Command, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func printSingularityPolicyList(cmd *cobra.Command, info agent.SingularityPolicyStoreInfo) error {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Policy file: %s\n", info.PolicyPath)
	_, _ = fmt.Fprintf(out, "History dir: %s (%d snapshots)\n", info.HistoryDir, info.SnapshotCount)
	_, _ = fmt.Fprintf(out, "Audit file: %s\n", info.AuditPath)
	if len(info.Policies) == 0 {
		_, _ = fmt.Fprintln(out, "No learned policies yet.")
		return nil
	}
	for _, policy := range info.Policies {
		_, _ = fmt.Fprintf(out, "\n%s\n", policy.TaskFamily)
		_, _ = fmt.Fprintf(out, "  state=%s confidence=%d evidence=%d applied=%d\n", policy.PromotionState, policy.Confidence, policy.EvidenceCount, policy.AppliedCount)
		_, _ = fmt.Fprintf(out, "  harness=%t parallel=%t index=%t bash_discovery_block=%t explicit_plan=%t post_write_verify=%t\n", policy.RequireHarness, policy.PreferParallel, policy.PreferIndexCodebase, policy.ForbidBashDiscovery, policy.RequireExplicitPlan, policy.RequirePostWriteVerification)
		if len(policy.PreferredDiscovery) > 0 {
			_, _ = fmt.Fprintf(out, "  discovery=%s\n", strings.Join(policy.PreferredDiscovery, " -> "))
		}
	}
	return nil
}

func printSingularityAudit(cmd *cobra.Command, info agent.SingularityAuditInfo) error {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Audit file: %s\n", info.AuditPath)
	if len(info.Records) == 0 {
		_, _ = fmt.Fprintln(out, "No singularity audit records yet.")
		return nil
	}
	for _, record := range info.Records {
		_, _ = fmt.Fprintf(out, "\n%s %s [%s]\n", record.Timestamp, record.TaskFamily, record.Status)
		_, _ = fmt.Fprintf(out, "  policy=%s applied=%t confidence=%d blocked_bash=%d\n", firstNonEmpty(record.ActivePolicyID, "-"), record.AppliedPolicy, record.PolicyConfidence, record.BlockedBashDiscovery)
		if len(record.ToolErrorCodes) > 0 {
			_, _ = fmt.Fprintf(out, "  tool_errors=%s\n", formatStringIntMap(record.ToolErrorCodes))
		}
		if len(record.OrderedTools) > 0 {
			_, _ = fmt.Fprintf(out, "  tools=%s\n", strings.Join(record.OrderedTools, " -> "))
		}
	}
	return nil
}

func printSingularityPolicy(cmd *cobra.Command, info agent.SingularityPolicyInfo) error {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Task family: %s\n", info.TaskFamily)
	_, _ = fmt.Fprintf(out, "State: %s\n", info.PromotionState)
	_, _ = fmt.Fprintf(out, "Confidence: %d\n", info.Confidence)
	_, _ = fmt.Fprintf(out, "Evidence: %d\n", info.EvidenceCount)
	_, _ = fmt.Fprintf(out, "Success/Failure: %d/%d\n", info.SuccessCount, info.FailureCount)
	_, _ = fmt.Fprintf(out, "Applied: %d\n", info.AppliedCount)
	_, _ = fmt.Fprintf(out, "Goal: %s\n", info.GoalType)
	_, _ = fmt.Fprintf(out, "Breadth: %s\n", info.Breadth)
	if len(info.Domains) > 0 {
		_, _ = fmt.Fprintf(out, "Domains: %s\n", strings.Join(info.Domains, ", "))
	}
	_, _ = fmt.Fprintf(out, "Require harness: %t\n", info.RequireHarness)
	_, _ = fmt.Fprintf(out, "Prefer parallel: %t\n", info.PreferParallel)
	_, _ = fmt.Fprintf(out, "Prefer index_codebase: %t\n", info.PreferIndexCodebase)
	_, _ = fmt.Fprintf(out, "Forbid discovery bash: %t\n", info.ForbidBashDiscovery)
	_, _ = fmt.Fprintf(out, "Require explicit plan: %t\n", info.RequireExplicitPlan)
	_, _ = fmt.Fprintf(out, "Require post-write verification: %t\n", info.RequirePostWriteVerification)
	if len(info.PreferredDiscovery) > 0 {
		_, _ = fmt.Fprintf(out, "Preferred discovery: %s\n", strings.Join(info.PreferredDiscovery, " -> "))
	}
	if len(info.PreferredSkills) > 0 {
		_, _ = fmt.Fprintf(out, "Preferred skills: %s\n", strings.Join(info.PreferredSkills, ", "))
	}
	if strings.TrimSpace(info.SkillFilePath) != "" {
		_, _ = fmt.Fprintf(out, "Skill file: %s\n", info.SkillFilePath)
	}
	if strings.TrimSpace(info.LastAppliedAt) != "" {
		_, _ = fmt.Fprintf(out, "Last applied: %s\n", info.LastAppliedAt)
	}
	if strings.TrimSpace(info.UpdatedAt) != "" {
		_, _ = fmt.Fprintf(out, "Updated: %s\n", info.UpdatedAt)
	}
	return nil
}

func printSingularityDiff(cmd *cobra.Command, diff agent.SingularityPolicyDiff) error {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Current: %s\n", diff.CurrentPath)
	if strings.TrimSpace(diff.PreviousPath) == "" {
		_, _ = fmt.Fprintf(out, "Previous: none (%d snapshots available)\n", diff.SnapshotCount)
		return nil
	}
	_, _ = fmt.Fprintf(out, "Previous: %s\n", diff.PreviousPath)
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		_, _ = fmt.Fprintln(out, "No policy changes.")
		return nil
	}
	if len(diff.Added) > 0 {
		_, _ = fmt.Fprintln(out, "Added:")
		for _, policy := range diff.Added {
			_, _ = fmt.Fprintf(out, "  %s\n", policy.TaskFamily)
		}
	}
	if len(diff.Removed) > 0 {
		_, _ = fmt.Fprintln(out, "Removed:")
		for _, policy := range diff.Removed {
			_, _ = fmt.Fprintf(out, "  %s\n", policy.TaskFamily)
		}
	}
	if len(diff.Changed) > 0 {
		_, _ = fmt.Fprintln(out, "Changed:")
		for _, change := range diff.Changed {
			_, _ = fmt.Fprintf(out, "  %s :: %s: %s -> %s\n", change.TaskFamily, change.Field, change.Before, change.After)
		}
	}
	return nil
}

func printSingularityReset(cmd *cobra.Command, result agent.SingularityResetResult) error {
	out := cmd.OutOrStdout()
	if len(result.RemovedPolicies) == 0 {
		_, _ = fmt.Fprintln(out, "No learned policies were removed.")
		return nil
	}
	_, _ = fmt.Fprintf(out, "Policy file: %s\n", result.PolicyPath)
	for _, taskFamily := range result.RemovedPolicies {
		_, _ = fmt.Fprintf(out, "Removed policy: %s\n", taskFamily)
	}
	for _, skillPath := range result.RemovedSkills {
		_, _ = fmt.Fprintf(out, "Removed skill: %s\n", skillPath)
	}
	return nil
}

func formatStringIntMap(values map[string]int) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
