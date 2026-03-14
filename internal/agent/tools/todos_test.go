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

func TestTodosToolPersistsTodos(t *testing.T) {
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
		Action: "create",
		Tasks: []TodoItem{
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
	require.NotEmpty(t, updated.Todos[0].ID)
	require.Equal(t, session.TodoStatusInProgress, updated.Todos[0].Status)
	require.Equal(t, "Reading codebase", updated.Todos[0].ActiveForm)
	require.Equal(t, session.TodoStatusPending, updated.Todos[1].Status)

	var meta TodosResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.IsNew)
	require.Equal(t, 2, meta.Total)
	require.Equal(t, "Reading codebase", meta.JustStarted)
}

func TestTodosToolNormalizesInvalidStatus(t *testing.T) {
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
		ID:    "todos-2",
		Name:  TodosToolName,
		Input: `{"action":"create","tasks":[{"content":"Inspect","status":"blocked","active_form":"Inspecting"}]}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, updated.Todos, 1)
	require.Equal(t, session.TodoStatusPending, updated.Todos[0].Status)
}

func TestTodosToolCompletesImplicitInProgressTodo(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Implicit Complete")
	require.NoError(t, err)
	sess.Todos = []session.Todo{
		{ID: "todo-1", Content: "Read codebase", Status: session.TodoStatusInProgress, ActiveForm: "Reading codebase"},
		{ID: "todo-2", Content: "Run tests", Status: session.TodoStatusPending},
	}
	_, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-implicit-complete",
		Name:  TodosToolName,
		Input: `{"action":"complete"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, session.TodoStatusCompleted, updated.Todos[0].Status)
}

func TestTodosToolStartsFirstPendingTodoWhenUnambiguous(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Implicit Start")
	require.NoError(t, err)
	sess.Todos = []session.Todo{
		{ID: "todo-1", Content: "Read codebase", Status: session.TodoStatusCompleted},
		{ID: "todo-2", Content: "Run tests", Status: session.TodoStatusPending},
	}
	_, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-implicit-start",
		Name:  TodosToolName,
		Input: `{"action":"start"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, session.TodoStatusInProgress, updated.Todos[1].Status)
	require.Equal(t, "Working on Run tests", updated.Todos[1].ActiveForm)
}

func TestTodosToolCoercesStringTasksPayload(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos String Payload")
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-string-payload",
		Name:  TodosToolName,
		Input: `{"tasks":"- Read codebase\n- Run tests"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, updated.Todos, 2)
	require.Equal(t, "Read codebase", updated.Todos[0].Content)
	require.Equal(t, "Run tests", updated.Todos[1].Content)
}

func TestTodosToolFallsBackFromStaleTaskIDToSinglePendingStart(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Stale ID Start")
	require.NoError(t, err)
	sess.Todos = []session.Todo{
		{ID: "todo-1", Content: "Read codebase", Status: session.TodoStatusCompleted},
		{ID: "todo-2", Content: "Run tests", Status: session.TodoStatusPending},
	}
	_, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-stale-id-start",
		Name:  TodosToolName,
		Input: `{"action":"start","task_id":"stale-task-id"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, session.TodoStatusInProgress, updated.Todos[1].Status)
}

func TestTodosToolFallsBackFromStaleTaskIDToOnlyInProgressComplete(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	sess, err := sessions.Create(t.Context(), "Todos Stale ID Complete")
	require.NoError(t, err)
	sess.Todos = []session.Todo{
		{ID: "todo-1", Content: "Read codebase", Status: session.TodoStatusInProgress, ActiveForm: "Reading codebase"},
		{ID: "todo-2", Content: "Run tests", Status: session.TodoStatusPending},
	}
	_, err = sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	tool := NewTodosTool(sessions)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-stale-id-complete",
		Name:  TodosToolName,
		Input: `{"action":"complete","task_id":"stale-task-id"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	updated, err := sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, session.TodoStatusCompleted, updated.Todos[0].Status)
}
