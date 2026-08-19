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
		Transport:     safeHTTPClient.Transport,
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
	return "Send notifications (stdout, stderr, slack, discord, telegram, feishu, dingtalk, wecom, webhook)"
}

func (n *NotifyNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "notify",
		Description: "Send notifications (stdout, stderr, slack, discord, telegram, feishu, dingtalk, wecom, webhook)",
		Input:       "string - message to notify (used if message param is empty)",
		Output:      "string - the notification message",
		Params: []ParamSchema{
			{Name: "channel", Type: "string", Description: "Notification channel: stdout, stderr, slack, discord, telegram, feishu, dingtalk, wecom, webhook (default: stdout)", Required: false, Default: "stdout"},
			{Name: "message", Type: "string", Description: "Notification message (overrides input)", Required: false},
			{Name: "url", Type: "string", Description: "Webhook URL for slack/discord/webhook/feishu/dingtalk/wecom, or Telegram API base (required for external channels)", Required: false},
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
	case "feishu":
		return n.sendFeishu(ctx, message, params)
	case "dingtalk":
		return n.sendDingtalk(ctx, message, params)
	case "wecom":
		return n.sendWecom(ctx, message, params)
	case "webhook":
		return n.sendWebhook(ctx, message, params)
	default:
		return "", fmt.Errorf("invalid notification channel: %s (supported: stdout, stderr, slack, discord, telegram, feishu, dingtalk, wecom, webhook)", channel)
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

// groupBotResp tolerates the three CN group-bot webhook response shapes:
// dingtalk/wecom answer {"errcode":0,"errmsg":"ok"} while feishu answers
// {"code":0,"msg":"success"} (or the legacy {"StatusCode":0,...}). All three
// return HTTP 200 even for an invalid token, so the error code in the body is
// the only success signal — an un-parsed body must fail loudly.
type groupBotResp struct {
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
	Code          int    `json:"code"`
	Msg           string `json:"msg"`
	StatusCode    int    `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
}

// postGroupBotWebhook is the shared transport for the CN group-bot channels
// (feishu/dingtalk/wecom): validate the webhook URL (HTTPS + SSRF checks),
// marshal the payload under the size cap, POST it, and surface the API error
// code when the platform rejected the message (e.g. bad token, rate limit).
func (n *NotifyNode) postGroupBotWebhook(ctx context.Context, channel string, webhookURL string, payload interface{}) error {
	if err := notifyURLValidator(webhookURL); err != nil {
		return fmt.Errorf("%s URL validation failed: %w", channel, err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal %s payload: %w", channel, err)
	}
	if len(payloadBytes) > maxNotifyBodySize {
		return fmt.Errorf("%s payload exceeds maximum size of %d bytes", channel, maxNotifyBodySize)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", channel, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", channel, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return fmt.Errorf("failed to read %s response: %w", channel, err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s returned status %d", channel, resp.StatusCode)
	}

	var r groupBotResp
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("%s returned non-JSON response: %w", channel, err)
	}
	switch {
	case r.ErrCode != 0:
		return fmt.Errorf("%s API error %d: %s", channel, r.ErrCode, r.ErrMsg)
	case r.Code != 0:
		return fmt.Errorf("%s API error %d: %s", channel, r.Code, r.Msg)
	case r.StatusCode != 0:
		return fmt.Errorf("%s API error %d: %s", channel, r.StatusCode, r.StatusMessage)
	}
	return nil
}

// sendFeishu posts a text message via a Feishu (Lark) group custom-bot
// webhook: the user creates the bot in a group and pastes the hook URL
// (https://open.feishu.cn/open-apis/bot/v2/hook/<token>).
func (n *NotifyNode) sendFeishu(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		return "", fmt.Errorf("url parameter is required when channel=feishu")
	}
	payload := map[string]interface{}{
		"msg_type": "text",
		"content":  map[string]string{"text": message},
	}
	if err := n.postGroupBotWebhook(ctx, "feishu", webhookURL, payload); err != nil {
		return "", err
	}
	fmt.Printf("[notify] feishu sent (status ok)\n")
	return message, nil
}

// sendDingtalk posts a text message via a DingTalk group custom-bot webhook
// (https://oapi.dingtalk.com/robot/send?access_token=<token>).
func (n *NotifyNode) sendDingtalk(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		return "", fmt.Errorf("url parameter is required when channel=dingtalk")
	}
	payload := map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": message},
	}
	if err := n.postGroupBotWebhook(ctx, "dingtalk", webhookURL, payload); err != nil {
		return "", err
	}
	fmt.Printf("[notify] dingtalk sent (status ok)\n")
	return message, nil
}

// sendWecom posts a text message via a WeCom (企业微信) group-bot webhook
// (https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=<token>). Personal
// WeChat and QQ have no official bot API — WeCom group bots are the only
// compliant WeChat-ecosystem push path.
func (n *NotifyNode) sendWecom(ctx context.Context, message string, params map[string]string) (string, error) {
	webhookURL := params["url"]
	if webhookURL == "" {
		return "", fmt.Errorf("url parameter is required when channel=wecom")
	}
	payload := map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": message},
	}
	if err := n.postGroupBotWebhook(ctx, "wecom", webhookURL, payload); err != nil {
		return "", err
	}
	fmt.Printf("[notify] wecom sent (status ok)\n")
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
