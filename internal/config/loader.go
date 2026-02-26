package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Load loads config from the default path (~/.nanobot/config.json).
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	return LoadFromFile(filepath.Join(home, ".nanobot", "config.json"))
}

// LoadFromFile loads config from a specific file path.
func LoadFromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer f.Close()
	return LoadFromReader(f)
}

// LoadFromReader loads config from an io.Reader, applying defaults and env overrides.
func LoadFromReader(r io.Reader) (*Config, error) {
	cfg := DefaultConfig()

	if err := json.NewDecoder(r).Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyEnvOverrides(cfg)
	expandWorkspacePath(cfg)

	return cfg, nil
}

// applyEnvOverrides applies NANOBOT_-prefixed environment variable overrides.
func applyEnvOverrides(cfg *Config) {
	envMap := map[string]*string{
		"NANOBOT_PROVIDERS_OPENAI_APIKEY":     &cfg.Providers.OpenAI.APIKey,
		"NANOBOT_PROVIDERS_ANTHROPIC_APIKEY":  &cfg.Providers.Anthropic.APIKey,
		"NANOBOT_PROVIDERS_DEEPSEEK_APIKEY":   &cfg.Providers.DeepSeek.APIKey,
		"NANOBOT_PROVIDERS_MOONSHOT_APIKEY":   &cfg.Providers.Moonshot.APIKey,
		"NANOBOT_PROVIDERS_ZHIPU_APIKEY":      &cfg.Providers.Zhipu.APIKey,
		"NANOBOT_PROVIDERS_DASHSCOPE_APIKEY":  &cfg.Providers.DashScope.APIKey,
		"NANOBOT_PROVIDERS_GROQ_APIKEY":       &cfg.Providers.Groq.APIKey,
		"NANOBOT_PROVIDERS_XAI_APIKEY":        &cfg.Providers.XAI.APIKey,
		"NANOBOT_PROVIDERS_MISTRAL_APIKEY":    &cfg.Providers.Mistral.APIKey,
		"NANOBOT_PROVIDERS_COHERE_APIKEY":     &cfg.Providers.Cohere.APIKey,
		"NANOBOT_PROVIDERS_OPENROUTER_APIKEY": &cfg.Providers.OpenRouter.APIKey,
		"NANOBOT_PROVIDERS_AIHUBMIX_APIKEY":   &cfg.Providers.AiHubMix.APIKey,
		"NANOBOT_PROVIDERS_CUSTOM_APIKEY":     &cfg.Providers.Custom.APIKey,
		"NANOBOT_AGENTS_DEFAULTS_MODEL":       &cfg.Agents.Defaults.Model,
		"NANOBOT_AGENTS_DEFAULTS_WORKSPACE":   &cfg.Agents.Defaults.Workspace,
	}

	for env, ptr := range envMap {
		if val := os.Getenv(env); val != "" {
			*ptr = val
		}
	}
}

// expandWorkspacePath expands a leading ~ in the workspace path.
func expandWorkspacePath(cfg *Config) {
	ws := cfg.Agents.Defaults.Workspace
	if len(ws) >= 2 && ws[0] == '~' && ws[1] == '/' {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Agents.Defaults.Workspace = filepath.Join(home, ws[2:])
		}
	}
}
