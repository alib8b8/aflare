package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "minimax",
		DefaultModel:    "abab6.5s-chat",
		DefaultEndpoint: "https://api.minimax.chat/v1",
		EnvAPIKey:       "MINIMAX_API_KEY",
		ProviderName:    "MiniMax",
	}))
}
