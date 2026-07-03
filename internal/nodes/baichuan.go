package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "baichuan",
		DefaultModel:    "Baichuan4",
		DefaultEndpoint: "https://api.baichuan-ai.com/v1",
		EnvAPIKey:       "BAICHUAN_API_KEY",
		ProviderName:    "Baichuan",
	}))
}
