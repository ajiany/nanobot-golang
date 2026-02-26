package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const maxOutputLen = 10000

type RunShellTool struct{}

func NewRunShellTool() *RunShellTool { return &RunShellTool{} }

func (t *RunShellTool) Name() string        { return "run_shell" }
func (t *RunShellTool) Description() string { return "Execute a shell command and return its output" }
func (t *RunShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default 30)"}
		},
		"required": ["command"]
	}`)
}

func (t *RunShellTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	timeout := 30
	if p.Timeout > 0 {
		timeout = p.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", p.Command)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := buf.String()
	if len(output) > maxOutputLen {
		output = output[:maxOutputLen] + "\n[output truncated]"
	}
	if err != nil {
		return "", fmt.Errorf("%s\n%w", output, err)
	}
	return output, nil
}
