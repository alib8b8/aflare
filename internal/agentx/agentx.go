// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌​​‌​‌‌​​​​​​​‌​​​‌‌​‌‌‌​​​​‌‌​‌​​‌​‌​​​​‌​‌‌​‌​‌​​​​​​​​​​​​​​​​​​‌‌​‌‌​​​‌‌​‌‌​⁠
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

// Package agentx implements external-agent orchestration: aflare acts as
// the supervisor that commands and monitors other agents instead of being
// packaged as a skill for them.
//
// Two interop channels share one delegation model:
//
//   - CLI channel: local agent binaries (codex, claude, gemini, custom)
//     spawned as hardened subprocesses.
//   - A2A channel: remote agents speaking the Agent2Agent protocol
//     (agent card discovery + JSON-RPC task lifecycle).
//
// aflare never exposes itself as the subordinate side: delegation is
// outbound only. Every execution goes through the audit hook so the
// existing HMAC audit chain records what was commanded.
package agentx

import (
	"time"
)

// DriverKind identifies which interop channel an agent definition uses.
type DriverKind string

const (
	// DriverCLI spawns a local agent binary as a subprocess.
	DriverCLI DriverKind = "cli"
	// DriverA2A talks to a remote agent over the Agent2Agent protocol.
	DriverA2A DriverKind = "a2a"
)

// Task is one delegated assignment handed to an external agent.
type Task struct {
	// Prompt is the instruction for the agent. Always forwarded as a
	// single argv element (CLI) or a single text part (A2A) — never
	// re-parsed as flags.
	Prompt string

	// Model optionally selects the model the agent should use.
	Model string

	// Sandbox is the CLI sandbox level (profile-specific semantics).
	Sandbox string

	// Approval is the CLI approval policy for non-interactive runs.
	Approval string

	// Cwd is the working directory for CLI agents (optional).
	Cwd string

	// MaxTurns bounds agent turns; 0 means the agent default.
	MaxTurns int

	// Timeout bounds the whole delegation. Values <= 0 fall back to
	// DefaultTimeout; values above MaxTimeout are clamped.
	Timeout time.Duration

	// Audit is a fail-closed hook invoked once before execution with a
	// human-readable description of the delegation. Returning an error
	// aborts the delegation.
	Audit func(entry string) error
}

// Timeout policy shared by both channels.
const (
	DefaultTimeout = 10 * time.Minute
	MaxTimeout     = 60 * time.Minute
)

// resolveTimeout applies the Task timeout policy.
func (t Task) resolveTimeout() time.Duration {
	if t.Timeout <= 0 {
		return DefaultTimeout
	}
	if t.Timeout > MaxTimeout {
		return MaxTimeout
	}
	return t.Timeout
}
