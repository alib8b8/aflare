package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "mimo",
		DefaultModel:    "mimo-v2.5-pro",
		DefaultEndpoint: "https://api.xiaomimimo.com/v1",
		EnvAPIKey:       "MIMO_API_KEY",
		ProviderName:    "MiMo",
	}))
}
