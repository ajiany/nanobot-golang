package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// SpawnFunc is a callback that spawns a background task agent.
// Returns a task ID string.
type SpawnFunc func(ctx context.Context, task, label string) string

type SpawnTaskTool struct {
	spawnFn SpawnFunc
}

func NewSpawnTaskTool(fn SpawnFunc) *SpawnTaskTool {
	return &SpawnTaskTool{spawnFn: fn}
}

func (t *SpawnTaskTool) Name() string        { return "spawn_task" }
func (t *SpawnTaskTool) Description() string { return "Spawn a background task agent to work on a subtask" }
func (t *SpawnTaskTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {"type": "string", "description": "Task description"},
			"label": {"type": "string", "description": "Short label for the task"}
		},
		"required": ["task"]
	}`)
}

func (t *SpawnTaskTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Task  string `json:"task"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	taskID := t.spawnFn(ctx, p.Task, p.Label)
	return fmt.Sprintf("Task spawned: %s", taskID), nil
}
