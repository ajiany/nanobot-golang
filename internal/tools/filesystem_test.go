package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)

	tool := NewReadFileTool()
	params, _ := json.Marshal(map[string]any{"path": path})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestReadFileTool_WithOffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne"), 0644)

	tool := NewReadFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "offset": 2, "limit": 2})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "b") {
		t.Errorf("expected line b in result: %s", result)
	}
	if strings.Contains(result, "a") {
		t.Errorf("line a should be excluded by offset: %s", result)
	}
}

func TestReadFileTool_NotFound(t *testing.T) {
	tool := NewReadFileTool()
	params, _ := json.Marshal(map[string]any{"path": "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadFileTool_OffsetExceedsLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.txt")
	os.WriteFile(path, []byte("one line"), 0644)

	tool := NewReadFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "offset": 999})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for offset exceeding file length")
	}
}

func TestReadFileTool_InvalidParams(t *testing.T) {
	tool := NewReadFileTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestWriteFileTool_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	tool := NewWriteFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "content": "hello"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "File written") {
		t.Errorf("unexpected result: %s", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("file content = %q, want %q", string(data), "hello")
	}
}

func TestWriteFileTool_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.txt")

	tool := NewWriteFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "content": "nested"})
	_, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "nested" {
		t.Errorf("file content = %q, want nested", string(data))
	}
}

func TestWriteFileTool_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old content"), 0644)

	tool := NewWriteFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "content": "new content"})
	_, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWriteFileTool_InvalidParams(t *testing.T) {
	tool := NewWriteFileTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestEditFileTool_ReplaceText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	tool := NewEditFileTool()
	params, _ := json.Marshal(map[string]any{
		"path":     path,
		"old_text": "world",
		"new_text": "Go",
	})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "File edited") {
		t.Errorf("unexpected result: %s", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello Go" {
		t.Errorf("file content = %q, want %q", string(data), "hello Go")
	}
}

func TestEditFileTool_OldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	tool := NewEditFileTool()
	params, _ := json.Marshal(map[string]any{
		"path":     path,
		"old_text": "nothere",
		"new_text": "x",
	})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when old_text not found")
	}
}

func TestEditFileTool_FileNotFound(t *testing.T) {
	tool := NewEditFileTool()
	params, _ := json.Marshal(map[string]any{
		"path":     "/nonexistent/file.txt",
		"old_text": "x",
		"new_text": "y",
	})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEditFileTool_InvalidParams(t *testing.T) {
	tool := NewEditFileTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestListDirTool_Contents(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	tool := NewListDirTool()
	params, _ := json.Marshal(map[string]any{"path": dir})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "file.txt") {
		t.Errorf("expected file.txt in listing: %s", result)
	}
	if !strings.Contains(result, "subdir/") {
		t.Errorf("expected subdir/ in listing: %s", result)
	}
}

func TestListDirTool_NotFound(t *testing.T) {
	tool := NewListDirTool()
	params, _ := json.Marshal(map[string]any{"path": "/nonexistent/dir"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestListDirTool_InvalidParams(t *testing.T) {
	tool := NewListDirTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestFilesystemToolNames(t *testing.T) {
	tests := []struct {
		tool Tool
		name string
	}{
		{NewReadFileTool(), "read_file"},
		{NewWriteFileTool(), "write_file"},
		{NewEditFileTool(), "edit_file"},
		{NewListDirTool(), "list_dir"},
	}
	for _, tt := range tests {
		if tt.tool.Name() != tt.name {
			t.Errorf("Name() = %q, want %q", tt.tool.Name(), tt.name)
		}
		if tt.tool.Description() == "" {
			t.Errorf("%s: Description() is empty", tt.name)
		}
		if len(tt.tool.Parameters()) == 0 {
			t.Errorf("%s: Parameters() is empty", tt.name)
		}
	}
}
