package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type NotifyNode struct{}

func init() {
	Register(&NotifyNode{})
}

func (n *NotifyNode) Name() string {
	return "notify"
}

func (n *NotifyNode) Description() string {
	return "Send notifications (stdout, stderr, webhook)"
}

func (n *NotifyNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "notify",
		Description: "Send notifications (stdout, stderr, webhook)",
		Input:       "string - message to notify (used if message param is empty)",
		Output:      "string - the notification message",
		Params: []ParamSchema{
			{Name: "channel", Type: "string", Description: "Notification channel: stdout, stderr, webhook (default: stdout)", Required: false, Default: "stdout"},
			{Name: "message", Type: "string", Description: "Notification message (overrides input)", Required: false},
			{Name: "webhook_url", Type: "string", Description: "Webhook URL (required when channel=webhook)", Required: false},
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
	case "webhook":
		webhookURL, ok := params["webhook_url"]
		if !ok || webhookURL == "" {
			return "", fmt.Errorf("webhook_url parameter is required when channel=webhook")
		}
		if err := validateURL(webhookURL); err != nil {
			return "", fmt.Errorf("webhook URL validation failed: %w", err)
		}

		payload := map[string]string{"text": message, "message": message}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("failed to marshal webhook payload: %w", err)
		}

		client := &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: httpRedirectValidator(validateURL),
		}

		req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("webhook request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
		}

		fmt.Printf("[notify] webhook sent to %s (status %d)\n", webhookURL, resp.StatusCode)
		return message, nil
	default:
		return "", fmt.Errorf("invalid notification channel: %s (supported: stdout, stderr, webhook)", channel)
	}
}
