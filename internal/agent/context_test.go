package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopco/nanobot/internal/tools"
)

func newTestRegistry(names ...string) *tools.Registry {
	r := tools.NewRegistry()
	for _, n := range names {
		r.Register(&stubTool{name: n})
	}
	return r
}

type stubTool struct{ name string }

func (s *stubTool) Name() string                                              { return s.name }
func (s *stubTool) Description() string                                       { return "" }
func (s *stubTool) Parameters() json.RawMessage                               { return json.RawMessage("{}") }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }

func TestBuildSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents content"), 0644)
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("soul content"), 0644)

	cb := NewContextBuilder(dir, newTestRegistry())
	out := cb.BuildSystemPrompt("", "")

	if !strings.Contains(out, "agents content") {
		t.Error("expected AGENTS.md content in output")
	}
	if !strings.Contains(out, "soul content") {
		t.Error("expected SOUL.md content in output")
	}
}

func TestBuildSystemPromptWithMemory(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir, newTestRegistry())
	out := cb.BuildSystemPrompt("some memory facts", "")

	if !strings.Contains(out, "## Memory") {
		t.Error("expected Memory section")
	}
	if !strings.Contains(out, "some memory facts") {
		t.Error("expected memory content")
	}
}

func TestBuildSystemPromptRuntime(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir, newTestRegistry("bash", "read_file"))
	out := cb.BuildSystemPrompt("", "")

	if !strings.Contains(out, "## Runtime Context") {
		t.Error("expected Runtime Context section")
	}
	if !strings.Contains(out, dir) {
		t.Error("expected workspace path in output")
	}
	if !strings.Contains(out, "bash") || !strings.Contains(out, "read_file") {
		t.Error("expected tool names in output")
	}
}
