package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/google/uuid"
)

//go:embed todos.md
var todosDescription []byte

const TodosToolName = "todos"

const (
	todosActionCreate   = "create"
	todosActionUpdate   = "update"
	todosActionStart    = "start"
	todosActionComplete = "complete"
	todosActionList     = "list"
	todosActionReset    = "reset"
)

type TodosParams struct {
	Action      string     `json:"action,omitempty" description:"create, update, start, complete, list, reset"`
	TaskID      string     `json:"task_id,omitempty" description:"Target task id for start/update/complete"`
	TaskKey     string     `json:"task_key,omitempty" description:"Stable target task key for start/update/complete"`
	TaskContent string     `json:"task_content,omitempty" description:"Target task content when id is unavailable"`
	Task        *TodoItem  `json:"task,omitempty" description:"Single task payload"`
	Tasks       []TodoItem `json:"tasks,omitempty" description:"Task list for create/reset"`
	Todos       []TodoItem `json:"todos,omitempty" description:"Deprecated alias for tasks"`
}

type TodoItem struct {
	ID         string `json:"id,omitempty" description:"Task id (auto-generated if omitted)"`
	Key        string `json:"key,omitempty" description:"Stable task key shared across planner/runtime updates"`
	Content    string `json:"content" description:"What needs to be done (imperative form)"`
	Status     string `json:"status,omitempty" description:"Task status: pending, in_progress, or completed"`
	ActiveForm string `json:"active_form,omitempty" description:"Present continuous form (e.g., 'Running tests')"`
}

type TodosResponseMetadata struct {
	Action        string         `json:"action"`
	IsNew         bool           `json:"is_new"`
	Todos         []session.Todo `json:"todos"`
	CreatedIDs    []string       `json:"created_ids,omitempty"`
	UpdatedIDs    []string       `json:"updated_ids,omitempty"`
	JustCompleted []string       `json:"just_completed,omitempty"`
	JustStarted   string         `json:"just_started,omitempty"`
	Completed     int            `json:"completed"`
	InProgress    int            `json:"in_progress"`
	Pending       int            `json:"pending"`
	Total         int            `json:"total"`
}

func NewTodosTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TodosToolName,
		string(todosDescription),
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for managing todos")
			}

			getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			currentSession, err := sessions.Get(getCtx, sessionID)
			cancel()
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			if params == nil {
				params = map[string]any{}
			}
			normalizeTodosInput(params)

			var typed TodosParams
			if err := decodeInto(params, &typed); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid parameters: %s", err)), nil
			}
			normalizeTodosParams(&typed)

			action := typed.Action
			oldTodos := currentSession.Todos

			var (
				newTodos      []session.Todo
				createdIDs    []string
				updatedIDs    []string
				justCompleted []string
				justStarted   string
			)

			switch action {
			case todosActionCreate:
				items := sanitizeTodoItems(typed.Tasks, typed.Task)
				if len(items) == 0 {
					items = []TodoItem{defaultTodoItem()}
				}
				items = ensureTodoIDs(items)
				newTodos = append([]session.Todo{}, oldTodos...)
				for _, item := range items {
					newTodos = append(newTodos, toSessionTodo(item))
					createdIDs = append(createdIDs, item.ID)
				}
				enforceSingleInProgress(&newTodos)
				justCompleted, justStarted = detectStatusTransitions(oldTodos, newTodos)
			case todosActionReset:
				items := sanitizeTodoItems(typed.Tasks, typed.Task)
				if len(items) == 0 {
					items = []TodoItem{defaultTodoItem()}
				}
				items = ensureTodoIDs(items)
				newTodos = make([]session.Todo, 0, len(items))
				for _, item := range items {
					newTodos = append(newTodos, toSessionTodo(item))
				}
				enforceSingleInProgress(&newTodos)
				createdIDs = collectIDs(items)
				justCompleted, justStarted = detectStatusTransitions(oldTodos, newTodos)
			case todosActionUpdate:
				newTodos = append([]session.Todo{}, oldTodos...)
				idx, err := resolveTodoIndex(newTodos, todosActionUpdate, typed.TaskID, typed.TaskKey, typed.TaskContent, typed.Task)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				updated, started := applyUpdate(&newTodos, idx, typed.Task)
				updatedIDs = append(updatedIDs, updated)
				if started != "" {
					justStarted = started
				}
				justCompleted, _ = detectStatusTransitions(oldTodos, newTodos)
			case todosActionStart:
				newTodos = append([]session.Todo{}, oldTodos...)
				idx, err := resolveTodoIndex(newTodos, todosActionStart, typed.TaskID, typed.TaskKey, typed.TaskContent, typed.Task)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				startTodo(&newTodos, idx)
				updatedIDs = append(updatedIDs, newTodos[idx].ID)
				justStarted = newTodos[idx].ActiveForm
				justCompleted, _ = detectStatusTransitions(oldTodos, newTodos)
			case todosActionComplete:
				newTodos = append([]session.Todo{}, oldTodos...)
				idx, err := resolveTodoIndex(newTodos, todosActionComplete, typed.TaskID, typed.TaskKey, typed.TaskContent, typed.Task)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				completeTodo(&newTodos, idx)
				updatedIDs = append(updatedIDs, newTodos[idx].ID)
				justCompleted, _ = detectStatusTransitions(oldTodos, newTodos)
			case todosActionList:
				newTodos = oldTodos
				justCompleted, justStarted = detectStatusTransitions(oldTodos, newTodos)
			default:
				return fantasy.NewTextErrorResponse("invalid action: use create, update, start, complete, list, or reset"), nil
			}

			if action != todosActionList {
				currentSession.Todos = newTodos
				saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := sessions.Save(saveCtx, currentSession)
				cancel()
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("failed to save todos: %w", err)
				}
			}

			pending, inProgress, completed := countStatuses(newTodos)
			meta := TodosResponseMetadata{
				Action:        action,
				IsNew:         len(oldTodos) == 0 && len(newTodos) > 0,
				Todos:         newTodos,
				CreatedIDs:    createdIDs,
				UpdatedIDs:    updatedIDs,
				JustCompleted: justCompleted,
				JustStarted:   justStarted,
				Completed:     completed,
				InProgress:    inProgress,
				Pending:       pending,
				Total:         len(newTodos),
			}

			payload, _ := json.Marshal(map[string]any{
				"action":         action,
				"todos":          newTodos,
				"counts":         map[string]int{"pending": pending, "in_progress": inProgress, "completed": completed, "total": len(newTodos)},
				"just_started":   justStarted,
				"just_completed": justCompleted,
			})

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(string(payload)), meta), nil
		},
	)
}

func normalizeTodosParams(params *TodosParams) {
	if params == nil {
		return
	}
	if len(params.Tasks) == 0 && len(params.Todos) > 0 {
		params.Tasks = params.Todos
	}
	if params.Task != nil {
		if params.Task.ID == "" && strings.TrimSpace(params.TaskID) != "" {
			params.Task.ID = strings.TrimSpace(params.TaskID)
		}
		if params.Task.Key == "" && strings.TrimSpace(params.TaskKey) != "" {
			params.Task.Key = strings.TrimSpace(params.TaskKey)
		}
		if params.Task.Content == "" && strings.TrimSpace(params.TaskContent) != "" {
			params.Task.Content = strings.TrimSpace(params.TaskContent)
		}
	}
	if params.Action == "" {
		switch {
		case len(params.Tasks) > 0:
			params.Action = todosActionCreate
		case params.Task != nil || strings.TrimSpace(params.TaskID) != "" || strings.TrimSpace(params.TaskKey) != "" || strings.TrimSpace(params.TaskContent) != "":
			params.Action = todosActionUpdate
		default:
			params.Action = todosActionList
		}
	}
	params.Action = strings.ToLower(strings.TrimSpace(params.Action))
}

func sanitizeTodoItems(items []TodoItem, single *TodoItem) []TodoItem {
	if len(items) == 0 && single != nil {
		items = []TodoItem{*single}
	}
	if len(items) == 0 {
		return nil
	}
	out := make([]TodoItem, 0, len(items))
	inProgressSet := false
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		status := normalizeStatus(item.Status)
		if status == "" {
			status = string(session.TodoStatusPending)
		}
		if status == string(session.TodoStatusInProgress) {
			if inProgressSet {
				status = string(session.TodoStatusPending)
			} else {
				inProgressSet = true
			}
		}
		activeForm := strings.TrimSpace(item.ActiveForm)
		if status == string(session.TodoStatusInProgress) && activeForm == "" {
			activeForm = "Working on " + content
		}
		out = append(out, TodoItem{
			ID:         strings.TrimSpace(item.ID),
			Key:        normalizeTodoKey(item.Key, content),
			Content:    content,
			Status:     status,
			ActiveForm: activeForm,
		})
	}
	return out
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(session.TodoStatusPending):
		return string(session.TodoStatusPending)
	case string(session.TodoStatusInProgress):
		return string(session.TodoStatusInProgress)
	case string(session.TodoStatusCompleted):
		return string(session.TodoStatusCompleted)
	default:
		return ""
	}
}

