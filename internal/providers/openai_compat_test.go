package providers

import (
	"testing"
)

func TestNewOpenAICompatProvider(t *testing.T) {
	p := NewOpenAICompatProvider("test-key", "https://api.example.com/v1", "gpt-4o")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.defaultModel != "gpt-4o" {
		t.Errorf("defaultModel = %q, want %q", p.defaultModel, "gpt-4o")
	}
}

func TestNewOpenAICompatProviderFromSpec(t *testing.T) {
	spec := &ProviderSpec{
		DefaultAPIBase: "https://api.deepseek.com/v1",
		ModelPrefix:    "ds-",
		SkipPrefixes:   []string{"ds-"},
	}
	p := NewOpenAICompatProviderFromSpec(spec, "key", "")
	if p.modelPrefix != "ds-" {
		t.Errorf("modelPrefix = %q, want %q", p.modelPrefix, "ds-")
	}
	if len(p.skipPrefixes) != 1 || p.skipPrefixes[0] != "ds-" {
		t.Errorf("skipPrefixes = %v, want [ds-]", p.skipPrefixes)
	}
}

func TestNewOpenAICompatProviderFromSpec_BaseURLOverride(t *testing.T) {
	spec := &ProviderSpec{
		DefaultAPIBase: "https://default.example.com/v1",
	}
	p := NewOpenAICompatProviderFromSpec(spec, "key", "https://override.example.com/v1")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestResolveModel_NoPrefix(t *testing.T) {
	p := &OpenAICompatProvider{}
	if got := p.resolveModel("gpt-4o"); got != "gpt-4o" {
		t.Errorf("resolveModel = %q, want %q", got, "gpt-4o")
	}
}

func TestResolveModel_WithPrefix(t *testing.T) {
	p := &OpenAICompatProvider{modelPrefix: "pfx/"}
	if got := p.resolveModel("mymodel"); got != "pfx/mymodel" {
		t.Errorf("resolveModel = %q, want %q", got, "pfx/mymodel")
	}
}

func TestResolveModel_SkipPrefix(t *testing.T) {
	p := &OpenAICompatProvider{modelPrefix: "pfx/", skipPrefixes: []string{"skip-"}}
	if got := p.resolveModel("skip-model"); got != "skip-model" {
		t.Errorf("resolveModel = %q, want %q", got, "skip-model")
	}
}
