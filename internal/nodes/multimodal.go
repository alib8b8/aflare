// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​‌‌​‌‌‌‌​‌​​​​‌​‌​​‌‌‌​​​‌‌​‌​‌​‌‌‌​​​‌‌‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌​​​‌‌‌​‌​‌​​‌⁠
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

package nodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

type MultimodalNode struct{}

func init() {
	Register(&MultimodalNode{})
}

func (n *MultimodalNode) Name() string {
	return "multimodal"
}

func (n *MultimodalNode) Description() string {
	return "Multimodal analysis: image understanding, OCR (local tesseract + LLM fallback), audio transcription (via LLM vision/audio APIs)"
}

func (n *MultimodalNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "multimodal",
		Description: "Multimodal node for image analysis, OCR, and audio transcription using vision-capable LLMs",
		Input:       "string - the question or instruction about the media",
		Output:      "string - analysis result from the multimodal model",
		Params: []ParamSchema{
			{Name: "mode", Type: "string", Description: "Mode: image, ocr, describe, compare (default: describe)", Required: false, Default: "describe"},
			{Name: "image_path", Type: "string", Description: "Path to image file (local path or URL)", Required: false},
			{Name: "image_paths", Type: "string", Description: "Comma-separated paths for compare mode", Required: false},
			{Name: "lang", Type: "string", Description: "OCR languages for tesseract (default: eng+chi_sim)", Required: false, Default: "eng+chi_sim"},
			{Name: "provider", Type: "string", Description: "LLM provider with vision support (default: openai)", Required: false, Default: "openai"},
			{Name: "model", Type: "string", Description: "Vision model name (default: gpt-4o)", Required: false, Default: "gpt-4o"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint", Required: false},
			{Name: "detail", Type: "string", Description: "Image detail level: low, high, auto (default: auto)", Required: false, Default: "auto"},
			{Name: "output_format", Type: "string", Description: "Output format: text, json, markdown (default: markdown)", Required: false, Default: "markdown"},
		},
	}
}

func (n *MultimodalNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	mode := getParam(params, "mode", "describe")
	imagePath := params["image_path"]
	imagePaths := params["image_paths"]
	provider := getParam(params, "provider", "openai")
	model := getParam(params, "model", "gpt-4o")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	detail := getParam(params, "detail", "auto")
	outputFormat := getParam(params, "output_format", "markdown")

	prompt := strings.TrimSpace(input)
	if prompt == "" {
		switch mode {
		case "describe":
			prompt = "Describe this image in detail."
		case "ocr":
			prompt = "Extract all text from this image. Return only the text content."
		default:
			prompt = "Analyze this image."
		}
	}

	switch mode {
	case "image", "describe":
		if imagePath == "" {
			return "", fmt.Errorf("image_path is required for mode %s", mode)
		}
		return n.analyzeImage(ctx, imagePath, prompt, provider, model, apiKey, endpoint, detail, mode, outputFormat)
	case "ocr":
		if imagePath == "" {
			return "", fmt.Errorf("image_path is required for mode ocr")
		}
		lang := getParam(params, "lang", "eng+chi_sim")
		return n.ocrImage(ctx, imagePath, prompt, lang, provider, model, apiKey, endpoint, detail, outputFormat)
	case "compare":
		if imagePaths == "" && imagePath == "" {
			return "", fmt.Errorf("image_paths is required for compare mode")
		}
		var paths []string
		if imagePaths != "" {
			paths = strings.Split(imagePaths, ",")
		} else {
			paths = strings.Split(imagePath, ",")
		}
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
		return n.compareImages(ctx, paths, prompt, provider, model, apiKey, endpoint, detail, outputFormat)
	default:
		return "", fmt.Errorf("unsupported mode: %s (supported: image, describe, ocr, compare)", mode)
	}
}

func (n *MultimodalNode) analyzeImage(ctx context.Context, imagePath, prompt, provider, model, apiKey, endpoint, detail, mode, outputFormat string) (string, error) {
	imageURL, err := resolveImageURL(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve image: %w", err)
	}

	systemPrompt := ""
	switch mode {
	case "ocr":
		systemPrompt = "You are an OCR expert. Extract all visible text from the image accurately. Preserve formatting where possible."
	case "describe":
		systemPrompt = "You are a visual analysis expert. Describe the image thoroughly and accurately."
	}

	content, err := callVisionLLM(ctx, provider, model, apiKey, endpoint, imageURL, prompt, systemPrompt, detail)
	if err != nil {
		return "", err
	}

	return formatMultimodalOutput(content, imagePath, mode, outputFormat), nil
}

