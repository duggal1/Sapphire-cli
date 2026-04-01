package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSingularityPoliciesCommandJSON(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dataDir := filepath.Join(repoRoot, ".sapphire")
	policyDir := filepath.Join(dataDir, "singularity")
	require.NoError(t, os.MkdirAll(policyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "route_policies.json"), []byte(`{
  "version": 1,
  "policies": {
    "initialize/broad/backend": {
      "task_family": "initialize/broad/backend",
      "goal_type": "initialize",
      "breadth": "broad",
      "domains": ["backend"],
      "evidence_count": 3,
      "success_count": 2,
      "failure_count": 0,
      "require_harness": true,
      "prefer_parallel": true,
      "prefer_index_codebase": true,
      "forbid_bash_discovery": false,
      "confidence": 82,
      "promotion_state": "promoted"
    }
  }
}`), 0o644))

	cmd := newSingularityCmd()
	cmd.PersistentFlags().String("cwd", "", "")
	cmd.PersistentFlags().String("data-dir", "", "")
	cmd.PersistentFlags().Bool("debug", false, "")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"policies", "--json", "--cwd", repoRoot, "--data-dir", dataDir})

	require.NoError(t, cmd.Execute())

	var payload struct {
		Policies []map[string]any `json:"policies"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Len(t, payload.Policies, 1)
	require.Equal(t, "initialize/broad/backend", payload.Policies[0]["task_family"])
	require.Equal(t, "promoted", payload.Policies[0]["promotion_state"])
}

func TestSingularityAuditCommandJSON(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dataDir := filepath.Join(repoRoot, ".sapphire")
	policyDir := filepath.Join(dataDir, "singularity")
	require.NoError(t, os.MkdirAll(policyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "turn_audit.jsonl"), []byte(`{"timestamp":"2026-04-01T10:00:00Z","session_id":"s1","task_family":"initialize/broad/codebase","goal_type":"initialize","breadth":"broad","status":"completed","active_policy_id":"initialize/broad/codebase","applied_policy":true,"policy_state":"candidate","policy_confidence":87,"tool_error_codes":{"learned_route_policy":1},"blocked_bash_discovery":1}`+"\n"), 0o644))

	cmd := newSingularityCmd()
	cmd.PersistentFlags().String("cwd", "", "")
	cmd.PersistentFlags().String("data-dir", "", "")
	cmd.PersistentFlags().Bool("debug", false, "")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"audit", "--json", "--cwd", repoRoot, "--data-dir", dataDir})

	require.NoError(t, cmd.Execute())

	var payload struct {
		Records []map[string]any `json:"records"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Len(t, payload.Records, 1)
	require.Equal(t, "initialize/broad/codebase", payload.Records[0]["task_family"])
	require.Equal(t, true, payload.Records[0]["applied_policy"])
}
