package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LLMProviderConfig struct {
	APIKey   string `yaml:"api_key,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

type Config struct {
	Providers map[string]LLMProviderConfig `yaml:"providers,omitempty"`
	SafeMode  bool                        `yaml:"safe_mode,omitempty"`
}

var globalConfig *Config

func LoadConfig() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	cfg := &Config{
		Providers: make(map[string]LLMProviderConfig),
	}

	configPaths := getConfigPaths()
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
			}
			break
		}
	}

	globalConfig = cfg
	return cfg, nil
}

func getConfigPaths() []string {
	var paths []string

	if envPath := os.Getenv("LLM_BOX_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, "llm-box.yaml"))
		paths = append(paths, filepath.Join(cwd, ".llm-box.yaml"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "llm-box", "config.yaml"))
		paths = append(paths, filepath.Join(home, ".llm-box.yaml"))
	}

	return paths
}

func GetProviderConfig(provider string) LLMProviderConfig {
	cfg, err := LoadConfig()
	if err != nil {
		return LLMProviderConfig{}
	}

	provider = strings.ToLower(provider)
	if p, ok := cfg.Providers[provider]; ok {
		return p
	}

	return LLMProviderConfig{}
}

func GetAPIKey(provider, envVar string) string {
	if apiKey := os.Getenv(envVar); apiKey != "" {
		return apiKey
	}

	pcfg := GetProviderConfig(provider)
	if pcfg.APIKey != "" {
		return pcfg.APIKey
	}

	return ""
}

func GetEndpoint(provider, envVar, defaultEndpoint string) string {
	if endpoint := os.Getenv(envVar); endpoint != "" {
		return endpoint
	}

	pcfg := GetProviderConfig(provider)
	if pcfg.Endpoint != "" {
		return pcfg.Endpoint
	}

	return defaultEndpoint
}

func GetDefaultModel(provider, envVar, defaultModel string) string {
	if model := os.Getenv(envVar); model != "" {
		return model
	}

	pcfg := GetProviderConfig(provider)
	if pcfg.Model != "" {
		return pcfg.Model
	}

	return defaultModel
}

func IsSafeMode() bool {
	if os.Getenv("LLM_BOX_SAFE_MODE") != "" {
		return true
	}

	cfg, err := LoadConfig()
	if err != nil {
		return false
	}

	return cfg.SafeMode
}

func SetConfig(cfg *Config) {
	globalConfig = cfg
}
