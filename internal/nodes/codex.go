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

package nodes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodexAgentNode delegates a workflow step to the OpenAI Codex CLI
// (https://github.com/openai/codex) via its non-interactive `codex exec`
// mode. This turns aflare into an orchestration layer around the Codex
// agent harness: aflare owns scheduling, retries, fan-out, approvals and
// notifications while Codex owns the agent loop (context gathering, tool
// use, sandboxed execution) for coding-adjacent steps.
//
// The prompt is passed as a single argv element (never through a shell),
// so prompt content cannot inject shell metacharacters into the command
// line. All flags are constructed from validated enum parameters — there
// is deliberately no free-form "extra args" parameter.
type CodexAgentNode struct{}

func init() {
	Register(&CodexAgentNode{})
}

func (n *CodexAgentNode) Name() string {
	return "codex_agent"
}

func (n *CodexAgentNode) Description() string {
	return "Delegate a step to the OpenAI Codex CLI agent (codex exec, non-interactive)"
}

func (n *CodexAgentNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "codex_agent",
		Description: "Runs one bounded Codex agent task via `codex exec` and returns its final output. Requires the codex CLI (https://github.com/openai/codex) installed and authenticated.",
		Input:       "string - the task/prompt for the Codex agent",
		Output:      "string - the Codex agent's final answer (stdout)",
		Params: []ParamSchema{
			{Name: "binary", Type: "string", Description: "Path to the codex executable (default: codex, resolved from PATH)", Required: false, Default: "codex"},
			{Name: "model", Type: "string", Description: "Model to use (passed as --model, e.g. gpt-5.6; empty = codex default)", Required: false},
			{Name: "sandbox", Type: "string", Description: "Codex sandbox level: strict, permissive, danger-full-access (default: strict)", Required: false, Default: "strict"},
			{Name: "approval_policy", Type: "string", Description: "Approval policy for non-interactive runs: never, on-failure, on-request, untrusted (default: never)", Required: false, Default: "never"},
			{Name: "max_turns", Type: "string", Description: "Maximum agent turns, 0 for unlimited (default: 0)", Required: false, Default: "0"},
			{Name: "cwd", Type: "string", Description: "Working directory for the agent (must exist; default: current directory)", Required: false},
			{Name: "timeout", Type: "string", Description: "Overall step timeout, e.g. 30s, 10m, 1h (default: 10m, max 60m)", Required: false, Default: "10m"},
		},
		Notes: "Safe mode disables this node. Requires codex CLI >= the version shipping `codex exec`. Progress/diagnostics go to stderr and are only surfaced on failure.",
	}
}

// validCodexSandboxes are the codex exec --sandbox values we forward.
var validCodexSandboxes = map[string]bool{
	"strict":             true,
	"permissive":         true,
	"danger-full-access": true,
}

// validCodexApprovalPolicies are the codex exec --approval-policy values.
var validCodexApprovalPolicies = map[string]bool{
	"never":      true,
	"on-failure": true,
	"on-request": true,
	"untrusted":  true,
}

