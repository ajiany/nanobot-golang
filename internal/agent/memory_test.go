package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopco/nanobot/internal/providers"
)

type mockMemoryProvider struct {
	historyEntry string
	memoryUpdate string
}

func (m *mockMemoryProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	args, _ := json.Marshal(map[string]string{
		"history_entry": m.historyEntry,
		"memory_update": m.memoryUpdate,
	})
	return &providers.ChatResponse{
		ToolCalls: []providers.ToolCall{{ID: "call_1", Name: "save_memory", Arguments: string(args)}},
	}, nil
}

func TestReadMemoryEmpty(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemoryStore(dir)
	if got := ms.ReadMemory(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestReadMemoryExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("key facts"), 0644)
	ms := NewMemoryStore(dir)
	if got := ms.ReadMemory(); got != "key facts" {
		t.Errorf("expected %q, got %q", "key facts", got)
	}
}

func TestReadHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemoryStore(dir)
	if got := ms.ReadHistory(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestReadHistoryExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "HISTORY.md"), []byte("past events"), 0644)
	ms := NewMemoryStore(dir)
	if got := ms.ReadHistory(); got != "past events" {
		t.Errorf("expected %q, got %q", "past events", got)
	}
}

func TestConsolidate(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemoryStore(dir)

	mock := &mockMemoryProvider{
		historyEntry: "user asked about Go",
		memoryUpdate: "User is a Go developer",
	}

	msgs := []providers.Message{
		{Role: "user", Content: "tell me about Go"},
		{Role: "assistant", Content: "Go is a compiled language"},
	}

	if err := ms.Consolidate(context.Background(), mock, "gpt-4", msgs); err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}

	history, err := os.ReadFile(filepath.Join(dir, "HISTORY.md"))
	if err != nil {
		t.Fatalf("HISTORY.md not created: %v", err)
	}
	if !strings.Contains(string(history), "user asked about Go") {
		t.Errorf("expected history entry in HISTORY.md, got %q", string(history))
	}

	memory, err := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("MEMORY.md not created: %v", err)
	}
	if string(memory) != "User is a Go developer" {
		t.Errorf("expected memory content, got %q", string(memory))
	}
}
