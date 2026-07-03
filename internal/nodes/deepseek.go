package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "deepseek",
		DefaultModel:    "deepseek-chat",
		DefaultEndpoint: "https://api.deepseek.com/v1",
		EnvAPIKey:       "DEEPSEEK_API_KEY",
		ProviderName:    "DeepSeek",
	}))
}
