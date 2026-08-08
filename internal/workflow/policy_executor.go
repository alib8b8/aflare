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

package workflow

import (
	"context"
	"fmt"

	"github.com/alib8b8/aflare/internal/policy"
)

// PolicyExecutor wraps the standard Executor with a Policy Engine that
// validates every step before execution. It is the recommended way to run
// AI-generated workflows in production.
//
// Usage:
//
//	pe := NewPolicyExecutor(exec, policyEngine)
//	out, results, err := pe.ExecuteWithTrace(ctx, wf, reg, nil)
type PolicyExecutor struct {
	*Executor
	policyEngine *policy.Engine
}

// NewPolicyExecutor creates a PolicyExecutor that wraps an existing Executor
// with policy enforcement. The policy engine validates every step before
// execution, blocking dangerous operations at the policy level.
func NewPolicyExecutor(exec *Executor, pe *policy.Engine) *PolicyExecutor {
	return &PolicyExecutor{
		Executor:     exec,
		policyEngine: pe,
	}
}

// ValidateWorkflow checks every step in a workflow against the policy before
// execution. Call this BEFORE ExecuteWithTrace to fail fast on policy violations
// rather than mid-execution.
func (pe *PolicyExecutor) ValidateWorkflow(ctx context.Context, wf *Workflow) error {
	if pe.policyEngine == nil {
		return nil // no policy engine configured, allow everything
	}

	for i, step := range wf.Steps {
		if err := pe.validateStep(ctx, i, &step); err != nil {
			return fmt.Errorf("policy violation in step %d (%s): %w", i+1, step.Node, err)
		}
	}
	return nil
}

// validateStep checks a single step against the policy.
func (pe *PolicyExecutor) validateStep(ctx context.Context, index int, step *WorkflowStep) error {
	switch {
	case step.Node == "shell" || step.Node == "exec":
		cmd := ""
		if step.Params != nil {
			cmd = step.Params["command"]
		}
		_, err := pe.policyEngine.Check(ctx, policy.ActionShellExec, cmd)
		if err != nil {
			return fmt.Errorf("shell execution not allowed: %w", err)
		}

	case step.Node == "http_request" || step.Node == "fetch_url":
		url := ""
		if step.Params != nil {
			url = step.Params["url"]
		}
		_, err := pe.policyEngine.Check(ctx, policy.ActionNetworkHTTP, url)
		if err != nil {
			return fmt.Errorf("network access not allowed: %w", err)
		}

	case step.Node == "file_write" || step.Node == "file_save":
		_, err := pe.policyEngine.Check(ctx, policy.ActionFileWrite, step.Node)
		if err != nil {
			return fmt.Errorf("file write not allowed: %w", err)
		}

	case step.Node == "file_delete":
		_, err := pe.policyEngine.Check(ctx, policy.ActionFileDelete, step.Node)
		if err != nil {
			return fmt.Errorf("file delete not allowed: %w", err)
		}

	case step.Node == "transfer" || step.Node == "payment":
		_, err := pe.policyEngine.Check(ctx, policy.ActionFinancial, step.Node)
		if err != nil {
			return fmt.Errorf("financial operation not allowed: %w", err)
		}
	}

	// Recurse into compound steps
	if step.IsParallel() {
		for _, s := range step.Parallel {
			ws := WorkflowStep{Node: s.Node, Params: s.Params}
			if err := pe.validateStep(ctx, index, &ws); err != nil {
				return err
			}
		}
	}
	if step.IsSaga() && step.Saga != nil {
		for _, ss := range step.Saga.Steps {
			if err := pe.validateStep(ctx, index, &ss.Forward); err != nil {
				return err
			}
			if ss.Compensate != nil {
				if err := pe.validateStep(ctx, index, ss.Compensate); err != nil {
					return err
				}
			}
		}
	}
	if step.IsIf() && step.If != nil {
		for _, s := range step.If.Then {
			if err := pe.validateStep(ctx, index, &s); err != nil {
				return err
			}
		}
		for _, s := range step.If.Else {
			if err := pe.validateStep(ctx, index, &s); err != nil {
				return err
			}
		}
	}

	return nil
}

// Engine returns the underlying policy engine for inspection.
func (pe *PolicyExecutor) Engine() *policy.Engine {
	return pe.policyEngine
}
