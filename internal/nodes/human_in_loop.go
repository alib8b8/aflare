package nodes

import (
	"context"
	"fmt"
	"os"
)

type HumanInLoopNode struct{}

func init() {
	Register(&HumanInLoopNode{})
}

func (n *HumanInLoopNode) Name() string {
	return "human_in_loop"
}

func (n *HumanInLoopNode) Description() string {
	return "Human-in-the-loop approval gate for workflow steps"
}

func (n *HumanInLoopNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "human_in_loop",
		Description: "Human approval gate — pauses workflow for human review and approval before continuing",
		Input:       "string - the content/data to present for human review",
		Output:      "string - approved content (or original if approved)",
		Params: []ParamSchema{
			{Name: "mode", Type: "string", Description: "Approval mode: file, env, stdin, auto_approve (default: file)", Required: false, Default: "file"},
			{Name: "approval_file", Type: "string", Description: "Path to approval flag file (mode=file)", Required: false, Default: ".llm-box-approval"},
			{Name: "approval_env", Type: "string", Description: "Environment variable to check for approval (mode=env)", Required: false, Default: "LLM_BOX_APPROVED"},
			{Name: "prompt", Type: "string", Description: "Custom prompt message for the human reviewer", Required: false},
			{Name: "timeout", Type: "string", Description: "Timeout in seconds before failing (default: 3600)", Required: false, Default: "3600"},
			{Name: "on_approve", Type: "string", Description: "What to output on approve: original, modified, passthrough (default: original)", Required: false, Default: "original"},
		},
	}
}

func (n *HumanInLoopNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	mode := getParam(params, "mode", "file")
	approvalFile := getParam(params, "approval_file", ".llm-box-approval")
	approvalEnv := getParam(params, "approval_env", "LLM_BOX_APPROVED")
	customPrompt := getParam(params, "prompt", "")
	onApprove := getParam(params, "on_approve", "original")

	switch mode {
	case "auto_approve":
		return buildApprovedOutput(input, onApprove, true, "auto-approved"), nil

	case "env":
		envVal := os.Getenv(approvalEnv)
		approved := envVal == "1" || envVal == "true" || envVal == "yes"
		if approved {
			return buildApprovedOutput(input, onApprove, true, fmt.Sprintf("env %s=approved", approvalEnv)), nil
		}
		return "", fmt.Errorf("human approval required: set %s=1 to approve", approvalEnv)

	case "file":
		_, err := os.Stat(approvalFile)
		if err == nil {
			return buildApprovedOutput(input, onApprove, true, fmt.Sprintf("file %s exists", approvalFile)), nil
		}
		reviewContent := input
		if customPrompt != "" {
			reviewContent = customPrompt + "\n\n" + input
		}
		reviewFile := approvalFile + ".review"
		if writeErr := os.WriteFile(reviewFile, []byte(reviewContent), 0644); writeErr != nil {
			return "", fmt.Errorf("failed to write review file: %w", writeErr)
		}
		return "", fmt.Errorf("human approval required: review %s, then create %s to approve", reviewFile, approvalFile)

	case "stdin":
		fmt.Fprintln(os.Stderr, "=== Human Approval Required ===")
		if customPrompt != "" {
			fmt.Fprintln(os.Stderr, customPrompt)
		}
		fmt.Fprintln(os.Stderr, "Input preview:")
		preview := input
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Fprintln(os.Stderr, preview)
		fmt.Fprintln(os.Stderr, "Approve? [y/N]: ")
		return "", fmt.Errorf("interactive stdin approval not supported in non-interactive mode")

	default:
		return "", fmt.Errorf("unknown approval mode: %s", mode)
	}
}

func buildApprovedOutput(input, mode string, approved bool, reason string) string {
	if !approved {
		return input
	}
	switch mode {
	case "passthrough":
		return input
	case "modified":
		return fmt.Sprintf("Approved (%s):\n%s", reason, input)
	default:
		return input
	}
}
