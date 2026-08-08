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

// Package policy provides the Policy Engine — a security layer that sits
// between AI-generated YAML workflows and the Executor. Before any step
// executes, the Policy Engine validates it against configured rules.
//
// The Policy Engine solves a fundamental problem with AI-generated workflows:
// even though YAML is deterministic, the AI might generate dangerous commands.
// The Policy Engine ensures that:
//
//   - Filesystem operations respect allow/deny/approval rules
//   - Network access is restricted to allowlists
//   - Shell execution can be disabled entirely or require approval
//   - Financial operations (transfers, payments) require explicit approval
//   - Sensitive operations are audited with policy decisions
//
// Policies are YAML files that can be loaded at startup:
//
//	policy:
//	  filesystem:
//	    read: allowed
//	    write: approval
//	    delete: denied
//	  network:
//	    outbound: allowlist
//	    allowlist:
//	      - "api.github.com"
//	      - "*.openai.com"
//	  shell:
//	    enabled: false
//	  financial:
//	    transfer: approval_required
//
// The Policy Engine integrates with the Executor via a pre-execution hook
// that validates every step before it runs.
package policy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Decision represents the outcome of a policy check.
type Decision string

const (
	DecisionAllowed  Decision = "allowed"
	DecisionDenied   Decision = "denied"
	DecisionApproval Decision = "approval_required"
)

// Action represents the type of operation being validated.
type Action string

const (
	ActionFileRead    Action = "filesystem:read"
	ActionFileWrite   Action = "filesystem:write"
	ActionFileDelete  Action = "filesystem:delete"
	ActionNetworkHTTP Action = "network:http"
	ActionShellExec   Action = "shell:exec"
	ActionFinancial   Action = "financial:transfer"
)

// Policy defines the security rules for workflow execution.
type Policy struct {
	Filesystem FilesystemPolicy `yaml:"filesystem"`
	Network    NetworkPolicy    `yaml:"network"`
	Shell      ShellPolicy      `yaml:"shell"`
	Financial  FinancialPolicy  `yaml:"financial"`
}

// FilesystemPolicy controls filesystem operations.
type FilesystemPolicy struct {
	Read    string `yaml:"read"`    // allowed | approval | denied
	Write   string `yaml:"write"`   // allowed | approval | denied
	Delete  string `yaml:"delete"`  // allowed | approval | denied
}

// NetworkPolicy controls network access.
type NetworkPolicy struct {
	Outbound   string   `yaml:"outbound"`   // allowed | allowlist | denied
	Allowlist  []string `yaml:"allowlist"`  // domains allowed when outbound=allowlist
	Denylist   []string `yaml:"denylist"`   // domains blocked even when outbound=allowed
}

// ShellPolicy controls shell execution.
type ShellPolicy struct {
	Enabled         bool     `yaml:"enabled"`          // false = completely disabled
	ApprovalRequired bool    `yaml:"approval_required"` // require human approval
	Allowlist       []string `yaml:"allowlist"`        // allowed commands (when enabled)
	Denylist        []string `yaml:"denylist"`         // blocked commands (even when enabled)
}

// FinancialPolicy controls financial operations.
type FinancialPolicy struct {
	Transfer          string `yaml:"transfer"`           // allowed | approval_required | denied
	MaxAmount         string `yaml:"max_amount"`         // max transfer amount (e.g. "1000 USD")
	ApprovalThreshold string `yaml:"approval_threshold"` // amount above which approval is required
}

// HumanApprovalFunc is called when a policy requires human approval.
// It should return true to approve the action, false to deny.
type HumanApprovalFunc func(ctx context.Context, action Action, details string) (bool, error)

// Engine is the Policy Engine that validates workflow steps against policies.
type Engine struct {
	policy   *Policy
	approval HumanApprovalFunc
}

// DefaultPolicy returns a sensible default policy for development use.
func DefaultPolicy() *Policy {
	return &Policy{
		Filesystem: FilesystemPolicy{
			Read:   "allowed",
			Write:  "allowed",
			Delete: "approval",
		},
		Network: NetworkPolicy{
			Outbound: "allowed",
		},
		Shell: ShellPolicy{
			Enabled:         true,
			ApprovalRequired: false,
		},
		Financial: FinancialPolicy{
			Transfer: "approval_required",
		},
	}
}

// StrictPolicy returns a production-safe policy with maximum restrictions.
func StrictPolicy() *Policy {
	return &Policy{
		Filesystem: FilesystemPolicy{
			Read:   "allowed",
			Write:  "approval",
			Delete: "denied",
		},
		Network: NetworkPolicy{
			Outbound: "allowlist",
			Allowlist: []string{
				"api.github.com",
				"api.openai.com",
				"api.anthropic.com",
			},
		},
		Shell: ShellPolicy{
			Enabled: false,
		},
		Financial: FinancialPolicy{
			Transfer:          "approval_required",
			ApprovalThreshold: "100 USD",
		},
	}
}

