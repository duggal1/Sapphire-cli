package agent

import (
	"context"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/worktreepolicy"
	"github.com/stretchr/testify/require"
)

func TestResolveSpawnWorktreePolicyDefaultsToSharedRepo(t *testing.T) {
	t.Parallel()

	coord := &coordinator{}
	policy := coord.resolveSpawnWorktreePolicy(context.Background(), "session-1", spawnAgentOptions{})
	require.Equal(t, worktreepolicy.SharedRepo, policy)
}

func TestResolveSpawnWorktreePolicyHonorsExplicitWorktreeSetting(t *testing.T) {
	t.Parallel()

	coord := &coordinator{}

	policy := coord.resolveSpawnWorktreePolicy(context.Background(), "session-1", spawnAgentOptions{
		AllowWorktree: true,
		Worktree:      true,
		WorktreeSet:   true,
	})
	require.Equal(t, worktreepolicy.Isolated, policy)

	policy = coord.resolveSpawnWorktreePolicy(context.Background(), "session-1", spawnAgentOptions{
		AllowWorktree: true,
		Worktree:      false,
		WorktreeSet:   true,
	})
	require.Equal(t, worktreepolicy.SharedRepo, policy)
}

func TestResolveSpawnWorktreePolicyUsesIsolationForExplicitWorktreeArtifacts(t *testing.T) {
	t.Parallel()

	coord := &coordinator{}

	cases := []spawnAgentOptions{
		{AllowWorktree: true, ReuseWorktree: true},
		{AllowWorktree: true, WorktreePath: ".sapphire/worktrees/agent/a/task"},
		{AllowWorktree: true, Branch: "agent/a/task"},
	}

	for _, tc := range cases {
		policy := coord.resolveSpawnWorktreePolicy(context.Background(), "session-1", tc)
		require.Equal(t, worktreepolicy.Isolated, policy)
	}
}

func TestResolveSpawnWorktreePolicyDisablesLifecycleWorktreesByDefault(t *testing.T) {
	t.Parallel()

	coord := &coordinator{}
	policy := coord.resolveSpawnWorktreePolicy(context.Background(), "session-1", spawnAgentOptions{
		Worktree:     true,
		WorktreeSet:  true,
		WorktreePath: ".sapphire/worktrees/agent/a/task",
		Branch:       "agent/a/task",
	})
	require.Equal(t, worktreepolicy.SharedRepo, policy)
}
