package agent

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
	"github.com/coopco/nanobot/internal/tools"
)

// BootstrapFiles are read from workspace in order to build the system prompt.
var BootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"TOOLS.md",
	"IDENTITY.md",
}

// ContextBuilder assembles system prompts from workspace files and runtime context.
type ContextBuilder struct {
	workspace string
	tools     *tools.Registry
}

func NewContextBuilder(workspace string, toolRegistry *tools.Registry) *ContextBuilder {
	return &ContextBuilder{workspace: workspace, tools: toolRegistry}
}

// BuildSystemPrompt reads bootstrap files from workspace and appends runtime context.
func (c *ContextBuilder) BuildSystemPrompt(memoryContent, skillsContent string) string {
	var parts []string

	for _, name := range BootstrapFiles {
		data, err := os.ReadFile(filepath.Join(c.workspace, name))
		if err != nil {
			continue
		}
		parts = append(parts, string(data))
	}

	base := strings.Join(parts, "\n\n---\n\n")

	if memoryContent != "" {
		base += "\n\n## Memory\n\n" + memoryContent
	}

	if skillsContent != "" {
		base += "\n\n## Available Skills\n\n" + skillsContent
	}

	// Collect tool names
	defs := c.tools.Definitions()
	toolNames := make([]string, 0, len(defs))
	for _, d := range defs {
		toolNames = append(toolNames, d.Function.Name)
	}

	base += fmt.Sprintf(
		"\n\n## Runtime Context\n- Current time: %s\n- Workspace: %s\n- Available tools: %s",
		time.Now().Format(time.RFC3339),
		c.workspace,
		strings.Join(toolNames, ", "),
	)

	return base
}

// ProcessMedia converts a slice of bus.Media items into ContentParts for multimodal messages.
// URL media becomes an image_url part directly; local file media is read, MIME-detected,
// and base64-encoded into a data URI; inline Data bytes are base64-encoded into a data URI.
func ProcessMedia(media []bus.Media) []providers.ContentPart {
	parts := make([]providers.ContentPart, 0, len(media))
	for _, m := range media {
		switch {
		case m.Data != nil:
			// Inline bytes — detect MIME if not provided, then encode as data URI.
			mime := m.MimeType
			if mime == "" {
				mime = http.DetectContentType(m.Data)
			}
			encoded := base64.StdEncoding.EncodeToString(m.Data)
			parts = append(parts, providers.ContentPart{
				Type: "image_url",
				ImageURL: &providers.ImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", mime, encoded),
					Detail: "auto",
				},
			})
		case isLocalPath(m.URL):
			// Local file — read, detect MIME, encode.
			data, err := os.ReadFile(m.URL)
			if err != nil {
				continue
			}
			mime := m.MimeType
			if mime == "" {
				mime = http.DetectContentType(data)
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			parts = append(parts, providers.ContentPart{
				Type: "image_url",
				ImageURL: &providers.ImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", mime, encoded),
					Detail: "auto",
				},
			})
		case m.URL != "":
			// Remote URL — pass through directly.
			parts = append(parts, providers.ContentPart{
				Type: "image_url",
				ImageURL: &providers.ImageURL{
					URL:    m.URL,
					Detail: "auto",
				},
			})
		}
	}
	return parts
}

// isLocalPath returns true when the string looks like a filesystem path rather than a URL.
func isLocalPath(s string) bool {
	return !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") && s != ""
}
