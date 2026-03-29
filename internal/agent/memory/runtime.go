package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appdb "github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/google/uuid"
)

const (
	maxRetainedHandoffs     = 24
	maxRetainedBootPackets  = 48
	maxRetainedResumePoints = 24
	maxRetainedIndexEpochs  = 16
	maxRetainedSubAgentRows = 128
	maxRetainedFindings     = 256
)

type ResumeRequest struct {
	SessionID           string
	AgentID             string
	WorkingDir          string
	Task                string
	OriginalPrompt      string
	ContinuationPrompt  string
	Reason              string
	ProjectConstitution string
	LongHorizonContext  string
	HistoricalContext   string
}

type SubAgentOutcomeInput struct {
	SessionID       string
	ParentSessionID string
	AgentID         string
	AssignmentID    string
	SubmissionID    string
	WorkingDir      string
	Status          string
	Summary         string
	Progress        string
	Risks           string
	Blockers        string
	NextAction      string
	Files           []string
	Commands        []string
	RawResult       string
}

func (c *Compiler) persistHandoffPacket(ctx context.Context, req CompileRequest, packet BootPacket) (string, string, error) {
	payload, err := json.Marshal(packet.RepoSnapshot)
	if err != nil {
		return "", "", err
	}
	artifactPath, err := c.writeHandoffArtifact(packet.RepoSnapshot.RepoRoot, packet)
	if err != nil {
		return "", "", err
	}
	checkpointID := ""
	if latest, err := c.latestCheckpoint(ctx, req.SessionID, req.AgentID); err == nil {
		checkpointID = latest.ID
	}
	handoffID := uuid.NewString()
	scopeID := c.lookupScopeID(ctx, packet.RepoSnapshot.RepoRoot, packet.RepoSnapshot.ScopePath, packet.RepoSnapshot.Branch)
	err = c.q.InsertMemoryHandoff(ctx, appdb.InsertMemoryHandoffParams{
		ID:             handoffID,
		SessionID:      strings.TrimSpace(req.SessionID),
		AgentID:        strings.TrimSpace(req.AgentID),
		RepoScopeID:    scopeID,
		CheckpointID:   checkpointID,
		Status:         fmt.Sprint(firstNonEmpty(packet.RuntimeState.ValidationStatus["status"], "active")),
		Objective:      firstNonEmptyString(packet.RuntimeState.CurrentTask, packet.Handoff.WhatRemains),
		Plan:           packet.RuntimeState.CurrentPlan,
		Blockers:       packet.RuntimeState.Blockers,
		Uncertainties:  packet.RuntimeState.Uncertainties,
		TouchedFiles:   packet.RuntimeState.TouchedFiles,
		TouchedSymbols: packet.RuntimeState.TouchedSymbols,
		SubAgents:      flattenSubAgentStatuses(packet.RuntimeState.ActiveSubAgents),
		Validation:     packet.RuntimeState.ValidationStatus,
		RepoSnapshot:   parseJSONMap(string(payload)),
		NextActions:    packet.Handoff.NextActions,
		ArtifactPath:   artifactPath,
		CreatedAt:      c.now().Unix(),
	})
	if err != nil {
		return "", "", err
	}
	provenanceID, err := c.createProvenance(ctx, appdb.InsertMemoryProvenanceParams{
		ID:           uuid.NewString(),
		RepoScopeID:  scopeID,
		SessionID:    strings.TrimSpace(req.SessionID),
		AgentID:      strings.TrimSpace(req.AgentID),
		SourceKind:   "handoff",
		ArtifactPath: artifactPath,
		HandoffID:    handoffID,
		HeadCommit:   packet.RepoSnapshot.HeadCommit,
		IndexEpoch:   packet.RepoSnapshot.IndexEpoch,
		Metadata: map[string]any{
			"task_class":   packet.TaskClass,
			"working_dir":  req.WorkingDir,
			"generated_at": packet.GeneratedAt,
		},
		CreatedAt: c.now().Unix(),
	})
	if err == nil {
		_ = c.linkFactProvenance(ctx, "handoff", handoffID, provenanceID)
	}
	c.scheduleDurableMemoryPrune(strings.TrimSpace(req.SessionID), scopeID)
	return handoffID, artifactPath, nil
}

