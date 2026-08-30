// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​‌​‌​​‌​‌‌​‌‌​​​​​​‌‌‌​‌​‌‌‌​​​​‌​​​​‌​​​‌​​​​​​​​​​​​​​​​​​​‌​​‌‌‌‌‌​​‌​‌​‌​⁠
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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/nodes/providers"
	"gopkg.in/yaml.v3"
)

// providerDefaults holds sensible defaults for each supported LLM provider.
type providerDefaults struct {
	model    string
	endpoint string
	needsKey bool
}

var providerDefaultsMap = map[string]providerDefaults{
	"ollama":   {model: "llama3", endpoint: "http://localhost:11434", needsKey: false},
	"openai":   {model: "gpt-4o-mini", endpoint: "", needsKey: true},
	"deepseek": {model: "deepseek-chat", endpoint: "https://api.deepseek.com", needsKey: true},
	"qwen":     {model: "qwen-plus", endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1", needsKey: true},
	"glm":      {model: "glm-4-flash", endpoint: "https://open.bigmodel.cn/api/paas/v4", needsKey: true},
	"kimi":     {model: "moonshot-v1-8k", endpoint: "https://api.moonshot.cn/v1", needsKey: true},
}

// runInitWizard runs the interactive first-run setup wizard.
// It detects the environment (Go, bubblewrap, ollama, existing LLM config),
// guides the user through LLM provider selection, and writes the config file.
func runInitWizard() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("欢迎使用 aflare 👋")
	fmt.Println("数据不出本地：默认不上报遥测，LLM 默认走本地 Ollama。")
	fmt.Println()
	fmt.Println("检测环境：")

	// Detect Go version.
	goVer := detectGoVersion()
	if goVer != "" {
		fmt.Printf("  ✓ Go %s\n", goVer)
	} else {
		fmt.Println("  ✗ 未检测到 Go（仅运行预编译二进制时可忽略）")
	}

	// Detect bubblewrap.
	// 断点F: Windows 上没有 bubblewrap，跳过检测，提示无沙箱 + WSL2 建议，
	// 而不是报"未安装"并给出无法执行的安装命令。
	if runtime.GOOS == "windows" {
		fmt.Println("  → Windows 下 code_interpreter 以无沙箱模式运行")
		fmt.Println("     建议在 WSL2 中使用以获得沙箱隔离")
	} else {
		bwrapPath, bwrapOK := detectCommand("bwrap")
		if bwrapOK {
			fmt.Printf("  ✓ bubblewrap 已安装 (%s)\n", bwrapPath)
		} else {
			fmt.Println("  ✗ bubblewrap 未安装（部分模板需要沙箱执行代码）")
			printBwrapInstallHint()
		}
	}

	// Detect ollama.
	ollamaPath, ollamaOK := detectCommand("ollama")
	if ollamaOK {
		fmt.Printf("  ✓ Ollama 已安装 (%s)\n", ollamaPath)
	} else {
		fmt.Println("  → 未检测到 Ollama")
	}

	// Detect existing LLM config.
	hasLLM := detectLLMConfig()
	if hasLLM {
		fmt.Println("  ✓ 已检测到 LLM 配置")
	} else {
		fmt.Println("  ✗ 未检测到 LLM 配置")
	}

	fmt.Println()
	fmt.Println("你需要 LLM 来使用 Agent 模式和工作流中的 agent 节点。")
	fmt.Println("选择你的方式：")
	fmt.Println()
	fmt.Println("  1. Ollama（本地，免费，推荐）")
	if !ollamaOK {
		fmt.Println("     → 检测到未安装 Ollama")
		fmt.Println("     → 安装命令：curl -fsSL https://ollama.com/install.sh | sh")
		fmt.Println("     → 装好后运行：ollama pull llama3")
	}
	fmt.Println("  2. OpenAI（云端，需要 API Key）")
	fmt.Println("  3. DeepSeek（云端，国内推荐）")
	fmt.Println("  4. 通义千问 Qwen（云端，需要 API Key）")
	fmt.Println("  5. 智谱 GLM（云端，需要 API Key）")
	fmt.Println("  6. Kimi（云端，需要 API Key）")
	fmt.Println("  7. 跳过（仅使用不需要 LLM 的工作流）")
	fmt.Println()

	choice := prompt(reader, "请选择 [1-7]（默认 1）：", "1")

	provider, model, apiKey, endpoint := readLLMChoice(reader, strings.TrimSpace(choice), ollamaOK)

	cfgPath, err := writeWizardConfig(provider, model, apiKey, endpoint)
	if err != nil {
		fmt.Printf("\n❌ 保存配置失败：%v\n", err)
		return exitErr(1)
	}
	fmt.Printf("\n配置已保存到 %s ✓\n", cfgPath)
	printAPIKeyHint(provider, apiKey)

	fmt.Println()
	fmt.Println("试试创建你的第一个工作流：")
	fmt.Println("  aflare create \"fetch example.com and save to file\"   # 需要已配置 LLM")
	fmt.Println()
	fmt.Println("或者启动 Agent 对话：")
	fmt.Println("  aflare chat")
	fmt.Println()
	return nil
}

