package providers

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAICompatProvider works with OpenAI and any OpenAI-compatible API.
type OpenAICompatProvider struct {
	client       *openai.Client
	defaultModel string
	modelPrefix  string
	skipPrefixes []string
}

// NewOpenAICompatProvider creates a provider with an explicit base URL.
func NewOpenAICompatProvider(apiKey, baseURL, defaultModel string) *OpenAICompatProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAICompatProvider{
		client:       openai.NewClientWithConfig(cfg),
		defaultModel: defaultModel,
	}
}

// NewOpenAICompatProviderFromSpec creates a provider using a ProviderSpec.
func NewOpenAICompatProviderFromSpec(spec *ProviderSpec, apiKey, baseURL string) *OpenAICompatProvider {
	base := baseURL
	if base == "" {
		base = spec.DefaultAPIBase
	}
	p := NewOpenAICompatProvider(apiKey, base, "")
	p.modelPrefix = spec.ModelPrefix
	p.skipPrefixes = spec.SkipPrefixes
	return p
}

// resolveModel applies the model prefix if needed.
func (p *OpenAICompatProvider) resolveModel(model string) string {
	if p.modelPrefix == "" {
		return model
	}
	for _, skip := range p.skipPrefixes {
		if strings.HasPrefix(model, skip) {
			return model
		}
	}
	return p.modelPrefix + model
}

// Chat sends a chat completion request and returns the response.
func (p *OpenAICompatProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	model = p.resolveModel(model)

	var msgs []openai.ChatCompletionMessage

	// Prepend system prompt if provided
	if req.SystemPrompt != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.SystemPrompt,
		})
	}

	for _, m := range req.Messages {
		msg := openai.ChatCompletionMessage{
			Role: m.Role,
		}
		if len(m.ContentParts) > 0 {
			// Multimodal message — build multi-part content.
			for _, p := range m.ContentParts {
				switch p.Type {
				case "text":
					msg.MultiContent = append(msg.MultiContent, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: p.Text,
					})
				case "image_url":
					if p.ImageURL != nil {
						msg.MultiContent = append(msg.MultiContent, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    p.ImageURL.URL,
								Detail: openai.ImageURLDetail(p.ImageURL.Detail),
							},
						})
					}
				}
			}
			// Prepend text content as a text part if both are set.
			if m.Content != "" {
				msg.MultiContent = append([]openai.ChatMessagePart{{
					Type: openai.ChatMessagePartTypeText,
					Text: m.Content,
				}}, msg.MultiContent...)
			}
		} else {
			msg.Content = m.Content
			// Some providers reject empty string content
			if msg.Content == "" {
				msg.Content = " "
			}
		}
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		msgs = append(msgs, msg)
	}

	oaiReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		oaiReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature != 0 {
		oaiReq.Temperature = float32(req.Temperature)
	}

	for _, t := range req.Tools {
		oaiReq.Tools = append(oaiReq.Tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	resp, err := p.client.CreateChatCompletion(ctx, oaiReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	out := &ChatResponse{
		Content:    choice.Message.Content,
		StopReason: string(choice.FinishReason),
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return out, nil
}
