package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRunShellTool_SimpleCommand(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRunShellTool_CommandWithError(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "exit 1"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
}

func TestRunShellTool_Timeout(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "sleep 10", "timeout": 1})
	start := time.Now()
	_, err := tool.Execute(context.Background(), params)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestRunShellTool_CustomTimeout(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "echo ok", "timeout": 5})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "ok") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRunShellTool_StderrCaptured(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "echo errout >&2; exit 1"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "errout") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestRunShellTool_InvalidParams(t *testing.T) {
	tool := NewRunShellTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestRunShellTool_Name(t *testing.T) {
	tool := NewRunShellTool()
	if tool.Name() != "run_shell" {
		t.Errorf("Name() = %q, want run_shell", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Parameters()) == 0 {
		t.Error("Parameters() is empty")
	}
}