func ensureTodoIDs(items []TodoItem) []TodoItem {
	for i := range items {
		if strings.TrimSpace(items[i].ID) == "" {
			items[i].ID = uuid.NewString()
		}
		if strings.TrimSpace(items[i].Key) == "" {
			items[i].Key = normalizeTodoKey("", items[i].Content)
		}
	}
	return items
}

func collectIDs(items []TodoItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.ID != "" {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func toSessionTodo(item TodoItem) session.Todo {
	return session.Todo{
		ID:         item.ID,
		Key:        item.Key,
		Content:    item.Content,
		Status:     session.TodoStatus(item.Status),
		ActiveForm: item.ActiveForm,
	}
}

func findTodoIndex(todos []session.Todo, taskID, taskKey, taskContent string, task *TodoItem) (int, error) {
	id := strings.TrimSpace(taskID)
	key := strings.TrimSpace(taskKey)
	content := strings.TrimSpace(taskContent)
	activeForm := ""
	if task != nil {
		if id == "" {
			id = strings.TrimSpace(task.ID)
		}
		if content == "" {
			content = strings.TrimSpace(task.Content)
		}
		if key == "" {
			key = strings.TrimSpace(task.Key)
		}
		if activeForm == "" {
			activeForm = strings.TrimSpace(task.ActiveForm)
		}
	}
	var idErr error
	if id != "" {
		for i, todo := range todos {
			if todo.ID == id {
				return i, nil
			}
		}
		idErr = fmt.Errorf("task_id not found: %s", id)
	}
	if key != "" {
		normalizedKey := normalizeTodoKey(key, "")
		for i, todo := range todos {
			if normalizeTodoKey(todo.Key, todo.Content) == normalizedKey {
				return i, nil
			}
		}
		if idx, ok := findApproxTodoKeyIndex(todos, normalizedKey); ok {
			return idx, nil
		}
	}
	if content != "" {
		contentCandidates := todoLookupCandidates(content)
		for i, todo := range todos {
			current := strings.TrimSpace(todo.Content)
			for _, candidate := range contentCandidates {
				if strings.EqualFold(current, candidate) {
					return i, nil
				}
			}
		}
		for i, todo := range todos {
			current := strings.ToLower(strings.TrimSpace(todo.Content))
			for _, candidate := range contentCandidates {
				if strings.Contains(current, strings.ToLower(candidate)) {
					return i, nil
				}
			}
		}
		return -1, fmt.Errorf("task not found by content: %s", content)
	}
	if activeForm != "" {
		activeFormCandidates := todoLookupCandidates(activeForm)
		for i, todo := range todos {
			current := strings.TrimSpace(todo.ActiveForm)
			for _, candidate := range activeFormCandidates {
				if strings.EqualFold(current, candidate) {
					return i, nil
				}
			}
		}
		for i, todo := range todos {
			current := strings.ToLower(strings.TrimSpace(todo.ActiveForm))
			for _, candidate := range activeFormCandidates {
				if strings.Contains(current, strings.ToLower(candidate)) {
					return i, nil
				}
			}
		}
		return -1, fmt.Errorf("task not found by active_form: %s", activeForm)
	}
	if idErr != nil {
		return -1, idErr
	}
	if key != "" {
		return -1, fmt.Errorf("task_key not found: %s", key)
	}
	return -1, fmt.Errorf("task_id, task_key, or task content is required")
}

func todoLookupCandidates(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	candidates := []string{value}
	if idx := strings.Index(value, ":"); idx >= 0 {
		suffix := strings.TrimSpace(value[idx+1:])
		if suffix != "" && !strings.EqualFold(suffix, value) {
			candidates = append(candidates, suffix)
		}
	}
	return candidates
}

func resolveTodoIndex(todos []session.Todo, action, taskID, taskKey, taskContent string, task *TodoItem) (int, error) {
	if idx, err := findTodoIndex(todos, taskID, taskKey, taskContent, task); err == nil {
		return idx, nil
	} else if !shouldFallbackTodoResolution(err, taskID, taskKey, taskContent, task) {
		return -1, err
	}

	inProgress := todoIndexesByStatus(todos, session.TodoStatusInProgress)
	pending := todoIndexesByStatus(todos, session.TodoStatusPending)
	incomplete := append(append([]int{}, inProgress...), pending...)

	switch action {
	case todosActionComplete:
		if len(inProgress) == 1 {
			return inProgress[0], nil
		}
		if len(incomplete) == 1 {
			return incomplete[0], nil
		}
		return -1, fmt.Errorf("task_id, task_key, or task content is required to complete a specific todo")
	case todosActionStart:
		if len(inProgress) == 1 && len(pending) == 0 {
			return inProgress[0], nil
		}
		if len(pending) == 1 {
			return pending[0], nil
		}
		if len(inProgress) == 0 && len(pending) > 0 {
			return pending[0], nil
		}
		return -1, fmt.Errorf("task_id, task_key, or task content is required to start a specific todo")
	case todosActionUpdate:
		if len(inProgress) == 1 {
			return inProgress[0], nil
		}
		if len(todos) == 1 {
			return 0, nil
		}
		return -1, fmt.Errorf("task_id, task_key, or task content is required to update a specific todo")
	default:
		return -1, fmt.Errorf("task_id, task_key, or task content is required")
	}
}

func shouldFallbackTodoResolution(err error, taskID, taskKey, taskContent string, task *TodoItem) bool {
	if err == nil {
		return false
	}

	contentHint := strings.TrimSpace(taskContent)
	activeFormHint := ""
	idHint := strings.TrimSpace(taskID)
	keyHint := strings.TrimSpace(taskKey)
	if task != nil {
		if contentHint == "" {
			contentHint = strings.TrimSpace(task.Content)
		}
		if keyHint == "" {
			keyHint = strings.TrimSpace(task.Key)
		}
		if activeFormHint == "" {
			activeFormHint = strings.TrimSpace(task.ActiveForm)
		}
		if idHint == "" {
			idHint = strings.TrimSpace(task.ID)
		}
	}

	if contentHint != "" || activeFormHint != "" {
		return false
	}

	if idHint == "" && keyHint == "" {
		return true
	}

	return strings.HasPrefix(err.Error(), "task_id not found:") || strings.HasPrefix(err.Error(), "task_key not found:")
}

func todoIndexesByStatus(todos []session.Todo, status session.TodoStatus) []int {
	indexes := make([]int, 0, len(todos))
	for i, todo := range todos {
		if todo.Status == status {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func applyUpdate(todos *[]session.Todo, idx int, task *TodoItem) (string, string) {
	if task == nil {
		return (*todos)[idx].ID, ""
	}
	updated := (*todos)[idx]
	if strings.TrimSpace(task.Content) != "" {
		updated.Content = strings.TrimSpace(task.Content)
	}
	if strings.TrimSpace(task.Key) != "" {
		updated.Key = normalizeTodoKey(task.Key, updated.Content)
	}
	if strings.TrimSpace(task.Status) != "" {
		updated.Status = session.TodoStatus(normalizeStatus(task.Status))
	}
	if strings.TrimSpace(task.ActiveForm) != "" {
		updated.ActiveForm = strings.TrimSpace(task.ActiveForm)
	}

	justStarted := ""
	if updated.Status == session.TodoStatusInProgress {
		if updated.ActiveForm == "" {
			updated.ActiveForm = "Working on " + updated.Content
		}
		justStarted = updated.ActiveForm
		for i := range *todos {
			if i == idx {
				continue
			}
			if (*todos)[i].Status == session.TodoStatusInProgress {
				(*todos)[i].Status = session.TodoStatusPending
				(*todos)[i].ActiveForm = ""
			}
		}
	}

	(*todos)[idx] = updated
	return updated.ID, justStarted
}

func startTodo(todos *[]session.Todo, idx int) {
	for i := range *todos {
		if i == idx {
			(*todos)[i].Status = session.TodoStatusInProgress
			if strings.TrimSpace((*todos)[i].ActiveForm) == "" {
				(*todos)[i].ActiveForm = "Working on " + (*todos)[i].Content
			}
			continue
		}
		if (*todos)[i].Status == session.TodoStatusInProgress {
			(*todos)[i].Status = session.TodoStatusPending
			(*todos)[i].ActiveForm = ""
		}
	}
}

func completeTodo(todos *[]session.Todo, idx int) {
	(*todos)[idx].Status = session.TodoStatusCompleted
	(*todos)[idx].ActiveForm = ""
}

func enforceSingleInProgress(todos *[]session.Todo) {
	found := false
	for i := range *todos {
		if (*todos)[i].Status != session.TodoStatusInProgress {
			continue
		}
		if !found {
			found = true
			if strings.TrimSpace((*todos)[i].ActiveForm) == "" {
				(*todos)[i].ActiveForm = "Working on " + (*todos)[i].Content
			}
			continue
		}
		(*todos)[i].Status = session.TodoStatusPending
		(*todos)[i].ActiveForm = ""
	}
}

func detectStatusTransitions(oldTodos, newTodos []session.Todo) ([]string, string) {
	oldStatus := make(map[string]session.TodoStatus, len(oldTodos))
	for _, todo := range oldTodos {
		oldStatus[todo.ID] = todo.Status
	}
	var justCompleted []string
	justStarted := ""
	for _, todo := range newTodos {
		prev, ok := oldStatus[todo.ID]
		if todo.Status == session.TodoStatusCompleted && (!ok || prev != session.TodoStatusCompleted) {
			justCompleted = append(justCompleted, todo.Content)
		}
		if todo.Status == session.TodoStatusInProgress && (!ok || prev != session.TodoStatusInProgress) {
			if todo.ActiveForm != "" {
				justStarted = todo.ActiveForm
			} else {
				justStarted = todo.Content
			}
		}
	}
	return justCompleted, justStarted
}

func countStatuses(todos []session.Todo) (pending, inProgress, completed int) {
	for _, todo := range todos {
		switch todo.Status {
		case session.TodoStatusPending:
			pending++
		case session.TodoStatusInProgress:
			inProgress++
		case session.TodoStatusCompleted:
			completed++
		}
	}
	return pending, inProgress, completed
}

func defaultTodoItem() TodoItem {
	return TodoItem{
		Content:    "Proceed with the requested task",
		Key:        "proceed_with_the_requested_task",
		Status:     string(session.TodoStatusInProgress),
		ActiveForm: "Working on the requested task",
	}
}

func normalizeTodoKey(key, content string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		if idx := strings.Index(content, ":"); idx > 0 {
			prefix := strings.TrimSpace(content[:idx])
			if isSimpleTodoKey(prefix) {
				key = prefix
			}
		}
	}
	if key == "" {
		key = content
	}
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", "\\", "_", ":", "_", ".", "_")
	key = replacer.Replace(key)
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	return strings.Trim(key, "_")
}

func findApproxTodoKeyIndex(todos []session.Todo, requestedKey string) (int, bool) {
	requestedTokens := tokenizeTodoKey(requestedKey)
	if len(requestedTokens) == 0 {
		return 0, false
	}

	bestIdx := -1
	bestScore := 0
	secondBest := 0
	for i, todo := range todos {
		score := todoKeySimilarityScore(requestedTokens, tokenizeTodoKey(normalizeTodoKey(todo.Key, todo.Content)))
		if score > bestScore {
			secondBest = bestScore
			bestScore = score
			bestIdx = i
			continue
		}
		if score > secondBest {
			secondBest = score
		}
	}

	if bestIdx >= 0 && bestScore >= 2 && bestScore > secondBest {
		return bestIdx, true
	}
	return 0, false
}

func tokenizeTodoKey(value string) []string {
	normalized := normalizeTodoKey(value, "")
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "_")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) > 3 && strings.HasSuffix(part, "s") {
			part = strings.TrimSuffix(part, "s")
		}
		tokens = append(tokens, part)
	}
	return tokens
}

func todoKeySimilarityScore(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	tokenSet := make(map[string]struct{}, len(b))
	for _, token := range b {
		tokenSet[token] = struct{}{}
	}
	score := 0
	for _, token := range a {
		if _, ok := tokenSet[token]; ok {
			score++
		}
	}
	return score
}

func isSimpleTodoKey(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
