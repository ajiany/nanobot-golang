package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const defaultTranscriptionURL = "https://api.groq.com/openai/v1/audio/transcriptions"

type TranscriptionProvider struct {
	apiKey  string
	baseURL string
}

func NewTranscriptionProvider(apiKey string) *TranscriptionProvider {
	return &TranscriptionProvider{
		apiKey:  apiKey,
		baseURL: defaultTranscriptionURL,
	}
}

// Transcribe sends an audio file to Groq's Whisper API and returns the text.
func (p *TranscriptionProvider) Transcribe(ctx context.Context, audioPath string) (string, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err = io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("failed to copy audio data: %w", err)
	}
	if err = mw.WriteField("model", "whisper-large-v3"); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription API error %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Text string `json:"text"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Text, nil
}
