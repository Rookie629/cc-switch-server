package preset

import "github.com/Rookie629/cc-switch-server/internal/store"

// Preset defines a pre-configured provider template.
type Preset struct {
	Name     string            `json:"name"`
	Type     store.ProviderType `json:"type"`
	BaseURL  string            `json:"base_url"`
	Models   []string          `json:"models"`
	Default  string            `json:"default"`
}

// All returns all built-in provider presets.
func All() []Preset {
	return []Preset{
		// Anthropic official
		{
			Name:    "anthropic-official",
			Type:    store.TypeAnthropic,
			BaseURL: "https://api.anthropic.com",
			Models:  []string{"claude-opus-4-8", "claude-sonnet-4-6", "claude-haiku-4-5"},
			Default: "claude-sonnet-4-6",
		},
		// DeepSeek
		{
			Name:    "deepseek",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.deepseek.com",
			Models:  []string{"deepseek-chat", "deepseek-reasoner"},
			Default: "deepseek-chat",
		},
		// OpenAI official
		{
			Name:    "openai",
			Type:    store.TypeOpenAI,
			BaseURL: "https://api.openai.com",
			Models:  []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini"},
			Default: "gpt-4o",
		},
		// AWS Bedrock (Anthropic)
		{
			Name:    "aws-bedrock-claude",
			Type:    store.TypeAnthropic,
			BaseURL: "https://bedrock-runtime.us-east-1.amazonaws.com",
			Models:  []string{"claude-opus-4-8", "claude-sonnet-4-6"},
			Default: "claude-sonnet-4-6",
		},
		// NVIDIA NIM
		{
			Name:    "nvidia-nim",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://integrate.api.nvidia.com",
			Models:  []string{"nvidia/llama-3.1-nemotron-70b-instruct"},
			Default: "nvidia/llama-3.1-nemotron-70b-instruct",
		},
		// Groq
		{
			Name:    "groq",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.groq.com/openai",
			Models:  []string{"llama-3.1-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768"},
			Default: "llama-3.1-70b-versatile",
		},
		// Together AI
		{
			Name:    "together",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.together.xyz",
			Models:  []string{"meta-llama/Llama-3.1-405B-Instruct-Turbo", "meta-llama/Llama-3.1-70B-Instruct-Turbo"},
			Default: "meta-llama/Llama-3.1-70B-Instruct-Turbo",
		},
		// Fireworks AI
		{
			Name:    "fireworks",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.fireworks.ai/inference",
			Models:  []string{"accounts/fireworks/models/llama-v3p1-70b-instruct"},
			Default: "accounts/fireworks/models/llama-v3p1-70b-instruct",
		},
		// Moonshot (Kimi)
		{
			Name:    "moonshot",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.moonshot.cn",
			Models:  []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
			Default: "moonshot-v1-32k",
		},
		// Zhipu (GLM)
		{
			Name:    "zhipu",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://open.bigmodel.cn/api/paas/v4",
			Models:  []string{"glm-4-plus", "glm-4-flash"},
			Default: "glm-4-plus",
		},
		// Qwen / Alibaba Bailian
		{
			Name:    "qwen",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Models:  []string{"qwen-max", "qwen-plus", "qwen-turbo"},
			Default: "qwen-plus",
		},
		// Baidu ERNIE
		{
			Name:    "baidu-ernie",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat",
			Models:  []string{"ernie-4.0-turbo-8k", "ernie-3.5-8k"},
			Default: "ernie-4.0-turbo-8k",
		},
		// XAI Grok
		{
			Name:    "xai-grok",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.x.ai",
			Models:  []string{"grok-2", "grok-2-mini"},
			Default: "grok-2",
		},
		// OpenRouter
		{
			Name:    "openrouter",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://openrouter.ai/api",
			Models:  []string{"anthropic/claude-sonnet-4-6", "google/gemini-2.5-pro", "openai/gpt-4o"},
			Default: "anthropic/claude-sonnet-4-6",
		},
		// Silicon Flow
		{
			Name:    "siliconflow",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "https://api.siliconflow.cn",
			Models:  []string{"deepseek-ai/DeepSeek-V3", "Qwen/Qwen2.5-72B-Instruct"},
			Default: "deepseek-ai/DeepSeek-V3",
		},
		// Ollama (local)
		{
			Name:    "ollama",
			Type:    store.TypeOpenAICompatible,
			BaseURL: "http://localhost:11434",
			Models:  []string{"llama3.1:70b", "qwen2.5:72b", "deepseek-r1:70b"},
			Default: "llama3.1:70b",
		},
	}
}

// FindPreset finds a preset by name.
func FindPreset(name string) *Preset {
	for _, p := range All() {
		if p.Name == name {
			return &p
		}
	}
	return nil
}
