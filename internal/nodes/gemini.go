package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "gemini",
		DefaultModel:    "gemini-2.0-flash",
		DefaultEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai",
		EnvAPIKey:       "GEMINI_API_KEY",
		ProviderName:    "Google Gemini",
	}))
}
