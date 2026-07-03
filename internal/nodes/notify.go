package nodes

import (
	"context"
	"fmt"
	"os"
)

type NotifyNode struct{}

func init() {
	Register(&NotifyNode{})
}

func (n *NotifyNode) Name() string {
	return "notify"
}

func (n *NotifyNode) Description() string {
	return "Send notifications (stdout, webhook)"
}

func (n *NotifyNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "notify",
		Description: "Send notifications (stdout, webhook)",
		Input:       "string - message to notify (used if message param is empty)",
		Output:      "string - the notification message",
		Params: []ParamSchema{
			{Name: "channel", Type: "string", Description: "Notification channel: stdout, stderr (default: stdout)", Required: false, Default: "stdout"},
			{Name: "message", Type: "string", Description: "Notification message (overrides input)", Required: false},
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
	default:
		fmt.Println("[notify]", message)
		return message, nil
	}
}
