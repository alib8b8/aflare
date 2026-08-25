// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌​‌‌‌‌​‌​‌‌​‌​‌​​​​‌‌‌‌​‌‌​‌‌​‌‌‌‌‌‌‌​‌‌‌‌​​​​​​​​​​​​​​​​​‌‌​‌‌​​​‌‌​‌​‌​‌⁠
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
	"strings"

	"github.com/alib8b8/aflare/internal/i18n"
)

var llmKeywords = map[string][]string{
	"deepseek": {"deepseek"},
	"coze":     {"coze"},
	"glm":      {"glm", "智谱", "zhipu", "glm-4"},
	"kimi":     {"kimi", "moonshot", "月之暗面"},
	"minimax":  {"minimax", "abab"},
	"qwen":     {"qwen", "通义", "千问", "tongyi"},
	"ima":      {"ima", "ima.copilot", "ima copilot"},
	"xverse":   {"xverse", "x-verse"},
	"yi":       {"yi", "零一万物", "lingyiwanwu"},
	"baichuan": {"baichuan", "百川"},
	"internlm": {"internlm", "书生", "浦语"},
	"mistral":  {"mistral"},
	"mimo":     {"mimo", "xiaomi", "xiaomimimo"},
}

var actionKeywords = map[string][]string{
	"github":    {"github", "pull request", "pr", "issue", "仓库", "repo", "repository"},
	"summarize": {"summarize", "总结", "摘要", "summarise", "summary", "概括"},
	"translate": {"translate", "翻译", "translator", "translation", "译成"},
	"git":       {"git", "commit", "release", "push", "pull", "提交"},
	"log":       {"log", "monitor", "日志", "监控", "monitoring"},
	"weather":   {"weather", "天气", "气温", "temperature"},
	"news":      {"news", "新闻", "头条", "headline"},
	"search":    {"search", "搜索", "查找", "find", "lookup"},
	"code":      {"code", "代码", "编程", "program", "script", "脚本"},
	"explain":   {"explain", "解释", "说明", "分析", "analyze", "analyse"},
	"rewrite":   {"rewrite", "重写", "改写", "refactor", "重构", "优化", "optimize"},
	"json":      {"json", "parse json", "解析json", "格式化", "format"},
	"email":     {"email", "邮件", "e-mail", "写信", "draft"},
	"report":    {"report", "报告", "报表", "生成报告"},
	"download":  {"download", "下载", "fetch", "抓取", "爬取"},
	"api":       {"api", "接口", "rest", "http request", "调用"},
	"test":      {"test", "测试", "单元测试", "unittest"},
	"doc":       {"doc", "文档", "documentation", "readme", "注释"},
	"notify":    {"notify", "通知", "alert", "警报", "提醒", "推送", "telegram", "slack", "webhook"},
	// 遗留修复: price/schedule/condition keywords so the keyword generator
	// recognizes the full "每 10 分钟检查 BTC 价格，超过 70000 发 Telegram 通知"
	// example instead of only matching the notify keyword.
	// Stock keywords (股票/股价/A股/沪深/行情/港股/美股) route to the Tencent
	// quote API when a recognizable stock code is present in the description
	// (A股 6 位代码 / 港股 hk 前缀或 5 位代码 / 美股 us+大写代码).
	"price":     {"price", "价格", "btc", "bitcoin", "crypto", "比特币", "以太坊", "eth", "股票", "股价", "A股", "a股", "沪深", "行情", "港股", "美股", "stock"},
	"schedule":  {"schedule", "cron", "定时", "定期", "周期", "every", "每", "每天", "每小时", "每分钟"},
	"condition": {"condition", "threshold", "超过", "大于", "高于", "低于", "小于", "if", "when", "如果"},
}

func containsAny(desc string, keywords []string) bool {
	for _, kw := range keywords {
		if containsWord(desc, kw) {
			return true
		}
	}
	return false
}