func (n *MultimodalNode) ocrImage(ctx context.Context, imagePath, prompt, lang, provider, model, apiKey, endpoint, detail, outputFormat string) (string, error) {
	// Sanitize the tesseract language code. lang is user-supplied and
	// passed as an argument to tesseract -l; although it does not go
	// through a shell, an attacker-controlled value could inject extra
	// tesseract flags (e.g. "--psm 0" or a leading "-") since argv parsing
	// treats any token starting with "-" as an option. Restrict to the
	// documented format: one or more 3-letter language codes joined by
	// '+' or '_' (e.g. "eng", "eng+chi_sim", "chi_sim"). Empty means
	// tesseract's default (eng).
	safeLang := sanitizeTesseractLang(lang)
	safePath, err := validateReadPath(imagePath)
	if err == nil {
		if _, statErr := os.Stat(safePath); statErr == nil { // codeql[go/path-injection] -- safePath is the validateReadPath result of imagePath
			if tess, lookErr := exec.LookPath("tesseract"); lookErr == nil {
				args := []string{safePath, "stdout", "-l", safeLang, "--oem", "1", "--psm", "6"}
				cmd := exec.CommandContext(ctx, tess, args...) // #nosec G204 -- tess from LookPath, args are standard OCR flags
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if runErr := cmd.Run(); runErr == nil {
					result := strings.TrimSpace(stdout.String())
					if result != "" {
						return formatMultimodalOutput("[Local OCR via tesseract]\n\n"+result, imagePath, "ocr", outputFormat), nil
					}
				}
			}
		}
	}

	systemPrompt := "You are an OCR expert. Extract all visible text from the image accurately. Preserve formatting where possible."
	imageURL, err := resolveImageURL(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve image: %w", err)
	}
	ocrPrompt := prompt
	if ocrPrompt == "" {
		ocrPrompt = "Extract all text from this image. Return only the text content."
	}
	content, err := callVisionLLM(ctx, provider, model, apiKey, endpoint, imageURL, ocrPrompt, systemPrompt, detail)
	if err != nil {
		return "", err
	}
	return formatMultimodalOutput(content, imagePath, "ocr", outputFormat), nil
}

func (n *MultimodalNode) compareImages(ctx context.Context, imagePaths []string, prompt, provider, model, apiKey, endpoint, detail, outputFormat string) (string, error) {
	if len(imagePaths) < 2 {
		return "", fmt.Errorf("compare mode requires at least 2 images")
	}

	var images []string
	for _, p := range imagePaths {
		url, err := resolveImageURL(p)
		if err != nil {
			return "", fmt.Errorf("failed to resolve image %s: %w", p, err)
		}
		images = append(images, url)
	}

	systemPrompt := "You are a visual comparison expert. Compare the images carefully and note similarities and differences."

	comparePrompt := prompt
	if comparePrompt == "" {
		comparePrompt = "Compare these images and describe the key similarities and differences."
	}

	content, err := callVisionLLMMultiImage(ctx, provider, model, apiKey, endpoint, images, comparePrompt, systemPrompt, detail)
	if err != nil {
		return "", err
	}

	return formatMultimodalOutput(content, strings.Join(imagePaths, ", "), "compare", outputFormat), nil
}

func resolveImageURL(imagePath string) (string, error) {
	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		return imagePath, nil
	}

	safePath, err := validateReadPath(imagePath)
	if err != nil {
		return "", fmt.Errorf("invalid image path: %w", err)
	}

	if _, err := os.Stat(safePath); os.IsNotExist(err) { // codeql[go/path-injection] -- safePath is the validateReadPath result of imagePath in resolveImageURL
		return "", fmt.Errorf("image file not found: %s", safePath)
	}

	data, err := os.ReadFile(safePath) // codeql[go/path-injection] -- safePath is the validateReadPath result of imagePath in resolveImageURL
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".bmp":
		mimeType = "image/bmp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

type visionMessageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL struct {
		URL    string `json:"url"`
		Detail string `json:"detail,omitempty"`
	} `json:"image_url,omitempty"`
}

type visionMessage struct {
	Role    string                 `json:"role"`
	Content []visionMessageContent `json:"content"`
}

// visionRequest is the OpenAI-compatible /chat/completions request body for
// multimodal (vision) calls. Unlike core.LLMRequest (whose Messages carry a
// plain-string Content), this uses visionMessage so each message can hold a
// mixed array of text and image_url content parts — the wire format vision
// providers require to actually see the image.
type visionRequest struct {
	Model    string          `json:"model"`
	Messages []visionMessage `json:"messages"`
}

func callVisionLLM(ctx context.Context, provider, model, apiKey, endpoint, imageURL, prompt, systemPrompt, detail string) (string, error) {
	var contents []visionMessageContent

	if systemPrompt != "" {
		contents = append(contents, visionMessageContent{
			Type: "text",
			Text: systemPrompt,
		})
	}

	contents = append(contents, visionMessageContent{
		Type: "text",
		Text: prompt,
	})

	imgContent := visionMessageContent{Type: "image_url"}
	imgContent.ImageURL.URL = imageURL
	if detail != "" {
		imgContent.ImageURL.Detail = detail
	}
	contents = append(contents, imgContent)

	msgs := []visionMessage{
		{Role: "user", Content: contents},
	}

	return callVisionAPI(ctx, provider, model, apiKey, endpoint, msgs)
}

