package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/coopco/nanobot/internal/providers"
)

// MemoryStore manages MEMORY.md (long-term facts) and HISTORY.md (timeline log).
type MemoryStore struct {
	workspace string
	mu        sync.Mutex
}

func NewMemoryStore(workspace string) *MemoryStore {
	return &MemoryStore{workspace: workspace}
}

// ReadMemory returns the content of MEMORY.md, or empty string if not found.
func (m *MemoryStore) ReadMemory() string {
	data, err := os.ReadFile(filepath.Join(m.workspace, "MEMORY.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadHistory returns the content of HISTORY.md, or empty string if not found.
func (m *MemoryStore) ReadHistory() string {
	data, err := os.ReadFile(filepath.Join(m.workspace, "HISTORY.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// Consolidate uses the LLM to extract key facts from messages and update memory files.
func (m *MemoryStore) Consolidate(ctx context.Context, provider providers.Provider, model string, messages []providers.Message) error {
	// Format messages as text
	var lines []string
	for _, msg := range messages {
		lines = append(lines, fmt.Sprintf("[%s]: %s", msg.Role, msg.Content))
	}

	systemPrompt := "Analyze the conversation and call save_memory with a one-line history entry and updated memory content capturing key facts about the user and context."

	saveMemoryTool := providers.ToolDef{
		Type: "function",
		Function: providers.FunctionDef{
			Name:        "save_memory",
			Description: "Save conversation summary to memory files",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"history_entry":{"type":"string","description":"One-line summary for HISTORY.md timeline"},"memory_update":{"type":"string","description":"Updated content for MEMORY.md (key facts about the user and context)"}},"required":["history_entry"]}`),
		},
	}

	req := providers.ChatRequest{
		Model:        model,
		Messages:     messages,
		Tools:        []providers.ToolDef{saveMemoryTool},
		SystemPrompt: systemPrompt,
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to consolidate memory: %w", err)
	}

	// Find the save_memory tool call
	for _, tc := range resp.ToolCalls {
		if tc.Name != "save_memory" {
			continue
		}

		var args struct {
			HistoryEntry string `json:"history_entry"`
			MemoryUpdate string `json:"memory_update"`
		}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return fmt.Errorf("failed to parse save_memory args: %w", err)
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Append to HISTORY.md
		if args.HistoryEntry != "" {
			historyLine := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format(time.RFC3339), args.HistoryEntry)
			f, err := os.OpenFile(filepath.Join(m.workspace, "HISTORY.md"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open HISTORY.md: %w", err)
			}
			_, werr := f.WriteString(historyLine)
			f.Close()
			if werr != nil {
				return fmt.Errorf("failed to write HISTORY.md: %w", werr)
			}
		}

		// Overwrite MEMORY.md
		if args.MemoryUpdate != "" {
			if err := os.WriteFile(filepath.Join(m.workspace, "MEMORY.md"), []byte(args.MemoryUpdate), 0644); err != nil {
				return fmt.Errorf("failed to write MEMORY.md: %w", err)
			}
		}

		return nil
	}

	return nil
}
