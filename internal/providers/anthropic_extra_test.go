package providers

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestConvertResponse_TextOnly(t *testing.T) {
	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "Hello world"},
		},
		StopReason: "end_turn",
		Usage:      anthropic.Usage{InputTokens: 10, OutputTokens: 5},
	}
	resp := convertResponse(msg)
	if resp.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello world")
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", resp.StopReason)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestConvertResponse_ToolUse(t *testing.T) {
	input, _ := json.Marshal(map[string]string{"key": "val"})
	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "tool_use", ID: "tu_1", Name: "my_tool", Input: input},
		},
		StopReason: "tool_use",
		Usage:      anthropic.Usage{InputTokens: 20, OutputTokens: 10},
	}
	resp := convertResponse(msg)
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "tu_1" {
		t.Errorf("ID = %q, want tu_1", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "my_tool" {
		t.Errorf("Name = %q, want my_tool", resp.ToolCalls[0].Name)
	}
}

func TestConvertResponse_Mixed(t *testing.T) {
	input, _ := json.Marshal(map[string]int{"x": 1})
	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "Let me use a tool. "},
			{Type: "tool_use", ID: "tu_2", Name: "calc", Input: input},
			{Type: "text", Text: "Done."},
		},
		StopReason: "tool_use",
		Usage:      anthropic.Usage{InputTokens: 30, OutputTokens: 15},
	}
	resp := convertResponse(msg)
	if resp.Content != "Let me use a tool. Done." {
		t.Errorf("Content = %q, want concatenated text", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
}

func TestConvertResponse_Empty(t *testing.T) {
	msg := &anthropic.Message{
		Content:    []anthropic.ContentBlockUnion{},
		StopReason: "end_turn",
		Usage:      anthropic.Usage{},
	}
	resp := convertResponse(msg)
	if resp.Content != "" {
		t.Errorf("Content = %q, want empty", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestConvertMessages_AllRoles(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "question"},
		{Role: "assistant", Content: "answer"},
		{Role: "assistant", Content: "with tool", ToolCalls: []ToolCall{
			{ID: "tc1", Name: "tool1", Arguments: `{"a":"b"}`},
		}},
		{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "tc2", Name: "tool2", Arguments: `{}`},
		}},
		{Role: "tool", Content: "result", ToolCallID: "tc1"},
	}
	out, err := convertMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(out))
	}
}

func TestConvertTools_Multiple(t *testing.T) {
	tools := []ToolDef{
		{Type: "function", Function: FunctionDef{Name: "a", Description: "desc a", Parameters: json.RawMessage(`{"type":"object"}`)}},
		{Type: "function", Function: FunctionDef{Name: "b", Description: "desc b", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}
	out := convertTools(tools)
	if len(out) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(out))
	}
	if out[0].OfTool.Name != "a" || out[1].OfTool.Name != "b" {
		t.Errorf("unexpected tool names: %q, %q", out[0].OfTool.Name, out[1].OfTool.Name)
	}
}
