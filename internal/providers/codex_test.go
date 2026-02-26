package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildCodexRequest_UserMessage(t *testing.T) {
	req := ChatRequest{
		Model:    "codex-mini",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}
	cr := buildCodexRequest(req)
	if cr.Model != "codex-mini" {
		t.Errorf("Model = %q, want %q", cr.Model, "codex-mini")
	}
	if !cr.Stream {
		t.Error("expected Stream = true")
	}
	if len(cr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(cr.Input))
	}
	if cr.Input[0].Role != "user" || cr.Input[0].Content != "hello" {
		t.Errorf("unexpected input item: %+v", cr.Input[0])
	}
}

func TestBuildCodexRequest_SystemPromptExtracted(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "user", Content: "hi"},
		},
	}
	cr := buildCodexRequest(req)
	if cr.Instructions != "you are helpful" {
		t.Errorf("Instructions = %q, want %q", cr.Instructions, "you are helpful")
	}
	// system message should not appear in Input
	for _, item := range cr.Input {
		if item.Role == "system" {
			t.Error("system message should not be in Input items")
		}
	}
}

func TestBuildCodexRequest_SystemPromptDirect(t *testing.T) {
	req := ChatRequest{
		SystemPrompt: "direct system",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	}
	cr := buildCodexRequest(req)
	if cr.Instructions != "direct system" {
		t.Errorf("Instructions = %q, want %q", cr.Instructions, "direct system")
	}
}

func TestBuildCodexRequest_AssistantWithToolCalls(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{
				Role:    "assistant",
				Content: "thinking",
				ToolCalls: []ToolCall{
					{ID: "call1", Name: "my_tool", Arguments: `{"x":1}`},
				},
			},
		},
	}
	cr := buildCodexRequest(req)
	// content message + function_call item
	if len(cr.Input) != 2 {
		t.Fatalf("expected 2 input items, got %d: %+v", len(cr.Input), cr.Input)
	}
	if cr.Input[0].Type != "message" {
		t.Errorf("first item type = %q, want message", cr.Input[0].Type)
	}
	if cr.Input[1].Type != "function_call" {
		t.Errorf("second item type = %q, want function_call", cr.Input[1].Type)
	}
	if cr.Input[1].CallID != "call1" {
		t.Errorf("CallID = %q, want call1", cr.Input[1].CallID)
	}
}

func TestBuildCodexRequest_AssistantNoContent(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call2", Name: "tool2", Arguments: `{}`},
				},
			},
		},
	}
	cr := buildCodexRequest(req)
	// only function_call, no message item since content is empty
	if len(cr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(cr.Input))
	}
	if cr.Input[0].Type != "function_call" {
		t.Errorf("item type = %q, want function_call", cr.Input[0].Type)
	}
}

func TestBuildCodexRequest_ToolResult(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "tool", Content: "result", ToolCallID: "call1"},
		},
	}
	cr := buildCodexRequest(req)
	if len(cr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(cr.Input))
	}
	if cr.Input[0].Type != "function_call_output" {
		t.Errorf("type = %q, want function_call_output", cr.Input[0].Type)
	}
	if cr.Input[0].Output != "result" {
		t.Errorf("Output = %q, want result", cr.Input[0].Output)
	}
}

func TestBuildCodexRequest_Tools(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools: []ToolDef{
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "my_func",
					Description: "does stuff",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}
	cr := buildCodexRequest(req)
	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Name != "my_func" {
		t.Errorf("tool name = %q, want my_func", cr.Tools[0].Name)
	}
}

func TestParseCodexSSE_TextOutput(t *testing.T) {
	sse := buildSSE(
		`{"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":"hello world"}]}}`,
		`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		"[DONE]",
	)
	resp, err := parseCodexSSE(strings.NewReader(sse))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello world")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", resp.Usage.CompletionTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("StopReason = %q, want stop", resp.StopReason)
	}
}

func TestParseCodexSSE_FunctionCall(t *testing.T) {
	sse := buildSSE(
		`{"type":"response.output_item.done","item":{"type":"function_call","name":"my_tool","arguments":"{\"x\":1}","call_id":"call1"}}`,
		"[DONE]",
	)
	resp, err := parseCodexSSE(strings.NewReader(sse))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "my_tool" {
		t.Errorf("Name = %q, want my_tool", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "call1" {
		t.Errorf("ID = %q, want call1", resp.ToolCalls[0].ID)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
}

func TestParseCodexSSE_Empty(t *testing.T) {
	resp, err := parseCodexSSE(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestParseCodexSSE_InvalidJSON(t *testing.T) {
	// Invalid JSON events should be skipped gracefully
	sse := buildSSE(`not-valid-json`, "[DONE]")
	resp, err := parseCodexSSE(strings.NewReader(sse))
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestParseCodexSSE_TextTypeText(t *testing.T) {
	// Also handles "text" type in content parts
	sse := buildSSE(
		`{"type":"response.output_item.done","item":{"type":"message","content":[{"type":"text","text":"alt text"}]}}`,
		"[DONE]",
	)
	resp, err := parseCodexSSE(strings.NewReader(sse))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "alt text" {
		t.Errorf("Content = %q, want %q", resp.Content, "alt text")
	}
}

// buildSSE formats SSE events as a stream string.
func buildSSE(events ...string) string {
	var sb strings.Builder
	for _, ev := range events {
		sb.WriteString("data: ")
		sb.WriteString(ev)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
