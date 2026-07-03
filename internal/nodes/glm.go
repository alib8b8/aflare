package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "glm",
		DefaultModel:    "glm-4",
		DefaultEndpoint: "https://open.bigmodel.cn/api/paas/v4",
		EnvAPIKey:       "GLM_API_KEY",
		ProviderName:    "GLM",
	}))
}
