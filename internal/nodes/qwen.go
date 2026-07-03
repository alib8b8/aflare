package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "qwen",
		DefaultModel:    "qwen-turbo",
		DefaultEndpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		EnvAPIKey:       "QWEN_API_KEY",
		ProviderName:    "Qwen",
	}))
}
