package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTranscriptionProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": "hello world"})
	}))
	defer srv.Close()

	p := &TranscriptionProvider{apiKey: "test-key", baseURL: srv.URL}

	// Create a temp audio file
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.wav")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatal(err)
	}

	text, err := p.Transcribe(t.Context(), audioPath)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
}

func TestTranscriptionProvider_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := &TranscriptionProvider{apiKey: "bad-key", baseURL: srv.URL}

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.wav")
	os.WriteFile(audioPath, []byte("data"), 0644)

	_, err := p.Transcribe(t.Context(), audioPath)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

func TestTranscriptionProvider_FileNotFound(t *testing.T) {
	p := NewTranscriptionProvider("key")
	_, err := p.Transcribe(t.Context(), "/nonexistent/path/audio.wav")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestNewTranscriptionProvider(t *testing.T) {
	p := NewTranscriptionProvider("my-key")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.apiKey != "my-key" {
		t.Errorf("apiKey = %q, want %q", p.apiKey, "my-key")
	}
	if p.baseURL != defaultTranscriptionURL {
		t.Errorf("baseURL = %q, want %q", p.baseURL, defaultTranscriptionURL)
	}
}
