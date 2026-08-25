// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​​​‌​‌‌‌​​​​​‌​‌​‌​‌​‌​‌‌​​​​​‌​​‌‌‌‌‌‌‌​‌​‌​​​​​​​​​​​​​​​​‌​‌​‌​​‌‌‌‌​​‌​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/agentx"
)

type LLMProviderConfig struct {
	APIKey   string `yaml:"api_key,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

type RouterProviderEntry struct {
	Name      string  `yaml:"name"`
	Model     string  `yaml:"model,omitempty"`
	Priority  int     `yaml:"priority,omitempty"`
	CostPer1K float64 `yaml:"cost_per_1k,omitempty"`
	Quota     int64   `yaml:"quota_daily,omitempty"`
	Enabled   bool    `yaml:"enabled,omitempty"`
}

type RouterConfig struct {
	Strategy       string                `yaml:"strategy,omitempty"`
	MaxRetries     int                   `yaml:"max_retries,omitempty"`
	FallbackOrder  []RouterProviderEntry `yaml:"providers,omitempty"`
	EnableFallback bool                  `yaml:"enable_fallback,omitempty"`
}

type Config struct {
	Providers     map[string]LLMProviderConfig `yaml:"providers,omitempty"`
	Router        RouterConfig                 `yaml:"router,omitempty"`
	SafeMode      bool                         `yaml:"safe_mode,omitempty"`
	SecurityLevel string                       `yaml:"security_level,omitempty"`

	// Agents registers external agents aflare can command (CLI or A2A).
	// Built-in presets (codex/claude/gemini) can be overridden by name.
	Agents map[string]agentx.AgentDef `yaml:"agents,omitempty"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configErr    error
	configMu     sync.RWMutex
)

// LoadConfig 加载全局配置，首次调用时按环境变量、当前目录与用户主目录顺序查找并解析 YAML。
// 后续调用直接返回缓存的配置实例。
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
				data, err := os.ReadFile(path) // #nosec G304 -- path from internal config search list
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
		if err := cfg.Validate(); err != nil {
			configErr = fmt.Errorf("invalid config: %w", err)
			return
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

	if envPath := os.Getenv("AFLARE_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, "aflare.yaml"))
		paths = append(paths, filepath.Join(cwd, ".aflare.yaml"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "aflare", "config.yaml"))
		paths = append(paths, filepath.Join(home, ".aflare.yaml"))
	}

	return paths
}

// GetProviderConfig 返回指定 provider 的配置，未配置时返回空实例。
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

// GetAPIKey 优先从环境变量获取 API Key，其次从配置文件，未找到返回空字符串。
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

// GetEndpoint 依次从环境变量、配置文件、默认值获取服务端点。
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

// GetDefaultModel 依次从环境变量、配置文件、默认值获取默认模型名称。
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

// IsSafeMode 判断是否启用安全模式，优先读取环境变量 AFLARE_SAFE_MODE。
func IsSafeMode() bool {
	if envVal := os.Getenv("AFLARE_SAFE_MODE"); envVal != "" {
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

// GetSecurityLevel 返回当前安全等级（L0/L1/L2/L3）。
// 优先级：环境变量 > 配置文件 > 默认值（safe mode 为 L3，否则 L1）。
func GetSecurityLevel() string {
	if envVal := os.Getenv("AFLARE_SECURITY_LEVEL"); envVal != "" {
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

// SecurityLevelAtLeast 判断当前安全等级是否不低于给定等级。
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

// GetRouterConfig 返回路由配置，加载失败时返回空实例。
func GetRouterConfig() RouterConfig {
	cfg, err := LoadConfig()
	if err != nil {
		return RouterConfig{}
	}
	return cfg.Router
}

const (
	RouterStrategyPriority   = "priority"
	RouterStrategyCost       = "cost"
	RouterStrategyLatency    = "latency"
	RouterStrategyRoundRobin = "round_robin"
	RouterStrategyRandom     = "random"
	RouterStrategyPareto     = "pareto"
)

// SetConfig 替换全局配置实例，主要用于测试与运行时注入。
func SetConfig(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = cfg
}

// Validate 校验配置字段合法性，包括安全等级、路由策略与 provider 字段约束。
// 同时确保 Providers map 已初始化。
func (c *Config) Validate() error {
	if c.Providers == nil {
		c.Providers = make(map[string]LLMProviderConfig)
	}

	validLevels := map[string]bool{
		"": true, SecurityLevelL0: true, SecurityLevelL1: true,
		SecurityLevelL2: true, SecurityLevelL3: true,
	}
	if !validLevels[c.SecurityLevel] {
		return fmt.Errorf("invalid security_level %q (must be L0, L1, L2, or L3)", c.SecurityLevel)
	}

	if c.Router.Strategy != "" {
		validStrategies := map[string]bool{
			RouterStrategyPriority: true, RouterStrategyCost: true,
			RouterStrategyLatency: true, RouterStrategyRoundRobin: true,
			RouterStrategyRandom: true, RouterStrategyPareto: true,
		}
		if !validStrategies[c.Router.Strategy] {
			return fmt.Errorf("invalid router strategy %q", c.Router.Strategy)
		}
	}

	if c.Router.MaxRetries < 0 {
		return fmt.Errorf("router max_retries must be >= 0, got %d", c.Router.MaxRetries)
	}

	for i, p := range c.Router.FallbackOrder {
		if p.Name == "" {
			return fmt.Errorf("router provider[%d]: name is required", i)
		}
		if p.Priority < 0 {
			return fmt.Errorf("router provider[%d] %q: priority must be >= 0", i, p.Name)
		}
		if p.CostPer1K < 0 {
			return fmt.Errorf("router provider[%d] %q: cost_per_1k must be >= 0", i, p.Name)
		}
		if p.Quota < 0 {
			return fmt.Errorf("router provider[%d] %q: quota_daily must be >= 0", i, p.Name)
		}
	}

	return nil
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
