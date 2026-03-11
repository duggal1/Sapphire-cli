package config

import (
	"fmt"
	"strings"
)

// SaveMCPConfigs persists the current MCP configuration map to the data config.
func (c *Config) SaveMCPConfigs() error {
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	if err := c.SetConfigField("mcp", c.MCP); err != nil {
		return fmt.Errorf("failed to persist mcp configuration: %w", err)
	}
	return nil
}

// UpsertMCPConfig adds or updates an MCP server configuration and persists it.
func (c *Config) UpsertMCPConfig(name string, m MCPConfig) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("mcp name is required")
	}
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	c.MCP[name] = m
	return c.SaveMCPConfigs()
}

// DeleteMCPConfig removes an MCP server configuration and persists the change.
func (c *Config) DeleteMCPConfig(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("mcp name is required")
	}
	if c.MCP != nil {
		delete(c.MCP, name)
	}
	return c.SaveMCPConfigs()
}
