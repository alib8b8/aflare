package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "mistral",
		DefaultModel:    "mistral-large-latest",
		DefaultEndpoint: "https://api.mistral.ai/v1",
		EnvAPIKey:       "MISTRAL_API_KEY",
		ProviderName:    "Mistral",
	}))
}
