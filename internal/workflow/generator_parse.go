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
	"regexp"
	"strconv"
	"strings"
)

// Pre-compiled regexes for workflow generation (avoid recompiling on each call)
var (
	urlRegex    = regexp.MustCompile(`(https?://[^\s]+)`)
	domainRegex = regexp.MustCompile(`\b([a-zA-Z0-9][-a-zA-Z0-9]*\.(?:com|org|net|io|edu|gov|me|dev|ai|app|xyz|co|info)\S*)\b`)
	fileRegex   = regexp.MustCompile(`(save|write|to)\s+([a-zA-Z0-9_-]+\.(txt|md|yaml|json|html|csv|xml))`)
	// readFileRegex matches "read notes.md" / "读取 cpu.log" / "open data.csv"
	// intent and emits a file_read step. Without it, "every 10 minutes read
	// cpu.log and alert via webhook" silently dropped the read step and the
	// generated workflow only contained notify. `log` is included here (but
	// not in fileRegex) because log files are a read-side staple.
	readFileRegex = regexp.MustCompile(`(?:read|读取|打开|open|load)\s+([a-zA-Z0-9_/-]+\.(?:txt|md|markdown|yaml|yml|json|html|csv|xml|log))`)
	// saveFileFallbackRegex matches "save to file" / "write file" / "export to
	// file" when no concrete filename was given. The generator then defaults to
	// output.txt so the user's save intent isn't silently dropped.
	saveFileFallbackRegex = regexp.MustCompile(`\b(save|write|export)\s+(?:to\s+)?file\b`)
	cleanCharRegex        = regexp.MustCompile(`[^a-z0-9._-]`)
	cleanNameRegex        = regexp.MustCompile(`[^a-z0-9 .]`)
	cleanFileRegex        = regexp.MustCompile(`[^a-z0-9_]`)
	// 遗留修复: threshold + schedule parsing for the condition/price/schedule
	// keywords. aboveRegex matches "超过 70000" / "above 70000" / "> 70000";
	// belowRegex matches "低于 70000" / "below 70000" / "< 70000".
	aboveRegex     = regexp.MustCompile(`(?:超过|大于|高于|above|over|greater\s*than|>)\s*(\d+(?:\.\d+)?)`)
	belowRegex     = regexp.MustCompile(`(?:低于|小于|below|under|less\s*than|<)\s*(\d+(?:\.\d+)?)`)
	everyMinRegex  = regexp.MustCompile(`(?:每|每隔)\s*(\d+)\s*分钟`)
	everyHourRegex = regexp.MustCompile(`(?:每|每隔)\s*(\d+)\s*小时`)
	// English schedule phrases: "every 10 minutes" / "every 2 hours". desc is
	// lowercased by GenerateWorkflow before parseScheduleCron is called.
	everyMinRegexEn  = regexp.MustCompile(`every\s+(\d+)\s+min(?:ute)?s?`)
	everyHourRegexEn = regexp.MustCompile(`every\s+(\d+)\s+hours?`)
)

// Stock symbol extraction for the Tencent quote API. Supported markets:
//
//   - A-share: explicit "sh600519"/"SZ000001" prefix, or a bare 6-digit code
//     mapped by its leading digit (6xx→sh Shanghai main board / STAR market,
//     0xx/3xx→sz Shenzhen main board / ChiNext).
//   - HK stock: explicit "hk00700"/"hk0700" prefix (zero-padded to 5 digits),
//     or a bare 5-digit leading-zero code ("00700"). Bare 4-digit codes are
//     rejected — they collide with plain numbers like prices.
//   - US stock: "usAAPL" / "US:AAPL" / "US AAPL" — the ticker part must be
//     UPPERCASE (2-6 letters) to avoid matching ordinary English words; the
//     API requires the uppercase form (lowercase "usaapl" returns a null qt
//     snapshot). Common non-ticker words after "us" are blacklisted.
//
// Descriptions mentioning crypto (BTC/ETH/比特币…) are never treated as
// stock queries even when they contain 6-digit numbers (e.g. thresholds).
var (
	prefixedAShareRegex = regexp.MustCompile(`(?i)\b(sh|sz)(\d{6})\b`)
	bareAShareRegex     = regexp.MustCompile(`\b([063]\d{5})\b`)
	prefixedHKRegex     = regexp.MustCompile(`(?i)\bhk:?\s*(\d{3,5})\b`)
	bareHKRegex         = regexp.MustCompile(`\b(0\d{4})\b`)
	prefixedUSRegex     = regexp.MustCompile(`\b(?i:us):?\s*([A-Z]{2,6})\b`)
	cryptoHintRegex     = regexp.MustCompile(`(?i)\b(btc|bitcoin|crypto|ethereum|eth|usdt|usdc)\b|比特币|以太坊`)
)

