package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestValidateEditInputMapAllowsCreateAndDeleteShapes(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateEditInputMap(map[string]any{
		"file_path":  "notes.txt",
		"new_string": "hello",
	}))
	require.NoError(t, validateEditInputMap(map[string]any{
		"file_path":  "notes.txt",
		"old_string": "hello",
	}))
	require.ErrorContains(t, validateEditInputMap(map[string]any{
		"file_path": "notes.txt",
	}), "edit requires old_string or new_string")
}

func TestValidateEditInputMapRejectsFileEditsShape(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, validateEditInputMap(map[string]any{
		"file_edits": []any{
			map[string]any{
				"file_path": "notes.txt",
				"edits": []any{
					map[string]any{"new_string": "hello"},
				},
			},
		},
	}), "edit only accepts a single file_path plus old_string/new_string")
}

func TestValidateGlobInputMapRejectsEmptyPattern(t *testing.T) {
	t.Parallel()

	globTool := fantasy.NewAgentTool(
		GlobToolName,
		"",
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	require.ErrorContains(t, validateToolCallInput(context.Background(), globTool, fantasy.ToolCall{
		Name: GlobToolName,
	}, map[string]any{}), "glob requires one pattern string in pattern")
}

func TestValidateAgenticEditInputMapAllowsCreateOperation(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateAgenticEditInputMap(map[string]any{
		"file_edits": []any{
			map[string]any{
				"file_path": "notes.txt",
				"edits": []any{
					map[string]any{"new_string": "hello"},
				},
			},
		},
	}))
}

func TestValidateAgenticEditInputMapRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, validateAgenticEditInputMap(map[string]any{}), "concrete file_path(s) and edit operations")
}

func TestValidateAgenticEditInputMapRejectsEmptyFileEdits(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, validateAgenticEditInputMap(map[string]any{
		"file_edits": []any{},
	}), "concrete file_path(s) and edit operations")
}

func TestValidateAgenticEditInputMapAllowsObjectShorthandShapes(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateAgenticEditInputMap(map[string]any{
		"file_edits": map[string]any{
			"file_path": "notes.txt",
			"edits": map[string]any{
				"old_string": "hello",
				"new_string": "world",
			},
		},
	}))
}

func TestValidateAgenticEditInputMapAllowsTopLevelEditsFileEditShape(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateAgenticEditInputMap(map[string]any{
		"edits": []any{
			map[string]any{
				"file_path":  "notes.txt",
				"old_string": "hello",
				"new_string": "world",
			},
		},
	}))
}