func (c *Compiler) CreateResumePoint(ctx context.Context, req ResumeRequest) (appdb.MemoryResumePoint, error) {
	packet, err := c.Compile(ctx, CompileRequest{
		SessionID:           req.SessionID,
		AgentID:             req.AgentID,
		WorkingDir:          req.WorkingDir,
		Task:                req.Task,
		IncludeMistakesRead: true,
		ProjectConstitution: req.ProjectConstitution,
		LongHorizonContext:  req.LongHorizonContext,
		HistoricalContext:   req.HistoricalContext,
	})
	if err != nil {
		return appdb.MemoryResumePoint{}, err
	}
	handoffID, handoffArtifactPath, err := c.persistHandoffPacket(ctx, CompileRequest{
		SessionID:           req.SessionID,
		AgentID:             req.AgentID,
		WorkingDir:          req.WorkingDir,
		Task:                req.Task,
		ProjectConstitution: req.ProjectConstitution,
		LongHorizonContext:  req.LongHorizonContext,
		HistoricalContext:   req.HistoricalContext,
	}, packet)
	if err != nil {
		return appdb.MemoryResumePoint{}, err
	}
	scopeID := c.lookupScopeID(ctx, packet.RepoSnapshot.RepoRoot, packet.RepoSnapshot.ScopePath, packet.RepoSnapshot.Branch)
	item := appdb.MemoryResumePoint{
		ID:                     uuid.NewString(),
		SessionID:              strings.TrimSpace(req.SessionID),
		AgentID:                strings.TrimSpace(req.AgentID),
		RepoScopeID:            scopeID,
		HandoffID:              handoffID,
		BootPacketArtifactPath: packet.ArtifactPath,
		HandoffArtifactPath:    handoffArtifactPath,
		ContinuationPrompt:     strings.TrimSpace(req.ContinuationPrompt),
		OriginalPrompt:         strings.TrimSpace(req.OriginalPrompt),
		ResumeReason:           firstNonEmptyText(req.Reason, "context_rollover"),
		Status:                 "pending",
		CreatedAt:              c.now().Unix(),
		ResumedAt:              0,
	}
	if err := c.q.InsertMemoryResumePoint(ctx, appdb.InsertMemoryResumePointParams{
		ID:                     item.ID,
		SessionID:              item.SessionID,
		AgentID:                item.AgentID,
		RepoScopeID:            item.RepoScopeID,
		HandoffID:              item.HandoffID,
		BootPacketArtifactPath: item.BootPacketArtifactPath,
		HandoffArtifactPath:    item.HandoffArtifactPath,
		ContinuationPrompt:     item.ContinuationPrompt,
		OriginalPrompt:         item.OriginalPrompt,
		ResumeReason:           item.ResumeReason,
		Status:                 item.Status,
		CreatedAt:              item.CreatedAt,
		ResumedAt:              item.ResumedAt,
	}); err != nil {
		return appdb.MemoryResumePoint{}, err
	}
	provenanceID, err := c.createProvenance(ctx, appdb.InsertMemoryProvenanceParams{
		ID:           uuid.NewString(),
		RepoScopeID:  scopeID,
		SessionID:    item.SessionID,
		AgentID:      item.AgentID,
		SourceKind:   "resume_point",
		ArtifactPath: packet.ArtifactPath,
		HandoffID:    handoffID,
		HeadCommit:   packet.RepoSnapshot.HeadCommit,
		IndexEpoch:   packet.RepoSnapshot.IndexEpoch,
		Metadata: map[string]any{
			"resume_reason":       item.ResumeReason,
			"continuation_prompt": item.ContinuationPrompt,
		},
		CreatedAt: item.CreatedAt,
	})
	if err == nil {
		_ = c.linkFactProvenance(ctx, "resume_point", item.ID, provenanceID)
	}
	c.scheduleDurableMemoryPrune(item.SessionID, scopeID)
	return item, nil
}

