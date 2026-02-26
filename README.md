# Nanobot

A lightweight, data-driven AI assistant framework in Go. Connect any LLM provider to any chat platform with minimal configuration.

## Features

- **18+ LLM Providers** — OpenAI, Anthropic, DeepSeek, Moonshot, Groq, Ollama, and more via auto-detection
- **9 Chat Channels** — Telegram, Discord, Slack, WhatsApp, Feishu, DingTalk, QQ, Email, Mochat
- **MCP Tool Support** — Connect external tools via Model Context Protocol (stdio JSON-RPC)
- **Built-in Tools** — Shell execution, file operations, web fetch, cron scheduling, sub-agent spawning
- **Session Management** — JSONL append-only storage with LLM-driven memory consolidation
- **Multimodal** — Image URL and base64-encoded media support for vision models
- **Single Binary** — Zero runtime dependencies, cross-compiles to Linux/macOS (amd64/arm64)

## Quick Start

```bash
# Build
make build

# Run a single conversation
export OPENAI_API_KEY=sk-...
nanobot agent -m "Hello, what can you do?"

# Run with a specific model
nanobot agent -m "Explain Go interfaces" --model gpt-4o

# Start gateway mode (multi-channel)
nanobot gateway --config config.json
```

## Configuration

Create a `config.json`:

```json
{
  "provider": "openai",
  "model": "gpt-4o",
  "workspace": "./workspace",
  "channels": {
    "telegram": {
      "token": "BOT_TOKEN",
      "allowedUsers": ["user_id"]
    }
  }
}
```

Supported environment variables for API keys:

| Provider | Env Variable |
|----------|-------------|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Groq | `GROQ_API_KEY` |
| Moonshot | `MOONSHOT_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |

## Architecture

```
cmd/nanobot/         CLI entry point (agent, gateway, status)
internal/
  agent/             AgentLoop, context builder, memory, skills, sub-agents
  bus/               Hub-and-spoke MessageBus (inbound/outbound channels)
  channels/          Chat platform adapters (Telegram, Discord, Slack, ...)
  config/            YAML/JSON config loader
  cron/              Cron scheduler for periodic tasks
  heartbeat/         Health check service
  providers/         LLM provider adapters (OpenAI-compat, Anthropic SDK, Codex OAuth)
  session/           JSONL session storage
  tools/             Tool registry, MCP client, built-in tools
```

## Building

```bash
# Standard build
make build

# Cross-compile all targets
make cross

# Docker
make docker

# Run tests
make test
```

## MCP Tools

Add MCP servers to your config:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}
```

Tools are auto-discovered and registered with the prefix `mcp_{server}_{tool}`.

## License

MIT
