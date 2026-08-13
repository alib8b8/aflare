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

package agent

import (
	"context"
	"strings"
	"testing"
)

func TestHumanInLoopCapability_Basic(t *testing.T) {
	h := NewHumanInLoopCapability()
	if h.Name() != "human-in-loop" {
		t.Errorf("Name() = %q, want %q", h.Name(), "human-in-loop")
	}
	if h.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(h.Description(), "Human") {
		t.Errorf("Description() = %q, want it to mention Human", h.Description())
	}
	if err := h.Init(&AgentLoop{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := h.Shutdown(); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestHumanInLoopCapability_SetPolicy(t *testing.T) {
	h := NewHumanInLoopCapability()
	h.SetPolicy(PolicyAlwaysAsk)
	if h.policy != PolicyAlwaysAsk {
		t.Errorf("policy = %v, want %v", h.policy, PolicyAlwaysAsk)
	}
	h.SetPolicy(PolicyNone)
	if h.policy != PolicyNone {
		t.Errorf("policy = %v, want %v", h.policy, PolicyNone)
	}
}

func TestHumanInLoopCapability_PreProcess(t *testing.T) {
	t.Run("not pending returns empty", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		out, err := h.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output when not pending, got %q", out)
		}
	})

	t.Run("approve resumes original input", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.pendingApproval = true
		h.pendingInput = "original task"
		h.pendingOutput = "dangerous output"

		out, err := h.PreProcess(context.Background(), "yes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "original task" {
			t.Errorf("expected original input resumed, got %q", out)
		}
		if h.pendingApproval {
			t.Error("pendingApproval should be cleared after approval")
		}
		if h.pendingInput != "" || h.pendingOutput != "" {
			t.Error("pending fields should be cleared after approval")
		}
		if len(h.approvalHistory) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(h.approvalHistory))
		}
		if !strings.HasPrefix(h.approvalHistory[0], "APPROVED") {
			t.Errorf("expected APPROVED prefix, got %q", h.approvalHistory[0])
		}
	})

	t.Run("deny cancels with error", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.pendingApproval = true
		h.pendingInput = "original task"

		out, err := h.PreProcess(context.Background(), "no")
		if err == nil {
			t.Fatal("expected error on deny")
		}
		if out != "" {
			t.Errorf("expected empty output on deny, got %q", out)
		}
		if h.pendingApproval {
			t.Error("pendingApproval should be cleared after deny")
		}
		if len(h.approvalHistory) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(h.approvalHistory))
		}
		if !strings.HasPrefix(h.approvalHistory[0], "REJECTED") {
			t.Errorf("expected REJECTED prefix, got %q", h.approvalHistory[0])
		}
	})

	t.Run("ambiguous response awaits", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.pendingApproval = true
		h.pendingInput = "original task"

		out, err := h.PreProcess(context.Background(), "maybe later")
		if err == nil {
			t.Fatal("expected error when awaiting approval")
		}
		if out != "" {
			t.Errorf("expected empty output when awaiting, got %q", out)
		}
		if !h.pendingApproval {
			t.Error("pendingApproval should remain true when awaiting clear yes/no")
		}
	})
}

func TestHumanInLoopCapability_PostProcess(t *testing.T) {
	t.Run("none policy returns empty", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.SetPolicy(PolicyNone)
		out, err := h.PostProcess(context.Background(), "in", "rm -rf /")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output for none policy, got %q", out)
		}
	})

	t.Run("pending returns output as-is", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.pendingApproval = true
		out, err := h.PostProcess(context.Background(), "in", "some output")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "some output" {
			t.Errorf("expected output returned as-is when pending, got %q", out)
		}
	})

	t.Run("smart policy safe output returns empty", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.SetPolicy(PolicySmart)
		out, err := h.PostProcess(context.Background(), "in", "just a harmless summary")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output for safe content, got %q", out)
		}
		if h.pendingApproval {
			t.Error("should not be pending for safe output")
		}
	})

	t.Run("smart policy dangerous injects prompt", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.SetPolicy(PolicySmart)
		out, err := h.PostProcess(context.Background(), "in", "I will delete the files now")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Human Approval Required") {
			t.Errorf("expected approval prompt injected, got %q", out)
		}
		if !h.pendingApproval {
			t.Error("should be pending after dangerous output")
		}
		if h.pendingInput != "in" {
			t.Errorf("pendingInput = %q, want %q", h.pendingInput, "in")
		}
	})

	t.Run("always-ask policy injects prompt", func(t *testing.T) {
		h := NewHumanInLoopCapability()
		h.SetPolicy(PolicyAlwaysAsk)
		out, err := h.PostProcess(context.Background(), "in", "harmless summary")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Human Approval Required") {
			t.Errorf("expected approval prompt for always-ask, got %q", out)
		}
	})
}

func TestHumanInLoopCapability_detectDangerousAction(t *testing.T) {
	h := NewHumanInLoopCapability()
	dangerous := []string{
		"execute command", "run_workflow now", "file_write data",
		"delete file", "remove dir", "uninstall package",
		"shutdown system", "restart server", "send email",
		"publish article", "deploy app", "charge card",
		"payment done", "transfer funds", "withdraw cash",
	}
	for _, d := range dangerous {
		if !h.detectDangerousAction(d) {
			t.Errorf("detectDangerousAction(%q) = false, want true", d)
		}
	}
	safe := []string{
		"just a summary", "hello world", "the weather is nice",
	}
	for _, s := range safe {
		if h.detectDangerousAction(s) {
			t.Errorf("detectDangerousAction(%q) = true, want false", s)
		}
	}
}

func TestHumanInLoopCapability_buildApprovalPrompt(t *testing.T) {
	h := NewHumanInLoopCapability()
	prompt := h.buildApprovalPrompt("some output")
	if !strings.Contains(prompt, "Human Approval Required") {
		t.Error("expected approval header")
	}
	if !strings.Contains(prompt, "yes") || !strings.Contains(prompt, "no") {
		t.Error("expected instructions to mention yes/no")
	}
}

func TestHumanInLoopCapability_IsPending(t *testing.T) {
	h := NewHumanInLoopCapability()
	if h.IsPending() {
		t.Error("new capability should not be pending")
	}
	h.pendingApproval = true
	if !h.IsPending() {
		t.Error("should be pending after setting flag")
	}
}

func TestHumanInLoopCapability_ApprovalHistory(t *testing.T) {
	h := NewHumanInLoopCapability()
	h.approvalHistory = []string{"APPROVED: a", "REJECTED: b"}
	hist := h.ApprovalHistory()
	if len(hist) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(hist))
	}
	// Returned slice should be a copy.
	hist[0] = "mutated"
	if h.approvalHistory[0] != "APPROVED: a" {
		t.Error("ApprovalHistory should return a copy, internal state was mutated")
	}
}
