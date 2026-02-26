package agent

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/session"
)

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"http://example.com/img.png", false},
		{"https://example.com/img.png", false},
		{"/tmp/file.png", true},
		{"relative/path.jpg", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := isLocalPath(tt.input); got != tt.want {
			t.Errorf("isLocalPath(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestProcessMedia_Empty(t *testing.T) {
	parts := ProcessMedia(nil)
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

func TestProcessMedia_URL(t *testing.T) {
	media := []bus.Media{
		{Type: "image", URL: "https://example.com/photo.jpg"},
	}
	parts := ProcessMedia(media)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Type != "image_url" {
		t.Errorf("type = %q, want image_url", parts[0].Type)
	}
	if parts[0].ImageURL == nil || parts[0].ImageURL.URL != "https://example.com/photo.jpg" {
		t.Errorf("unexpected URL: %+v", parts[0].ImageURL)
	}
}

func TestProcessMedia_InlineData(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	media := []bus.Media{
		{Type: "image", Data: data, MimeType: "image/png"},
	}
	parts := ProcessMedia(media)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	expected := "data:image/png;base64," + encoded
	if parts[0].ImageURL == nil || parts[0].ImageURL.URL != expected {
		t.Errorf("unexpected data URI: got %q", parts[0].ImageURL.URL)
	}
}

func TestProcessMedia_InlineDataAutoMime(t *testing.T) {
	// PNG header bytes — http.DetectContentType should detect image/png
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0, 0, 0, 0, 0}
	media := []bus.Media{
		{Type: "image", Data: data},
	}
	parts := ProcessMedia(media)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if !strings.HasPrefix(parts[0].ImageURL.URL, "data:image/png;base64,") {
		t.Errorf("expected auto-detected image/png, got %q", parts[0].ImageURL.URL)
	}
}

func TestProcessMedia_LocalFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.bin")
	content := []byte("hello world binary")
	os.WriteFile(fpath, content, 0644)

	media := []bus.Media{
		{Type: "file", URL: fpath},
	}
	parts := ProcessMedia(media)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	encoded := base64.StdEncoding.EncodeToString(content)
	if !strings.Contains(parts[0].ImageURL.URL, encoded) {
		t.Error("expected base64-encoded file content in data URI")
	}
}

func TestProcessMedia_LocalFileNotFound(t *testing.T) {
	media := []bus.Media{
		{Type: "file", URL: "/nonexistent/path/file.png"},
	}
	parts := ProcessMedia(media)
	if len(parts) != 0 {
		t.Errorf("expected 0 parts for missing file, got %d", len(parts))
	}
}

func TestSessionToProviderMessages_Empty(t *testing.T) {
	msgs := sessionToProviderMessages(nil)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestSessionToProviderMessages_WithToolCalls(t *testing.T) {
	history := []session.Message{
		{Role: "user", Content: "hello"},
		{
			Role:    "assistant",
			Content: "thinking",
			ToolCalls: []session.ToolCallRecord{
				{ID: "tc1", Name: "bash", Arguments: `{"cmd":"ls"}`},
			},
		},
		{Role: "tool", Content: "file.txt", ToolCallID: "tc1"},
	}
	msgs := sessionToProviderMessages(history)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msgs[1].ToolCalls))
	}
	if msgs[1].ToolCalls[0].Name != "bash" {
		t.Errorf("tool call name = %q, want bash", msgs[1].ToolCalls[0].Name)
	}
	if msgs[2].ToolCallID != "tc1" {
		t.Errorf("ToolCallID = %q, want tc1", msgs[2].ToolCallID)
	}
}

func TestNewAgentLoop_DefaultMaxIter(t *testing.T) {
	loop := NewAgentLoop(AgentLoopConfig{})
	if loop.maxIter != 40 {
		t.Errorf("maxIter = %d, want 40", loop.maxIter)
	}
}

func TestBuildSystemPromptWithSkills(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir, newTestRegistry())
	out := cb.BuildSystemPrompt("", "skill1: does X\nskill2: does Y")
	if !strings.Contains(out, "## Available Skills") {
		t.Error("expected Available Skills section")
	}
	if !strings.Contains(out, "skill1") {
		t.Error("expected skill1 in output")
	}
}
