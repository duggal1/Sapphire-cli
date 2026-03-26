package planmode

import "testing"

func TestExtractStructuredBlockForPlanModeUsesProposedPlanTag(t *testing.T) {
	t.Parallel()

	content := "Intro\n\n<proposed_plan>\n# Title\n\n- Step one\n- Step two\n</proposed_plan>\n"
	block, ok := ExtractStructuredBlockForMode(PlanMode, content)
	if !ok {
		t.Fatalf("expected proposed plan block")
	}
	if !block.IsValid {
		t.Fatalf("expected valid block, got %q", block.ValidationError)
	}
	if block.Mode != PlanMode {
		t.Fatalf("expected plan mode, got %s", block.Mode)
	}
	if block.Title != "Plan" {
		t.Fatalf("expected plan title, got %q", block.Title)
	}
}

func TestExtractStructuredBlockSupportsOtherModeTags(t *testing.T) {
	t.Parallel()

	content := "<security_report>\n## Findings\n- Example\n</security_report>"
	block, ok := ExtractStructuredBlock(content)
	if !ok {
		t.Fatalf("expected structured block")
	}
	if block.Mode != SecurityMode {
		t.Fatalf("expected security mode, got %s", block.Mode)
	}
	if block.Title != "Security Report" {
		t.Fatalf("expected security title, got %q", block.Title)
	}
}

func TestExtractStructuredBlockForPlanModeAcceptsUnterminatedPlanBlockAtEnd(t *testing.T) {
	t.Parallel()

	content := "Intro\n<proposed_plan>\n# Title\n\n- Step one\n- Step two\n"
	block, ok := ExtractStructuredBlockForMode(PlanMode, content)
	if !ok {
		t.Fatalf("expected proposed plan block")
	}
	if !block.IsValid {
		t.Fatalf("expected valid block, got %q", block.ValidationError)
	}
	if block.Content != "# Title\n\n- Step one\n- Step two" {
		t.Fatalf("unexpected content: %q", block.Content)
	}
}

func TestStructuredBlockTagsExposeExpectedPlanTags(t *testing.T) {
	t.Parallel()

	open, close, ok := StructuredBlockTags(PlanMode)
	if !ok {
		t.Fatal("expected plan mode tags")
	}
	if open != "<proposed_plan>" || close != "</proposed_plan>" {
		t.Fatalf("unexpected tags: %q %q", open, close)
	}
}

func TestRemoveStructuredBlocksStripsOrphanPlanTags(t *testing.T) {
	t.Parallel()

	content := "</proposed_plan>\nVisible"
	if got := RemoveStructuredBlocks(content); got != "Visible" {
		t.Fatalf("expected orphan tag stripped, got %q", got)
	}
}

func TestAvailableModesIncludesExtendedModeCatalog(t *testing.T) {
	t.Parallel()

	modes := AvailableModes()
	expected := map[SessionMode]bool{
		DefaultSessionMode: true,
		PlanMode:           true,
		ArchitectureMode:   true,
		DebugMode:          true,
		SecurityMode:       true,
		ReviewMode:         true,
		OrchestratorMode:   true,
	}
	for _, mode := range modes {
		delete(expected, mode.Mode)
	}
	if len(expected) != 0 {
		t.Fatalf("missing modes from available catalog: %v", expected)
	}
}