func (c *Compiler) MatchPendingResumePoint(ctx context.Context, sessionID, prompt string) (appdb.MemoryResumePoint, bool) {
	if c == nil || c.q == nil || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(prompt) == "" {
		return appdb.MemoryResumePoint{}, false
	}
	item, err := c.q.GetLatestPendingMemoryResumePointBySession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return appdb.MemoryResumePoint{}, false
	}
	normalizedPrompt := normalizePromptForResume(prompt)
	if normalizedPrompt == normalizePromptForResume(item.OriginalPrompt) || normalizedPrompt == normalizePromptForResume(item.ContinuationPrompt) {
		return item, true
	}
	return appdb.MemoryResumePoint{}, false
}

func (c *Compiler) RenderResumePointInjection(ctx context.Context, resumePointID string) string {
	if c == nil || c.q == nil || strings.TrimSpace(resumePointID) == "" {
		return ""
	}
	item, err := c.q.GetMemoryResumePoint(ctx, strings.TrimSpace(resumePointID))
	if err != nil {
		return ""
	}
	payload, err := os.ReadFile(item.BootPacketArtifactPath)
	if err != nil {
		return ""
	}
	_ = c.q.MarkMemoryResumePointResumed(ctx, item.ID, c.now().Unix())
	return "## DURABLE RESUME BOOT PACKET\n" +
		"Resume reason: " + item.ResumeReason + "\n" +
		"Continue the interrupted task from this durable state.\n" +
		"```json\n" + string(payload) + "\n```"
}

func (c *Compiler) PersistSubAgentOutcome(ctx context.Context, input SubAgentOutcomeInput) error {
	if c == nil || c.q == nil || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.AgentID) == "" {
		return nil
	}
	scope, err := c.ensureIndexedScope(ctx, input.WorkingDir)
	if err != nil {
		return err
	}
	files := normalizeReportFiles(scope.RepoRoot, input.Files)
	touchedSymbols := uniqueSortedStrings(explicitSymbolMentions(strings.Join([]string{input.RawResult, input.Summary, input.Progress}, "\n")))
	reportID := uuid.NewString()
	artifactPath, err := c.writeArtifact(scope.RepoRoot, "subagent_reports", []byte(input.RawResult))
	if err != nil {
		return err
	}
	nowUnix := c.now().Unix()
	if err := c.q.InsertMemorySubAgentReport(ctx, appdb.InsertMemorySubAgentReportParams{
		ID:              reportID,
		SessionID:       strings.TrimSpace(input.SessionID),
		ParentSessionID: strings.TrimSpace(input.ParentSessionID),
		AgentID:         strings.TrimSpace(input.AgentID),
		AssignmentID:    strings.TrimSpace(input.AssignmentID),
		SubmissionID:    strings.TrimSpace(input.SubmissionID),
		RepoScopeID:     scope.ID,
		Status:          strings.TrimSpace(input.Status),
		Summary:         strings.TrimSpace(input.Summary),
		Progress:        strings.TrimSpace(input.Progress),
		Risks:           strings.TrimSpace(input.Risks),
		Blockers:        strings.TrimSpace(input.Blockers),
		NextAction:      strings.TrimSpace(input.NextAction),
		Files:           files,
		Commands:        compactStrings(input.Commands),
		TouchedSymbols:  touchedSymbols,
		RawResult:       strings.TrimSpace(input.RawResult),
		ArtifactPath:    artifactPath,
		CreatedAt:       nowUnix,
		UpdatedAt:       nowUnix,
	}); err != nil {
		return err
	}
	provenanceID, err := c.createProvenance(ctx, appdb.InsertMemoryProvenanceParams{
		ID:               uuid.NewString(),
		RepoScopeID:      scope.ID,
		SessionID:        strings.TrimSpace(input.SessionID),
		AgentID:          strings.TrimSpace(input.AgentID),
		SourceKind:       "subagent_report",
		ArtifactPath:     artifactPath,
		SubAgentReportID: reportID,
		FilePath:         firstString(files),
		SymbolKey:        firstString(touchedSymbols),
		HeadCommit:       scope.HeadCommit,
		IndexEpoch:       scope.LatestEpoch,
		Metadata: map[string]any{
			"assignment_id": input.AssignmentID,
			"submission_id": input.SubmissionID,
			"status":        input.Status,
		},
		CreatedAt: nowUnix,
	})
	if err == nil {
		_ = c.linkFactProvenance(ctx, "subagent_report", reportID, provenanceID)
	}

	createFinding := func(kind, title, content, filePath, symbolKey string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		findingID := uuid.NewString()
		if err := c.q.InsertMemoryFinding(ctx, appdb.InsertMemoryFindingParams{
			ID:             findingID,
			SessionID:      strings.TrimSpace(input.SessionID),
			AgentID:        strings.TrimSpace(input.AgentID),
			RepoScopeID:    scope.ID,
			Kind:           kind,
			Title:          title,
			Content:        compactText(content, 400),
			FilePath:       filePath,
			SymbolKey:      symbolKey,
			Status:         "active",
			SourceReportID: reportID,
			CreatedAt:      nowUnix,
			UpdatedAt:      nowUnix,
		}); err == nil && provenanceID != "" {
			_ = c.linkFactProvenance(ctx, "finding", findingID, provenanceID)
		}
	}

	createFinding("finding", "Subagent finding", firstNonEmptyText(input.Summary, input.Progress), firstString(files), firstString(touchedSymbols))
	createFinding("uncertainty", "Subagent blocker", input.Blockers, firstString(files), firstString(touchedSymbols))
	createFinding("uncertainty", "Subagent risk", input.Risks, firstString(files), firstString(touchedSymbols))
	createFinding("next_action", "Recommended next action", input.NextAction, firstString(files), firstString(touchedSymbols))

	if c.store != nil {
		if decisions, err := c.store.ListDecisionRecords(ctx, strings.TrimSpace(input.SessionID), 12); err == nil {
			for _, item := range decisions {
				createFinding("decision", item.Category+"."+item.Key, item.Value, firstString(files), firstString(touchedSymbols))
			}
		}
	}
	c.scheduleDurableMemoryPrune(strings.TrimSpace(input.ParentSessionID), scope.ID)
	_ = c.pruneSubAgentReports(ctx, strings.TrimSpace(input.SessionID))
	return nil
}

