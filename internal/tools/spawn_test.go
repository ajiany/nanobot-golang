package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSpawnTaskTool_CallsSpawnFunc(t *testing.T) {
	var capturedTask, capturedLabel string
	spawnFn := func(ctx context.Context, task, label string) string {
		capturedTask = task
		capturedLabel = label
		return "task-id-42"
	}

	tool := NewSpawnTaskTool(spawnFn)
	params, _ := json.Marshal(map[string]any{
		"task":  "do something important",
		"label": "important-task",
	})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "task-id-42") {
		t.Errorf("unexpected result: %s", result)
	}
	if capturedTask != "do something important" {
		t.Errorf("task = %q, want %q", capturedTask, "do something important")
	}
	if capturedLabel != "important-task" {
		t.Errorf("label = %q, want %q", capturedLabel, "important-task")
	}
}

func TestSpawnTaskTool_NoLabel(t *testing.T) {
	called := false
	spawnFn := func(ctx context.Context, task, label string) string {
		called = true
		return "tid"
	}

	tool := NewSpawnTaskTool(spawnFn)
	params, _ := json.Marshal(map[string]any{"task": "some task"})
	_, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected spawnFn to be called")
	}
}

func TestSpawnTaskTool_EmptyTask(t *testing.T) {
	spawnFn := func(ctx context.Context, task, label string) string { return "id" }
	tool := NewSpawnTaskTool(spawnFn)
	params, _ := json.Marshal(map[string]any{"task": ""})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for empty task")
	}
}

func TestSpawnTaskTool_InvalidParams(t *testing.T) {
	spawnFn := func(ctx context.Context, task, label string) string { return "id" }
	tool := NewSpawnTaskTool(spawnFn)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestSpawnTaskTool_Name(t *testing.T) {
	spawnFn := func(ctx context.Context, task, label string) string { return "" }
	tool := NewSpawnTaskTool(spawnFn)
	if tool.Name() != "spawn_task" {
		t.Errorf("Name() = %q, want spawn_task", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Parameters()) == 0 {
		t.Error("Parameters() is empty")
	}
}
