package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func TestSendMessageTool_PublishesToBus(t *testing.T) {
	msgBus := bus.NewMessageBus(10)

	received := make(chan bus.OutboundMessage, 1)
	msgBus.Subscribe("telegram", func(msg bus.OutboundMessage) {
		received <- msg
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go msgBus.DispatchOutbound(ctx)

	tool := NewSendMessageTool(msgBus)
	params, _ := json.Marshal(map[string]any{
		"channel": "telegram",
		"chat_id": "123",
		"content": "hello there",
	})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Message sent") {
		t.Errorf("unexpected result: %s", result)
	}

	select {
	case msg := <-received:
		if msg.Content != "hello there" {
			t.Errorf("Content = %q, want %q", msg.Content, "hello there")
		}
		if msg.Channel != "telegram" {
			t.Errorf("Channel = %q, want telegram", msg.Channel)
		}
		if msg.ChatID != "123" {
			t.Errorf("ChatID = %q, want 123", msg.ChatID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message dispatch")
	}
}

func TestSendMessageTool_MissingFields(t *testing.T) {
	msgBus := bus.NewMessageBus(10)
	tool := NewSendMessageTool(msgBus)

	tests := []struct {
		name   string
		params map[string]any
	}{
		{"missing channel", map[string]any{"chat_id": "1", "content": "hi"}},
		{"missing chat_id", map[string]any{"channel": "tg", "content": "hi"}},
		{"missing content", map[string]any{"channel": "tg", "chat_id": "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(tt.params)
			_, err := tool.Execute(context.Background(), params)
			if err == nil {
				t.Fatal("expected error for missing required field")
			}
		})
	}
}

func TestSendMessageTool_InvalidParams(t *testing.T) {
	msgBus := bus.NewMessageBus(10)
	tool := NewSendMessageTool(msgBus)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestSendMessageTool_Name(t *testing.T) {
	msgBus := bus.NewMessageBus(10)
	tool := NewSendMessageTool(msgBus)
	if tool.Name() != "send_message" {
		t.Errorf("Name() = %q, want send_message", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Parameters()) == 0 {
		t.Error("Parameters() is empty")
	}
}
