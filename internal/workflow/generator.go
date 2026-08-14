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

package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/watermark"
)

// Pre-compiled regexes for workflow generation (avoid recompiling on each call)
var (
	urlRegex       = regexp.MustCompile(`(https?://[^\s]+)`)
	domainRegex    = regexp.MustCompile(`\b([a-zA-Z0-9][-a-zA-Z0-9]*\.(?:com|org|net|io|edu|gov|me|dev|ai|app|xyz|co|info)\S*)\b`)
	fileRegex      = regexp.MustCompile(`(save|write|to)\s+([a-zA-Z0-9_-]+\.(txt|md|yaml|json|html|csv|xml))`)
	// saveFileFallbackRegex matches "save to file" / "write file" / "export to
	// file" when no concrete filename was given. The generator then defaults to
	// output.txt so the user's save intent isn't silently dropped.
	saveFileFallbackRegex = regexp.MustCompile(`\b(save|write|export)\s+(?:to\s+)?file\b`)
	cleanCharRegex = regexp.MustCompile(`[^a-z0-9._-]`)
	cleanNameRegex = regexp.MustCompile(`[^a-z0-9 .]`)
	cleanFileRegex = regexp.MustCompile(`[^a-z0-9_]`)
	// 遗留修复: threshold + schedule parsing for the condition/price/schedule
	// keywords. aboveRegex matches "超过 70000" / "above 70000" / "> 70000";
	// belowRegex matches "低于 70000" / "below 70000" / "< 70000".
	aboveRegex     = regexp.MustCompile(`(?:超过|大于|高于|above|over|greater\s*than|>)\s*(\d+(?:\.\d+)?)`)
	belowRegex     = regexp.MustCompile(`(?:低于|小于|below|under|less\s*than|<)\s*(\d+(?:\.\d+)?)`)
	everyMinRegex  = regexp.MustCompile(`(?:每|每隔)\s*(\d+)\s*分钟`)
	everyHourRegex = regexp.MustCompile(`(?:每|每隔)\s*(\d+)\s*小时`)
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
	if containsActionKeyword(desc, "price") {
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

	// 断点C + 遗留修复: notify — recognize 通知/telegram/slack/webhook and
	// emit a real notify step. If a condition (超过/低于 N) is also present,
	// the notify step is wrapped in an if-branch so "超过 70000 发 Telegram
	// 通知" produces if(gt:70000, then: notify) instead of an unconditional
	// notify.
	var notifyStep *WorkflowStep
	if containsActionKeyword(desc, "notify") {
		channel := "webhook"
		if strings.Contains(desc, "telegram") {
			channel = "telegram"
		} else if strings.Contains(desc, "slack") {
			channel = "slack"
		}
		params := map[string]string{"channel": channel}
		if channel == "telegram" {
			params["token"] = "{{var.telegram_token}}"
			params["chat_id"] = "{{var.telegram_chat_id}}"
		} else if channel == "webhook" {
			params["url"] = "{{var.webhook_url}}"
		}
		notifyStep = &WorkflowStep{Node: "notify", Params: params}
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

// parseScheduleCron extracts a cron expression from a (lowercased) description
// containing a schedule phrase. Supported forms: "每N分钟" → "*/N * * * *",
// "每N小时" → "0 */N * * *", "每小时" → "0 * * * *", "每分钟" → "* * * * *".
// Returns "" if no recognizable schedule phrase is found.
func parseScheduleCron(desc string) string {
	if m := everyMinRegex.FindStringSubmatch(desc); len(m) > 1 {
		return "*/" + m[1] + " * * * *"
	}
	if m := everyHourRegex.FindStringSubmatch(desc); len(m) > 1 {
		return "0 */" + m[1] + " * * *"
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

// SaveWorkflow saves a workflow to a YAML file with a provenance watermark.
func SaveWorkflow(wf *Workflow, filename string) error {
	// Ensure .yaml extension
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}

	// Sanitize filename to prevent path traversal
	cleanPath := filepath.Base(filename)
	if cleanPath == "." || cleanPath == "/" || cleanPath == string(filepath.Separator) {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	filename = cleanPath

	// Generate YAML content
	content := wf.ToYAML()

	// Prepend provenance watermark comment
	wm := watermark.EncodeYAML(content)
	if wm != "" {
		content = wm + "\n" + content
	}

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetSuggestedFilename returns a suggested filename based on the description
func GetSuggestedFilename(description string) string {
	desc := strings.ToLower(description)

	// Extract key words
	words := strings.Fields(desc)
	var keywords []string
	for _, word := range words {
		// Skip common words
		if strings.Contains("the a an to and or fetch save write run from", word) {
			continue
		}
		// Clean word - keep alphanumeric, dots, hyphens, underscores
		word = cleanCharRegex.ReplaceAllString(word, "")
		if len(word) > 2 {
			keywords = append(keywords, word)
		}
	}

	// Take first 3 keywords
	if len(keywords) > 3 {
		keywords = keywords[:3]
	}

	filename := strings.Join(keywords, "_")
	if filename == "" {
		filename = "workflow"
	}

	return filename + ".yaml"
}

func generateWorkflowName(description string) string {
	desc := strings.ToLower(description)

	words := strings.Fields(desc)
	var nameParts []string
	for _, word := range words {
		// Remove all non-alphanumeric characters except spaces and dots
		word = cleanNameRegex.ReplaceAllString(word, "")
		if len(word) > 3 && !strings.Contains("the a an to and or fetch save write run from with", word) {
			// Simple title case: capitalize first letter
			if len(word) > 0 {
				word = strings.ToUpper(word[:1]) + word[1:]
			}
			nameParts = append(nameParts, word)
		}
		if len(nameParts) >= 3 {
			break
		}
	}

	if len(nameParts) == 0 {
		return "Custom Workflow"
	}

	return strings.Join(nameParts, " ")
}

// ToYAML converts the workflow to YAML string using the standard yaml library
// which properly handles all special characters including newlines, tabs, and quotes
func (wf *Workflow) ToYAML() string {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Sprintf("# Error: failed to marshal workflow: %v\n", err)
	}
	return string(data)
}

// GetWorkflowFilename returns the filename for a workflow
func GetWorkflowFilename(wf *Workflow) string {
	name := strings.ToLower(wf.Name)
	name = strings.ReplaceAll(name, " ", "_")
	name = cleanFileRegex.ReplaceAllString(name, "")
	if name == "" {
		name = "workflow"
	}
	return name + ".yaml"
}

// ValidateWorkflow validates a workflow and returns suggestions
func ValidateWorkflow(wf *Workflow) []string {
	var suggestions []string

	if wf.Name == "" {
		suggestions = append(suggestions, "Consider adding a workflow name")
	}

	if len(wf.Steps) == 0 {
		suggestions = append(suggestions, "Workflow has no steps")
	}

	// Check for common patterns
	hasOutput := hasFileWriteStep(wf.Steps)

	if !hasOutput && len(wf.Steps) > 0 {
		suggestions = append(suggestions, "Consider adding a file_write step to save output")
	}

	return suggestions
}

// hasFileWriteStep reports whether any step uses the file_write node, recursing
// into compound steps (if/then/else, parallel, map, reduce, saga,
// capture_error, on_error) so that file_write steps nested inside branches are
// detected.
func hasFileWriteStep(steps []WorkflowStep) bool {
	for _, s := range steps {
		if s.Node == "file_write" {
			return true
		}
		if s.IsIf() && (hasFileWriteStep(s.If.Then) || hasFileWriteStep(s.If.Else)) {
			return true
		}
		if s.IsMap() && hasFileWriteStep(s.Map.Steps) {
			return true
		}
		if s.IsReduce() && hasFileWriteStep(s.Reduce.Steps) {
			return true
		}
		if s.IsSaga() {
			for _, sg := range s.Saga.Steps {
				if hasFileWriteStep([]WorkflowStep{sg.Forward}) {
					return true
				}
				if sg.Compensate != nil && hasFileWriteStep([]WorkflowStep{*sg.Compensate}) {
					return true
				}
			}
		}
		if s.HasCaptureError() && hasFileWriteStep(s.CaptureError) {
			return true
		}
		if s.IsParallel() {
			for _, p := range s.Parallel {
				if p.Node == "file_write" {
					return true
				}
			}
		}
		if s.OnError != nil && s.OnError.Node == "file_write" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// LLM-based workflow generation
// ---------------------------------------------------------------------------

const (
	llmGenerateTimeout   = 60 * time.Second
	llmGenerateMaxTokens = 4096
)

// llmGenerateSystemPrompt is the system prompt sent to the LLM for YAML generation.
const llmGenerateSystemPrompt = `You are a workflow YAML generator for aflare. Given a natural-language description of a task, output ONLY a valid YAML document that defines the workflow.

The YAML must follow this structure:
- name: (string) A short descriptive name for the workflow
- description: (string) The original user description
- steps: (list) Each step has:
    - node: (string) One of the available node types
    - params: (map, optional) Parameters for the node

Available node types:
- fetch_url: Fetch content from a URL. Params: url
- file_read: Read a file. Params: path
- file_write: Write output to a file. Params: path
- json_parse: Parse JSON data. No required params.
- execute: Run a shell command. Params: command
- http_request: Send HTTP request. Params: method, url, headers, body
- combine: Combine multiple inputs. Params: format (text/json)
- template_render: Render a template. Params: template
- notify: Send a notification. Params: message
- openai, deepseek, qwen, kimi, glm, mistral, baichuan, internlm, yi, xverse, minimax, ollama: LLM nodes. Params: model, system, prompt, temperature, max_tokens

Output ONLY the YAML, no markdown code fences, no explanations.`

// llmGenerateUserPromptTemplate formats the user prompt for LLM generation.
const llmGenerateUserPromptTemplate = `Generate a workflow YAML for the following task description:

%s`

// llmChatMessage is a minimal chat message for the LLM API call.
type llmChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// llmChatRequest is the request body for an OpenAI-compatible /chat/completions call.
type llmChatRequest struct {
	Model       string           `json:"model"`
	Messages    []llmChatMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
}

// llmChatResponse is the response from an OpenAI-compatible /chat/completions call.
type llmChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// getLLMConfig returns the API key, endpoint, and model for LLM workflow
// generation. Resolution order (断点B: --ai 配置与 init 脱节):
//  1. config.yaml providers written by `aflare init` — so a user who configured
//     DeepSeek/OpenAI/Qwen etc. via init can use `aflare create --ai` without
//     separately exporting env vars. A keyed cloud provider is preferred;
//     ollama (keyless local) is used only when no keyed provider is configured.
//  2. AFLARE_LLM_GENERATOR_* env vars (explicit override).
//  3. Common provider env vars (OPENAI_API_KEY, DEEPSEEK_API_KEY, ...).
func getLLMConfig() (apiKey, endpoint, model string) {
	// 1. config.yaml providers (written by `aflare init`).
	if cfg, err := config.LoadConfig(); err == nil && cfg != nil && len(cfg.Providers) > 0 {
		var chosen string
		for name, pcfg := range cfg.Providers {
			if pcfg.APIKey != "" {
				chosen = name
				break
			}
		}
		if chosen == "" {
			if _, ok := cfg.Providers["ollama"]; ok {
				chosen = "ollama"
			}
		}
		if chosen != "" {
			pcfg := cfg.Providers[chosen]
			apiKey = pcfg.APIKey
			endpoint = pcfg.Endpoint
			model = pcfg.Model
		}
	}

	// 2. AFLARE_LLM_GENERATOR_* env vars (explicit override).
	if key := os.Getenv("AFLARE_LLM_GENERATOR_API_KEY"); key != "" {
		apiKey = key
	}
	if ep := os.Getenv("AFLARE_LLM_GENERATOR_ENDPOINT"); ep != "" {
		endpoint = ep
	}
	if m := os.Getenv("AFLARE_LLM_GENERATOR_MODEL"); m != "" {
		model = m
	}

	// 3. Fall back to common provider env vars.
	if apiKey == "" {
		for _, envVar := range []string{
			"OPENAI_API_KEY", "DEEPSEEK_API_KEY", "QWEN_API_KEY",
			"GLM_API_KEY", "KIMI_API_KEY", "MISTRAL_API_KEY",
		} {
			if key := os.Getenv(envVar); key != "" {
				apiKey = key
				break
			}
		}
	}
	if endpoint == "" {
		if os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("OPENAI_API_BASE") != "" {
			endpoint = os.Getenv("OPENAI_API_BASE")
		}
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1"
		}
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	return
}

// callLLMForGeneration sends a chat completion request to an OpenAI-compatible
// API and returns the generated content.
func callLLMForGeneration(ctx context.Context, description string) (string, error) {
	apiKey, endpoint, model := getLLMConfig()
	if apiKey == "" {
		return "", fmt.Errorf("no API key configured for LLM workflow generation; set AFLARE_LLM_GENERATOR_API_KEY or a provider key (OPENAI_API_KEY, DEEPSEEK_API_KEY, etc.)")
	}

	reqBody := llmChatRequest{
		Model: model,
		Messages: []llmChatMessage{
			{Role: "system", Content: llmGenerateSystemPrompt},
			{Role: "user", Content: fmt.Sprintf(llmGenerateUserPromptTemplate, description)},
		},
		MaxTokens:   llmGenerateMaxTokens,
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	url := endpoint + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: llmGenerateTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp llmChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// extractYAMLFromLLMOutput strips markdown code fences and leading/trailing
// whitespace from the LLM output to extract the raw YAML content.
func extractYAMLFromLLMOutput(output string) string {
	output = strings.TrimSpace(output)

	// Remove markdown code fences: ```yaml ... ``` or ``` ... ```
	if strings.HasPrefix(output, "```") {
		// Find the first newline after the opening fence
		idx := strings.Index(output, "\n")
		if idx == -1 {
			// Single line with fences, just return empty
			return ""
		}
		output = output[idx+1:]
		// Remove trailing ```
		if lastIdx := strings.LastIndex(output, "```"); lastIdx != -1 {
			output = output[:lastIdx]
		}
	}

	return strings.TrimSpace(output)
}

// GenerateWorkflowWithLLM attempts to generate a workflow YAML by calling an LLM.
// It validates the generated YAML. On failure, it returns an error so the caller
// can fall back to rule-based generation.
func GenerateWorkflowWithLLM(description string) (*Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), llmGenerateTimeout)
	defer cancel()

	rawContent, err := callLLMForGeneration(ctx, description)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	yamlContent := extractYAMLFromLLMOutput(rawContent)
	if yamlContent == "" {
		return nil, fmt.Errorf("LLM returned empty YAML content")
	}

	// Validate the generated YAML by parsing it
	wf, err := ParseWorkflowFromContent(yamlContent)
	if err != nil {
		return nil, fmt.Errorf("LLM YAML validation failed: %w", err)
	}

	// Ensure the workflow has at least a name and steps
	if wf.Name == "" {
		wf.Name = generateWorkflowName(description)
	}
	if wf.Description == "" {
		wf.Description = description
	}
	if len(wf.Steps) == 0 {
		return nil, fmt.Errorf("LLM generated workflow has no steps")
	}

	return wf, nil
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
