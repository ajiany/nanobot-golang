package heartbeat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/providers"
)

type mockHeartbeatProvider struct {
	action  string
	message string
}

func (m *mockHeartbeatProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	args, _ := json.Marshal(map[string]string{
		"action":  m.action,
		"message": m.message,
	})
	return &providers.ChatResponse{
		ToolCalls: []providers.ToolCall{{ID: "call_1", Name: "heartbeat_decision", Arguments: string(args)}},
	}, nil
}

func writeHeartbeat(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "HEARTBEAT.md"), []byte("# Heartbeat\nCheck for pending tasks."), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestHeartbeatSkip(t *testing.T) {
	dir := t.TempDir()
	writeHeartbeat(t, dir)

	called := false
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "test-model",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: func(ctx context.Context, message string) { called = true },
	})

	svc.tick(context.Background())

	if called {
		t.Error("onExecute should not be called when action=skip")
	}
}

func TestHeartbeatRun(t *testing.T) {
	dir := t.TempDir()
	writeHeartbeat(t, dir)

	var gotMessage string
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "run", message: "do the thing"},
		Model:     "test-model",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: func(ctx context.Context, message string) { gotMessage = message },
	})

	svc.tick(context.Background())

	if gotMessage != "do the thing" {
		t.Errorf("expected message %q, got %q", "do the thing", gotMessage)
	}
}

func TestTriggerNow(t *testing.T) {
	dir := t.TempDir()
	writeHeartbeat(t, dir)

	called := false
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "run", message: "triggered"},
		Model:     "test-model",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: func(ctx context.Context, message string) { called = true },
	})

	svc.TriggerNow(context.Background())

	if !called {
		t.Error("onExecute should be called after TriggerNow")
	}
}

func TestNoHeartbeatFile(t *testing.T) {
	dir := t.TempDir()
	// no HEARTBEAT.md written

	called := false
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "run"},
		Model:     "test-model",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: func(ctx context.Context, message string) { called = true },
	})

	// should return without error or panic
	svc.tick(context.Background())

	if called {
		t.Error("onExecute should not be called when HEARTBEAT.md is missing")
	}
}
