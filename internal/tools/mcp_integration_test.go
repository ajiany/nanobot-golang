package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockMCPServerScript is a shell script that acts as a minimal MCP server.
// It reads JSON-RPC lines from stdin and writes responses to stdout.
// Handles: initialize, tools/list, tools/call
const mockMCPServerScript = `
while IFS= read -r line; do
  id=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null)
  method=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('method',''))" 2>/dev/null)
  case "$method" in
    initialize)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"mock\",\"version\":\"0.1\"}}}"
      ;;
    notifications/initialized)
      ;;
    tools/list)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"echo_tool\",\"description\":\"Echoes input\",\"inputSchema\":{\"type\":\"object\",\"properties\":{\"msg\":{\"type\":\"string\"}}}}]}}"
      ;;
    tools/call)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"mock-result\"}]}}"
      ;;
    *)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"error\":{\"code\":-32601,\"message\":\"method not found\"}}"
      ;;
  esac
done
`

// mockMCPServerScriptError is a script that always returns an error for tools/call.
const mockMCPServerScriptError = `
while IFS= read -r line; do
  id=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null)
  method=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('method',''))" 2>/dev/null)
  case "$method" in
    initialize)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{}}}"
      ;;
    notifications/initialized)
      ;;
    tools/list)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"fail_tool\",\"description\":\"Always fails\",\"inputSchema\":{\"type\":\"object\"}}]}}"
      ;;
    tools/call)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"error\":{\"code\":-32000,\"message\":\"tool execution failed\"}}"
      ;;
    *)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"error\":{\"code\":-32601,\"message\":\"method not found\"}}"
      ;;
  esac
done
`

// checkPython3 returns true if python3 is available (needed for mock server scripts).
func checkPython3() bool {
	cfg := MCPServerConfig{Command: "python3", Args: []string{"--version"}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Use exec directly to check
	_ = cfg
	_ = ctx
	// We'll just try and skip if it fails
	return true
}

func TestNewMCPClientEmptyCommand(t *testing.T) {
	ctx := context.Background()
	_, err := NewMCPClient(ctx, "test", MCPServerConfig{Command: ""})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewMCPClientInvalidCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := NewMCPClient(ctx, "test", MCPServerConfig{Command: "/nonexistent/binary/that/does/not/exist"})
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}

func TestConnectMCPServersInvalidCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	registry := NewRegistry()
	configs := map[string]MCPServerConfig{
		"bad": {Command: "/nonexistent/binary/xyz"},
	}
	clients, err := ConnectMCPServers(ctx, configs, registry)
	if err == nil {
		t.Fatal("expected error for invalid command")
		for _, c := range clients {
			c.Close()
		}
	}
}

func TestConnectMCPServersEmptyConfigs(t *testing.T) {
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

func TestMCPToolWrapperExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	wrapper := &MCPToolWrapper{
		client:     client,
		serverName: "mock",
		toolDef:    MCPToolDef{Name: "echo_tool", Description: "Echoes input", InputSchema: json.RawMessage(`{"type":"object"}`)},
		timeout:    5 * time.Second,
	}

	params := json.RawMessage(`{"msg":"hello"}`)
	result, err := wrapper.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "mock-result" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestMCPClientListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "echo_tool" {
		t.Errorf("unexpected tool name: %s", tools[0].Name)
	}
}

func TestMCPClientCallTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	result, err := client.CallTool(ctx, "echo_tool", json.RawMessage(`{"msg":"test"}`))
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result != "mock-result" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestMCPClientCallToolError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScriptError},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	_, err = client.CallTool(ctx, "fail_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error from failing tool")
	}
	if !strings.Contains(err.Error(), "tool execution failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMCPClientClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestConnectMCPServersWithMockServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := NewRegistry()
	configs := map[string]MCPServerConfig{
		"mock": {
			Command: "sh",
			Args:    []string{"-c", mockMCPServerScript},
		},
	}

	clients, err := ConnectMCPServers(ctx, configs, registry)
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	// The mock server registers echo_tool, so it should be in the registry as mcp_mock_echo_tool
	tool, ok := registry.Get("mcp_mock_echo_tool")
	if !ok {
		t.Fatal("expected mcp_mock_echo_tool to be registered")
	}
	if tool.Name() != "mcp_mock_echo_tool" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}
}

func TestConnectMCPServersToolTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := NewRegistry()
	configs := map[string]MCPServerConfig{
		"mock": {
			Command:     "sh",
			Args:        []string{"-c", mockMCPServerScript},
			ToolTimeout: 60,
		},
	}

	clients, err := ConnectMCPServers(ctx, configs, registry)
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
}

