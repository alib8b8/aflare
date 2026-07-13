package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type NotifyNode struct{}

func init() {
	Register(&NotifyNode{})
}

const (
	notifyTimeout     = 10 * time.Second
	maxNotifyBodySize = 100 * 1024 // 100KB
)

var (
	notifyHTTPClient = &http.Client{
		Timeout:       notifyTimeout,
		CheckRedirect: httpRedirectValidator(validateNotifyURL),
	}
	notifyURLValidator = validateNotifyURL
)

func validateNotifyURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed for notifications, got: %s", u.Scheme)
	}
	return validateURL(rawURL)
}

func (n *NotifyNode) Name() string {
	return "notify"
}

func (n *NotifyNode) Description() string {
	return "Send notifications (stdout, stderr, slack, discord, telegram, webhook)"
}

func (n *NotifyNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "notify",
		Description: "Send notifications (stdout, stderr, slack, discord, telegram, webhook)",
		Input:       "string - message to notify (used if message param is empty)",
		Output:      "string - the notification message",
		Params: []ParamSchema{
			{Name: "channel", Type: "string", Description: "Notification channel: stdout, stderr, slack, discord, telegram, webhook (default: stdout)", Required: false, Default: "stdout"},
			{Name: "message", Type: "string", Description: "Notification message (overrides input)", Required: false},
			{Name: "url", Type: "string", Description: "Webhook URL for slack/discord/webhook, or Telegram API base (required for external channels)", Required: false},
			{Name: "webhook_url", Type: "string", Description: "Deprecated: use url instead", Required: false},
			{Name: "token", Type: "string", Description: "Bot token (required when channel=telegram)", Required: false},
			{Name: "chat_id", Type: "string", Description: "Telegram chat ID (required when channel=telegram)", Required: false},
			{Name: "username", Type: "string", Description: "Discord webhook username (optional)", Required: false},
			{Name: "method", Type: "string", Description: "HTTP method for webhook: GET/POST/PUT (default: POST)", Required: false, Default: "POST"},
			{Name: "headers", Type: "string", Description: "JSON headers for webhook (optional)", Required: false},
			{Name: "body", Type: "string", Description: "Custom body for webhook (optional)", Required: false},
		},
	}
}

func (n *NotifyNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	channel, ok := params["channel"]
	if !ok || channel == "" {
		channel = "stdout"
	}

	message, ok := params["message"]
	if !ok || message == "" {
		message = input
	}

	switch channel {
	case "stdout":
		fmt.Println("[notify]", message)
		return message, nil
	case "stderr":
		fmt.Fprintln(os.Stderr, "[notify]", message)
		return message, nil
	case "slack":
		return n.sendSlack(ctx, message, params)
	case "discord":
		return n.sendDiscord(ctx, message, params)
	case "telegram":
		return n.sendTelegram(ctx, message, params)
	case "webhook":
		return n.sendWebhook(ctx, message, params)
	default:
		return "", fmt.Errorf("invalid notification channel: %s (supported: stdout, stderr, slack, discord, telegram, webhook)", channel)
	}
}

func (n *NotifyNode) sendSlack(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		return "", fmt.Errorf("url parameter is required when channel=slack")
	}
	if err := notifyURLValidator(webhookURL); err != nil {
		return "", fmt.Errorf("slack URL validation failed: %w", err)
	}

	payload := map[string]string{"text": message}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal slack payload: %w", err)
	}
	if len(payloadBytes) > maxNotifyBodySize {
		return "", fmt.Errorf("slack payload exceeds maximum size of %d bytes", maxNotifyBodySize)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	safeURL := RedactSensitive(webhookURL)
	fmt.Printf("[notify] slack sent to %s (status %d)\n", safeURL, resp.StatusCode)
	return message, nil
}

func (n *NotifyNode) sendDiscord(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		return "", fmt.Errorf("url parameter is required when channel=discord")
	}
	if err := notifyURLValidator(webhookURL); err != nil {
		return "", fmt.Errorf("discord URL validation failed: %w", err)
	}

	payload := map[string]string{"content": message}
	if username := params["username"]; username != "" {
		payload["username"] = username
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal discord payload: %w", err)
	}
	if len(payloadBytes) > maxNotifyBodySize {
		return "", fmt.Errorf("discord payload exceeds maximum size of %d bytes", maxNotifyBodySize)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	safeURL := RedactSensitive(webhookURL)
	fmt.Printf("[notify] discord sent to %s (status %d)\n", safeURL, resp.StatusCode)
	return message, nil
}

func (n *NotifyNode) sendTelegram(ctx context.Context, message string, params map[string]string) (string, error) {
	token := params["token"]
	if token == "" {
		return "", fmt.Errorf("token parameter is required when channel=telegram")
	}
	chatID := params["chat_id"]
	if chatID == "" {
		return "", fmt.Errorf("chat_id parameter is required when channel=telegram")
	}

	telegramURL := "https://api.telegram.org/bot" + url.PathEscape(token) + "/sendMessage"
	if err := notifyURLValidator(telegramURL); err != nil {
		return "", fmt.Errorf("telegram URL validation failed: %w", err)
	}

	payload := map[string]string{
		"chat_id": chatID,
		"text":    message,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal telegram payload: %w", err)
	}
	if len(payloadBytes) > maxNotifyBodySize {
		return "", fmt.Errorf("telegram payload exceeds maximum size of %d bytes", maxNotifyBodySize)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}

	safeToken := redactAPIKey(token)
	fmt.Printf("[notify] telegram sent to chat %s via bot %s (status %d)\n", chatID, safeToken, resp.StatusCode)
	return message, nil
}

func (n *NotifyNode) sendWebhook(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		webhookURL = params["webhook_url"]
	}
	if webhookURL == "" {
		return "", fmt.Errorf("url or webhook_url parameter is required when channel=webhook")
	}
	if err := notifyURLValidator(webhookURL); err != nil {
		return "", fmt.Errorf("webhook URL validation failed: %w", err)
	}

	method := params["method"]
	if method == "" {
		method = http.MethodPost
	}
	method = strings.ToUpper(method)
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut {
		return "", fmt.Errorf("unsupported webhook method: %s (supported: GET, POST, PUT)", method)
	}

	var body []byte
	if customBody := params["body"]; customBody != "" {
		body = []byte(customBody)
	} else {
		payload := map[string]string{"text": message, "message": message}
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("failed to marshal webhook payload: %w", err)
		}
	}
	if len(body) > maxNotifyBodySize {
		return "", fmt.Errorf("webhook payload exceeds maximum size of %d bytes", maxNotifyBodySize)
	}

	var bodyReader io.Reader
	if method == http.MethodGet {
		bodyReader = nil
	} else {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, webhookURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook request: %w", err)
	}

	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	if headersJSON := params["headers"]; headersJSON != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			return "", fmt.Errorf("failed to parse headers JSON: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	safeURL := RedactSensitive(webhookURL)
	fmt.Printf("[notify] webhook sent to %s (method %s, status %d)\n", safeURL, method, resp.StatusCode)
	return message, nil
}
