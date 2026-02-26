package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMCPToolWrapperName(t *testing.T) {
	w := &MCPToolWrapper{
		serverName: "test",
		toolDef:    MCPToolDef{Name: "read_file", Description: "Read a file", InputSchema: json.RawMessage(`{}`)},
		timeout:    30 * time.Second,
	}
	if w.Name() != "mcp_test_read_file" {
		t.Errorf("expected mcp_test_read_file, got %s", w.Name())
	}
	if w.Description() != "Read a file" {
		t.Errorf("unexpected description: %s", w.Description())
	}
}

func TestMCPToolWrapperParameters(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)
	w := &MCPToolWrapper{
		serverName: "fs",
		toolDef:    MCPToolDef{Name: "write_file", Description: "Write a file", InputSchema: schema},
		timeout:    10 * time.Second,
	}
	if string(w.Parameters()) != string(schema) {
		t.Errorf("unexpected parameters: %s", w.Parameters())
	}
}

func TestConnectMCPServersEmpty(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	clients, err := ConnectMCPServers(ctx, map[string]MCPServerConfig{}, registry)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}