func callVisionLLMMultiImage(ctx context.Context, provider, model, apiKey, endpoint string, imageURLs []string, prompt, systemPrompt, detail string) (string, error) {
	var contents []visionMessageContent

	if systemPrompt != "" {
		contents = append(contents, visionMessageContent{
			Type: "text",
			Text: systemPrompt,
		})
	}

	contents = append(contents, visionMessageContent{
		Type: "text",
		Text: prompt,
	})

	for _, url := range imageURLs {
		imgContent := visionMessageContent{Type: "image_url"}
		imgContent.ImageURL.URL = url
		if detail != "" {
			imgContent.ImageURL.Detail = detail
		}
		contents = append(contents, imgContent)
	}

	msgs := []visionMessage{
		{Role: "user", Content: contents},
	}

	return callVisionAPI(ctx, provider, model, apiKey, endpoint, msgs)
}

func callVisionAPI(ctx context.Context, provider, model, apiKey, endpoint string, messages []visionMessage) (string, error) {
	if apiKey == "" {
		envKey := strings.ToUpper(provider) + "_API_KEY"
		apiKey = os.Getenv(envKey)
	}

	return fallbackVisionCall(ctx, provider, model, apiKey, endpoint, messages)
}

// fallbackVisionCall sends the multimodal (vision) request directly to the
// provider's OpenAI-compatible /chat/completions endpoint. Unlike the old
// text-only path, this serializes the full visionMessage array — including
// image_url content parts — so the vision model actually receives the image.
// It reuses core.SafeLLMHTTPClient (SSRF-safe transport), core.ValidateLMLEndpoint
// and core.HTTPRedirectValidator so multimodal calls inherit the same network
// hardening as text LLM calls, and parses the response with core.LLMResponse
// (the wire format is identical to a text completion).
func fallbackVisionCall(ctx context.Context, provider, model, apiKey, endpoint string, messages []visionMessage) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no API key provided for %s (set %s_API_KEY)", provider, strings.ToUpper(provider))
	}

	if err := core.ValidateLMLEndpoint(endpoint); err != nil {
		return "", fmt.Errorf("endpoint URL validation failed: %w", err)
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	reqBody := visionRequest{
		Model:    model,
		Messages: messages,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal vision request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create vision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       core.DefaultLLMTimeout,
		Transport:     core.SafeLLMHTTPClient.Transport,
		CheckRedirect: core.HTTPRedirectValidator(core.ValidateLMLEndpoint),
	}

	resp, err := client.Do(req) // codeql[go/request-forgery] -- vision endpoint from provider config, pre-validated by core.ValidateLMLEndpoint; client uses core.SafeLLMHTTPClient.Transport (dial-time SSRF/IP validation, DNS-rebinding protection), core.HTTPRedirectValidator(core.ValidateLMLEndpoint) re-checks redirects, core.DefaultLLMTimeout; response capped by core.MaxHTTPResponseSize
	if err != nil {
		return "", fmt.Errorf("failed to call %s vision API: %w", provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp core.LLMResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, core.MaxHTTPResponseSize)).Decode(&errResp); err != nil {
			logger.Warn("failed to decode vision API error response", "provider", provider, "err", err)
		}
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("%s vision API error (%d): %s", provider, resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("%s vision API returned status %d", provider, resp.StatusCode)
	}

	var llmResp core.LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, core.MaxHTTPResponseSize)).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to parse %s vision response: %w", provider, err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in %s vision response", provider)
	}

	return llmResp.Choices[0].Message.Content, nil
}

// sanitizeTesseractLang validates and normalizes a tesseract language code
// (the -l argument). It permits one or more lowercase language codes (2-8
// ASCII letters) joined by '+' or '_' (e.g. "eng", "eng+chi_sim",
// "chi_sim+eng"). Any value containing path separators, whitespace,
// leading dashes, or other shell/argv metacharacters is rejected and the
// tesseract default ("eng") is used instead. This prevents argument
// injection where a malicious lang value could inject extra tesseract
// options.
func sanitizeTesseractLang(lang string) string {
	if lang == "" {
		return "eng"
	}
	if len(lang) > 64 {
		return "eng"
	}
	// Each code is 2-8 lowercase letters; separators are '+' or '_'.
	for _, r := range lang {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '+' || r == '_' {
			continue
		}
		return "eng"
	}
	// Reject leading separator / dash-like prefixes.
	if lang[0] == '+' || lang[0] == '_' {
		return "eng"
	}
	return lang
}

func formatMultimodalOutput(content, source, mode, outputFormat string) string {
	if outputFormat == "json" {
		// Marshal via encoding/json so user-controlled values (mode,
		// source, content) can never break the JSON structure.
		payload := struct {
			Mode   string `json:"mode"`
			Source string `json:"source"`
			Result string `json:"result"`
		}{Mode: mode, Source: source, Result: content}
		if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
			return string(b)
		}
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("## Multimodal Analysis (%s)\n\n", mode))
	builder.WriteString(fmt.Sprintf("**Source:** %s\n\n", source))
	builder.WriteString("---\n\n")
	builder.WriteString(content)
	return builder.String()
}

// escapeJSON returns s as a JSON-encoded string (with surrounding quotes).
// Uses json.Marshal so all control characters, Unicode escapes, and other
// JSON-special characters are handled correctly regardless of the LLM output.
func escapeJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal never fails for a string input, but guard defensively.
		return "\"\""
	}
	return string(b)
}
