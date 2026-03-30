package tools

import (
	"context"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
)

type (
	sessionIDContextKey   string
	sessionModeContextKey string
	messageIDContextKey   string
	supportsImagesKey     string
	modelNameKey          string
	workingDirKey         string
	runtimeControlKey     string
	writeScopeKey         string
	turnPolicyKey         string
)

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// SessionModeContextKey is the key for the current collaboration mode in the context.
	SessionModeContextKey sessionModeContextKey = "session_mode"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// WorkingDirContextKey is the key for the working directory in the context.
	WorkingDirContextKey workingDirKey = "working_dir"
	// RuntimeControlContextKey is the key for the runtime control loop in the context.
	RuntimeControlContextKey runtimeControlKey = "runtime_control"
	// WriteScopeContextKey is the key for sub-agent write scope constraints.
	WriteScopeContextKey writeScopeKey = "write_scope"
	// TurnPolicyContextKey is the key for per-turn runtime guardrails.
	TurnPolicyContextKey turnPolicyKey = "turn_policy"

	// LoadSkillToolName is the name of the tool used to load skills.
	LoadSkillToolName = "load_skill"

	// InstallSkillToolName is the name of the tool used to search and install skills.
	InstallSkillToolName = "install_skill"

	// IndexCodebaseToolName warms the durable codebase graph and prompt boot packet context.
	IndexCodebaseToolName = "index_codebase"

	// ToolSuggestToolName is the name of the tool used to suggest MCP tools.
	ToolSuggestToolName = "tool_suggest"
)

type RuntimeControl interface {
	AllowToolCall(toolName string) error
	BeginToolExecution(toolName string)
	FinishToolExecution(toolName string)
}

type TurnPolicy struct {
	DirectResponseOnly       bool
	AllowMemoryRead          bool
	AllowMemoryWrite         bool
	AllowAutoMemoryInjection bool
}

func DefaultTurnPolicy() TurnPolicy {
	return TurnPolicy{
		AllowMemoryRead:          true,
		AllowMemoryWrite:         true,
		AllowAutoMemoryInjection: true,
	}
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

// GetSessionModeFromContext retrieves the current collaboration mode from the context.
func GetSessionModeFromContext(ctx context.Context) planmode.SessionMode {
	value := ctx.Value(SessionModeContextKey)
	switch typed := value.(type) {
	case planmode.SessionMode:
		return planmode.NormalizeMode(typed)
	case string:
		return planmode.NormalizeMode(planmode.SessionMode(typed))
	default:
		return planmode.DefaultMode()
	}
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

func GetTurnPolicyFromContext(ctx context.Context) TurnPolicy {
	return getContextValue(ctx, TurnPolicyContextKey, DefaultTurnPolicy())
}
