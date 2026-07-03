package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "kimi",
		DefaultModel:    "moonshot-v1-8k",
		DefaultEndpoint: "https://api.moonshot.cn/v1",
		EnvAPIKey:       "KIMI_API_KEY",
		ProviderName:    "Kimi",
	}))
}