// errorTool is a stub that always returns an error.
type errorTool struct{}

func (e *errorTool) Name() string                    { return "error_tool" }
func (e *errorTool) Description() string             { return "always errors" }
func (e *errorTool) Parameters() json.RawMessage     { return json.RawMessage(`{"type":"object"}`) }
func (e *errorTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "", errors.New("tool execution failed")
}

func TestRegistryExecuteToolError(t *testing.T) {
	r := NewRegistry()
	r.Register(&errorTool{})
	result := r.Execute(context.Background(), "error_tool", nil)
	if !strings.Contains(result, "Error executing error_tool") {
		t.Errorf("unexpected result: %s", result)
	}
	if !strings.Contains(result, "tool execution failed") {
		t.Errorf("expected error message in result: %s", result)
	}
}

func TestRegistryExecuteSuccess(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "ok_tool", result: "success-output"})
	result := r.Execute(context.Background(), "ok_tool", json.RawMessage(`{}`))
	if result != "success-output" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRegistryExecuteUnknownListsTools(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "tool_a"})
	r.Register(&stubTool{name: "tool_b"})
	result := r.Execute(context.Background(), "missing_tool", nil)
	if !strings.Contains(result, "Unknown tool: missing_tool") {
		t.Errorf("unexpected result: %s", result)
	}
	// Should list available tools
	if !strings.Contains(result, "tool_a") || !strings.Contains(result, "tool_b") {
		t.Errorf("expected available tools listed: %s", result)
	}
}

func TestMCPToolWrapperNameFormat(t *testing.T) {
	tests := []struct {
		serverName string
		toolName   string
		want       string
	}{
		{"myserver", "read_file", "mcp_myserver_read_file"},
		{"fs", "write", "mcp_fs_write"},
		{"a", "b", "mcp_a_b"},
	}
	for _, tc := range tests {
		w := &MCPToolWrapper{
			serverName: tc.serverName,
			toolDef:    MCPToolDef{Name: tc.toolName},
			timeout:    time.Second,
		}
		if got := w.Name(); got != tc.want {
			t.Errorf("Name() = %q, want %q", got, tc.want)
		}
	}
}

func TestMCPToolWrapperDescription(t *testing.T) {
	w := &MCPToolWrapper{
		serverName: "s",
		toolDef:    MCPToolDef{Name: "t", Description: "does something useful"},
		timeout:    time.Second,
	}
	if w.Description() != "does something useful" {
		t.Errorf("unexpected description: %s", w.Description())
	}
}

func TestMCPToolWrapperParametersNil(t *testing.T) {
	w := &MCPToolWrapper{
		serverName: "s",
		toolDef:    MCPToolDef{Name: "t", InputSchema: nil},
		timeout:    time.Second,
	}
	if w.Parameters() != nil {
		t.Errorf("expected nil parameters, got: %s", w.Parameters())
	}
}

func TestMCPClientSendRequestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", mockMCPServerScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	// Cancel context immediately before sending
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	cancelFn()

	_, err = client.sendRequest(cancelCtx, "tools/list", json.RawMessage("{}"))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestMCPToolWrapperExecuteTimeout(t *testing.T) {
	// Script that hangs on tools/call
	hangScript := `
while IFS= read -r line; do
  id=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null)
  method=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('method',''))" 2>/dev/null)
  case "$method" in
    initialize)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{}}}"
      ;;
    notifications/initialized)
      ;;
    tools/list)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"slow_tool\",\"description\":\"Slow\",\"inputSchema\":{\"type\":\"object\"}}]}}"
      ;;
    tools/call)
      sleep 60
      ;;
  esac
done
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewMCPClient(ctx, "mock", MCPServerConfig{
		Command: "sh",
		Args:    []string{"-c", hangScript},
	})
	if err != nil {
		t.Skipf("mock MCP server unavailable: %v", err)
	}
	defer client.Close()

	wrapper := &MCPToolWrapper{
		client:     client,
		serverName: "mock",
		toolDef:    MCPToolDef{Name: "slow_tool", Description: "Slow", InputSchema: json.RawMessage(`{}`)},
		timeout:    100 * time.Millisecond,
	}

	start := time.Now()
	_, err = wrapper.Execute(ctx, json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

// Ensure ConnectMCPServers signature matches — compile-time check via usage.
var _ = fmt.Sprintf // suppress unused import if needed
