package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// MCPClient manages a connection to an MCP server via stdio.
type MCPClient struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	serverName string
	mu         sync.Mutex
	reqID      atomic.Int64
	pending    map[int64]chan jsonRPCResponse
	pendingMu  sync.Mutex
	done       chan struct{}
}

// MCPServerConfig mirrors config.MCPServerConfig to avoid import cycle.
type MCPServerConfig struct {
	Command     string
	Args        []string
	Env         map[string]string
	URL         string
	ToolTimeout int // seconds, default 30
}

// jsonRPCRequest represents a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error.
type jsonRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewMCPClient starts an MCP server process and initializes the connection.
func NewMCPClient(ctx context.Context, name string, cfg MCPServerConfig) (*MCPClient, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("MCP server %s: command is required", name)
	}

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	client := &MCPClient{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewReader(stdout),
		serverName: name,
		pending:    make(map[int64]chan jsonRPCResponse),
		done:       make(chan struct{}),
	}

	// Start read loop
	go client.readLoop()

	// Initialize the connection
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "nanobot",
			"version": "0.1.0",
		},
	}

	initParamsJSON, err := json.Marshal(initParams)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to marshal init params: %w", err)
	}

	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = client.sendRequest(initCtx, "initialize", initParamsJSON)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	// Send initialized notification
	if err := client.sendNotification("notifications/initialized", nil); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	slog.Info("MCP client connected", "server", name)
	return client, nil
}

// Close shuts down the MCP server process.
func (c *MCPClient) Close() error {
	close(c.done)

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}

	return nil
}

// readLoop reads JSON-RPC responses from stdout.
func (c *MCPClient) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Warn("failed to parse JSON-RPC response", "error", err, "line", string(line))
			continue
		}

		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ok {
			select {
			case ch <- resp:
			case <-c.done:
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("MCP read loop error", "server", c.serverName, "error", err)
	}
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (c *MCPClient) sendRequest(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := c.reqID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	respCh := make(chan jsonRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	c.mu.Lock()
	_, err = c.stdin.Write(append(reqJSON, '\n'))
	c.mu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("MCP client closed")
	}
}

// sendNotification sends a JSON-RPC notification (no response expected).
func (c *MCPClient) sendNotification(method string, params json.RawMessage) error {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err = c.stdin.Write(append(reqJSON, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// MCPToolDef represents a tool definition from an MCP server.
type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListTools calls tools/list on the MCP server and returns tool definitions.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	result, err := c.sendRequest(ctx, "tools/list", json.RawMessage("{}"))
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var response struct {
		Tools []MCPToolDef `json:"tools"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	return response.Tools, nil
}

// CallTool calls a specific tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, args json.RawMessage) (string, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool params: %w", err)
	}

	result, err := c.sendRequest(ctx, "tools/call", paramsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %w", err)
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to parse tool response: %w", err)
	}

	// Concatenate all text content
	var output string
	for _, content := range response.Content {
		if content.Type == "text" {
			output += content.Text
		}
	}

	return output, nil
}

// MCPToolWrapper wraps an MCP server tool as a native nanobot Tool.
type MCPToolWrapper struct {
	client     *MCPClient
	serverName string
	toolDef    MCPToolDef
	timeout    time.Duration
}

func (w *MCPToolWrapper) Name() string {
	return fmt.Sprintf("mcp_%s_%s", w.serverName, w.toolDef.Name)
}

func (w *MCPToolWrapper) Description() string {
	return w.toolDef.Description
}

func (w *MCPToolWrapper) Parameters() json.RawMessage {
	return w.toolDef.InputSchema
}

func (w *MCPToolWrapper) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	return w.client.CallTool(execCtx, w.toolDef.Name, params)
}

// ConnectMCPServers connects to all configured MCP servers and registers their tools.
func ConnectMCPServers(ctx context.Context, configs map[string]MCPServerConfig, registry *Registry) ([]*MCPClient, error) {
	if len(configs) == 0 {
		return []*MCPClient{}, nil
	}

	var clients []*MCPClient
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(configs))

	for name, cfg := range configs {
		wg.Add(1)
		go func(name string, cfg MCPServerConfig) {
			defer wg.Done()

			client, err := NewMCPClient(ctx, name, cfg)
			if err != nil {
				errCh <- fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
				return
			}

			tools, err := client.ListTools(ctx)
			if err != nil {
				client.Close()
				errCh <- fmt.Errorf("failed to list tools from MCP server %s: %w", name, err)
				return
			}

			timeout := time.Duration(cfg.ToolTimeout) * time.Second
			if timeout == 0 {
				timeout = 30 * time.Second
			}

			for _, toolDef := range tools {
				wrapper := &MCPToolWrapper{
					client:     client,
					serverName: name,
					toolDef:    toolDef,
					timeout:    timeout,
				}
				registry.Register(wrapper)
				slog.Info("Registered MCP tool", "server", name, "tool", toolDef.Name, "as", wrapper.Name())
			}

			mu.Lock()
			clients = append(clients, client)
			mu.Unlock()
		}(name, cfg)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// Close all successfully connected clients
		for _, client := range clients {
			client.Close()
		}
		return nil, fmt.Errorf("failed to connect to MCP servers: %v", errs)
	}

	return clients, nil
}
