package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	defaultMaxTokens      = 4096
)

type AnthropicProvider struct {
	client       *anthropic.Client
	defaultModel string
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{
		client:       &client,
		defaultModel: defaultAnthropicModel,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	messages, err := convertMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}

	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat failed: %w", err)
	}

	return convertResponse(resp), nil
}

func convertMessages(msgs []Message) ([]anthropic.MessageParam, error) {
	var out []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case "user":
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				if m.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(m.Content))
				}
				for _, tc := range m.ToolCalls {
					var input any
					if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
						input = tc.Arguments
					}
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
				}
				out = append(out, anthropic.NewAssistantMessage(blocks...))
			} else {
				out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
			}
		case "tool":
			out = append(out, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		}
	}
	return out, nil
}

func convertTools(tools []ToolDef) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		var schema anthropic.ToolInputSchemaParam
		if err := json.Unmarshal(t.Function.Parameters, &schema); err != nil {
			schema = anthropic.ToolInputSchemaParam{}
		}
		out[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Function.Name,
				Description: anthropic.String(t.Function.Description),
				InputSchema: schema,
			},
		}
	}
	return out
}

func convertResponse(resp *anthropic.Message) *ChatResponse {
	var text string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			text += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(args),
			})
		}
	}

	return &ChatResponse{
		Content:   text,
		ToolCalls: toolCalls,
		StopReason: string(resp.StopReason),
		Usage: Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}