// usTickerBlacklist rejects uppercase words that legitimately follow "us" in
// price-monitoring descriptions but are not stock tickers ("US MARKET price",
// "USDT", ...). Prevents the generator from emitting garbage symbols like
// usMARKET from "check US MARKET price every 10 minutes".
var usTickerBlacklist = map[string]bool{
	"USD": true, "USA": true, "USB": true, "USR": true, "UST": true,
	"USE": true, "USDT": true, "USDC": true,
	"IS": true, "IT": true, "IN": true, "ON": true, "TO": true, "OF": true,
	"AT": true, "BY": true, "DO": true, "GO": true, "NO": true, "SO": true,
	"UP": true, "WE": true, "HE": true,
	"MARKET": true, "MARKETS": true, "STOCK": true, "STOCKS": true,
	"PRICE": true, "PRICES": true, "ALERT": true, "ALERTS": true,
	"EVERY": true, "CHECK": true, "CHECKS": true, "DAILY": true,
	"WEEKLY": true, "ABOVE": true, "BELOW": true, "OVER": true,
	"UNDER": true, "WHEN": true, "THEN": true, "THAN": true,
	"THIS": true, "THAT": true, "WITH": true, "FROM": true,
	"SEND": true, "NOTIFY": true, "EMAIL": true, "TELEGRAM": true,
	"SLACK": true, "WEBHOOK": true,
}

// buildNotifyStep constructs the notify step for a (lowercased) description
// that already matched a notify action keyword. Channel detection prefers
// the CN group-bot webhooks (飞书/钉钉/企业微信 — the compliant push path
// for CN users) over the generic webhook default; telegram keeps its
// token/chat_id pair. Every webhook-style channel carries a
// <channel>_webhook_url var reference so the generated step is runnable
// after `--set <channel>_webhook_url=<hook>` (slack previously emitted no
// url at all — a real bug, the node requires it).
func buildNotifyStep(desc string) *WorkflowStep {
	channel := "webhook"
	switch {
	case strings.Contains(desc, "telegram"):
		channel = "telegram"
	case strings.Contains(desc, "slack"):
		channel = "slack"
	case strings.Contains(desc, "飞书") || strings.Contains(desc, "feishu") || strings.Contains(desc, "lark"):
		channel = "feishu"
	case strings.Contains(desc, "钉钉") || strings.Contains(desc, "dingtalk"):
		channel = "dingtalk"
	case strings.Contains(desc, "企业微信") || strings.Contains(desc, "微信") || strings.Contains(desc, "wecom"):
		channel = "wecom"
	}
	params := map[string]string{"channel": channel}
	if channel == "telegram" {
		params["token"] = "{{var.telegram_token}}"
		params["chat_id"] = "{{var.telegram_chat_id}}"
	} else {
		params["url"] = "{{var." + channel + "_webhook_url}}"
	}
	return &WorkflowStep{Node: "notify", Params: params}
}

// extractCondition scans a (lowercased) description for a threshold phrase
// like "超过 70000" / "above 70000" / "低于 70000" / "below 70000" and returns
// the corresponding condition expression ("gt:70000" / "lt:70000") plus ok.
// Used by GenerateWorkflow to wrap a notify step in an if-branch.
func extractCondition(desc string) (string, bool) {
	if m := aboveRegex.FindStringSubmatch(desc); len(m) > 1 {
		return "gt:" + m[1], true
	}
	if m := belowRegex.FindStringSubmatch(desc); len(m) > 1 {
		return "lt:" + m[1], true
	}
	return "", false
}

