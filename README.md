# Nanobot

轻量级、数据驱动的 AI 助手框架，Go 语言实现。通过一份配置文件，将任意 LLM 接入任意聊天平台。

## 特性

- **18+ LLM 提供商** — OpenAI、Anthropic、DeepSeek、Moonshot、Groq、Ollama 等，支持自动检测
- **9 个聊天渠道** — Telegram、Discord、Slack、WhatsApp、飞书、钉钉、QQ、邮件、Mochat
- **MCP 工具协议** — 通过 stdio JSON-RPC 连接外部工具服务器
- **内置工具** — Shell 执行、文件操作、网页抓取、定时任务、子 Agent 派生
- **会话管理** — JSONL 追加存储 + LLM 驱动的记忆整合
- **多模态** — 支持图片 URL 和 base64 编码媒体

## 快速开始

### 1. 安装

```bash
# 从源码编译
git clone https://github.com/ajiany/nanobot-golang.git
cd nanobot-golang
make build

# 编译产物在 bin/nanobot
```

### 2. 创建配置文件

配置文件路径：`~/.nanobot/config.json`

```bash
mkdir -p ~/.nanobot
```

最小配置（使用 OpenAI）：

```json
{
  "providers": {
    "openai": {
      "apiKey": "sk-your-api-key"
    }
  },
  "agents": {
    "defaults": {
      "model": "gpt-4o",
      "workspace": "~/.nanobot/workspace"
    }
  }
}
```

也可以通过环境变量设置：

```bash
export NANOBOT_PROVIDERS_OPENAI_APIKEY="sk-your-api-key"
export NANOBOT_AGENTS_DEFAULTS_MODEL="gpt-4o"
```

### 3. 运行

Nanobot 有两种运行模式：

#### Agent 模式（单次对话）

```bash
# 交互式对话
bin/nanobot agent

# 直接提问
bin/nanobot agent -m "帮我写一个快速排序"

# 指定模型
bin/nanobot agent -m "你好" --model claude-sonnet-4-20250514
```

#### Gateway 模式（多渠道网关）

```bash
# 启动网关，监听所有已配置的聊天渠道
bin/nanobot gateway

# 指定配置文件
bin/nanobot gateway -c /path/to/config.json
```

Gateway 模式会同时启动所有已配置的聊天渠道（Telegram、Discord 等），接收消息后调用 LLM 处理并回复。

#### 查看状态

```bash
bin/nanobot status
```

输出当前配置的模型、工作目录、各渠道启用状态。

## 完整配置说明

```json
{
  "providers": {
    "openai": {
      "apiKey": "sk-xxx",
      "baseUrl": "https://api.openai.com/v1"
    },
    "anthropic": {
      "apiKey": "sk-ant-xxx"
    },
    "deepseek": {
      "apiKey": "sk-xxx"
    },
    "moonshot": { "apiKey": "sk-xxx" },
    "groq": { "apiKey": "gsk_xxx" },
    "openrouter": { "apiKey": "sk-or-xxx" },
    "ollama": {
      "baseUrl": "http://localhost:11434/v1"
    },
    "custom": {
      "apiKey": "xxx",
      "baseUrl": "https://your-api.example.com/v1"
    }
  },

  "agents": {
    "defaults": {
      "model": "gpt-4o",
      "workspace": "~/.nanobot/workspace",
      "maxTokens": 4096,
      "temperature": 0.7,
      "maxToolIterations": 40,
      "systemPrompt": "你是一个有用的助手。",
      "skills": ["~/.nanobot/skills"]
    }
  },

  "channels": {
    "telegram": {
      "token": "123456:ABC-DEF",
      "allowedUsers": ["user_id_1"]
    },
    "discord": {
      "token": "your-bot-token",
      "allowedUsers": ["user_id_1"]
    },
    "slack": {
      "botToken": "xoxb-xxx",
      "appToken": "xapp-xxx",
      "allowedUsers": ["U12345"]
    },
    "whatsapp": {
      "access_token": "your-token",
      "phone_number_id": "12345",
      "verify_token": "your-verify-token",
      "webhook_port": 8443
    },
    "feishu": {
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "webhookPort": 9000
    },
    "dingtalk": {
      "clientId": "xxx",
      "clientSecret": "xxx",
      "webhookPort": 9001
    },
    "qq": {
      "appId": "xxx",
      "token": "xxx",
      "appSecret": "xxx",
      "webhookPort": 9002
    },
    "email": {
      "imapServer": "imap.gmail.com:993",
      "smtpServer": "smtp.gmail.com:587",
      "username": "you@gmail.com",
      "password": "app-password",
      "allowedUsers": ["sender@example.com"]
    },
    "mochat": {
      "url": "http://localhost:3000"
    }
  },

  "mcp": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "toolTimeout": 30
    },
    "custom-server": {
      "command": "/path/to/mcp-server",
      "env": { "API_KEY": "xxx" }
    }
  },

  "gateway": {
    "host": "0.0.0.0",
    "port": 8080
  }
}
```

