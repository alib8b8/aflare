package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "xverse",
		DefaultModel:    "XVERSE-7B-Chat",
		DefaultEndpoint: "https://api.xverse.cn/v1",
		EnvAPIKey:       "XVERSE_API_KEY",
		ProviderName:    "XVERSE",
	}))
}
