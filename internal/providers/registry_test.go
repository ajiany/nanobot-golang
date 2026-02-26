package providers

import (
	"testing"
)

func TestFindByModel(t *testing.T) {
	tests := []struct {
		model    string
		wantName string
	}{
		{"gpt-4o", "openai"},
		{"claude-3-5-sonnet", "anthropic"},
		{"deepseek-chat", "deepseek"},
	}
	for _, tt := range tests {
		spec := FindByModel(tt.model)
		if spec == nil {
			t.Errorf("FindByModel(%q) = nil, want %q", tt.model, tt.wantName)
			continue
		}
		if spec.Name != tt.wantName {
			t.Errorf("FindByModel(%q).Name = %q, want %q", tt.model, spec.Name, tt.wantName)
		}
	}
}

func TestFindByModelUnknown(t *testing.T) {
	spec := FindByModel("totally-unknown-model-xyz")
	if spec != nil {
		t.Errorf("FindByModel(unknown) = %q, want nil", spec.Name)
	}
}

func TestFindGatewayByKeyPrefix(t *testing.T) {
	spec := FindGateway("sk-or-xxx", "")
	if spec == nil {
		t.Fatal("FindGateway(sk-or-xxx) = nil, want openrouter")
	}
	if spec.Name != "openrouter" {
		t.Errorf("FindGateway(sk-or-xxx).Name = %q, want openrouter", spec.Name)
	}
}

func TestFindGatewayByBaseURL(t *testing.T) {
	spec := FindGateway("", "http://localhost:11434/v1")
	if spec == nil {
		t.Fatal("FindGateway(11434 URL) = nil, want ollama")
	}
	if spec.Name != "ollama" {
		t.Errorf("FindGateway(11434 URL).Name = %q, want ollama", spec.Name)
	}
}

func TestFindByName(t *testing.T) {
	spec := FindByName("anthropic")
	if spec == nil {
		t.Fatal("FindByName(anthropic) = nil")
	}
	if spec.Name != "anthropic" {
		t.Errorf("FindByName(anthropic).Name = %q, want anthropic", spec.Name)
	}
}
