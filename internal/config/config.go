package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type LLMProviderConfig struct {
	APIKey   string `yaml:"api_key,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

type Config struct {
	Providers     map[string]LLMProviderConfig `yaml:"providers,omitempty"`
	SafeMode      bool                         `yaml:"safe_mode,omitempty"`
	SecurityLevel string                       `yaml:"security_level,omitempty"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configErr    error
	configMu     sync.RWMutex
)

func LoadConfig() (*Config, error) {
	configMu.RLock()
	if globalConfig != nil {
		cfg := globalConfig
		configMu.RUnlock()
		return cfg, nil
	}
	configMu.RUnlock()

	configOnce.Do(func() {
		cfg := &Config{
			Providers: make(map[string]LLMProviderConfig),
		}
		configPaths := getConfigPaths()
		for _, path := range configPaths {
			if _, err := os.Stat(path); err == nil {
				data, err := os.ReadFile(path)
				if err != nil {
					// File exists but is not readable (e.g. permission denied);
					// skip it and try the next path. Only format errors abort loading.
					continue
				}
				if err := yaml.Unmarshal(data, cfg); err != nil {
					configErr = fmt.Errorf("failed to parse config file %s: %w", path, err)
					return
				}
				break
			}
		}
		configMu.Lock()
		globalConfig = cfg
		configMu.Unlock()
	})
	if configErr != nil {
		return nil, configErr
	}
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig, nil
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
	if envVal := os.Getenv("LLM_BOX_SAFE_MODE"); envVal != "" {
		lower := strings.ToLower(strings.TrimSpace(envVal))
		switch lower {
		case "false", "0", "no", "off", "disable", "disabled":
			return false
		}
		return true
	}

	cfg, err := LoadConfig()
	if err != nil {
		return false
	}

	return cfg.SafeMode
}

const (
	SecurityLevelL0 = "L0"
	SecurityLevelL1 = "L1"
	SecurityLevelL2 = "L2"
	SecurityLevelL3 = "L3"
)

func GetSecurityLevel() string {
	if envVal := os.Getenv("LLM_BOX_SECURITY_LEVEL"); envVal != "" {
		upper := strings.ToUpper(strings.TrimSpace(envVal))
		switch upper {
		case SecurityLevelL0, SecurityLevelL1, SecurityLevelL2, SecurityLevelL3:
			return upper
		}
	}

	cfg, err := LoadConfig()
	if err == nil && cfg.SecurityLevel != "" {
		upper := strings.ToUpper(strings.TrimSpace(cfg.SecurityLevel))
		switch upper {
		case SecurityLevelL0, SecurityLevelL1, SecurityLevelL2, SecurityLevelL3:
			return upper
		}
	}

	if IsSafeMode() {
		return SecurityLevelL3
	}
	return SecurityLevelL1
}

func SecurityLevelAtLeast(level string) bool {
	current := GetSecurityLevel()
	levels := map[string]int{SecurityLevelL0: 0, SecurityLevelL1: 1, SecurityLevelL2: 2, SecurityLevelL3: 3}
	curVal, ok1 := levels[current]
	reqVal, ok2 := levels[level]
	if !ok1 || !ok2 {
		return false
	}
	return curVal >= reqVal
}

func SetConfig(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = cfg
}

// resetForTesting resets the config state for unit tests.
// This is only safe to call in single-threaded test setup.
func resetForTesting() {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = nil
	configOnce = sync.Once{}
	configErr = nil
}
