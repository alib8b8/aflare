// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/agentx"
	"github.com/alib8b8/aflare/internal/config"
)

// The agentx registry is fed from the `agents:` config section. The
// loader is lazy: config is only touched when an agent is first
// resolved, so importing this package never triggers config file IO.
func init() {
	agentx.SetLoader(func() map[string]agentx.AgentDef {
		cfg, err := config.LoadConfig()
		if err != nil || cfg == nil {
			return nil
		}
		return cfg.Agents
	})
}

// resolveAgentDef builds the effective AgentDef for a node execution:
// registry lookup (by name) overlaid with per-step param overrides, or a
// fully inline definition when no agent name is given.
func resolveAgentDef(kind agentx.DriverKind, params map[string]string) (agentx.AgentDef, error) {
	name := strings.TrimSpace(getParam(params, "agent", ""))
	var def agentx.AgentDef
	if name != "" {
		registered, ok := agentx.Get(name)
		if !ok {
			return def, fmt.Errorf("agent %q is not registered (see `aflare agent list`)", name)
		}
		def = registered
	} else {
		def = agentx.AgentDef{Driver: kind, Name: "(inline)"}
	}

	if v := strings.TrimSpace(getParam(params, "binary", "")); v != "" {
		def.Binary = v
	}
	if v := strings.TrimSpace(getParam(params, "profile", "")); v != "" {
		def.Profile = v
	}
	if v := strings.TrimSpace(getParam(params, "url", "")); v != "" {
		def.URL = v
	}
	if v := strings.TrimSpace(getParam(params, "api_key_env", "")); v != "" {
		def.APIKeyEnv = v
	}
	if v := strings.TrimSpace(getParam(params, "model", "")); v != "" {
		def.Model = v
	}
	if v := strings.TrimSpace(getParam(params, "sandbox", "")); v != "" {
		def.Sandbox = v
	}
	if v := strings.TrimSpace(getParam(params, "approval_policy", "")); v != "" {
		def.Approval = v
	}
	if name != "" {
		def.Name = name
	}
	return def, nil
}

// buildAgentTask assembles the delegation Task from node params.
func buildAgentTask(input string, params map[string]string) agentx.Task {
	t := agentx.Task{Prompt: input}
	if v := strings.TrimSpace(getParam(params, "model", "")); v != "" {
		t.Model = v
	}
	if v := strings.TrimSpace(getParam(params, "sandbox", "")); v != "" {
		t.Sandbox = v
	}
	if v := strings.TrimSpace(getParam(params, "approval_policy", "")); v != "" {
		t.Approval = v
	}
	if v := strings.TrimSpace(getParam(params, "cwd", "")); v != "" {
		t.Cwd = v
	}
	if n := parseIntDefault(getParam(params, "max_turns", "0"), 0); n > 0 {
		t.MaxTurns = n
	}
	if d, err := time.ParseDuration(getParam(params, "timeout", "")); err == nil && d > 0 {
		t.Timeout = d
	}
	return t
}

func parseIntDefault(s string, fallback int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return fallback
	}
	return n
}

// ---------------------------------------------------------------------------
// cli_agent node
// ---------------------------------------------------------------------------

// CLIAgentNode delegates one bounded task to a local CLI agent (codex,
// claude, gemini or a generic command) — aflare commands, the agent
// executes.
type CLIAgentNode struct{}

func init() {
	Register(&CLIAgentNode{})
}

func (n *CLIAgentNode) Name() string { return "cli_agent" }

func (n *CLIAgentNode) Description() string {
	return "Delegate a step to a local CLI agent (codex / claude / gemini / generic) and supervise its execution"
}

