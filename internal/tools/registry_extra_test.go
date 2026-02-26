package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// dummyTool is a simple tool for testing the registry.
type dummyTool struct {
	name   string
	result string
	err    error
}

func (d *dummyTool) Name() string                { return d.name }
func (d *dummyTool) Description() string          { return "dummy " + d.name }
func (d *dummyTool) Parameters() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (d *dummyTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return d.result, d.err
}

func TestRegistryExecute_UnknownTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "known", result: "ok"})

	result := r.Execute(context.Background(), "unknown_tool", json.RawMessage(`{}`))
	if !strings.Contains(result, "Unknown tool: unknown_tool") {
		t.Errorf("expected unknown tool message, got %q", result)
	}
	if !strings.Contains(result, "known") {
		t.Errorf("expected available tools list to contain 'known', got %q", result)
	}
}

func TestRegistryExecute_Success(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "greet", result: "hello world"})

	result := r.Execute(context.Background(), "greet", json.RawMessage(`{}`))
	if result != "hello world" {
		t.Errorf("result = %q, want %q", result, "hello world")
	}
}

func TestRegistryExecute_Error(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "fail", err: context.DeadlineExceeded})

	result := r.Execute(context.Background(), "fail", json.RawMessage(`{}`))
	if !strings.Contains(result, "Error executing fail") {
		t.Errorf("expected error message, got %q", result)
	}
	if !strings.Contains(result, "different approach") {
		t.Errorf("expected retry hint, got %q", result)
	}
}

func TestRegistryDefinitions(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "a"})
	r.Register(&dummyTool{name: "b"})

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Function.Name] = true
		if d.Type != "function" {
			t.Errorf("Type = %q, want function", d.Type)
		}
	}
	if !names["a"] || !names["b"] {
		t.Errorf("expected tools a and b, got %v", names)
	}
}

func TestRegistryClone(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "original"})

	clone := r.Clone()
	clone.Register(&dummyTool{name: "extra"})

	// Original should not have the extra tool
	if _, ok := r.Get("extra"); ok {
		t.Error("original registry should not have 'extra' tool")
	}
	// Clone should have both
	if _, ok := clone.Get("original"); !ok {
		t.Error("clone should have 'original' tool")
	}
	if _, ok := clone.Get("extra"); !ok {
		t.Error("clone should have 'extra' tool")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "mytool"})

	tool, ok := r.Get("mytool")
	if !ok {
		t.Fatal("expected to find mytool")
	}
	if tool.Name() != "mytool" {
		t.Errorf("Name = %q, want mytool", tool.Name())
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent tool")
	}
}

// --- MCP types tests ---

func TestMCPToolWrapper_Accessors(t *testing.T) {
	wrapper := &MCPToolWrapper{
		serverName: "myserver",
		toolDef: MCPToolDef{
			Name:        "server_tool",
			Description: "does things",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`),
		},
	}

	expectedName := "mcp_myserver_server_tool"
	if wrapper.Name() != expectedName {
		t.Errorf("Name = %q, want %q", wrapper.Name(), expectedName)
	}
	if wrapper.Description() != "does things" {
		t.Errorf("Description = %q, want %q", wrapper.Description(), "does things")
	}
	params := wrapper.Parameters()
	if !strings.Contains(string(params), `"type":"object"`) {
		t.Errorf("Parameters = %q, expected JSON schema", string(params))
	}
}

func TestConnectMCPServers_EmptyConfig(t *testing.T) {
	r := NewRegistry()
	clients, err := ConnectMCPServers(context.Background(), nil, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}

func TestConnectMCPServers_InvalidCommand(t *testing.T) {
	r := NewRegistry()
	configs := map[string]MCPServerConfig{
		"bad": {Command: "/nonexistent/binary/path"},
	}
	_, err := ConnectMCPServers(context.Background(), configs, r)
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}

func TestNewMCPClient_EmptyCommand(t *testing.T) {
	_, err := NewMCPClient(context.Background(), "test", MCPServerConfig{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}
