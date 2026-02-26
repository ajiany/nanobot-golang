package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// read_file tool

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool { return &ReadFileTool{} }

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read file content with optional line offset and limit" }
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":   {"type": "string", "description": "File path to read"},
			"offset": {"type": "integer", "description": "Line offset (1-based, optional)"},
			"limit":  {"type": "integer", "description": "Max lines to return (optional)"}
		},
		"required": ["path"]
	}`)
}

func (t *ReadFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if p.Offset > 0 {
		start = p.Offset - 1
	}
	if start >= len(lines) {
		return "", fmt.Errorf("offset %d exceeds file length %d", p.Offset, len(lines))
	}
	end := len(lines)
	if p.Limit > 0 && start+p.Limit < end {
		end = start + p.Limit
	}
	var sb strings.Builder
	for i, line := range lines[start:end] {
		fmt.Fprintf(&sb, "%d\t%s\n", start+i+1, line)
	}
	return sb.String(), nil
}

// write_file tool

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file, creating parent directories as needed" }
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":    {"type": "string", "description": "File path to write"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}
	if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return fmt.Sprintf("File written: %s", p.Path), nil
}

// edit_file tool

type EditFileTool struct{}

func NewEditFileTool() *EditFileTool { return &EditFileTool{} }

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Replace first occurrence of old_text with new_text in a file" }
func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":     {"type": "string", "description": "File path to edit"},
			"old_text": {"type": "string", "description": "Text to replace"},
			"new_text": {"type": "string", "description": "Replacement text"}
		},
		"required": ["path", "old_text", "new_text"]
	}`)
}

func (t *EditFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(data)
	if !strings.Contains(content, p.OldText) {
		return "", fmt.Errorf("old_text not found in %s", p.Path)
	}
	updated := strings.Replace(content, p.OldText, p.NewText, 1)
	if err := os.WriteFile(p.Path, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return fmt.Sprintf("File edited: %s", p.Path), nil
}

// list_dir tool

type ListDirTool struct{}

func NewListDirTool() *ListDirTool { return &ListDirTool{} }

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List directory contents with type indicators" }
func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to list"}
		},
		"required": ["path"]
	}`)
}

func (t *ListDirTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(&sb, "%s/\n", e.Name())
		} else {
			fmt.Fprintf(&sb, "%s\n", e.Name())
		}
	}
	return sb.String(), nil
}
