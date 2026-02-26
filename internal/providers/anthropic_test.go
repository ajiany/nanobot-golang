package providers

import (
	"encoding/json"
	"testing"
)

func TestConvertMessages_User(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hello"}}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_Assistant(t *testing.T) {
	msgs := []Message{{Role: "assistant", Content: "hi there"}}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_AssistantWithToolCalls(t *testing.T) {
	msgs := []Message{
		{
			Role:    "assistant",
			Content: "thinking",
			ToolCalls: []ToolCall{
				{ID: "tc1", Name: "my_tool", Arguments: `{"key":"val"}`},
			},
		},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_AssistantWithToolCallsNoContent(t *testing.T) {
	msgs := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "tc2", Name: "tool2", Arguments: `{}`},
			},
		},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_ToolResult(t *testing.T) {
	msgs := []Message{
		{Role: "tool", Content: "result text", ToolCallID: "tc1"},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_InvalidToolCallArgs(t *testing.T) {
	// Invalid JSON in arguments should fall back gracefully
	msgs := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "tc3", Name: "tool3", Arguments: `not-json`},
			},
		},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestConvertMessages_Mixed(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "question"},
		{Role: "assistant", Content: "answer"},
		{Role: "tool", Content: "tool result", ToolCallID: "id1"},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
}

func TestConvertTools(t *testing.T) {
	tools := []ToolDef{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "my_func",
				Description: "does something",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
	}
	out := convertTools(tools)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	if out[0].OfTool == nil {
		t.Fatal("expected OfTool to be set")
	}
	if out[0].OfTool.Name != "my_func" {
		t.Errorf("Name = %q, want %q", out[0].OfTool.Name, "my_func")
	}
}

func TestConvertTools_InvalidSchema(t *testing.T) {
	tools := []ToolDef{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "bad_schema",
				Description: "bad",
				Parameters:  json.RawMessage(`not-valid-json`),
			},
		},
	}
	// Should not panic, falls back to empty schema
	out := convertTools(tools)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
}

func TestConvertTools_Empty(t *testing.T) {
	out := convertTools(nil)
	if len(out) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(out))
	}
}

func TestNewAnthropicProvider(t *testing.T) {
	p := NewAnthropicProvider("test-api-key")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.defaultModel != defaultAnthropicModel {
		t.Errorf("defaultModel = %q, want %q", p.defaultModel, defaultAnthropicModel)
	}
}
