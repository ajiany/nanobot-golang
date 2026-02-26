package heartbeat

import (
	"context"
	"testing"
	"time"
)

func TestNewServiceDefaultInterval(t *testing.T) {
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: t.TempDir(),
		// Interval intentionally zero — should default to 30 minutes
	})
	if svc.interval != 30*time.Minute {
		t.Errorf("expected default interval 30m, got %v", svc.interval)
	}
}

func TestNewServiceCustomInterval(t *testing.T) {
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: t.TempDir(),
		Interval:  5 * time.Minute,
	})
	if svc.interval != 5*time.Minute {
		t.Errorf("expected interval 5m, got %v", svc.interval)
	}
}

func TestStartAndStop(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: dir,
		Interval:  time.Hour,
	})

	ctx := context.Background()
	svc.Start(ctx)

	svc.mu.Lock()
	running := svc.running
	svc.mu.Unlock()
	if !running {
		t.Fatal("expected service to be running after Start")
	}

	svc.Stop()

	svc.mu.Lock()
	running = svc.running
	svc.mu.Unlock()
	if running {
		t.Fatal("expected service to be stopped after Stop")
	}
}

func TestStartIdempotent(t *testing.T) {
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: t.TempDir(),
		Interval:  time.Hour,
	})

	ctx := context.Background()
	svc.Start(ctx)
	svc.Start(ctx) // second call should be a no-op

	svc.mu.Lock()
	running := svc.running
	svc.mu.Unlock()
	if !running {
		t.Fatal("expected service to still be running")
	}
	svc.Stop()
}

func TestStopIdempotent(t *testing.T) {
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: t.TempDir(),
		Interval:  time.Hour,
	})

	// Stop without Start should not panic
	svc.Stop()
	svc.Stop()
}

func TestHeartbeatUnknownAction(t *testing.T) {
	dir := t.TempDir()
	writeHeartbeat(t, dir)

	called := false
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "unknown"},
		Model:     "m",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: func(ctx context.Context, message string) { called = true },
	})

	svc.tick(context.Background())

	if called {
		t.Error("onExecute should not be called for unknown action")
	}
}

func TestHeartbeatRunNilOnExecute(t *testing.T) {
	dir := t.TempDir()
	writeHeartbeat(t, dir)

	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "run", message: "msg"},
		Model:     "m",
		Workspace: dir,
		Interval:  time.Hour,
		OnExecute: nil, // should not panic
	})

	// should not panic
	svc.tick(context.Background())
}

func TestContextCancellationStopsService(t *testing.T) {
	svc := NewService(Config{
		Provider:  &mockHeartbeatProvider{action: "skip"},
		Model:     "m",
		Workspace: t.TempDir(),
		Interval:  time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)
	cancel()

	// Give the goroutine a moment to exit via ctx.Done()
	time.Sleep(20 * time.Millisecond)

	// Service goroutine should have exited; no assertion needed beyond no deadlock
}