func (n *CodexAgentNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("codex_agent node is disabled in safe mode")
	}

	prompt := strings.TrimSpace(input)
	if prompt == "" {
		return "", fmt.Errorf("codex_agent requires a non-empty prompt as step input")
	}
	if len(prompt) > 100_000 {
		return "", fmt.Errorf("prompt too long (%d chars, max 100000)", len(prompt))
	}

	binary := getParam(params, "binary", "codex")
	if binary == "" {
		binary = "codex"
	}
	model := strings.TrimSpace(getParam(params, "model", ""))
	sandbox := getParam(params, "sandbox", "strict")
	approvalPolicy := getParam(params, "approval_policy", "never")
	maxTurnsStr := getParam(params, "max_turns", "0")
	cwd := getParam(params, "cwd", "")
	timeoutStr := getParam(params, "timeout", "10m")

	if !validCodexSandboxes[sandbox] {
		return "", fmt.Errorf("invalid sandbox %q (valid: strict, permissive, danger-full-access)", sandbox)
	}
	if !validCodexApprovalPolicies[approvalPolicy] {
		return "", fmt.Errorf("invalid approval_policy %q (valid: never, on-failure, on-request, untrusted)", approvalPolicy)
	}

	maxTurns := 0
	if _, err := fmt.Sscanf(maxTurnsStr, "%d", &maxTurns); err != nil {
		maxTurns = 0
	}
	if maxTurns < 0 || maxTurns > 1000 {
		return "", fmt.Errorf("max_turns out of range (0-1000): %d", maxTurns)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if timeout > 60*time.Minute {
		timeout = 60 * time.Minute
	}

	if cwd != "" {
		info, err := os.Stat(cwd)
		if err != nil {
			return "", fmt.Errorf("cwd %q is not accessible: %w", cwd, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("cwd %q is not a directory", cwd)
		}
	}

	// Fail-closed audit trail, mirroring the execute node.
	if err := auditLog(fmt.Sprintf("codex_agent sandbox=%s approval=%s model=%s cwd=%s", sandbox, approvalPolicy, model, cwd)); err != nil {
		return "", fmt.Errorf("failed to write audit log: %w", err)
	}

	// Build the fixed argv. The prompt is one argv element — no shell is
	// involved, so prompt content cannot alter the command line.
	args := []string{"exec", "--sandbox", sandbox, "--approval-policy", approvalPolicy}
	if model != "" {
		args = append(args, "--model", model)
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, prompt)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Collect stdout/stderr via temp files rather than pipes: with a pipe,
	// Wait() blocks until every process holding the write end exits — an
	// orphaned grandchild (e.g. a tool spawned by the agent) would keep the
	// run hanging well past the deadline kill. With files there are no copy
	// goroutines, so Wait() returns as soon as the direct child is gone.
	outFile, err := os.CreateTemp("", "aflare-codex-out-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp output file: %w", err)
	}
	defer func() {
		_ = outFile.Close()           // read-only post-Run; close error is irrelevant
		_ = os.Remove(outFile.Name()) // best-effort temp cleanup
	}()
	errFile, err := os.CreateTemp("", "aflare-codex-err-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp error file: %w", err)
	}
	defer func() {
		_ = errFile.Close()           // read-only post-Run; close error is irrelevant
		_ = os.Remove(errFile.Name()) // best-effort temp cleanup
	}()

	cmd := exec.CommandContext(ctx, binary, args...) // #nosec G204 -- fixed flag set from validated enums; prompt is a single argv element
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	// Inherit the environment: codex needs OPENAI_API_KEY / CODEX_HOME etc.
	cmd.Env = os.Environ()

	runErr := cmd.Run()
	stdoutData, _ := os.ReadFile(outFile.Name())
	stderrData, _ := os.ReadFile(errFile.Name())

	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("codex exec timed out after %s", timeoutStr)
		}
		// Surface stderr (codex diagnostics) to make failures actionable.
		return "", fmt.Errorf("codex exec failed: %w\nstderr: %s", runErr, strings.TrimSpace(string(stderrData)))
	}

	out := strings.TrimSpace(string(stdoutData))
	if out == "" {
		// Some codex versions emit the final message on stderr; fall back
		// so an empty stdout doesn't turn a successful run into silence.
		if fb := strings.TrimSpace(string(stderrData)); fb != "" {
			return fb, nil
		}
		return "", fmt.Errorf("codex exec produced no output")
	}
	return out, nil
}

// CodexBinaryPath resolves the codex binary that the node would invoke,
// primarily so `aflare doctor`-style checks can verify the installation.
// It returns the param override as-is when given; otherwise it resolves
// "codex" from PATH.
func CodexBinaryPath(paramOverride string) string {
	if paramOverride != "" {
		return paramOverride
	}
	if p, err := exec.LookPath("codex"); err == nil {
		return p
	}
	return filepath.Join("codex")
}