// extractStockSymbol returns the Tencent quote symbol ("sh600519" / "hk00700"
// / "usAAPL" style) when the description names a stock. Descriptions that
// mention crypto are never treated as stock queries even if they contain
// 6-digit numbers.
func extractStockSymbol(desc string) (string, bool) {
	if cryptoHintRegex.MatchString(desc) {
		return "", false
	}
	// Explicit prefixed forms win over bare numeric codes.
	if m := prefixedAShareRegex.FindStringSubmatch(desc); len(m) > 2 {
		return strings.ToLower(m[1]) + m[2], true
	}
	if m := prefixedHKRegex.FindStringSubmatch(desc); len(m) > 1 {
		// HK codes are 5 digits: hk700 → hk00700. The API returns an empty
		// qt snapshot for the unpadded form.
		code := m[1]
		for len(code) < 5 {
			code = "0" + code
		}
		return "hk" + code, true
	}
	if m := prefixedUSRegex.FindStringSubmatch(desc); len(m) > 1 {
		ticker := strings.ToUpper(m[1])
		if !usTickerBlacklist[ticker] {
			return "us" + ticker, true
		}
	}
	// Bare codes (no exchange prefix).
	if m := bareAShareRegex.FindStringSubmatch(desc); len(m) > 1 {
		switch m[1][0] {
		case '6':
			return "sh" + m[1], true
		case '0', '3':
			return "sz" + m[1], true
		}
	}
	if m := bareHKRegex.FindStringSubmatch(desc); len(m) > 1 {
		return "hk" + m[1], true
	}
	return "", false
}

// parseScheduleCron extracts a cron expression from a (lowercased) description
// containing a schedule phrase. Supported forms: "每N分钟" / "every N minutes"
// → "*/N * * * *", "每N小时" / "every N hours" → "0 */N * * *", "每小时" →
// "0 * * * *", "每分钟" / "every minute" → "* * * * *", "每天" → "0 9 * * *".
// Returns "" if no recognizable schedule phrase is found.
func parseScheduleCron(desc string) string {
	if m := everyMinRegex.FindStringSubmatch(desc); len(m) > 1 {
		if n, ok := validCronStep(m[1], 59); ok {
			return "*/" + n + " * * * *"
		}
	}
	if m := everyMinRegexEn.FindStringSubmatch(desc); len(m) > 1 {
		if n, ok := validCronStep(m[1], 59); ok {
			return "*/" + n + " * * * *"
		}
	}
	if m := everyHourRegex.FindStringSubmatch(desc); len(m) > 1 {
		if n, ok := validCronStep(m[1], 23); ok {
			return "0 */" + n + " * * *"
		}
	}
	if m := everyHourRegexEn.FindStringSubmatch(desc); len(m) > 1 {
		if n, ok := validCronStep(m[1], 23); ok {
			return "0 */" + n + " * * *"
		}
	}
	if strings.Contains(desc, "每小时") {
		return "0 * * * *"
	}
	if strings.Contains(desc, "每分钟") || strings.Contains(desc, "every minute") {
		return "* * * * *"
	}
	if strings.Contains(desc, "每天") {
		return "0 9 * * *"
	}
	return ""
}

// validCronStep rejects degenerate interval phrases ("每 0 分钟" / "every 0
// minutes" → "*/0", or intervals beyond the cron field range like "*/99")
// that would produce an invalid or never-firing cron expression. The caller
// then falls through to the next schedule form or leaves the workflow
// unscheduled instead of emitting a broken cron.
func validCronStep(nStr string, max int) (string, bool) {
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 || n > max {
		return "", false
	}
	return nStr, true
}

func addLLMStep(wf *Workflow, llmNode, llmModel, action string) {
	systemPrompt := getSystemPrompt(action)
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant."
	}
	step := WorkflowStep{
		Node:   llmNode,
		Params: map[string]string{"model": llmModel, "system": systemPrompt},
	}
	if len(wf.Steps) > 0 && wf.Steps[len(wf.Steps)-1].Node == "file_write" {
		lastStep := wf.Steps[len(wf.Steps)-1]
		steps := make([]WorkflowStep, len(wf.Steps)-1)
		copy(steps, wf.Steps[:len(wf.Steps)-1])
		steps = append(steps, step)
		steps = append(steps, lastStep)
		wf.Steps = steps
	} else {
		wf.Steps = append(wf.Steps, step)
	}
}
