package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
	"github.com/coopco/nanobot/internal/tools"
)

// SubagentManager manages background task agents.
type SubagentManager struct {
	provider    providers.Provider
	model       string
	maxTokens   int
	temperature float64
	bus         *bus.MessageBus
	mu          sync.Mutex
	running     map[string]context.CancelFunc
	counter     int
}

// NewSubagentManager creates a new SubagentManager.
func NewSubagentManager(provider providers.Provider, model string, maxTokens int, temperature float64, msgBus *bus.MessageBus) *SubagentManager {
	return &SubagentManager{
		provider:    provider,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		bus:         msgBus,
		running:     make(map[string]context.CancelFunc),
	}
}

// Spawn starts a background subagent goroutine. Returns a task ID.
func (m *SubagentManager) Spawn(ctx context.Context, task, label, originChannel, originChatID string) string {
	m.mu.Lock()
	taskID := fmt.Sprintf("task_%d", m.counter)
	m.counter++
	childCtx, cancel := context.WithCancel(ctx)
	m.running[taskID] = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.running, taskID)
			m.mu.Unlock()
		}()

		isolatedTools := tools.NewRegistry()
		isolatedTools.Register(tools.NewReadFileTool())
		isolatedTools.Register(tools.NewWriteFileTool())
		isolatedTools.Register(tools.NewEditFileTool())
		isolatedTools.Register(tools.NewListDirTool())
		isolatedTools.Register(tools.NewRunShellTool())

		systemPrompt := fmt.Sprintf(
			"You are a focused task agent. Complete the following task:\n%s\n\nUse the available tools to accomplish this task. Be thorough and report your findings.",
			task,
		)

		toolDefs := toolDefsToProviderTools(isolatedTools.Definitions())
		messages := []providers.Message{
			{Role: "user", Content: task},
		}

		var result string
		const maxIter = 15
		for i := 0; i < maxIter; i++ {
			req := providers.ChatRequest{
				Model:        m.model,
				Messages:     messages,
				Tools:        toolDefs,
				MaxTokens:    m.maxTokens,
				Temperature:  m.temperature,
				SystemPrompt: systemPrompt,
			}

			resp, err := m.provider.Chat(childCtx, req)
			if err != nil {
				slog.Error("subagent provider error", "taskID", taskID, "err", err)
				result = fmt.Sprintf("error: %v", err)
				break
			}

			assistantMsg := providers.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			}
			messages = append(messages, assistantMsg)

			if len(resp.ToolCalls) == 0 {
				result = resp.Content
				break
			}

			for _, tc := range resp.ToolCalls {
				slog.Debug("subagent executing tool", "taskID", taskID, "name", tc.Name)
				toolResult := isolatedTools.Execute(childCtx, tc.Name, json.RawMessage(tc.Arguments))
				messages = append(messages, providers.Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}

			// If we exhausted iterations, grab last assistant content
			if i == maxIter-1 {
				for j := len(messages) - 1; j >= 0; j-- {
					if messages[j].Role == "assistant" {
						result = messages[j].Content
						break
					}
				}
			}
		}

		m.bus.PublishInbound(bus.InboundMessage{
			Channel:            "system",
			Content:            fmt.Sprintf("[Subagent %q completed]\n\n%s", label, result),
			SessionKeyOverride: fmt.Sprintf("%s:%s", originChannel, originChatID),
		})
	}()

	return taskID
}

// Cancel cancels a running subagent by task ID. Returns true if found.
func (m *SubagentManager) Cancel(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancel, ok := m.running[taskID]
	if !ok {
		return false
	}
	cancel()
	delete(m.running, taskID)
	return true
}

// ListRunning returns IDs of currently running subagents.
func (m *SubagentManager) ListRunning() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	return ids
}
