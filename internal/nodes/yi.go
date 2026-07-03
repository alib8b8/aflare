package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "yi",
		DefaultModel:    "yi-lightning",
		DefaultEndpoint: "https://api.lingyiwanwu.com/v1",
		EnvAPIKey:       "YI_API_KEY",
		ProviderName:    "Yi",
	}))
}
