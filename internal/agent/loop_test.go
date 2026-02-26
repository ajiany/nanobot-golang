package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
	"github.com/coopco/nanobot/internal/session"
	"github.com/coopco/nanobot/internal/tools"
)

// mockProvider replays a fixed sequence of ChatResponse values.
type mockProvider struct {
	responses []*providers.ChatResponse
	callIndex int
}

func (m *mockProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.callIndex >= len(m.responses) {
		return &providers.ChatResponse{Content: "no more responses"}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

// echoTool echoes its "text" parameter back.
type echoTool struct{}

func (t *echoTool) Name() string        { return "echo" }
func (t *echoTool) Description() string { return "Echoes input" }
func (t *echoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)
}
func (t *echoTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Text string `json:"text"`
	}
	json.Unmarshal(params, &p) //nolint:errcheck
	return "echo: " + p.Text, nil
}

// newTestLoop builds an AgentLoop wired to a temp session dir.
func newTestLoop(t *testing.T, provider providers.Provider, maxIter int) *AgentLoop {
	t.Helper()
	reg := tools.NewRegistry()
	reg.Register(&echoTool{})

	mgr := session.NewManager(t.TempDir())
	mb := bus.NewMessageBus(10)

	return NewAgentLoop(AgentLoopConfig{
		Bus:           mb,
		Provider:      provider,
		Sessions:      mgr,
		Tools:         reg,
		Model:         "test-model",
		MaxTokens:     1024,
		Temperature:   0,
		MaxIterations: maxIter,
		SystemPrompt:  "",
	})
}

func TestProcessDirect_SimpleResponse(t *testing.T) {
	mock := &mockProvider{
		responses: []*providers.ChatResponse{
			{Content: "Hello!", StopReason: "stop"},
		},
	}
	loop := newTestLoop(t, mock, 10)

	got, err := loop.ProcessDirect(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello!" {
		t.Errorf("expected %q, got %q", "Hello!", got)
	}
}

func TestProcessDirect_WithToolCall(t *testing.T) {
	mock := &mockProvider{
		responses: []*providers.ChatResponse{
			{
				Content: "",
				ToolCalls: []providers.ToolCall{
					{ID: "tc1", Name: "echo", Arguments: `{"text":"world"}`},
				},
				StopReason: "tool_use",
			},
			{Content: "done", StopReason: "stop"},
		},
	}
	loop := newTestLoop(t, mock, 10)

	got, err := loop.ProcessDirect(context.Background(), "use echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "done" {
		t.Errorf("expected %q, got %q", "done", got)
	}
	if mock.callIndex != 2 {
		t.Errorf("expected 2 provider calls, got %d", mock.callIndex)
	}
}

func TestProcessDirect_MaxIterations(t *testing.T) {
	// Provider always returns a tool call — loop must stop at maxIter.
	infiniteResp := &providers.ChatResponse{
		Content: "thinking",
		ToolCalls: []providers.ToolCall{
			{ID: "tc1", Name: "echo", Arguments: `{"text":"loop"}`},
		},
		StopReason: "tool_use",
	}
	mock := &mockProvider{}
	for i := 0; i < 50; i++ {
		mock.responses = append(mock.responses, infiniteResp)
	}

	loop := newTestLoop(t, mock, 5)

	// Should not hang or error — just return the last assistant content.
	got, err := loop.ProcessDirect(context.Background(), "loop forever")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After maxIter the loop returns the last assistant content ("thinking").
	if got != "thinking" {
		t.Errorf("expected %q after max iterations, got %q", "thinking", got)
	}
	if mock.callIndex != 5 {
		t.Errorf("expected exactly 5 provider calls (maxIter), got %d", mock.callIndex)
	}
}

func TestRun_ProcessesMessages(t *testing.T) {
	mock := &mockProvider{
		responses: []*providers.ChatResponse{
			{Content: "pong", StopReason: "stop"},
		},
	}

	reg := tools.NewRegistry()
	mgr := session.NewManager(t.TempDir())
	mb := bus.NewMessageBus(10)

	loop := NewAgentLoop(AgentLoopConfig{
		Bus:           mb,
		Provider:      mock,
		Sessions:      mgr,
		Tools:         reg,
		Model:         "test-model",
		MaxTokens:     1024,
		MaxIterations: 10,
	})

	received := make(chan bus.OutboundMessage, 1)
	mb.Subscribe("test", func(msg bus.OutboundMessage) {
		received <- msg
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mb.DispatchOutbound(ctx)
	go loop.Run(ctx) //nolint:errcheck

	mb.PublishInbound(bus.InboundMessage{
		Channel: "test",
		ChatID:  "chat1",
		Content: "ping",
	})

	select {
	case msg := <-received:
		if msg.Content != "pong" {
			t.Errorf("expected %q, got %q", "pong", msg.Content)
		}
		if msg.Type != "text" {
			t.Errorf("expected type %q, got %q", "text", msg.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for outbound message")
	}
}
