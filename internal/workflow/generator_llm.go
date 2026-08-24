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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

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
	// Validate the endpoint with SSRF protection before dialing. The
	// generator endpoint is user-configured (env/config), so we use the
	// LLM-endpoint validator which permits loopback (local Ollama / vLLM)
	// and public cloud hosts but still blocks link-local / unspecified /
	// multicast / reserved ranges. The dial-time re-check below closes the
	// DNS-rebinding window.
	if err := core.ValidateLMLEndpoint(url); err != nil {
		return "", fmt.Errorf("invalid LLM endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Reuse the SSRF-safe LLM transport (dial-time IP validation) and
	// re-validate every redirect target, matching the pattern used by the
	// ollama / fastgpt / multimodal providers.
	client := &http.Client{
		Timeout:       llmGenerateTimeout,
		Transport:     core.SafeLLMHTTPClient.Transport,
		CheckRedirect: core.HTTPRedirectValidator(core.ValidateLMLEndpoint),
	}
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