// containsWord checks if a keyword appears as a whole word in the description
// (not as a substring of another word). This prevents "git" from matching "digital".
// Supports both English (space-separated) and Chinese (no spaces) text.
// For Chinese keywords (non-ASCII), uses simple substring matching since
// Chinese has no word boundaries. For English keywords, uses word boundary checks.
func containsWord(desc, kw string) bool {
	if desc == kw {
		return true
	}

	if containsNonASCII(kw) {
		return stringsContainsIgnoreCase(desc, kw)
	}

	descRunes := []rune(desc)
	kwRunes := []rune(kw)

	if len(descRunes) < len(kwRunes) {
		return false
	}

	for i := 0; i <= len(descRunes)-len(kwRunes); i++ {
		match := true
		for j := 0; j < len(kwRunes); j++ {
			dc := descRunes[i+j]
			kc := kwRunes[j]
			if dc != kc {
				if dc >= 'A' && dc <= 'Z' {
					dc += 32
				}
				if kc >= 'A' && kc <= 'Z' {
					kc += 32
				}
				if dc != kc {
					match = false
					break
				}
			}
		}
		if match {
			beforeOK := i == 0 || !isLetterOrDigitRune(descRunes[i-1])
			afterOK := i+len(kwRunes) == len(descRunes) || !isLetterOrDigitRune(descRunes[i+len(kwRunes)])
			if beforeOK && afterOK {
				return true
			}
		}
	}
	return false
}

func containsNonASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return true
		}
	}
	return false
}

func stringsContainsIgnoreCase(s, substr string) bool {
	sLower := strings.ToLower(s)
	subLower := strings.ToLower(substr)
	return strings.Contains(sLower, subLower)
}

func isLetterOrDigitRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func containsLLMKeyword(desc, provider string) bool {
	keywords, ok := llmKeywords[provider]
	if !ok {
		return false
	}
	return containsAny(desc, keywords)
}

func containsActionKeyword(desc, action string) bool {
	keywords, ok := actionKeywords[action]
	if !ok {
		return false
	}
	return containsAny(desc, keywords)
}

var systemPrompts = map[string]map[string]string{
	"en": {
		"summarize": "You are a helpful assistant that summarizes text concisely.",
		"translate": "You are a translator. Translate the following text to English.",
		"explain":   "You are an expert educator. Explain the following content clearly and thoroughly.",
		"rewrite":   "You are a skilled writer. Rewrite and improve the following text to make it clearer and more engaging.",
		"code":      "You are a senior software engineer. Write clean, efficient, well-documented code.",
		"email":     "You are a professional writer. Draft a clear, polite, and effective email.",
		"report":    "You are a research analyst. Create a well-structured, comprehensive report.",
		"doc":       "You are a technical writer. Create clear and comprehensive documentation.",
		"test":      "You are a QA engineer. Write comprehensive test cases for the given code.",
	},
	"zh": {
		"summarize": "你是一个帮助性助手，能够简明扼要地总结文本。",
		"translate": "你是一个翻译器。请将以下文本翻译成中文。",
		"explain":   "你是一位专家级讲师。请清晰、深入地解释以下内容。",
		"rewrite":   "你是一位资深作家。请重写并改进以下文本，使其更清晰、更有吸引力。",
		"code":      "你是一位高级软件工程师。请编写简洁、高效、有良好文档的代码。",
		"email":     "你是一位专业文案。请撰写一封清晰、礼貌、高效的邮件。",
		"report":    "你是一位研究分析师。请创建一份结构清晰、内容全面的报告。",
		"doc":       "你是一位技术写作人员。请创建清晰、全面的文档。",
		"test":      "你是一位质量保证工程师。请为给定的代码编写全面的测试用例。",
	},
}

func getSystemPrompt(action string) string {
	lang := i18n.GetLanguage()
	prompts, ok := systemPrompts[lang]
	if !ok {
		prompts = systemPrompts["en"]
	}
	prompt, ok := prompts[action]
	if !ok {
		return ""
	}
	return prompt
}
