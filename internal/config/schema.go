package config

// Config is the top-level configuration
type Config struct {
	Providers ProvidersConfig            `json:"providers"`
	Agents    AgentsConfig               `json:"agents"`
	Tools     ToolsConfig                `json:"tools"`
	Channels  ChannelsConfig             `json:"channels"`
	Gateway   GatewayConfig              `json:"gateway"`
	MCP       map[string]MCPServerConfig `json:"mcp"`
}

// ProvidersConfig holds API keys and settings for LLM providers
type ProvidersConfig struct {
	OpenAI     ProviderConfig `json:"openai"`
	Anthropic  ProviderConfig `json:"anthropic"`
	DeepSeek   ProviderConfig `json:"deepseek"`
	Moonshot   ProviderConfig `json:"moonshot"`
	Zhipu      ProviderConfig `json:"zhipu"`
	DashScope  ProviderConfig `json:"dashscope"`
	Groq       ProviderConfig `json:"groq"`
	XAI        ProviderConfig `json:"xai"`
	Mistral    ProviderConfig `json:"mistral"`
	Cohere     ProviderConfig `json:"cohere"`
	OpenRouter ProviderConfig `json:"openrouter"`
	AiHubMix   ProviderConfig `json:"aihubmix"`
	Custom     ProviderConfig `json:"custom"`
}

type ProviderConfig struct {
	APIKey       string            `json:"apiKey"`
	BaseURL      string            `json:"baseUrl"`
	DefaultModel string            `json:"defaultModel"`
	ExtraHeaders map[string]string `json:"extraHeaders"`
}

type AgentsConfig struct {
	Defaults AgentDefaults            `json:"defaults"`
	Named    map[string]AgentConfig   `json:"named"`
}

type AgentDefaults struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"maxTokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"maxToolIterations"`
	SystemPromptFile  string  `json:"systemPromptFile"`
}

type AgentConfig struct {
	Model             string  `json:"model,omitempty"`
	MaxTokens         int     `json:"maxTokens,omitempty"`
	Temperature       float64 `json:"temperature,omitempty"`
	MaxToolIterations int     `json:"maxToolIterations,omitempty"`
	SystemPromptFile  string  `json:"systemPromptFile,omitempty"`
}

type ToolsConfig struct {
	Enabled  []string `json:"enabled"`
	Disabled []string `json:"disabled"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
	Slack    SlackConfig    `json:"slack"`
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Feishu   FeishuConfig   `json:"feishu"`
	DingTalk DingTalkConfig `json:"dingtalk"`
	QQ       QQConfig       `json:"qq"`
	Email    EmailConfig    `json:"email"`
	Mochat   MochatConfig   `json:"mochat"`
}

type TelegramConfig struct {
	Token        string   `json:"token"`
	AllowedUsers []string `json:"allowedUsers"`
}

type DiscordConfig struct {
	Token        string   `json:"token"`
	AllowedUsers []string `json:"allowedUsers"`
}

type SlackConfig struct {
	BotToken     string   `json:"botToken"`
	AppToken     string   `json:"appToken"`
	AllowedUsers []string `json:"allowedUsers"`
}

type WhatsAppConfig struct {
	AccessToken   string   `json:"access_token"`
	PhoneNumberID string   `json:"phone_number_id"`
	VerifyToken   string   `json:"verify_token"`
	WebhookPort   int      `json:"webhook_port"`
	AllowedUsers  []string `json:"allowed_users"`
}

type FeishuConfig struct {
	AppID        string   `json:"appId"`
	AppSecret    string   `json:"appSecret"`
	AllowedUsers []string `json:"allowedUsers"`
}

type DingTalkConfig struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	AllowedUsers []string `json:"allowedUsers"`
}

type QQConfig struct {
	AppID        string   `json:"appId"`
	Token        string   `json:"token"`
	AppSecret    string   `json:"appSecret"`
	AllowedUsers []string `json:"allowedUsers"`
}

type EmailConfig struct {
	IMAPServer   string   `json:"imapServer"`
	SMTPServer   string   `json:"smtpServer"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	AllowedUsers []string `json:"allowedUsers"`
}

type MochatConfig struct {
	URL          string   `json:"url"`
	AllowedUsers []string `json:"allowedUsers"`
}

type GatewayConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type MCPServerConfig struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	ToolTimeout int               `json:"toolTimeout"` // seconds, default 30
}

// DefaultConfig returns a Config with sensible defaults applied.
func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         "~/.nanobot/workspace",
				Model:             "gpt-4o",
				MaxTokens:         4096,
				Temperature:       0.7,
				MaxToolIterations: 40,
			},
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}
}
