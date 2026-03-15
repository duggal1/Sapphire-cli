package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/stretchr/testify/require"
)

func TestTodosToolPersistsFullTodoList(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Test")
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	input, err := json.Marshal(TodosParams{
		Todos: []TodoItem{
			{Content: "Read codebase", Status: "in_progress", ActiveForm: "Reading codebase"},
			{Content: "Run tests", Status: "pending", ActiveForm: "Running tests"},
		},
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-1",
		Name:  TodosToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, updated.Todos, 2)
	require.Empty(t, updated.Todos[0].ID)
	require.Equal(t, session.TodoStatusInProgress, updated.Todos[0].Status)
	require.Equal(t, "Reading codebase", updated.Todos[0].ActiveForm)
	require.Equal(t, session.TodoStatusPending, updated.Todos[1].Status)

	var meta TodosResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.IsNew)
	require.Equal(t, 2, meta.Total)
	require.Equal(t, 0, meta.Completed)
	require.Equal(t, "Reading codebase", meta.JustStarted)
}

func TestTodosToolRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Invalid Status")
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-invalid-status",
		Name:  TodosToolName,
		Input: `{"todos":[{"content":"Inspect","status":"blocked","active_form":"Inspecting"}]}`,
	})
	require.ErrorContains(t, err, `invalid status "blocked"`)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, updated.Todos)
}

func TestTodosToolReplacesExistingListInsteadOfMutatingImplicitly(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Replace")
	require.NoError(t, err)
	sess.Todos = []session.Todo{
		{Content: "Read codebase", Status: session.TodoStatusInProgress, ActiveForm: "Reading codebase"},
		{Content: "Run tests", Status: session.TodoStatusPending},
	}
	_, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-replace-list",
		Name:  TodosToolName,
		Input: `{"todos":[{"content":"Read codebase","status":"completed"},{"content":"Run tests","status":"in_progress","active_form":"Running tests"}]}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, []session.Todo{
		{Content: "Read codebase", Status: session.TodoStatusCompleted},
		{Content: "Run tests", Status: session.TodoStatusInProgress, ActiveForm: "Running tests"},
	}, updated.Todos)

	var meta TodosResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, []string{"Read codebase"}, meta.JustCompleted)
	require.Equal(t, "Running tests", meta.JustStarted)
	require.Equal(t, 1, meta.Completed)
}
