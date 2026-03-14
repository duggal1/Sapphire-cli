package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/pubsub"
)

// ApplyConfig starts or updates a single MCP client based on the current config.
func ApplyConfig(ctx context.Context, cfg *config.Config, name string) error {
	m, ok := cfg.MCP[name]
	if !ok {
		return fmt.Errorf("mcp '%s' not configured", name)
	}
	if m.Disabled {
		DisableClient(name)
		return nil
	}
	return startClient(ctx, cfg, name, m)
}

// DisableClient stops an MCP client and marks it as disabled.
func DisableClient(name string) {
	closeClient(name)
	clearClientCaches(name)
	updateState(name, StateDisabled, nil, nil, Counts{})
}

// RemoveClient stops an MCP client and removes its state and cached data.
func RemoveClient(name string) {
	closeClient(name)
	clearClientCaches(name)
	states.Del(name)
	broker.Publish(pubsub.UpdatedEvent, Event{
		Type:  EventStateChanged,
		Name:  name,
		State: StateDisabled,
	})
}

func startClient(ctx context.Context, cfg *config.Config, name string, m config.MCPConfig) error {
	closeClient(name)
	updateState(name, StateStarting, nil, nil, Counts{})

	session, err := createSession(ctx, name, m, cfg.Resolver())
	if err != nil {
		return err
	}

	tools, err := getTools(ctx, cfg, name, session)
	if err != nil {
		slog.Error("Error listing tools", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		session.Close()
		return err
	}

	prompts, err := getPrompts(ctx, cfg, name, session)
	if err != nil {
		slog.Error("Error listing prompts", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		session.Close()
		return err
	}

	resources, err := getResources(ctx, cfg, name, session)
	if err != nil {
		slog.Error("Error listing resources", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		session.Close()
		return err
	}

	toolCount := updateTools(cfg, name, tools)
	updatePrompts(name, prompts)
	resourceCount := updateResources(name, resources)
	sessions.Set(name, session)

	updateState(name, StateConnected, nil, session, Counts{
		Tools:     toolCount,
		Prompts:   len(prompts),
		Resources: resourceCount,
	})

	return nil
}

func closeClient(name string) {
	sess, ok := sessions.Take(name)
	if !ok {
		return
	}
	if err := sess.Close(); err != nil {
		slog.Debug("Failed to close MCP client", "name", name, "error", err)
	}
}

func clearClientCaches(name string) {
	allTools.Del(name)
	allPrompts.Del(name)
	allResources.Del(name)
}
