// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌​​‌​​​​‌‌​​‌​​​​​‌​​​‌​‌‌​​​‌‌‌‌‌​‌‌‌​​​‌‌​​​​​​​​​​​​​​​​​​‌​‌‌‌‌​‌​‌​​​‌​⁠
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

package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateWorkflow creates a workflow from a description using rule-based
// keyword matching. It is NOT an AI / LLM-based generator — it recognizes a
// fixed set of keywords (e.g. "summarize", "translate", "github") and maps
// them to built-in node steps. For complex or dynamic workflows, define the
// YAML directly.
func GenerateWorkflow(description string) (*Workflow, error) {
	desc := strings.ToLower(description)
	wf := &Workflow{}

	var llmNode string
	var llmModel string
	switch {
	case containsLLMKeyword(desc, "deepseek"):
		llmNode = "deepseek"
		llmModel = "deepseek-chat"
	case containsLLMKeyword(desc, "qwen"):
		llmNode = "qwen"
		llmModel = "qwen-turbo"
	case containsLLMKeyword(desc, "xverse"):
		llmNode = "xverse"
		llmModel = "XVERSE-7B-Chat"
	case containsLLMKeyword(desc, "yi"):
		llmNode = "yi"
		llmModel = "yi-lightning"
	case containsLLMKeyword(desc, "baichuan"):
		llmNode = "baichuan"
		llmModel = "Baichuan4"
	case containsLLMKeyword(desc, "internlm"):
		llmNode = "internlm"
		llmModel = "internlm3-latest"
	case containsLLMKeyword(desc, "mistral"):
		llmNode = "mistral"
		llmModel = "mistral-large-latest"
	case containsLLMKeyword(desc, "mimo"):
		llmNode = "mimo"
		llmModel = "mimo-v2.5-pro"
	case containsLLMKeyword(desc, "ima"):
		llmNode = "ima"
		llmModel = "gpt-4o"
	case containsLLMKeyword(desc, "kimi"):
		llmNode = "kimi"
		llmModel = "moonshot-v1-8k"
	case containsLLMKeyword(desc, "minimax"):
		llmNode = "minimax"
		llmModel = "abab6.5s-chat"
	case containsLLMKeyword(desc, "coze"):
		llmNode = "coze"
		llmModel = "glm-4"
	case containsLLMKeyword(desc, "glm"):
		llmNode = "glm"
		llmModel = "glm-4"
	default:
		llmNode = "ollama"
		llmModel = "llama3"
	}

	// Try to extract URL (with or without protocol)
	var urlMatch string
	if m := urlRegex.FindString(description); m != "" {
		urlMatch = m
	} else {
		// Try to match a plain domain like example.com, github.com, etc.
		if m := domainRegex.FindString(description); m != "" {
			urlMatch = "https://" + m
		}
	}
	if urlMatch != "" {
		step := WorkflowStep{Node: "fetch_url", Params: map[string]string{"url": urlMatch}}
		wf.Steps = append(wf.Steps, step)
	}

	// Read intent: "read cpu.log" / "读取 notes.md" → file_read step, so the
	// monitor-style descriptions produce real input steps instead of only a
	// notify step. Matched before the write branch; both may coexist
	// ("read a.log and save to out.txt" → read then write).
	if m := readFileRegex.FindStringSubmatch(desc); len(m) >= 2 {
		wf.Steps = append(wf.Steps, WorkflowStep{
			Node:   "file_read",
			Params: map[string]string{"path": m[1]},
		})
	}

	// Try to extract file path (only allow simple filenames, not paths)
	fileMatch := fileRegex.FindStringSubmatch(desc)
	if len(fileMatch) >= 3 {
		path := fileMatch[2]
		step := WorkflowStep{Node: "file_write", Params: map[string]string{"path": path}}
		wf.Steps = append(wf.Steps, step)
	} else if !hasFileWriteStep(wf.Steps) && saveFileFallbackRegex.MatchString(desc) {
		// "save/write/export to file" with no concrete filename → default to
		// output.txt so the save intent isn't silently dropped.
		wf.Steps = append(wf.Steps, WorkflowStep{Node: "file_write", Params: map[string]string{"path": "output.txt"}})
	}

	// Check for common patterns
	if containsActionKeyword(desc, "github") {
		if urlMatch == "" {
			step := WorkflowStep{Node: "fetch_url", Params: map[string]string{"url": "https://github.com/"}}
			wf.Steps = append(wf.Steps, step)
		}
	}

	if containsActionKeyword(desc, "summarize") {
		addLLMStep(wf, llmNode, llmModel, "summarize")
	}

	if containsActionKeyword(desc, "translate") {
		addLLMStep(wf, llmNode, llmModel, "translate")
	}

	if containsActionKeyword(desc, "explain") {
		addLLMStep(wf, llmNode, llmModel, "explain")
	}

	if containsActionKeyword(desc, "rewrite") {
		addLLMStep(wf, llmNode, llmModel, "rewrite")
	}

	if containsActionKeyword(desc, "code") {
		addLLMStep(wf, llmNode, llmModel, "code")
	}

	if containsActionKeyword(desc, "email") {
		addLLMStep(wf, llmNode, llmModel, "email")
	}

	if containsActionKeyword(desc, "report") {
		addLLMStep(wf, llmNode, llmModel, "report")
	}

	if containsActionKeyword(desc, "doc") {
		addLLMStep(wf, llmNode, llmModel, "doc")
	}

	if containsActionKeyword(desc, "test") {
		addLLMStep(wf, llmNode, llmModel, "test")
	}

	if containsActionKeyword(desc, "json") {
		step := WorkflowStep{Node: "json_parse", Params: map[string]string{}}
		wf.Steps = append(wf.Steps, step)
	}

	if containsActionKeyword(desc, "git") {
		step := WorkflowStep{
			Node:   "execute",
			Params: map[string]string{"command": "git log --oneline -10"},
		}
		wf.Steps = append(wf.Steps, step)
	}

	// 遗留修复: price — recognize BTC/价格/crypto and emit a CoinGecko
	// http_request + json_parse pair so "检查 BTC 价格" produces real fetch
	// steps instead of only matching the notify keyword.
	// A-share descriptions (股票/股价/A股/沪深/行情 + a 6-digit code like
	// 600519) route to the Tencent quote API instead: its qt snapshot returns
	// the live price as a plain decimal string (e.g. "1307.88"), which the
	// gt/lt condition operators can compare directly — unlike eastmoney's
	// f43 (price×100 integer) or sina's GBK text format.
	if containsActionKeyword(desc, "price") {
		// Pass the ORIGINAL description: US ticker symbols are case-sensitive
		// (the Tencent API needs "usAAPL", lowercase "usaapl" returns a null
		// qt snapshot), so the lowercased desc would destroy them.
		if symbol, ok := extractStockSymbol(description); ok {
			wf.Steps = append(wf.Steps, WorkflowStep{
				Node: "http_request",
				Params: map[string]string{
					"url":    "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=" + symbol + ",day,,,1,qfq",
					"method": "GET",
				},
			})
			// data.<symbol>.qt.<symbol>.[3] is the live price field of the
			// Tencent qt snapshot (index 3 = current price, decimal string).
			wf.Steps = append(wf.Steps, WorkflowStep{
				Node:   "json_parse",
				Params: map[string]string{"path": "data." + symbol + ".qt." + symbol + ".[3]"},
			})
		} else if cryptoHintRegex.MatchString(description) {
			// CoinGecko route only for descriptions that actually name a
			// crypto asset. 安全自检: the previous unconditional else made
			// every symbol-less price query ("check gold price", "监控油价")
			// silently generate a BITCOIN monitor — wrong market, wrong
			// asset, misleading output. Now such queries generate no price
			// steps at all.
			coin := "bitcoin"
			if strings.Contains(desc, "eth") || strings.Contains(desc, "以太坊") {
				coin = "ethereum"
			}
			wf.Steps = append(wf.Steps, WorkflowStep{
				Node: "http_request",
				Params: map[string]string{
					"url":    "https://api.coingecko.com/api/v3/simple/price?ids=" + coin + "&vs_currencies=usd",
					"method": "GET",
				},
			})
			wf.Steps = append(wf.Steps, WorkflowStep{
				Node:   "json_parse",
				Params: map[string]string{"path": coin + ".usd"},
			})
		}
	}

	// 断点C + 遗留修复: notify — recognize 通知/telegram/slack/webhook and
	// emit a real notify step. If a condition (超过/低于 N) is also present,
	// the notify step is wrapped in an if-branch so "超过 70000 发 Telegram
	// 通知" produces if(gt:70000, then: notify) instead of an unconditional
	// notify.
	var notifyStep *WorkflowStep
	if containsActionKeyword(desc, "notify") {
		notifyStep = buildNotifyStep(desc)
	}

	// 遗留修复: condition — "超过 70000" / "低于 70000" wraps the notify step
	// (if any) in an if-branch using the gt/lt numeric operators added to
	// evaluateCondition. The if-step's input is the previous step's output
	// (e.g. the parsed price), so gt:N compares that value against N.
	if cond, ok := extractCondition(desc); ok {
		ifStep := WorkflowStep{If: &IfConfig{Condition: cond}}
		if notifyStep != nil {
			ifStep.If.Then = []WorkflowStep{*notifyStep}
		}
		wf.Steps = append(wf.Steps, ifStep)
	} else if notifyStep != nil {
		wf.Steps = append(wf.Steps, *notifyStep)
	}

	// 遗留修复: schedule — "每 10 分钟" / "定时" / "每天" sets a cron hint on
	// the workflow. The engine does not auto-schedule; `aflare run` prints an
	// activation hint. This makes the generated YAML carry the intended
	// cadence instead of silently dropping it.
	if containsActionKeyword(desc, "schedule") {
		if cron := parseScheduleCron(desc); cron != "" {
			wf.Schedule = &ScheduleConfig{Cron: cron, Enabled: true}
		}
	}

	// Generate workflow name
	wf.Name = generateWorkflowName(description)
	wf.Description = description

	// If no steps were generated, add a default execute step
	if len(wf.Steps) == 0 {
		wf.Steps = append(wf.Steps, WorkflowStep{
			Node:   "combine",
			Params: map[string]string{"format": "text"},
		})
	}

	return wf, nil
}

