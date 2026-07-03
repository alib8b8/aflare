package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "internlm",
		DefaultModel:    "internlm3-latest",
		DefaultEndpoint: "https://internlm-chat.intern-ai.org.cn/api/v1",
		EnvAPIKey:       "INTERNLM_API_KEY",
		ProviderName:    "InternLM",
	}))
}