func (n *CLIAgentNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "cli_agent",
		Description: "Runs one bounded task via an external CLI agent subprocess (codex exec, claude -p, gemini -p, or a generic command) with timeout, sandbox and audit. Requires the agent CLI installed and authenticated.",
		Input:       "string - the task/prompt for the agent",
		Output:      "string - the agent's final answer (stdout)",
		Params: []ParamSchema{
			{Name: "agent", Type: "string", Description: "Registered agent name (see `aflare agent list`); overrides: binary/profile/model/sandbox can still be set per step", Required: false},
			{Name: "binary", Type: "string", Description: "Agent executable (bare name or absolute path); required when no agent name is given", Required: false},
			{Name: "profile", Type: "string", Description: "CLI profile: codex, claude, gemini, generic (default: generic when inline)", Required: false},
			{Name: "model", Type: "string", Description: "Model the agent should use (forwarded as a validated flag value)", Required: false},
			{Name: "sandbox", Type: "string", Description: "codex: strict, permissive, danger-full-access (default strict)", Required: false},
			{Name: "approval_policy", Type: "string", Description: "codex: never, on-failure, on-request, untrusted (default never); claude/generic: only never is supported", Required: false},
			{Name: "max_turns", Type: "string", Description: "Maximum agent turns, 0 for unlimited (default 0)", Required: false},
			{Name: "cwd", Type: "string", Description: "Working directory for the agent (must exist)", Required: false},
			{Name: "timeout", Type: "string", Description: "Overall step timeout, e.g. 30s, 10m, 1h (default 10m, max 60m)", Required: false},
		},
		Notes: "Safe mode disables this node. All delegations are audit-logged (fail-closed). The agent subprocess inherits the environment (its own API keys), never aflare's secrets store.",
	}
}

func (n *CLIAgentNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("cli_agent node is disabled in safe mode")
	}
	def, err := resolveAgentDef(agentx.DriverCLI, params)
	if err != nil {
		return "", err
	}
	if def.Binary == "" {
		return "", fmt.Errorf("cli_agent requires an `agent` name or a `binary` param")
	}
	task := buildAgentTask(input, params)
	task.Audit = auditLog // fail-closed, mirroring codex_agent / execute
	return agentx.RunCLI(ctx, def, task)
}

// ---------------------------------------------------------------------------
// a2a_agent node
// ---------------------------------------------------------------------------

// A2AAgentNode delegates one task to a remote agent speaking the
// Agent2Agent protocol: task submission plus lifecycle polling.
type A2AAgentNode struct{}

func init() {
	Register(&A2AAgentNode{})
}

func (n *A2AAgentNode) Name() string { return "a2a_agent" }

func (n *A2AAgentNode) Description() string {
	return "Delegate a step to a remote A2A (Agent2Agent protocol) agent and monitor it to completion"
}

func (n *A2AAgentNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "a2a_agent",
		Description: "Sends the input as one task to an A2A agent (message/send with tasks/send fallback), polls tasks/get until a terminal state and returns the artifacts/status text.",
		Input:       "string - the task/prompt for the remote agent",
		Output:      "string - the agent's artifacts/status message text",
		Params: []ParamSchema{
			{Name: "agent", Type: "string", Description: "Registered A2A agent name (see `aflare agent list`)", Required: false},
			{Name: "url", Type: "string", Description: "A2A service endpoint (http/https); required when no agent name is given", Required: false},
			{Name: "api_key_env", Type: "string", Description: "Environment variable holding the bearer token for the remote agent", Required: false},
			{Name: "timeout", Type: "string", Description: "Overall step timeout, e.g. 30s, 10m, 1h (default 10m, max 60m)", Required: false},
		},
		Notes: "Safe mode disables this node. URL must be http/https; SSRF-protected dialing, 10MB response cap, audit-logged (fail-closed).",
	}
}

func (n *A2AAgentNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("a2a_agent node is disabled in safe mode")
	}
	def, err := resolveAgentDef(agentx.DriverA2A, params)
	if err != nil {
		return "", err
	}
	if def.URL == "" {
		return "", fmt.Errorf("a2a_agent requires an `agent` name or a `url` param")
	}
	task := buildAgentTask(input, params)
	task.Model = "" // A2A has no per-call model flag
	task.Audit = auditLog
	return agentx.SendMessage(ctx, def, task)
}
