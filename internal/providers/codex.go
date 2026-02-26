package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const codexResponsesAPI = "https://api.openai.com/v1/responses"
const codexTokenRefreshURL = "https://auth.openai.com/oauth/token"

// CodexProvider implements Provider using OpenAI's Responses API with OAuth.
type CodexProvider struct {
	auth       codexAuth
	httpClient *http.Client
}

type codexAuth struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // Unix timestamp
}

// NewCodexProvider reads ~/.codex/auth.json and returns a CodexProvider.
func NewCodexProvider() (*CodexProvider, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}
	authPath := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("codex auth.json not found at %s: %w", authPath, err)
	}
	var auth codexAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("failed to parse codex auth.json: %w", err)
	}
	return &CodexProvider{
		auth:       auth,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *CodexProvider) accessToken(ctx context.Context) (string, error) {
	if time.Now().Unix() < p.auth.ExpiresAt-60 {
		return p.auth.AccessToken, nil
	}
	// Refresh the token
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": p.auth.RefreshToken,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenRefreshURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token refresh failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh returned status %d", resp.StatusCode)
	}
	var refreshed codexAuth
	if err := json.NewDecoder(resp.Body).Decode(&refreshed); err != nil {
		return "", fmt.Errorf("failed to decode refresh response: %w", err)
	}
	p.auth = refreshed
	return p.auth.AccessToken, nil
}

// Chat implements Provider.
func (p *CodexProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("codex: failed to get access token: %w", err)
	}

	payload := buildCodexRequest(req)
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("codex: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codexResponsesAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("codex: failed to build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("codex: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex: API returned status %d", httpResp.StatusCode)
	}

	return parseCodexSSE(httpResp.Body)
}

// --- request building ---

type codexRequest struct {
	Model        string           `json:"model"`
	Instructions string           `json:"instructions,omitempty"`
	Input        []codexInputItem `json:"input"`
	Tools        []codexTool      `json:"tools,omitempty"`
	Stream       bool             `json:"stream"`
}

type codexInputItem struct {
	Type       string            `json:"type"`
	Role       string            `json:"role,omitempty"`
	Content    string            `json:"content,omitempty"`
	CallID     string            `json:"call_id,omitempty"`
	Name       string            `json:"name,omitempty"`
	Arguments  string            `json:"arguments,omitempty"`
	Output     string            `json:"output,omitempty"`
}

type codexTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func buildCodexRequest(req ChatRequest) codexRequest {
	var items []codexInputItem
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			// system messages become instructions; skip here
		case "user":
			items = append(items, codexInputItem{Type: "message", Role: "user", Content: m.Content})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				if m.Content != "" {
					items = append(items, codexInputItem{Type: "message", Role: "assistant", Content: m.Content})
				}
				for _, tc := range m.ToolCalls {
					items = append(items, codexInputItem{
						Type:      "function_call",
						CallID:    tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
					})
				}
			} else {
				items = append(items, codexInputItem{Type: "message", Role: "assistant", Content: m.Content})
			}
		case "tool":
			items = append(items, codexInputItem{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: m.Content,
			})
		}
	}

	var tools []codexTool
	for _, t := range req.Tools {
		tools = append(tools, codexTool{
			Type:        "function",
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}

	// Extract system prompt from messages if not set directly
	instructions := req.SystemPrompt
	if instructions == "" {
		for _, m := range req.Messages {
			if m.Role == "system" {
				instructions = m.Content
				break
			}
		}
	}

	return codexRequest{
		Model:        req.Model,
		Instructions: instructions,
		Input:        items,
		Tools:        tools,
		Stream:       true,
	}
}

// --- SSE parsing ---

type codexSSEEvent struct {
	Type string          `json:"type"`
	Item json.RawMessage `json:"item,omitempty"`
	// for response.completed
	Response *codexResponseBody `json:"response,omitempty"`
}

type codexResponseBody struct {
	Usage *codexUsage `json:"usage,omitempty"`
}

type codexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type codexOutputItem struct {
	Type    string             `json:"type"`
	Content []codexContentPart `json:"content,omitempty"`
	// for function_call
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`
}

type codexContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func parseCodexSSE(body interface{ Read([]byte) (int, error) }) (*ChatResponse, error) {
	var textParts []string
	var toolCalls []ToolCall
	var usage Usage

	scanner := bufio.NewScanner(body)
	var dataLine string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			continue
		}
		if line == "" && dataLine != "" {
			if dataLine == "[DONE]" {
				dataLine = ""
				continue
			}
			var ev codexSSEEvent
			if err := json.Unmarshal([]byte(dataLine), &ev); err != nil {
				dataLine = ""
				continue
			}
			switch ev.Type {
			case "response.output_item.done":
				var item codexOutputItem
				if err := json.Unmarshal(ev.Item, &item); err == nil {
					switch item.Type {
					case "message":
						for _, part := range item.Content {
							if part.Type == "output_text" || part.Type == "text" {
								textParts = append(textParts, part.Text)
							}
						}
					case "function_call":
						toolCalls = append(toolCalls, ToolCall{
							ID:        item.CallID,
							Name:      item.Name,
							Arguments: item.Arguments,
						})
					}
				}
			case "response.completed":
				if ev.Response != nil && ev.Response.Usage != nil {
					u := ev.Response.Usage
					usage = Usage{
						PromptTokens:     u.InputTokens,
						CompletionTokens: u.OutputTokens,
						TotalTokens:      u.TotalTokens,
					}
				}
			}
			dataLine = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("codex: SSE read error: %w", err)
	}

	stopReason := "stop"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	}

	return &ChatResponse{
		Content:    strings.Join(textParts, ""),
		ToolCalls:  toolCalls,
		Usage:      usage,
		StopReason: stopReason,
	}, nil
}
