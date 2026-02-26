package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coopco/nanobot/internal/bus"
)

type SendMessageTool struct {
	bus *bus.MessageBus
}

func NewSendMessageTool(msgBus *bus.MessageBus) *SendMessageTool {
	return &SendMessageTool{bus: msgBus}
}

func (t *SendMessageTool) Name() string        { return "send_message" }
func (t *SendMessageTool) Description() string { return "Send a message to a specific channel and chat" }
func (t *SendMessageTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel": {"type": "string", "description": "Target channel name"},
			"chat_id": {"type": "string", "description": "Target chat ID"},
			"content": {"type": "string", "description": "Message content"}
		},
		"required": ["channel", "chat_id", "content"]
	}`)
}

func (t *SendMessageTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Channel string `json:"channel"`
		ChatID  string `json:"chat_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Channel == "" || p.ChatID == "" || p.Content == "" {
		return "", fmt.Errorf("channel, chat_id, and content are required")
	}

	msg := bus.OutboundMessage{
		Channel: p.Channel,
		ChatID:  p.ChatID,
		Content: p.Content,
		Type:    "text",
	}

	t.bus.PublishOutbound(msg)
	return "Message sent", nil
}
