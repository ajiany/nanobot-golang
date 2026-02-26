package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFileValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"agents": {"defaults": {"model": "claude-3"}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "claude-3" {
		t.Errorf("expected model %q, got %q", "claude-3", cfg.Agents.Defaults.Model)
	}
}

func TestLoadFromFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEnvOverrideAgentsDefaultsModel(t *testing.T) {
	os.Setenv("NANOBOT_AGENTS_DEFAULTS_MODEL", "env-model-xyz")
	defer os.Unsetenv("NANOBOT_AGENTS_DEFAULTS_MODEL")

	cfg, err := LoadFromReader(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "env-model-xyz" {
		t.Errorf("expected env override %q, got %q", "env-model-xyz", cfg.Agents.Defaults.Model)
	}
}

func TestEnvOverrideAgentsDefaultsWorkspace(t *testing.T) {
	os.Setenv("NANOBOT_AGENTS_DEFAULTS_WORKSPACE", "/tmp/env-workspace")
	defer os.Unsetenv("NANOBOT_AGENTS_DEFAULTS_WORKSPACE")

	cfg, err := LoadFromReader(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}
	if cfg.Agents.Defaults.Workspace != "/tmp/env-workspace" {
		t.Errorf("expected workspace %q, got %q", "/tmp/env-workspace", cfg.Agents.Defaults.Workspace)
	}
}

func TestTildeExpansionInWorkspace(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	cfg, err := LoadFromReader(strings.NewReader(`{"agents": {"defaults": {"workspace": "~/myworkspace"}}}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	expected := filepath.Join(home, "myworkspace")
	if cfg.Agents.Defaults.Workspace != expected {
		t.Errorf("expected expanded workspace %q, got %q", expected, cfg.Agents.Defaults.Workspace)
	}
}

func TestNoTildeExpansionForAbsolutePath(t *testing.T) {
	cfg, err := LoadFromReader(strings.NewReader(`{"agents": {"defaults": {"workspace": "/absolute/path"}}}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}
	if cfg.Agents.Defaults.Workspace != "/absolute/path" {
		t.Errorf("expected unchanged path %q, got %q", "/absolute/path", cfg.Agents.Defaults.Workspace)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"workspace", cfg.Agents.Defaults.Workspace, "~/.nanobot/workspace"},
		{"model", cfg.Agents.Defaults.Model, "gpt-4o"},
		{"maxTokens", cfg.Agents.Defaults.MaxTokens, 4096},
		{"maxToolIterations", cfg.Agents.Defaults.MaxToolIterations, 40},
		{"gatewayHost", cfg.Gateway.Host, "0.0.0.0"},
		{"gatewayPort", cfg.Gateway.Port, 8080},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, tc.got)
			}
		})
	}
}

func TestLoadFromReaderEmptyObject(t *testing.T) {
	cfg, err := LoadFromReader(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}
	// defaults should still be applied
	if cfg.Agents.Defaults.Model != "gpt-4o" {
		t.Errorf("expected default model, got %q", cfg.Agents.Defaults.Model)
	}
}

func TestAllProviderEnvOverrides(t *testing.T) {
	envVars := map[string]string{
		"NANOBOT_PROVIDERS_ANTHROPIC_APIKEY":  "ant-key",
		"NANOBOT_PROVIDERS_DEEPSEEK_APIKEY":   "ds-key",
		"NANOBOT_PROVIDERS_MOONSHOT_APIKEY":   "ms-key",
		"NANOBOT_PROVIDERS_ZHIPU_APIKEY":      "zp-key",
		"NANOBOT_PROVIDERS_DASHSCOPE_APIKEY":  "dsc-key",
		"NANOBOT_PROVIDERS_GROQ_APIKEY":       "groq-key",
		"NANOBOT_PROVIDERS_XAI_APIKEY":        "xai-key",
		"NANOBOT_PROVIDERS_MISTRAL_APIKEY":    "mist-key",
		"NANOBOT_PROVIDERS_COHERE_APIKEY":     "coh-key",
		"NANOBOT_PROVIDERS_OPENROUTER_APIKEY": "or-key",
		"NANOBOT_PROVIDERS_AIHUBMIX_APIKEY":   "ahm-key",
		"NANOBOT_PROVIDERS_CUSTOM_APIKEY":     "cust-key",
	}
	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cfg, err := LoadFromReader(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	checks := []struct{ got, want string }{
		{cfg.Providers.Anthropic.APIKey, "ant-key"},
		{cfg.Providers.DeepSeek.APIKey, "ds-key"},
		{cfg.Providers.Moonshot.APIKey, "ms-key"},
		{cfg.Providers.Zhipu.APIKey, "zp-key"},
		{cfg.Providers.DashScope.APIKey, "dsc-key"},
		{cfg.Providers.Groq.APIKey, "groq-key"},
		{cfg.Providers.XAI.APIKey, "xai-key"},
		{cfg.Providers.Mistral.APIKey, "mist-key"},
		{cfg.Providers.Cohere.APIKey, "coh-key"},
		{cfg.Providers.OpenRouter.APIKey, "or-key"},
		{cfg.Providers.AiHubMix.APIKey, "ahm-key"},
		{cfg.Providers.Custom.APIKey, "cust-key"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("expected %q, got %q", c.want, c.got)
		}
	}
}
