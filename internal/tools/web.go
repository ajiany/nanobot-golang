package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxWebContentLen = 100 * 1024 // 100KB

type WebGetTool struct{}

func NewWebGetTool() *WebGetTool { return &WebGetTool{} }

func (t *WebGetTool) Name() string        { return "web_get" }
func (t *WebGetTool) Description() string { return "Fetch a URL and return its text content" }
func (t *WebGetTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to fetch"}
		},
		"required": ["url"]
	}`)
}

func (t *WebGetTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "nanobot/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, maxWebContentLen)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Strip HTML tags
	text := stripHTML(string(body))

	// Clean up whitespace
	text = cleanWhitespace(text)

	return text, nil
}

// stripHTML removes HTML tags from text
func stripHTML(html string) string {
	// Remove script elements with their content
	re := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, "")
	// Remove style elements with their content
	re = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = re.ReplaceAllString(html, "")

	// Remove all HTML tags
	re = regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(html, " ")
}

// cleanWhitespace collapses multiple spaces and newlines
func cleanWhitespace(text string) string {
	// Collapse multiple spaces
	re := regexp.MustCompile(`[ \t]+`)
	text = re.ReplaceAllString(text, " ")

	// Collapse multiple newlines
	re = regexp.MustCompile(`\n\s*\n\s*\n+`)
	text = re.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
