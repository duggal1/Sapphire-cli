package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeCoreConstitutionPreservesDurableRules(t *testing.T) {
	t.Parallel()

	existing := appendConstitutionDecision("", "Always read the full file before editing.")
	merged := mergeCoreConstitution(existing, []ArchitecturalDecision{
		{Decision: "Use worktree isolation for parallel edits", Rationale: "Avoid cross-agent file clobbering"},
	})

	require.Contains(t, merged, constitutionDurableHeader)
	require.Contains(t, merged, "Always read the full file before editing.")
	require.Contains(t, merged, "Use worktree isolation for parallel edits")
}
