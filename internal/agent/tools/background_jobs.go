package tools

import (
	"context"
	"slices"

	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/duggal1/Sapphire-cli/internal/shell"
)

var (
	lastBackgroundShellBySession = csync.NewMap[string, string]()
	backgroundShellsBySession    = csync.NewMap[string, map[string]struct{}]()
)

func setLastBackgroundShellID(sessionID, shellID string) {
	if sessionID == "" || shellID == "" {
		return
	}
	lastBackgroundShellBySession.Set(sessionID, shellID)
	addBackgroundShellID(sessionID, shellID)
}

func getLastBackgroundShellID(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	if v, ok := lastBackgroundShellBySession.Get(sessionID); ok {
		return v
	}
	return ""
}

func addBackgroundShellID(sessionID, shellID string) {
	if sessionID == "" || shellID == "" {
		return
	}
	current := map[string]struct{}{}
	if existing, ok := backgroundShellsBySession.Get(sessionID); ok && existing != nil {
		for id := range existing {
			current[id] = struct{}{}
		}
	}
	current[shellID] = struct{}{}
	backgroundShellsBySession.Set(sessionID, current)
}

func removeBackgroundShellID(sessionID, shellID string) {
	if sessionID == "" || shellID == "" {
		return
	}
	existing, ok := backgroundShellsBySession.Get(sessionID)
	if !ok || existing == nil {
		return
	}
	if _, ok := existing[shellID]; !ok {
		return
	}
	next := map[string]struct{}{}
	for id := range existing {
		if id == shellID {
			continue
		}
		next[id] = struct{}{}
	}
	if len(next) == 0 {
		backgroundShellsBySession.Del(sessionID)
		return
	}
	backgroundShellsBySession.Set(sessionID, next)
}

func listBackgroundShellIDs(sessionID string) []string {
	if sessionID == "" {
		return nil
	}
	existing, ok := backgroundShellsBySession.Get(sessionID)
	if !ok || existing == nil {
		return nil
	}
	ids := make([]string, 0, len(existing))
	for id := range existing {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func WaitForBackgroundShells(ctx context.Context, sessionID string) {
	for {
		ids := listBackgroundShellIDs(sessionID)
		if len(ids) == 0 {
			return
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			if fastShell, ok := shell.GetFastBackgroundShellManager().Get(id); ok {
				if !fastShell.WaitContext(ctx) {
					return
				}
				_, _, done, _ := fastShell.GetOutput()
				if done {
					_ = shell.GetFastBackgroundShellManager().Remove(id)
					removeBackgroundShellID(sessionID, id)
				}
				continue
			}
			if bgShell, ok := shell.GetBackgroundShellManager().Get(id); ok {
				if !bgShell.WaitContext(ctx) {
					return
				}
				_, _, done, _ := bgShell.GetOutput()
				if done {
					_ = shell.GetBackgroundShellManager().Remove(id)
					removeBackgroundShellID(sessionID, id)
				}
				continue
			}
			removeBackgroundShellID(sessionID, id)
		}
	}
}

func PollBackgroundShells(sessionID string) {
	ids := listBackgroundShellIDs(sessionID)
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		if fastShell, ok := shell.GetFastBackgroundShellManager().Get(id); ok {
			_, _, done, _ := fastShell.GetOutput()
			if done {
				_ = shell.GetFastBackgroundShellManager().Remove(id)
				removeBackgroundShellID(sessionID, id)
			}
			continue
		}
		if bgShell, ok := shell.GetBackgroundShellManager().Get(id); ok {
			_, _, done, _ := bgShell.GetOutput()
			if done {
				_ = shell.GetBackgroundShellManager().Remove(id)
				removeBackgroundShellID(sessionID, id)
			}
			continue
		}
		removeBackgroundShellID(sessionID, id)
	}
}
