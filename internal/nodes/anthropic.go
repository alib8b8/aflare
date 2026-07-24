package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "anthropic",
		DefaultModel:    "claude-3-5-sonnet-latest",
		DefaultEndpoint: "https://api.anthropic.com/v1",
		EnvAPIKey:       "ANTHROPIC_API_KEY",
		ProviderName:    "Anthropic",
	}))
}
