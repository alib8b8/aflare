package nodes

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ExecuteNode struct{}

func init() {
	Register(&ExecuteNode{})
}

func (n *ExecuteNode) Name() string {
	return "execute"
}

func (n *ExecuteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("execute node is disabled in safe mode")
	}

	command, ok := params["command"]
	if !ok || command == "" {
		return "", fmt.Errorf("command parameter is required")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