// cloudProviderByChoice maps the wizard menu choice (2-6) to the provider key
// in providerDefaultsMap. Choices outside 2-6 return "".
func cloudProviderByChoice(choice string) string {
	switch choice {
	case "2":
		return "openai"
	case "3":
		return "deepseek"
	case "4":
		return "qwen"
	case "5":
		return "glm"
	case "6":
		return "kimi"
	default:
		return ""
	}
}

// envVarForProvider returns the <PROVIDER>_API_KEY env var name aflare reads
// for the given provider. Matches the names in internal/nodes/providers.
func envVarForProvider(provider string) string {
	return strings.ToUpper(provider) + "_API_KEY"
}

// printAPIKeyHint tells the user how to export their API key instead of
// storing it in plaintext config. No-op for providers without a key (ollama)
// or when no key was entered.
func printAPIKeyHint(provider, apiKey string) {
	if apiKey == "" {
		return
	}
	envVar := envVarForProvider(provider)
	if envVar == "" {
		return
	}
	fmt.Println()
	fmt.Println("ℹ 为避免明文存储，API Key 未写入配置文件。请设置环境变量：")
	fmt.Printf("   export %s=%s\n", envVar, apiKey)
	fmt.Printf("   (加到 ~/.bashrc / ~/.zshrc 持久化；config.GetAPIKey 会优先读取此环境变量)\n")
}

// readLLMChoice resolves a menu choice into provider credentials. Used by both
// the wizard and the flag-mode LLM offer. provider is "" when the user skips.
func readLLMChoice(reader *bufio.Reader, choice string, ollamaOK bool) (provider, model, apiKey, endpoint string) {
	if choice == "1" {
		provider = "ollama"
		d := providerDefaultsMap["ollama"]
		model = prompt(reader, fmt.Sprintf("模型（默认 %s）：", d.model), d.model)
		if ollamaOK {
			fmt.Println("  → 提示：确保已拉取模型：ollama pull " + model)
		}
		endpoint = d.endpoint
		return
	}
	if name := cloudProviderByChoice(choice); name != "" {
		provider = name
		d := providerDefaultsMap[name]
		apiKey = prompt(reader, "输入 API Key：", "")
		if apiKey == "" {
			fmt.Println("  ⚠ 未输入 API Key，可稍后用 aflare config set llm.api_key <key> 配置")
		}
		model = prompt(reader, fmt.Sprintf("默认模型（默认 %s）：", d.model), d.model)
		endpoint = d.endpoint
		return
	}
	// default: skip
	fmt.Println()
	fmt.Println("已跳过 LLM 配置。你仍可使用不需要 LLM 的工作流。")
	return "", "", "", ""
}

