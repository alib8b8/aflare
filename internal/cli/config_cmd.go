// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌‌‌‌‌‌​‌‌‌​‌​‌‌‌‌​‌‌‌​‌​‌​​​​‌​‌‌​‌​​‌‌‌​‌​​​​​​​​​​​​​​​​​​​​‌​​‌‌‌​‌​‌​​​​‌⁠
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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/config"
	"gopkg.in/yaml.v3"
)

// HandleConfig handles the "config" command with show/set subcommands.
func HandleConfig(args []string) error {
	if len(args) == 0 {
		PrintConfigUsage()
		return exitErr(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "show":
		handleConfigShow()
	case "set":
		if len(args) < 3 {
			fmt.Println("Error: config set requires <key> <value>")
			fmt.Println("Usage: aflare config set <key> <value>")
			fmt.Println("\nKeys:")
			fmt.Println("  llm.provider <name>     Set default LLM provider (ollama/openai/deepseek/qwen/glm/kimi)")
			fmt.Println("  llm.model <model>       Set model for the default provider")
			fmt.Println("  llm.api_key <key>       Set API key for the default provider")
			fmt.Println("  llm.endpoint <url>      Set endpoint for the default provider")
			fmt.Println("  security_level <L>      Set security level (L0/L1/L2/L3)")
			fmt.Println("  safe_mode <bool>        Enable/disable safe mode (true/false)")
			return exitErr(1)
		}
		if err := handleConfigSet(args[1], args[2]); err != nil {
			return err
		}
	case "--help", "-h", "help":
		PrintConfigUsage()
	default:
		fmt.Printf("Unknown config subcommand: %s\n\n", subCmd)
		PrintConfigUsage()
		return exitErr(1)
	}
	return nil
}

// handleConfigShow prints the current configuration with API keys redacted.
func handleConfigShow() {
	cfgPath := configFilePath()
	cfg, err := config.LoadConfig()
	if err != nil {
		// Config may not exist yet; show the path and a hint.
		fmt.Printf("配置文件：%s\n", cfgPath)
		fmt.Printf("状态：未配置或加载失败（%v）\n", err)
		fmt.Println("\n运行 aflare init 进行首次配置。")
		return
	}

	fmt.Printf("配置文件：%s\n\n", cfgPath)

	if len(cfg.Providers) == 0 {
		fmt.Println("LLM Providers：（未配置）")
	} else {
		// Sort provider names for stable output.
		names := make([]string, 0, len(cfg.Providers))
		for name := range cfg.Providers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			pcfg := cfg.Providers[name]
			keyDisplay := "（未设置）"
			if pcfg.APIKey != "" {
				keyDisplay = redactAPIKey(pcfg.APIKey)
			}
			endpoint := pcfg.Endpoint
			if endpoint == "" {
				endpoint = "（默认）"
			}
			model := pcfg.Model
			if model == "" {
				model = "（默认）"
			}
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    model:    %s\n", model)
			fmt.Printf("    api_key:  %s\n", keyDisplay)
			fmt.Printf("    endpoint: %s\n", endpoint)
		}
	}

	fmt.Println()
	level := cfg.SecurityLevel
	if level == "" {
		if cfg.SafeMode {
			level = config.SecurityLevelL3 + " (safe_mode)"
		} else {
			level = config.SecurityLevelL1 + " (default)"
		}
	}
	fmt.Printf("安全等级：%s\n", level)
	fmt.Printf("Safe Mode：%v\n", cfg.SafeMode)
}

