package tools

import (
	"context"

	"github.com/charmbracelet/sapphire/internal/config"
)

type (
	sessionIDContextKey string
	messageIDContextKey string
	supportsImagesKey   string
	modelNameKey        string
	agentModeKey        string
	workingDirKey       string
	runtimeControlKey   string
	writeScopeKey       string
)

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// AgentModeContextKey is the key for the agent mode in the context.
	AgentModeContextKey agentModeKey = "agent_mode"
	// WorkingDirContextKey is the key for the working directory in the context.
	WorkingDirContextKey workingDirKey = "working_dir"
	// RuntimeControlContextKey is the key for the runtime control loop in the context.
	RuntimeControlContextKey runtimeControlKey = "runtime_control"
	// WriteScopeContextKey is the key for sub-agent write scope constraints.
	WriteScopeContextKey writeScopeKey = "write_scope"

	// LoadSkillToolName is the name of the tool used to load skills.
	LoadSkillToolName = "load_skill"

	// ToolSuggestToolName is the name of the tool used to suggest MCP tools.
	ToolSuggestToolName = "tool_suggest"
)

type RuntimeControl interface {
	AllowToolCall(toolName string) error
	BeginToolExecution(toolName string)
	FinishToolExecution(toolName string)
}

// getContextValue is a generic helper that retrieves a typed value from context.
// If the value is not found or has the wrong type, it returns the default value.
func getContextValue[T any](ctx context.Context, key any, defaultValue T) T {
	value := ctx.Value(key)
	if value == nil {
		return defaultValue
	}
	if typedValue, ok := value.(T); ok {
		return typedValue
	}
	return defaultValue
}

// GetSessionFromContext retrieves the session ID from the context.
func GetSessionFromContext(ctx context.Context) string {
	return getContextValue(ctx, SessionIDContextKey, "")
}

// GetMessageFromContext retrieves the message ID from the context.
func GetMessageFromContext(ctx context.Context) string {
	return getContextValue(ctx, MessageIDContextKey, "")
}

// GetSupportsImagesFromContext retrieves whether the model supports images from the context.
func GetSupportsImagesFromContext(ctx context.Context) bool {
	return getContextValue(ctx, SupportsImagesContextKey, false)
}

// GetModelNameFromContext retrieves the model name from the context.
func GetModelNameFromContext(ctx context.Context) string {
	return getContextValue(ctx, ModelNameContextKey, "")
}

// GetAgentModeFromContext retrieves the agent mode from the context.
func GetAgentModeFromContext(ctx context.Context) config.AgentMode {
	return getContextValue(ctx, AgentModeContextKey, config.AgentModeDefault)
}

// GetWorkingDirFromContext retrieves the working directory from the context.
func GetWorkingDirFromContext(ctx context.Context) string {
	return getContextValue(ctx, WorkingDirContextKey, "")
}

func GetRuntimeControlFromContext(ctx context.Context) RuntimeControl {
	value := ctx.Value(RuntimeControlContextKey)
	if value == nil {
		return nil
	}
	if control, ok := value.(RuntimeControl); ok {
		return control
	}
	return nil
}

func GetWriteScopeFromContext(ctx context.Context) *WriteScope {
	value := ctx.Value(WriteScopeContextKey)
	if value == nil {
		return nil
	}
	if scope, ok := value.(*WriteScope); ok {
		return scope
	}
	return nil
}