// offerLLMConfig checks whether an LLM provider is configured and, if not,
// offers to walk the user through setup. Used by flag-mode init (e.g.
// `aflare init --mcp all`) so that LLM guidance is not skipped. Returns true
// when a provider was configured (or already present).
func offerLLMConfig() bool {
	if detectLLMConfig() {
		return true
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Println("ℹ 未检测到 LLM 配置。Agent 模式和 agent 节点需要 LLM。")
	fmt.Println("选择你的方式：")
	fmt.Println()
	fmt.Println("  1. Ollama（本地，免费，推荐）")
	fmt.Println("  2. OpenAI（云端，需要 API Key）")
	fmt.Println("  3. DeepSeek（云端，国内推荐）")
	fmt.Println("  4. 通义千问 Qwen（云端，需要 API Key）")
	fmt.Println("  5. 智谱 GLM（云端，需要 API Key）")
	fmt.Println("  6. Kimi（云端，需要 API Key）")
	fmt.Println("  7. 跳过（仅使用不需要 LLM 的工作流）")
	fmt.Println()

	choice := prompt(reader, "请选择 [1-7]（默认 1）：", "1")
	_, ollamaOK := detectCommand("ollama")
	provider, model, apiKey, endpoint := readLLMChoice(reader, strings.TrimSpace(choice), ollamaOK)
	if provider == "" {
		fmt.Println("可稍后运行 `aflare init`（无参数）重新配置。")
		return false
	}
	cfgPath, err := writeWizardConfig(provider, model, apiKey, endpoint)
	if err != nil {
		fmt.Printf("\n❌ 保存 LLM 配置失败：%v\n", err)
		return false
	}
	fmt.Printf("\n配置已保存到 %s ✓\n", cfgPath)
	printAPIKeyHint(provider, apiKey)
	return true
}

// prompt reads a line from stdin with the given prompt, returning the default
// value when the user presses Enter.
func prompt(reader *bufio.Reader, msg, def string) string {
	fmt.Print(msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// detectGoVersion returns the runtime Go version string.
func detectGoVersion() string {
	v := runtime.Version()
	if strings.HasPrefix(v, "go") {
		return strings.TrimPrefix(v, "go")
	}
	return v
}

// detectCommand returns the path and true if the command is found on PATH.
func detectCommand(name string) (string, bool) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return p, true
}

// detectLLMConfig checks whether a usable LLM provider is configured.
func detectLLMConfig() bool {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil {
		return false
	}
	for name, pcfg := range cfg.Providers {
		if name == "ollama" {
			return true
		}
		if pcfg.APIKey != "" {
			return true
		}
	}
	// Env-var-only configuration counts too. "export OPENAI_API_KEY=..." is
	// the documented zero-config path (docs/openrouter.md), but this check
	// used to read only the config file and blocked such runs at preflight
	// with "no LLM provider configured" — contradicting its own hint text
	// ("配置 DeepSeek/OpenAI API Key").
	for _, pcfg := range providers.OpenAICompatibleConfigs() {
		if pcfg.EnvAPIKey != "" && os.Getenv(pcfg.EnvAPIKey) != "" {
			return true
		}
	}
	return false
}

// configFilePath resolves the config file path, matching config.getConfigPaths
// precedence: AFLARE_CONFIG env > ~/.config/aflare/config.yaml.
func configFilePath() string {
	if envPath := os.Getenv("AFLARE_CONFIG"); envPath != "" {
		return envPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "aflare", "config.yaml")
}

// writeWizardConfig writes the LLM provider configuration to the config file.
// When provider is empty, it writes a minimal config (security level only)
// without clobbering any existing provider entries.
func writeWizardConfig(provider, model, apiKey, endpoint string) (string, error) {
	cfgPath := configFilePath()

	// Load existing config to preserve other providers.
	existing := &config.Config{
		Providers: make(map[string]config.LLMProviderConfig),
	}
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = yaml.Unmarshal(data, existing) // best-effort: ignore parse errors, start fresh
	}
	if existing.Providers == nil {
		existing.Providers = make(map[string]config.LLMProviderConfig)
	}

	if provider != "" {
		d := providerDefaultsMap[provider]
		if model == "" {
			model = d.model
		}
		if endpoint == "" {
			endpoint = d.endpoint
		}
		// Do NOT persist api_key to config.yaml — it is plaintext on disk and
		// a real risk for data-sensitive / enterprise users (backups, sync,
		// accidental git commit). config.GetAPIKey already prefers the env
		// var <PROVIDER>_API_KEY, so we leave the field empty and print the
		// export instruction to the caller instead.
		existing.Providers[provider] = config.LLMProviderConfig{
			APIKey:   "",
			Endpoint: endpoint,
			Model:    model,
		}
	}

	// Ensure a sensible default security level.
	if existing.SecurityLevel == "" {
		existing.SecurityLevel = config.SecurityLevelL1
	}

	if err := existing.Validate(); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(existing)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	header := "# aflare 配置文件\n# 文档：https://github.com/alib8b8/aflare/blob/main/docs/getting-started.md\n\n"
	if err := os.WriteFile(cfgPath, []byte(header+string(data)), 0600); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	// Inject into the config cache so subsequent config.LoadConfig calls in
	// the same process see the updated values without requiring a restart.
	config.SetConfig(existing)

	return cfgPath, nil
}

// printBwrapInstallHint prints platform-specific bubblewrap install commands.
func printBwrapInstallHint() {
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("     安装命令：brew install bubblewrap")
	case "linux":
		// Detect package manager.
		if _, err := exec.LookPath("apt"); err == nil {
			fmt.Println("     安装命令：sudo apt install bubblewrap")
		} else if _, err := exec.LookPath("dnf"); err == nil {
			fmt.Println("     安装命令：sudo dnf install bubblewrap")
		} else if _, err := exec.LookPath("pacman"); err == nil {
			fmt.Println("     安装命令：sudo pacman -S bubblewrap")
		} else {
			fmt.Println("     安装命令：请使用你的包管理器安装 bubblewrap")
		}
	default:
		fmt.Println("     bubblewrap 在此平台不可用，部分模板将不可用")
	}
	fmt.Println("     跳过不影响其他功能。")
}