// handleConfigSet updates a single configuration key and persists the config.
func handleConfigSet(key, value string) error {
	cfgPath := configFilePath()

	// Load existing config from file (bypassing the cached singleton so we
	// always read what is on disk).
	cfg := &config.Config{
		Providers: make(map[string]config.LLMProviderConfig),
	}
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			fmt.Printf("❌ 解析配置文件失败：%v\n", err)
			return exitErr(1)
		}
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]config.LLMProviderConfig)
	}

	// Track the "current" provider: the most recently set llm.provider, or the
	// first configured provider, defaulting to "ollama".
	currentProvider := os.Getenv("AFLARE_PROVIDER")
	if currentProvider == "" {
		for name := range cfg.Providers {
			currentProvider = name
			break
		}
	}
	if currentProvider == "" {
		currentProvider = "ollama"
	}

	keyLower := strings.ToLower(key)
	switch {
	case keyLower == "llm.provider":
		currentProvider = value
		// Ensure the provider entry exists with defaults.
		if _, ok := cfg.Providers[currentProvider]; !ok {
			d := providerDefaultsMap[currentProvider]
			cfg.Providers[currentProvider] = config.LLMProviderConfig{
				Model:    d.model,
				Endpoint: d.endpoint,
			}
		}
		// Persist current provider marker via env-like comment is not supported
		// in YAML struct; we rely on it being the first key in the map.
	case keyLower == "llm.model":
		pcfg := cfg.Providers[currentProvider]
		pcfg.Model = value
		cfg.Providers[currentProvider] = pcfg
	case keyLower == "llm.api_key":
		pcfg := cfg.Providers[currentProvider]
		pcfg.APIKey = value
		cfg.Providers[currentProvider] = pcfg
	case keyLower == "llm.endpoint":
		pcfg := cfg.Providers[currentProvider]
		pcfg.Endpoint = value
		cfg.Providers[currentProvider] = pcfg
	case keyLower == "security_level":
		upper := strings.ToUpper(strings.TrimSpace(value))
		switch upper {
		case config.SecurityLevelL0, config.SecurityLevelL1, config.SecurityLevelL2, config.SecurityLevelL3:
			cfg.SecurityLevel = upper
		default:
			fmt.Printf("❌ 无效的安全等级：%s（可选 L0/L1/L2/L3）\n", value)
			return exitErr(1)
		}
	case keyLower == "safe_mode":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes", "on":
			cfg.SafeMode = true
		case "false", "0", "no", "off":
			cfg.SafeMode = false
		default:
			fmt.Printf("❌ 无效的布尔值：%s（可选 true/false）\n", value)
			return exitErr(1)
		}
	default:
		fmt.Printf("❌ 未知的配置项：%s\n", key)
		fmt.Println("支持的配置项：llm.provider, llm.model, llm.api_key, llm.endpoint, security_level, safe_mode")
		return exitErr(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("❌ 配置校验失败：%v\n", err)
		return exitErr(1)
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		fmt.Printf("❌ 创建配置目录失败：%v\n", err)
		return exitErr(1)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Printf("❌ 序列化配置失败：%v\n", err)
		return exitErr(1)
	}

	header := "# aflare 配置文件\n# 文档：https://github.com/alib8b8/aflare/blob/main/docs/getting-started.md\n\n"
	if err := os.WriteFile(cfgPath, []byte(header+string(data)), 0600); err != nil {
		fmt.Printf("❌ 写入配置失败：%v\n", err)
		return exitErr(1)
	}

	fmt.Printf("✅ 已设置 %s = %s\n", key, redactConfigValue(key, value))
	fmt.Printf("   配置文件：%s\n", cfgPath)
	return nil
}

// redactAPIKey masks all but the first and last 4 characters of an API key.
func redactAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// redactConfigValue redacts sensitive values when echoing back the set result.
func redactConfigValue(key, value string) string {
	if strings.Contains(strings.ToLower(key), "api_key") {
		return redactAPIKey(value)
	}
	return value
}

// PrintConfigUsage prints usage for the config command.
func PrintConfigUsage() {
	fmt.Println("Usage: aflare config <show|set> [key] [value]")
	fmt.Println("\n查看或修改 aflare 配置。")
	fmt.Println()
	fmt.Println("子命令：")
	fmt.Println("  show                          显示当前配置（API Key 自动脱敏）")
	fmt.Println("  set <key> <value>             修改单项配置")
	fmt.Println()
	fmt.Println("可设置的 key：")
	fmt.Println("  llm.provider <name>           设置默认 LLM provider (ollama/openai/deepseek/qwen/glm/kimi)")
	fmt.Println("  llm.model <model>             设置默认 provider 的模型")
	fmt.Println("  llm.api_key <key>             设置默认 provider 的 API Key")
	fmt.Println("  llm.endpoint <url>            设置默认 provider 的 endpoint")
	fmt.Println("  security_level <L0|L1|L2|L3>  设置安全等级")
	fmt.Println("  safe_mode <true|false>        启用/关闭安全模式")
	fmt.Println()
	fmt.Println("示例：")
	fmt.Println("  aflare config show")
	fmt.Println("  aflare config set llm.provider openai")
	fmt.Println("  aflare config set llm.api_key sk-xxxx")
	fmt.Println("  aflare config set security_level L2")
}
