package tools

import (
	"testing"

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
	}), "at least one of old_string or new_string is required")
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
