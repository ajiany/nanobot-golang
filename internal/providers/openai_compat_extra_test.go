package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockOpenAIServer creates a test server that returns a valid ChatCompletion response.
func mockOpenAIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func defaultChatHandler(content string, toolCalls []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg := map[string]any{
			"role":    "assistant",
			"content": content,
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "gpt-4o",
			"choices": []map[string]any{{
				"index":         0,
				"message":       msg,
				"finish_reason": "stop",
			}},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func TestOpenAIChat_BasicResponse(t *testing.T) {
	srv := mockOpenAIServer(t, defaultChatHandler("Hello!", nil))
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello!")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestOpenAIChat_DefaultModel(t *testing.T) {
	var receivedModel string
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receivedModel, _ = body["model"].(string)
		defaultChatHandler("ok", nil)(w, r)
	})
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "my-default-model")
	_, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedModel != "my-default-model" {
		t.Errorf("model = %q, want %q", receivedModel, "my-default-model")
	}
}

func TestOpenAIChat_WithSystemPrompt(t *testing.T) {
	var receivedMessages []map[string]any
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msgs, _ := body["messages"].([]any)
		for _, m := range msgs {
			receivedMessages = append(receivedMessages, m.(map[string]any))
		}
		defaultChatHandler("ok", nil)(w, r)
	})
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	_, err := p.Chat(context.Background(), ChatRequest{
		SystemPrompt: "You are helpful",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedMessages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(receivedMessages))
	}
	if receivedMessages[0]["role"] != "system" {
		t.Errorf("first message role = %q, want system", receivedMessages[0]["role"])
	}
}

func TestOpenAIChat_WithTools(t *testing.T) {
	toolCalls := []map[string]any{{
		"id":   "call_1",
		"type": "function",
		"function": map[string]any{
			"name":      "my_tool",
			"arguments": `{"x":1}`,
		},
	}}
	srv := mockOpenAIServer(t, defaultChatHandler("", toolCalls))
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "use tool"}},
		Tools: []ToolDef{{
			Type: "function",
			Function: FunctionDef{
				Name:        "my_tool",
				Description: "does stuff",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "my_tool" {
		t.Errorf("tool call name = %q, want my_tool", resp.ToolCalls[0].Name)
	}
}

func TestOpenAIChat_ErrorResponse(t *testing.T) {
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	})
	defer srv.Close()

	p := NewOpenAICompatProvider("bad-key", srv.URL, "gpt-4o")
	_, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestOpenAIChat_NoChoices(t *testing.T) {
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "gpt-4o",
			"choices": []map[string]any{},
			"usage":   map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	_, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestOpenAIChat_WithMaxTokensAndTemp(t *testing.T) {
	var receivedBody map[string]any
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		defaultChatHandler("ok", nil)(w, r)
	})
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	_, err := p.Chat(context.Background(), ChatRequest{
		Messages:    []Message{{Role: "user", Content: "hi"}},
		MaxTokens:   2048,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mt, ok := receivedBody["max_tokens"].(float64); !ok || int(mt) != 2048 {
		t.Errorf("max_tokens = %v, want 2048", receivedBody["max_tokens"])
	}
}

func TestOpenAIChat_MultimodalContentParts(t *testing.T) {
	srv := mockOpenAIServer(t, defaultChatHandler("I see an image", nil))
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{
			Role:    "user",
			Content: "What's in this image?",
			ContentParts: []ContentPart{
				{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/img.png", Detail: "auto"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I see an image" {
		t.Errorf("Content = %q, want %q", resp.Content, "I see an image")
	}
}

func TestOpenAIChat_ToolCallIDAndToolRole(t *testing.T) {
	srv := mockOpenAIServer(t, defaultChatHandler("final answer", nil))
	defer srv.Close()

	p := NewOpenAICompatProvider("test-key", srv.URL, "gpt-4o")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "use tool"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "tc1", Name: "bash", Arguments: `{}`}}},
			{Role: "tool", Content: "tool result", ToolCallID: "tc1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "final answer" {
		t.Errorf("Content = %q, want %q", resp.Content, "final answer")
	}
}
