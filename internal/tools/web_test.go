package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebGetTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>Hello from server</p></body></html>`))
	}))
	defer srv.Close()

	tool := NewWebGetTool()
	params, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Hello from server") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestWebGetTool_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	tool := NewWebGetTool()
	params, _ := json.Marshal(map[string]any{"url": srv.URL})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestWebGetTool_EmptyURL(t *testing.T) {
	tool := NewWebGetTool()
	params, _ := json.Marshal(map[string]any{"url": ""})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestWebGetTool_InvalidParams(t *testing.T) {
	tool := NewWebGetTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestWebGetTool_StripHTMLTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Test</title></head><body><h1>Title</h1><p>Content here</p></body></html>`))
	}))
	defer srv.Close()

	tool := NewWebGetTool()
	params, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "<h1>") || strings.Contains(result, "<p>") {
		t.Errorf("HTML tags should be stripped, got: %s", result)
	}
	if !strings.Contains(result, "Content here") {
		t.Errorf("expected text content, got: %s", result)
	}
}

func TestWebGetTool_ScriptStripped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><script>alert('xss')</script><p>real content</p></body></html>`))
	}))
	defer srv.Close()

	tool := NewWebGetTool()
	params, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "alert") {
		t.Errorf("script content should be stripped, got: %s", result)
	}
}

func TestWebGetTool_Name(t *testing.T) {
	tool := NewWebGetTool()
	if tool.Name() != "web_get" {
		t.Errorf("Name() = %q, want web_get", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Parameters()) == 0 {
		t.Error("Parameters() is empty")
	}
}
