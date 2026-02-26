package providers

import "strings"

type ProviderSpec struct {
	Name              string
	Keywords          []string          // model name keywords for matching
	EnvKey            string            // environment variable for API key
	DefaultAPIBase    string            // default base URL
	IsGateway         bool              // multi-provider gateway (OpenRouter, AiHubMix)
	IsLocal           bool              // local inference (Ollama, vLLM)
	IsDirect          bool              // bypass litellm, use direct HTTP
	IsOAuth           bool              // OAuth authentication
	DetectByKeyPrefix string            // detect by API key prefix (e.g. "sk-or-" for OpenRouter)
	DetectByBaseKW    string            // detect by base URL keyword
	ModelPrefix       string            // prefix to add to model name
	SkipPrefixes      []string          // prefixes to skip when adding ModelPrefix
	PromptCaching     bool              // supports prompt caching
	ModelOverrides    map[string]map[string]any // per-model parameter overrides
}

// Providers is the complete registry of known LLM providers
var Providers = []ProviderSpec{
	{Name: "openrouter", Keywords: []string{"openrouter"}, EnvKey: "OPENROUTER_API_KEY", DefaultAPIBase: "https://openrouter.ai/api/v1", IsGateway: true, DetectByKeyPrefix: "sk-or-"},
	{Name: "aihubmix", Keywords: []string{"aihubmix"}, EnvKey: "AIHUBMIX_API_KEY", DefaultAPIBase: "https://aihubmix.com/v1", IsGateway: true, DetectByKeyPrefix: "sk-aihub"},
	{Name: "anthropic", Keywords: []string{"claude", "anthropic"}, EnvKey: "ANTHROPIC_API_KEY", PromptCaching: true},
	{Name: "openai", Keywords: []string{"gpt", "o1", "o3", "chatgpt"}, EnvKey: "OPENAI_API_KEY", PromptCaching: true},
	{Name: "deepseek", Keywords: []string{"deepseek"}, EnvKey: "DEEPSEEK_API_KEY", DefaultAPIBase: "https://api.deepseek.com/v1"},
	{Name: "moonshot", Keywords: []string{"moonshot", "kimi"}, EnvKey: "MOONSHOT_API_KEY", DefaultAPIBase: "https://api.moonshot.cn/v1"},
	{Name: "zhipu", Keywords: []string{"glm", "zhipu"}, EnvKey: "ZHIPUAI_API_KEY", DefaultAPIBase: "https://open.bigmodel.cn/api/paas/v4"},
	{Name: "dashscope", Keywords: []string{"qwen", "dashscope"}, EnvKey: "DASHSCOPE_API_KEY", DefaultAPIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{Name: "minimax", Keywords: []string{"minimax", "abab"}, EnvKey: "MINIMAX_API_KEY", DefaultAPIBase: "https://api.minimax.chat/v1"},
	{Name: "stepfun", Keywords: []string{"step"}, EnvKey: "STEPFUN_API_KEY", DefaultAPIBase: "https://api.stepfun.com/v1"},
	{Name: "groq", Keywords: []string{"groq"}, EnvKey: "GROQ_API_KEY", DefaultAPIBase: "https://api.groq.com/openai/v1"},
	{Name: "xai", Keywords: []string{"grok", "xai"}, EnvKey: "XAI_API_KEY", DefaultAPIBase: "https://api.x.ai/v1"},
	{Name: "mistral", Keywords: []string{"mistral", "mixtral", "codestral"}, EnvKey: "MISTRAL_API_KEY", DefaultAPIBase: "https://api.mistral.ai/v1"},
	{Name: "cohere", Keywords: []string{"command"}, EnvKey: "COHERE_API_KEY", DefaultAPIBase: "https://api.cohere.com/v2"},
	{Name: "gemini", Keywords: []string{"gemini"}, EnvKey: "GOOGLE_API_KEY"},
	{Name: "ollama", Keywords: []string{"ollama"}, DefaultAPIBase: "http://localhost:11434/v1", IsLocal: true, DetectByBaseKW: "11434"},
	{Name: "vllm", Keywords: []string{"vllm"}, IsLocal: true, IsGateway: true, DetectByBaseKW: "vllm"},
	{Name: "codex", Keywords: []string{"codex"}, IsOAuth: true, IsDirect: true},
	{Name: "custom", IsDirect: true},
}

// FindByModel matches model name against Keywords, returns first match.
func FindByModel(model string) *ProviderSpec {
	lower := strings.ToLower(model)
	for i := range Providers {
		for _, kw := range Providers[i].Keywords {
			if strings.Contains(lower, kw) {
				return &Providers[i]
			}
		}
	}
	return nil
}

// FindGateway detects a gateway provider by API key prefix or base URL keyword.
func FindGateway(apiKey, baseURL string) *ProviderSpec {
	for i := range Providers {
		spec := &Providers[i]
		if spec.DetectByKeyPrefix != "" && strings.HasPrefix(apiKey, spec.DetectByKeyPrefix) {
			return spec
		}
		if spec.DetectByBaseKW != "" && strings.Contains(baseURL, spec.DetectByBaseKW) {
			return spec
		}
	}
	return nil
}

// FindByName returns the provider spec with an exact name match.
func FindByName(name string) *ProviderSpec {
	for i := range Providers {
		if Providers[i].Name == name {
			return &Providers[i]
		}
	}
	return nil
}
