package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
)

type mockSubagentProvider struct {
	responses []*providers.ChatResponse
	idx       int
	mu        sync.Mutex
}

func (m *mockSubagentProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.responses) {
		return &providers.ChatResponse{Content: "done"}, nil
	}
	r := m.responses[m.idx]
	m.idx++
	return r, nil
}

func newTestSubagentManager(t *testing.T, p providers.Provider) (*SubagentManager, *bus.MessageBus) {
	t.Helper()
	mb := bus.NewMessageBus(10)
	mgr := NewSubagentManager(p, "test-model", 1024, 0, mb)
	return mgr, mb
}

func TestSpawnAndComplete(t *testing.T) {
	mock := &mockSubagentProvider{
		responses: []*providers.ChatResponse{
			{Content: "task result", StopReason: "stop"},
		},
	}
	mgr, mb := newTestSubagentManager(t, mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskID := mgr.Spawn(ctx, "do something", "my-label", "telegram", "chat42")
	if taskID != "task_0" {
		t.Errorf("expected task_0, got %s", taskID)
	}

	// Wait for the inbound message to be published
	select {
	case msg := <-drainInbound(mb):
		if msg.SessionKeyOverride != "telegram:chat42" {
			t.Errorf("unexpected session key: %s", msg.SessionKeyOverride)
		}
		if msg.Channel != "system" {
			t.Errorf("expected channel 'system', got %s", msg.Channel)
		}
		expected := `[Subagent "my-label" completed]`
		if len(msg.Content) < len(expected) || msg.Content[:len(expected)] != expected {
			t.Errorf("unexpected content prefix: %s", msg.Content)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for subagent completion")
	}
}

func TestSpawnWithToolCall(t *testing.T) {
	mock := &mockSubagentProvider{
		responses: []*providers.ChatResponse{
			{
				Content: "",
				ToolCalls: []providers.ToolCall{
					{ID: "tc1", Name: "list_dir", Arguments: `{"path":"."}`},
				},
				StopReason: "tool_use",
			},
			{Content: "listed files", StopReason: "stop"},
		},
	}
	mgr, mb := newTestSubagentManager(t, mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.Spawn(ctx, "list files", "lister", "discord", "room1")

	select {
	case msg := <-drainInbound(mb):
		if msg.SessionKeyOverride != "discord:room1" {
			t.Errorf("unexpected session key: %s", msg.SessionKeyOverride)
		}
		// Provider should have been called twice (tool call + final)
		mock.mu.Lock()
		calls := mock.idx
		mock.mu.Unlock()
		if calls != 2 {
			t.Errorf("expected 2 provider calls, got %d", calls)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for subagent completion")
	}
}

func TestCancel(t *testing.T) {
	// Provider blocks until context is cancelled
	blocker := &blockingProvider{ready: make(chan struct{})}
	mgr, _ := newTestSubagentManager(t, blocker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskID := mgr.Spawn(ctx, "long task", "blocker", "ch", "id")

	// Wait until the provider is actually called before cancelling
	select {
	case <-blocker.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for provider to be called")
	}

	found := mgr.Cancel(taskID)
	if !found {
		t.Error("expected Cancel to return true")
	}

	// After cancel, task should not be in running list
	time.Sleep(50 * time.Millisecond)
	for _, id := range mgr.ListRunning() {
		if id == taskID {
			t.Error("cancelled task still in running list")
		}
	}
}

func TestListRunning(t *testing.T) {
	// Use a blocking provider so tasks stay alive
	b1 := &blockingProvider{ready: make(chan struct{})}
	b2 := &blockingProvider{ready: make(chan struct{})}

	mb := bus.NewMessageBus(10)
	mgr1 := NewSubagentManager(b1, "test-model", 1024, 0, mb)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id1 := mgr1.Spawn(ctx, "task one", "one", "ch", "c1")
	id2 := mgr1.Spawn(ctx, "task two", "two", "ch", "c2")

	// Wait for both providers to be called (tasks are running)
	select {
	case <-b1.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for task 1")
	}

	// Second task uses same manager but different provider — swap trick:
	// Actually both tasks share mgr1 which uses b1. Let's just wait a bit.
	_ = b2
	time.Sleep(50 * time.Millisecond)

	running := mgr1.ListRunning()
	found := map[string]bool{}
	for _, id := range running {
		found[id] = true
	}
	if !found[id1] {
		t.Errorf("expected %s in running list", id1)
	}
	if !found[id2] {
		t.Errorf("expected %s in running list", id2)
	}
}

// blockingProvider blocks until its context is cancelled.
type blockingProvider struct {
	ready chan struct{}
	once  sync.Once
}

func (b *blockingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	b.once.Do(func() { close(b.ready) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// drainInbound returns a channel that receives the next inbound message from the bus.
func drainInbound(mb *bus.MessageBus) <-chan bus.InboundMessage {
	ch := make(chan bus.InboundMessage, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := mb.ConsumeInbound(ctx)
		if err == nil {
			ch <- msg
		}
	}()
	return ch
}
