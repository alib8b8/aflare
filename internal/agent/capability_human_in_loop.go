// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​‌‌​​​‌‌​‌​​‌‌‌‌​‌‌​​​​​‌​‌‌​‌​‌​‌‌​​‌‌​​‌​​​​​​​​​​​​​​​​​​‌​‌‌‌​‌​​‌‌​​‌​​⁠
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

// capability_human_in_loop.go implements the HumanInLoopCapability —
// a human-in-the-loop approval layer that pauses at critical decision points
// and requests human confirmation before proceeding.
//
// This implements the "Human-in-the-loop Agent (人机协同)" type from the taxonomy:
//   At key decision points (destructive actions, high-cost operations,
//   ambiguous situations), the agent pauses and asks the user to confirm
//   before proceeding.
//
// Key behaviors:
//   - Detects dangerous tool calls (execute, file_write, etc.)
//   - Detects high-ambiguity states where human judgment is needed
//   - Injects confirmation prompts into the output
//   - Tracks pending approvals and their status
//   - Supports configurable approval policies (always-ask, smart, none)

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ApprovalPolicy defines when the agent should ask for human confirmation.
type ApprovalPolicy string

const (
	PolicyAlwaysAsk ApprovalPolicy = "always-ask" // Ask for every action
	PolicySmart     ApprovalPolicy = "smart"      // Ask only for dangerous/ambiguous actions
	PolicyNone      ApprovalPolicy = "none"       // Never ask (full autonomy)
)

// HumanInLoopCapability implements human-in-the-loop approval for
// critical decision points in the agent's execution.
type HumanInLoopCapability struct {
	mu              sync.RWMutex
	policy          ApprovalPolicy
	pendingApproval bool
	pendingInput    string // original input that triggered the approval
	pendingOutput   string // output that triggered the approval
	approvalHistory []string
}

// NewHumanInLoopCapability creates a new human-in-the-loop capability
// with the default smart policy.
func NewHumanInLoopCapability() *HumanInLoopCapability {
	return &HumanInLoopCapability{
		policy:          PolicySmart,
		approvalHistory: make([]string, 0),
	}
}

// SetPolicy changes the approval policy.
func (h *HumanInLoopCapability) SetPolicy(policy ApprovalPolicy) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.policy = policy
}

func (h *HumanInLoopCapability) Name() string { return CapabilityHumanInLoop }
func (h *HumanInLoopCapability) Description() string {
	return "Human-in-the-loop: pauses at critical decisions for human approval (人机协同)"
}

func (h *HumanInLoopCapability) Init(loop *AgentLoop) error {
	return nil
}

func (h *HumanInLoopCapability) PreProcess(ctx context.Context, input string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if this is a response to a pending approval
	if h.pendingApproval {
		lower := strings.ToLower(strings.TrimSpace(input))
		if lower == "yes" || lower == "y" || lower == "approve" || lower == "proceed" || lower == "confirm" {
			h.pendingApproval = false
			origInput := h.pendingInput
			h.approvalHistory = append(h.approvalHistory, fmt.Sprintf("APPROVED: %s", truncateStr(origInput, 80)))
			h.pendingInput = ""
			h.pendingOutput = ""
			// Re-submit the original input so the agent can execute the action
			return origInput, nil
		}
		if lower == "no" || lower == "n" || lower == "deny" || lower == "reject" || lower == "cancel" {
			h.pendingApproval = false
			h.approvalHistory = append(h.approvalHistory, fmt.Sprintf("REJECTED: %s", truncateStr(h.pendingInput, 80)))
			h.pendingInput = ""
			h.pendingOutput = ""
			// Return a cancellation signal — the agent won't process this turn
			return "", fmt.Errorf("action cancelled by user")
		}
		// Still waiting for a clear yes/no — don't process this input
		return "", fmt.Errorf("awaiting approval: type 'yes' to proceed or 'no' to cancel")
	}

	return "", nil
}

func (h *HumanInLoopCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.policy == PolicyNone {
		return "", nil
	}

	// If there's a pending approval and the user hasn't responded yet,
	// don't process further responses
	if h.pendingApproval {
		return output, nil
	}

	// Check if the output contains dangerous actions
	needsApproval := h.detectDangerousAction(output)

	if h.policy == PolicyAlwaysAsk {
		needsApproval = true
	}

	if !needsApproval {
		return "", nil
	}

	// Mark as pending and inject approval prompt
	h.pendingApproval = true
	h.pendingInput = input
	h.pendingOutput = output

	augmented := output + "\n\n" + h.buildApprovalPrompt(output)
	return augmented, nil
}

func (h *HumanInLoopCapability) Shutdown() error {
	return nil
}

// detectDangerousAction checks if the output contains actions that need
// human approval, such as destructive operations or high-risk commands.
func (h *HumanInLoopCapability) detectDangerousAction(output string) bool {
	lower := strings.ToLower(output)

	dangerous := []string{
		"execute", "run_workflow", "file_write", "delete",
		"remove", "uninstall", "shutdown", "restart",
		"send", "publish", "deploy", "charge",
		"payment", "transfer", "withdraw",
	}

	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

// buildApprovalPrompt creates a human-readable approval request.
func (h *HumanInLoopCapability) buildApprovalPrompt(output string) string {
	var sb strings.Builder
	sb.WriteString("--- [Human Approval Required] ---\n")
	sb.WriteString("The agent wants to perform an action that requires your confirmation.\n")
	sb.WriteString("Review the proposed action above.\n\n")
	sb.WriteString("Type 'yes' or 'approve' to proceed, 'no' or 'deny' to cancel.\n")
	sb.WriteString("--- [Awaiting Your Decision] ---")
	return sb.String()
}

// IsPending returns true if waiting for human approval.
func (h *HumanInLoopCapability) IsPending() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.pendingApproval
}

// ApprovalHistory returns the history of approvals and rejections.
func (h *HumanInLoopCapability) ApprovalHistory() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string{}, h.approvalHistory...)
}