func (c *Compiler) pruneDurableMemory(ctx context.Context, sessionID, scopeID string) error {
	if c == nil || c.q == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) != "" {
		if resumePoints, err := c.q.ListMemoryResumePointsBySession(ctx, sessionID); err == nil && len(resumePoints) > maxRetainedResumePoints {
			stale := resumePoints[maxRetainedResumePoints:]
			if err := c.q.DeleteMemoryResumePointsByIDs(ctx, extractResumeIDs(stale)); err != nil {
				return err
			}
			if err := deleteArtifactPaths(extractResumeArtifacts(stale)); err != nil {
				return err
			}
		}
		if handoffs, err := c.q.ListMemoryHandoffsBySession(ctx, sessionID); err == nil && len(handoffs) > maxRetainedHandoffs {
			stale := handoffs[maxRetainedHandoffs:]
			if err := c.q.DeleteMemoryHandoffsByIDs(ctx, extractHandoffIDs(stale)); err != nil {
				return err
			}
			if err := deleteArtifactPaths(extractHandoffArtifacts(stale)); err != nil {
				return err
			}
		}
		if packets, err := c.q.ListMemoryBootPacketsBySession(ctx, sessionID); err == nil && len(packets) > maxRetainedBootPackets {
			stale := packets[maxRetainedBootPackets:]
			if err := c.q.DeleteMemoryBootPacketsByIDs(ctx, extractBootPacketIDs(stale)); err != nil {
				return err
			}
			if err := deleteArtifactPaths(extractBootPacketArtifacts(stale)); err != nil {
				return err
			}
		}
		if findings, err := c.q.ListMemoryFindingsBySession(ctx, sessionID, maxRetainedFindings+64); err == nil && len(findings) > maxRetainedFindings {
			stale := findings[maxRetainedFindings:]
			if err := c.q.DeleteMemoryFindingsByIDs(ctx, extractFindingIDs(stale)); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(scopeID) != "" {
		if epochs, err := c.q.ListMemoryIndexEpochsByScope(ctx, scopeID); err == nil && len(epochs) > maxRetainedIndexEpochs {
			if err := c.q.DeleteMemoryIndexEpochsByIDs(ctx, extractEpochIDs(epochs[maxRetainedIndexEpochs:])); err != nil {
				return err
			}
		}
	}
	return c.q.DeleteOrphanMemoryProvenance(ctx)
}

func (c *Compiler) pruneSubAgentReports(ctx context.Context, sessionID string) error {
	if c == nil || c.q == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	reports, err := c.q.ListMemorySubAgentReportsBySession(ctx, sessionID, maxRetainedSubAgentRows+32)
	if err != nil || len(reports) <= maxRetainedSubAgentRows {
		return nil
	}
	stale := reports[maxRetainedSubAgentRows:]
	if err := c.q.DeleteMemorySubAgentReportsByIDs(ctx, extractSubAgentReportIDs(stale)); err != nil {
		return err
	}
	return deleteArtifactPaths(extractSubAgentArtifacts(stale))
}

func (c *Compiler) createProvenance(ctx context.Context, arg appdb.InsertMemoryProvenanceParams) (string, error) {
	if strings.TrimSpace(arg.ID) == "" {
		arg.ID = uuid.NewString()
	}
	if err := c.q.InsertMemoryProvenance(ctx, arg); err != nil {
		return "", err
	}
	return arg.ID, nil
}

func (c *Compiler) linkFactProvenance(ctx context.Context, factKind, factID, provenanceID string) error {
	if factKind == "" || factID == "" || provenanceID == "" {
		return nil
	}
	return c.q.LinkMemoryFactProvenance(ctx, appdb.LinkMemoryFactProvenanceParams{
		FactKind:     factKind,
		FactID:       factID,
		ProvenanceID: provenanceID,
		CreatedAt:    c.now().Unix(),
	})
}

func normalizePromptForResume(prompt string) string {
	parts := strings.Fields(strings.TrimSpace(prompt))
	return strings.ToLower(strings.Join(parts, " "))
}

func normalizeReportFiles(repoRoot string, items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if filepath.IsAbs(item) && strings.TrimSpace(repoRoot) != "" {
			if rel, err := filepath.Rel(repoRoot, item); err == nil && !strings.HasPrefix(rel, "..") {
				item = rel
			}
		}
		out = append(out, filepath.ToSlash(item))
	}
	return uniqueSortedStrings(out)
}