// HasMeaningfulSteps reports whether GenerateWorkflow actually matched any
// keyword/domain/file in the description and produced real workflow steps,
// as opposed to falling back to the default placeholder `combine` step.
//
// This is the signal used by the CLI (断点9) to decide whether to silently
// accept the generated workflow or offer suggestions / fall back to LLM
// generation: a workflow that contains only the default combine placeholder
// conveys no real intent and would mislead the user into thinking a useful
// workflow was produced.
func HasMeaningfulSteps(wf *Workflow) bool {
	if wf == nil || len(wf.Steps) == 0 {
		return false
	}
	// The rule-based generator emits exactly one `combine` step with
	// format=text only as a last-resort placeholder when nothing matched.
	if len(wf.Steps) == 1 && wf.Steps[0].Node == "combine" {
		return false
	}
	return true
}

// CreateWorkflowFromDescriptionWithAI creates a workflow from a description.
// When useAI is true, it first tries LLM-based generation with YAML validation.
// If LLM generation fails (API error, invalid YAML, empty steps), it falls back
// to rule-based keyword matching. 断点C: when the fallback also produces no
// meaningful steps, it returns an error instead of silently saving a useless
// combine-only placeholder YAML — the caller (CLI) is responsible for surfacing
// actionable suggestions to the user.
func CreateWorkflowFromDescriptionWithAI(description string, useAI bool) (string, error) {
	if !useAI {
		return CreateWorkflowFromDescription(description)
	}

	// Try LLM-based generation
	wf, err := GenerateWorkflowWithLLM(description)
	if err != nil {
		// Fall back to rule-based generation.
		fmt.Fprintf(os.Stderr, "⚠️  AI 生成失败，尝试关键词匹配 (%v)\n", err)
		wf, gerr := GenerateWorkflow(description)
		if gerr != nil {
			return "", gerr
		}
		if !HasMeaningfulSteps(wf) {
			// 断点C: 不要给用户一个看起来像结果但实际没用的 YAML。
			return "", fmt.Errorf("无法从该描述生成工作流：关键词未匹配到可用步骤，且 LLM 生成失败（%w）。请用 `aflare template list` 查找现成模板，或配置 LLM 后用 `aflare create \"%s\" --ai`", err, description)
		}
		filename := GetSuggestedFilename(description)
		if err := SaveWorkflow(wf, filename); err != nil {
			return "", err
		}
		return filepath.Join(".", filename), nil
	}

	filename := GetSuggestedFilename(description)
	if err := SaveWorkflow(wf, filename); err != nil {
		return "", err
	}

	return filepath.Join(".", filename), nil
}

// CreateWorkflowFromDescription creates and saves a workflow from description
func CreateWorkflowFromDescription(description string) (string, error) {
	wf, err := GenerateWorkflow(description)
	if err != nil {
		return "", err
	}

	filename := GetSuggestedFilename(description)
	if err := SaveWorkflow(wf, filename); err != nil {
		return "", err
	}

	return filepath.Join(".", filename), nil
}
