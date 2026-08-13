// Copyright (c) 2026 aflare Contributors
//
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
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/meta"
)

// HandleDoctor runs a one-shot environment diagnostic (断点16: 没有 aflare doctor
// 诊断命令). It checks aflare version, Go, bubblewrap, config file, LLM
// provider reachability, and network connectivity, then prints a summary with
// actionable suggestions for any problems found.
func HandleDoctor(args []string) {
	fmt.Println("环境检查：")
	fmt.Println()

	var problems []doctorProblem

	// 1. aflare version + platform.
	fmt.Printf("  ✓ aflare %s (%s/%s)\n", meta.GetVersion(), runtime.GOOS, runtime.GOARCH)

	// 2. Go toolchain (runtime version; always available for a Go binary).
	fmt.Printf("  ✓ Go %s\n", detectGoVersion())

	// 3. bubblewrap.
	if bwrapPath, ok := detectCommand("bwrap"); ok {
		fmt.Printf("  ✓ bubblewrap 已安装 (%s)\n", bwrapPath)
	} else {
		fmt.Println("  ✗ bubblewrap 未安装（部分模板需要沙箱执行代码）")
		problems = append(problems, doctorProblem{
			category: "依赖",
			desc:     "bubblewrap 未安装",
			hint:     bwrapInstallHint(),
		})
	}

	// 4. Config file.
	if cfgPath := findConfigFile(); cfgPath != "" {
		fmt.Printf("  ✓ 配置文件 %s\n", cfgPath)
	} else {
		fmt.Println("  ✗ 配置文件未找到（运行 aflare init 创建）")
		problems = append(problems, doctorProblem{
			category: "配置",
			desc:     "配置文件未找到",
			hint:     "运行 aflare init 创建配置文件",
		})
	}

	// 5. LLM provider reachability.
	if llmStatus, llmProblem := checkLLMStatus(); llmStatus != "" {
		fmt.Println(llmStatus)
		if llmProblem.desc != "" {
			problems = append(problems, llmProblem)
		}
	}

	// 6. Network connectivity (non-blocking, short timeout).
	if netOK, netDetail := checkNetworkConnectivity(); netOK {
		fmt.Printf("  ✓ 网络连接正常%s\n", netDetail)
	} else {
		fmt.Println("  ✗ 网络连接异常" + netDetail)
		problems = append(problems, doctorProblem{
			category: "网络",
			desc:     "网络连接异常",
			hint: "部分模板需要访问外部 API，检查代理或网络设置\n" +
				"  - 如果使用代理：export HTTPS_PROXY=http://127.0.0.1:7890\n" +
				"  - 如果不需要外网：使用本地模板（aflare list 查看 easy 模板）",
		})
	}

	// Summary.
	fmt.Println()
	if len(problems) == 0 {
		fmt.Println("✅ 所有检查通过，aflare 环境正常。")
		return
	}

	fmt.Printf("发现 %d 个问题：\n", len(problems))
	for i, p := range problems {
		fmt.Printf("  %d. %s — %s\n", i+1, p.category, p.desc)
	}

	fmt.Println()
	fmt.Println("建议：")
	for i, p := range problems {
		fmt.Printf("  %d. [%s] %s\n", i+1, p.category, p.desc)
		for _, line := range strings.Split(p.hint, "\n") {
			fmt.Printf("     %s\n", line)
		}
	}
}

// doctorProblem holds a diagnosed issue with its category and fix hint.
type doctorProblem struct {
	category string
	desc     string
	hint     string
}

// bwrapInstallHint returns a platform-specific install command for bubblewrap.
func bwrapInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "安装命令：brew install bubblewrap"
	case "linux":
		if _, ok := detectCommand("apt"); ok {
			return "安装命令：sudo apt install bubblewrap"
		}
		if _, ok := detectCommand("dnf"); ok {
			return "安装命令：sudo dnf install bubblewrap"
		}
		if _, ok := detectCommand("pacman"); ok {
			return "安装命令：sudo pacman -S bubblewrap"
		}
		return "请使用你的包管理器安装 bubblewrap"
	default:
		return "请参考 bubblewrap 官方文档安装"
	}
}

// findConfigFile returns the path to the existing config file, or "".
// It reuses configFilePath() precedence (AFLARE_CONFIG > ~/.config/aflare).
func findConfigFile() string {
	cfgPath := configFilePath()
	if cfgPath == "" {
		return ""
	}
	if _, err := os.Stat(cfgPath); err == nil {
		return cfgPath
	}
	return ""
}

// checkLLMStatus checks the configured LLM provider and returns a status line
// plus an optional problem. It iterates cfg.Providers (the canonical config
// shape) and probes ollama reachability or API-key presence for cloud ones.
func checkLLMStatus() (string, doctorProblem) {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil || len(cfg.Providers) == 0 {
		return "  ⊘ LLM 未配置（运行 aflare init 配置 LLM）", doctorProblem{}
	}

	// Pick the first configured provider for the status line.
	var name string
	var pcfg config.LLMProviderConfig
	for n, p := range cfg.Providers {
		name, pcfg = n, p
		break
	}

	if name == "ollama" {
		ep := strings.TrimRight(pcfg.Endpoint, "/")
		if ep == "" {
			ep = "http://localhost:11434"
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(ep + "/api/tags")
		if err != nil {
			line := fmt.Sprintf("  ✗ Ollama 不可达 (%s)", ep)
			return line, doctorProblem{
				category: "LLM",
				desc:     "Ollama 服务未运行",
				hint:     "启动 Ollama：ollama serve\n拉取模型：ollama pull " + pcfg.Model,
			}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("  ✗ Ollama 返回状态 %d", resp.StatusCode), doctorProblem{
				category: "LLM",
				desc:     "Ollama 服务异常",
				hint:     "重启 Ollama：ollama serve",
			}
		}
		model := pcfg.Model
		if model == "" {
			model = "未指定"
		}
		return fmt.Sprintf("  ✓ Ollama 已运行 (模型: %s)", model), doctorProblem{}
	}

	// Cloud providers: check API key presence (config or env).
	if pcfg.APIKey == "" {
		envKey := strings.ToUpper(name) + "_API_KEY"
		if os.Getenv(envKey) == "" {
			return fmt.Sprintf("  ✗ %s API Key 未配置", name), doctorProblem{
				category: "LLM",
				desc:     name + " API Key 未配置",
				hint:     fmt.Sprintf("设置环境变量：export %s=your-api-key\n或运行 aflare init 重新配置", envKey),
			}
		}
	}
	model := pcfg.Model
	if model == "" {
		model = "未指定"
	}
	return fmt.Sprintf("  ✓ %s 已配置 (模型: %s)", name, model), doctorProblem{}
}

// checkNetworkConnectivity tests outbound HTTPS to a well-known host.
// Returns (true, detail) on success, (false, detail) on failure.
func checkNetworkConnectivity() (bool, string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://github.com")
	if err != nil {
		return false, "（github.com 不可达）"
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, "（github.com 可达）"
	}
	return false, fmt.Sprintf("（github.com 返回状态 %d）", resp.StatusCode)
}