func flattenSubAgentStatuses(items []BootSubAgentRuntimeState) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AgentID) == "" {
			continue
		}
		out = append(out, item.AgentID+":"+item.Status)
	}
	return out
}

func compactStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, compactText(v, 240))
		}
	}
	return out
}

func firstString(items []string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func deleteArtifactPaths(paths []string) error {
	var firstErr error
	for _, path := range compactPathSlice(paths) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func extractHandoffIDs(items []appdb.MemoryHandoff) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractHandoffArtifacts(items []appdb.MemoryHandoff) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ArtifactPath)
	}
	return out
}

func extractBootPacketIDs(items []appdb.MemoryBootPacket) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractBootPacketArtifacts(items []appdb.MemoryBootPacket) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ArtifactPath)
	}
	return out
}

func extractResumeIDs(items []appdb.MemoryResumePoint) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractResumeArtifacts(items []appdb.MemoryResumePoint) []string {
	out := make([]string, 0, len(items)*2)
	for _, item := range items {
		out = append(out, item.BootPacketArtifactPath, item.HandoffArtifactPath)
	}
	return out
}

func extractSubAgentReportIDs(items []appdb.MemorySubAgentReport) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractSubAgentArtifacts(items []appdb.MemorySubAgentReport) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ArtifactPath)
	}
	return out
}

func extractEpochIDs(items []appdb.MemoryIndexEpoch) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractFindingIDs(items []appdb.MemoryFinding) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func compactPathSlice(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