// NewEngine creates a Policy Engine with the given policy and approval function.
func NewEngine(p *Policy, approval HumanApprovalFunc) *Engine {
	if p == nil {
		p = DefaultPolicy()
	}
	return &Engine{
		policy:   p,
		approval: approval,
	}
}

// LoadPolicy loads a policy from a YAML file.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse policy YAML: %w", err)
	}
	return &p, nil
}

// Check runs a policy check for the given action and returns the decision.
func (e *Engine) Check(ctx context.Context, action Action, details string) (Decision, error) {
	allowed, needsApproval, err := e.evaluate(action, details)
	if err != nil {
		return DecisionDenied, err
	}
	if !allowed {
		return DecisionDenied, fmt.Errorf("policy denies %s: %s", action, details)
	}
	if needsApproval {
		if e.approval == nil {
			return DecisionDenied, fmt.Errorf("policy requires approval for %s but no approval function configured", action)
		}
		approved, err := e.approval(ctx, action, details)
		if err != nil {
			return DecisionDenied, fmt.Errorf("approval check failed: %w", err)
		}
		if !approved {
			return DecisionDenied, fmt.Errorf("human approval denied for %s: %s", action, details)
		}
		return DecisionApproval, nil
	}
	return DecisionAllowed, nil
}

// evaluate returns (allowed, needsApproval, error).
func (e *Engine) evaluate(action Action, details string) (bool, bool, error) {
	switch {
	case strings.HasPrefix(string(action), "filesystem:"):
		return e.evaluateFilesystem(action, details)
	case strings.HasPrefix(string(action), "network:"):
		return e.evaluateNetwork(action, details)
	case strings.HasPrefix(string(action), "shell:"):
		return e.evaluateShell(action, details)
	case strings.HasPrefix(string(action), "financial:"):
		return e.evaluateFinancial(action, details)
	default:
		return false, false, fmt.Errorf("unknown action: %s", action)
	}
}

func (e *Engine) evaluateFilesystem(action Action, _ string) (bool, bool, error) {
	var rule string
	switch action {
	case ActionFileRead:
		rule = e.policy.Filesystem.Read
	case ActionFileWrite:
		rule = e.policy.Filesystem.Write
	case ActionFileDelete:
		rule = e.policy.Filesystem.Delete
	default:
		return false, false, fmt.Errorf("unknown filesystem action: %s", action)
	}
	return e.parseRule(rule)
}

func (e *Engine) evaluateNetwork(_ Action, details string) (bool, bool, error) {
	rule := e.policy.Network.Outbound
	switch rule {
	case "denied":
		return false, false, nil
	case "allowed":
		// Check denylist
		for _, blocked := range e.policy.Network.Denylist {
			if matchDomain(details, blocked) {
				return false, false, fmt.Errorf("network access to %s is blocked by denylist", details)
			}
		}
		return true, false, nil
	case "allowlist":
		for _, allowed := range e.policy.Network.Allowlist {
			if matchDomain(details, allowed) {
				return true, false, nil
			}
		}
		return false, false, fmt.Errorf("network access to %s is not in allowlist", details)
	default:
		return false, false, fmt.Errorf("unknown network policy: %s", rule)
	}
}

func (e *Engine) evaluateShell(_ Action, details string) (bool, bool, error) {
	if !e.policy.Shell.Enabled {
		return false, false, fmt.Errorf("shell execution is disabled by policy")
	}

	cmd := strings.Fields(details)
	cmdName := ""
	if len(cmd) > 0 {
		cmdName = cmd[0]
	}

	// Check denylist first
	for _, blocked := range e.policy.Shell.Denylist {
		if cmdName == blocked || strings.HasPrefix(details, blocked) {
			return false, false, fmt.Errorf("shell command %q is blocked by denylist", cmdName)
		}
	}

	// If allowlist is configured, only allowlisted commands
	if len(e.policy.Shell.Allowlist) > 0 {
		allowed := false
		for _, a := range e.policy.Shell.Allowlist {
			if cmdName == a {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, false, fmt.Errorf("shell command %q is not in allowlist", cmdName)
		}
	}

	if e.policy.Shell.ApprovalRequired {
		return true, true, nil
	}

	return true, false, nil
}

func (e *Engine) evaluateFinancial(_ Action, _ string) (bool, bool, error) {
	rule := e.policy.Financial.Transfer
	return e.parseRule(strings.ReplaceAll(rule, "approval_required", "approval"))
}

func (e *Engine) parseRule(rule string) (bool, bool, error) {
	switch rule {
	case "allowed":
		return true, false, nil
	case "denied":
		return false, false, nil
	case "approval", "approval_required":
		return true, true, nil
	default:
		return false, false, fmt.Errorf("unknown policy rule: %s", rule)
	}
}

// matchDomain checks if host matches a pattern (supports * wildcard).
func matchDomain(host, pattern string) bool {
	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)

	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) || host == pattern[2:]
	}

	return host == pattern
}

// Policy returns the current policy (read-only, for inspection).
func (e *Engine) Policy() *Policy {
	return e.policy
}