package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stub tool for registry tests
type stubTool struct {
	name   string
	result string
}

func (s *stubTool) Name() string                    { return s.name }
func (s *stubTool) Description() string             { return "stub" }
func (s *stubTool) Parameters() json.RawMessage     { return json.RawMessage(`{"type":"object"}`) }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return s.result, nil
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &stubTool{name: "mytool", result: "ok"}
	r.Register(tool)
	got, ok := r.Get("mytool")
	if !ok {
		t.Fatal("expected to find registered tool")
	}
	if got.Name() != "mytool" {
		t.Fatalf("expected mytool, got %s", got.Name())
	}
}

func TestExecuteUnknown(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(context.Background(), "nope", nil)
	if !strings.Contains(result, "Unknown tool: nope") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestDefinitions(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "a"})
	r.Register(&stubTool{name: "b"})
	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	for _, d := range defs {
		if d.Type != "function" {
			t.Fatalf("expected type function, got %s", d.Type)
		}
	}
}

func TestClone(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "orig"})
	clone := r.Clone()
	clone.Register(&stubTool{name: "extra"})
	if _, ok := r.Get("extra"); ok {
		t.Fatal("clone modification should not affect original")
	}
	if _, ok := clone.Get("orig"); !ok {
		t.Fatal("clone should have original tools")
	}
}

func TestReadFile(t *testing.T) {
	f, err := os.CreateTemp("", "readtest*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("line1\nline2\nline3")
	f.Close()

	tool := NewReadFileTool()
	params, _ := json.Marshal(map[string]any{"path": f.Name()})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "out.txt")
	tool := NewWriteFileTool()
	params, _ := json.Marshal(map[string]any{"path": path, "content": "hello"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "File written") {
		t.Fatalf("unexpected result: %s", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %s", data)
	}
}

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	tool := NewListDirTool()
	params, _ := json.Marshal(map[string]any{"path": dir})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "a.txt") {
		t.Fatalf("expected a.txt in listing: %s", result)
	}
	if !strings.Contains(result, "subdir/") {
		t.Fatalf("expected subdir/ in listing: %s", result)
	}
}

func TestRunShell(t *testing.T) {
	tool := NewRunShellTool()
	params, _ := json.Marshal(map[string]any{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hello") {
		t.Fatalf("unexpected result: %s", result)
	}
}
