package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadFromReader(t *testing.T) {
	jsonData := `{
		"providers": {
			"openai": {
				"apiKey": "sk-test123",
				"baseUrl": "https://api.openai.com/v1",
				"defaultModel": "gpt-4"
			}
		},
		"agents": {
			"defaults": {
				"workspace": "/tmp/workspace",
				"model": "gpt-3.5-turbo",
				"maxTokens": 2048,
				"temperature": 0.5,
				"maxToolIterations": 20
			}
		},
		"gateway": {
			"host": "127.0.0.1",
			"port": 9090
		}
	}`

	cfg, err := LoadFromReader(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	if cfg.Providers.OpenAI.APIKey != "sk-test123" {
		t.Errorf("expected apiKey sk-test123, got %s", cfg.Providers.OpenAI.APIKey)
	}
	if cfg.Agents.Defaults.Model != "gpt-3.5-turbo" {
		t.Errorf("expected model gpt-3.5-turbo, got %s", cfg.Agents.Defaults.Model)
	}
	if cfg.Gateway.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Gateway.Port)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Agents.Defaults.Workspace != "~/.nanobot/workspace" {
		t.Errorf("expected workspace ~/.nanobot/workspace, got %s", cfg.Agents.Defaults.Workspace)
	}
	if cfg.Agents.Defaults.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", cfg.Agents.Defaults.Model)
	}
	if cfg.Agents.Defaults.MaxTokens != 4096 {
		t.Errorf("expected maxTokens 4096, got %d", cfg.Agents.Defaults.MaxTokens)
	}
	if cfg.Agents.Defaults.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", cfg.Agents.Defaults.Temperature)
	}
	if cfg.Agents.Defaults.MaxToolIterations != 40 {
		t.Errorf("expected maxToolIterations 40, got %d", cfg.Agents.Defaults.MaxToolIterations)
	}
	if cfg.Gateway.Host != "0.0.0.0" {
		t.Errorf("expected gateway host 0.0.0.0, got %s", cfg.Gateway.Host)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("expected gateway port 8080, got %d", cfg.Gateway.Port)
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("NANOBOT_PROVIDERS_OPENAI_APIKEY", "env-key-123")
	defer os.Unsetenv("NANOBOT_PROVIDERS_OPENAI_APIKEY")

	jsonData := `{
		"providers": {
			"openai": {
				"apiKey": "file-key-456"
			}
		}
	}`

	cfg, err := LoadFromReader(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	if cfg.Providers.OpenAI.APIKey != "env-key-123" {
		t.Errorf("expected env override env-key-123, got %s", cfg.Providers.OpenAI.APIKey)
	}
}

func TestMissingFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestPartialConfig(t *testing.T) {
	jsonData := `{
		"providers": {
			"openai": {
				"apiKey": "partial-key"
			}
		}
	}`

	cfg, err := LoadFromReader(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	// Verify partial config was loaded
	if cfg.Providers.OpenAI.APIKey != "partial-key" {
		t.Errorf("expected apiKey partial-key, got %s", cfg.Providers.OpenAI.APIKey)
	}

	// Verify defaults were applied for missing fields
	if cfg.Agents.Defaults.Model != "gpt-4o" {
		t.Errorf("expected default model gpt-4o, got %s", cfg.Agents.Defaults.Model)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Gateway.Port)
	}
}
