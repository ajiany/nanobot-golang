package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
	"github.com/coopco/nanobot/internal/session"
	"github.com/coopco/nanobot/internal/tools"
)

// AgentLoop consumes inbound messages, calls the LLM, executes tool calls, and publishes responses.
type AgentLoop struct {
	bus          *bus.MessageBus
	provider     providers.Provider
	sessions     *session.Manager
	tools        *tools.Registry
	model        string
	maxTokens    int
	temperature  float64
	maxIter      int
	systemPrompt string
	mu           sync.Mutex
}

// AgentLoopConfig holds all dependencies and settings for AgentLoop.
type AgentLoopConfig struct {
	Bus           *bus.MessageBus
	Provider      providers.Provider
	Sessions      *session.Manager
	Tools         *tools.Registry
	Model         string
	MaxTokens     int
	Temperature   float64
	MaxIterations int
	SystemPrompt  string
}

// NewAgentLoop creates an AgentLoop from the given config.
func NewAgentLoop(cfg AgentLoopConfig) *AgentLoop {
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 40
	}
	return &AgentLoop{
		bus:          cfg.Bus,
		provider:     cfg.Provider,
		sessions:     cfg.Sessions,
		tools:        cfg.Tools,
		model:        cfg.Model,
		maxTokens:    cfg.MaxTokens,
		temperature:  cfg.Temperature,
		maxIter:      maxIter,
		systemPrompt: cfg.SystemPrompt,
	}
}

// Run consumes inbound messages from the bus and processes each in a goroutine.
// Returns when ctx is cancelled.
func (a *AgentLoop) Run(ctx context.Context) error {
	for {
		msg, err := a.bus.ConsumeInbound(ctx)
		if err != nil {
			return err
		}
		go a.processMessage(ctx, msg)
	}
}

// processMessage handles a single inbound message: builds context, runs the tool loop,
// saves the session, and publishes the outbound response.
func (a *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) {
	sess := a.sessions.GetOrCreate(msg.SessionKey())

	messages := sessionToProviderMessages(sess.GetHistory())
	messages = append(messages, providers.Message{Role: "user", Content: msg.Content})

	finalContent, err := a.runToolLoop(ctx, messages)
	if err != nil {
		slog.Error("agent tool loop error", "session", msg.SessionKey(), "err", err)
		a.bus.PublishOutbound(bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: fmt.Sprintf("Error: %v", err),
			Type:    "error",
		})
		return
	}

	sess.AppendMessage(session.Message{Role: "user", Content: msg.Content})
	sess.AppendMessage(session.Message{Role: "assistant", Content: finalContent})
	if err := a.sessions.Save(sess); err != nil {
		slog.Error("failed to save session", "session", msg.SessionKey(), "err", err)
	}

	a.bus.PublishOutbound(bus.OutboundMessage{
		Channel: msg.Channel,
		ChatID:  msg.ChatID,
		Content: finalContent,
		Type:    "text",
	})
}

// ProcessDirect processes a single message without the bus, for CLI mode.
func (a *AgentLoop) ProcessDirect(ctx context.Context, message string) (string, error) {
	sess := a.sessions.GetOrCreate("direct")

	messages := sessionToProviderMessages(sess.GetHistory())
	messages = append(messages, providers.Message{Role: "user", Content: message})

	finalContent, err := a.runToolLoop(ctx, messages)
	if err != nil {
		return "", err
	}

	sess.AppendMessage(session.Message{Role: "user", Content: message})
	sess.AppendMessage(session.Message{Role: "assistant", Content: finalContent})
	if err := a.sessions.Save(sess); err != nil {
		slog.Error("failed to save direct session", "err", err)
	}

	return finalContent, nil
}

// runToolLoop executes the LLM + tool call loop and returns the final text response.
func (a *AgentLoop) runToolLoop(ctx context.Context, messages []providers.Message) (string, error) {
	toolDefs := toolDefsToProviderTools(a.tools.Definitions())

	for i := 0; i < a.maxIter; i++ {
		req := providers.ChatRequest{
			Model:        a.model,
			Messages:     messages,
			Tools:        toolDefs,
			MaxTokens:    a.maxTokens,
			Temperature:  a.temperature,
			SystemPrompt: a.systemPrompt,
		}

		resp, err := a.provider.Chat(ctx, req)
		if err != nil {
			return "", fmt.Errorf("provider chat error: %w", err)
		}

		// Build assistant message with any tool calls
		assistantMsg := providers.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		// Execute each tool call and append results
		for _, tc := range resp.ToolCalls {
			slog.Debug("executing tool", "name", tc.Name, "id", tc.ID)
			result := a.tools.Execute(ctx, tc.Name, json.RawMessage(tc.Arguments))
			messages = append(messages, providers.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Exceeded maxIter — return whatever the last assistant content was
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].Content, nil
		}
	}
	return "", fmt.Errorf("max iterations (%d) reached without a final response", a.maxIter)
}

// sessionToProviderMessages converts session history to provider message format.
func sessionToProviderMessages(history []session.Message) []providers.Message {
	msgs := make([]providers.Message, 0, len(history))
	for _, m := range history {
		pm := providers.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		if len(m.ToolCalls) > 0 {
			pm.ToolCalls = make([]providers.ToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				pm.ToolCalls[i] = providers.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		}
		msgs = append(msgs, pm)
	}
	return msgs
}

// toolDefsToProviderTools converts tool registry definitions to provider tool format.
func toolDefsToProviderTools(defs []tools.ToolDefinition) []providers.ToolDef {
	result := make([]providers.ToolDef, len(defs))
	for i, d := range defs {
		result[i] = providers.ToolDef{
			Type: d.Type,
			Function: providers.FunctionDef{
				Name:        d.Function.Name,
				Description: d.Function.Description,
				Parameters:  d.Function.Parameters,
			},
		}
	}
	return result
}