## 模型自动检测

Nanobot 会根据 API Key 前缀或 Base URL 自动选择提供商：

| 前缀/关键词 | 提供商 |
|-------------|--------|
| `sk-ant-` | Anthropic |
| `sk-or-` | OpenRouter |
| URL 含 `11434` | Ollama |
| URL 含 `deepseek` | DeepSeek |

也可以在 model 名中直接指定：`deepseek/deepseek-chat`、`anthropic/claude-sonnet-4-20250514`。

## 内置工具

Agent 模式下自动注册以下工具：

| 工具 | 说明 |
|------|------|
| `run_shell` | 执行 Shell 命令 |
| `read_file` | 读取文件内容 |
| `write_file` | 写入文件 |
| `web_get` | 抓取网页内容（自动去 HTML 标签） |
| `send_message` | 向指定渠道发送消息 |
| `spawn_agent` | 派生子 Agent 处理子任务 |
| `schedule_cron` | 创建定时任务 |

## MCP 工具

MCP（Model Context Protocol）允许通过 stdio 连接外部工具服务器。配置后工具会自动发现并注册，命名格式为 `mcp_{服务名}_{工具名}`。

```bash
# 示例：连接文件系统 MCP 服务器后，Agent 可使用：
# mcp_filesystem_read_file, mcp_filesystem_write_file 等工具
```

## 工作目录结构

```
~/.nanobot/
├── config.json          # 主配置文件
├── workspace/           # Agent 工作目录
│   ├── AGENTS.md        # Agent 角色定义（可选）
│   ├── SOUL.md          # 人格设定（可选）
│   ├── USER.md          # 用户信息（可选）
│   ├── TOOLS.md         # 工具使用指南（可选）
│   ├── IDENTITY.md      # 身份描述（可选）
│   ├── MEMORY.md        # LLM 自动维护的长期记忆
│   ├── HISTORY.md       # 对话历史时间线
│   └── sessions/        # 会话存储（JSONL）
└── skills/              # 技能文件目录（Markdown）
```

启动时，Agent 会读取工作目录下的 `AGENTS.md`、`SOUL.md`、`USER.md`、`TOOLS.md`、`IDENTITY.md` 拼接为系统提示词。

## 项目结构

```
cmd/nanobot/         CLI 入口（agent、gateway、status 命令）
internal/
  agent/             AgentLoop、上下文构建、记忆、技能、子 Agent
  bus/               消息总线（入站/出站 hub-and-spoke）
  channels/          聊天平台适配器
  config/            配置加载（JSON + 环境变量）
  cron/              定时任务调度器
  heartbeat/         健康检查服务
  providers/         LLM 提供商适配器
  session/           JSONL 会话存储
  tools/             工具注册表、MCP 客户端、内置工具
```

## 构建

```bash
make build          # 编译到 bin/nanobot
make test           # 运行测试
make cross          # 交叉编译（linux/darwin × amd64/arm64）
make docker         # 构建 Docker 镜像
make clean          # 清理构建产物
```

## Docker

```bash
# 构建镜像
docker build -t nanobot .

# 运行（挂载配置文件）
docker run -v ~/.nanobot:/root/.nanobot nanobot gateway
```

## 许可证

MIT
