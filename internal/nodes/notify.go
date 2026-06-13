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
