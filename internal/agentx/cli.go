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

package agentx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// validProfiles lists the built-in CLI profiles. The generic profile
// forwards AgentDef.Args as literal argv elements and appends the prompt
// as the final argument.
var validProfiles = map[string]bool{
	"codex":   true,
	"claude":  true,
	"gemini":  true,
	"generic": true,
}

// maxPromptChars bounds the delegated prompt: it is forwarded as one argv
// element / one text part, so a runaway upstream step cannot OOM us.
const maxPromptChars = 100_000

// maxOutputBytes bounds how much subprocess output is read back.
const maxOutputBytes = 10 * 1024 * 1024

// RunCLI executes one delegation to a local CLI agent as a hardened
// subprocess. It generalizes the codex_agent execution model:
//
//   - binary resolved via LookPath before any side effect
//   - prompt forwarded as a single argv element after "--" (never
//     re-parsed as flags)
//   - stdout/stderr captured via temp files so orphaned grandchildren
//     cannot hang Wait() past the deadline
//   - fail-closed audit hook
//   - context deadline enforced by CommandContext
func RunCLI(ctx context.Context, def AgentDef, t Task) (string, error) {
	def, err := def.Resolve()
	if err != nil {
		return "", err
	}

	prompt := strings.TrimSpace(t.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("agent %q requires a non-empty prompt", def.Name)
	}
	if len(prompt) > maxPromptChars {
		return "", fmt.Errorf("prompt too long (%d chars, max %d)", len(prompt), maxPromptChars)
	}

	model := orDefault(t.Model, def.Model)
	sandbox := orDefault(t.Sandbox, def.Sandbox)
	approval := orDefault(t.Approval, def.Approval)

	if model != "" && !isValidModelName(model) {
		return "", fmt.Errorf("agent %q: invalid model %q: allowed characters are letters, digits, '.', '-', '_' and '/'", def.Name, model)
	}
	if strings.ContainsAny(def.Binary, `/\`) && !filepath.IsAbs(def.Binary) {
		return "", fmt.Errorf("agent %q: binary must be a bare command name or an absolute path: %q", def.Name, def.Binary)
	}

	cwd := strings.TrimSpace(t.Cwd)
	if cwd != "" {
		resolved, err := canonicalWorkdir(cwd)
		if err != nil {
			return "", fmt.Errorf("agent %q: %w", def.Name, err)
		}
		cwd = resolved
	}

	// Validate the delegation argv before touching the environment:
	// config errors (bad sandbox, unknown profile) must not depend on
	// whether the binary happens to be installed.
	args, err := buildProfileArgs(def, prompt, model, sandbox, approval, t.MaxTurns, cwd)
	if err != nil {
		return "", err
	}

	resolvedBinary, err := exec.LookPath(def.Binary)
	if err != nil {
		return "", fmt.Errorf("agent %q: binary %q not found: %w", def.Name, def.Binary, err)
	}

	timeout := t.resolveTimeout()
	timeoutStr := formatDuration(timeout)

	if t.Audit != nil {
		entry := fmt.Sprintf("agentx cli agent=%s binary=%s profile=%s sandbox=%s approval=%s model=%s cwd=%s timeout=%s",
			def.Name, resolvedBinary, def.Profile, sandbox, approval, model, cwd, timeoutStr)
		if err := t.Audit(entry); err != nil {
			return "", fmt.Errorf("agent %q: failed to write audit log: %w", def.Name, err)
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outFile, err := os.CreateTemp("", "aflare-agentx-out-*")
	if err != nil {
		return "", fmt.Errorf("agent %q: failed to create temp output file: %w", def.Name, err)
	}
	defer func() {
		_ = outFile.Close()
		_ = os.Remove(outFile.Name())
	}()
	errFile, err := os.CreateTemp("", "aflare-agentx-err-*")
	if err != nil {
		return "", fmt.Errorf("agent %q: failed to create temp error file: %w", def.Name, err)
	}
	defer func() {
		_ = errFile.Close()
		_ = os.Remove(errFile.Name())
	}()

	cmd := exec.CommandContext(runCtx, resolvedBinary, args...) // #nosec G204 -- argv built from validated enums + single prompt element behind "--"
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	// Inherit the environment: the agent CLI needs its own credentials
	// (OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY, ...). aflare
	// never forwards its own secrets store contents into the subprocess.
	cmd.Env = os.Environ()

	runErr := cmd.Run()
	stdoutData, _ := os.ReadFile(outFile.Name())
	stderrData, _ := os.ReadFile(errFile.Name())
	if len(stdoutData) > maxOutputBytes {
		stdoutData = stdoutData[:maxOutputBytes]
	}
	if len(stderrData) > maxOutputBytes {
		stderrData = stderrData[:maxOutputBytes]
	}

	if runErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("agent %q timed out after %s", def.Name, timeoutStr)
		}
		return "", fmt.Errorf("agent %q failed: %w\nstderr: %s", def.Name, runErr, strings.TrimSpace(string(stderrData)))
	}

	out := strings.TrimSpace(string(stdoutData))
	if out == "" {
		// Some CLIs emit the final message on stderr; fall back so a
		// successful run doesn't turn into silence.
		if fb := strings.TrimSpace(string(stderrData)); fb != "" {
			return fb, nil
		}
		return "", fmt.Errorf("agent %q produced no output", def.Name)
	}
	return out, nil
}

// buildProfileArgs maps a delegation onto the CLI's non-interactive argv.
// Every profile appends the prompt as one argv element; profiles whose
// CLIs support flag injection put "--" before the prompt so a prompt
// starting with "-" cannot be re-parsed as a flag.
func buildProfileArgs(def AgentDef, prompt, model, sandbox, approval string, maxTurns int, cwd string) ([]string, error) {
	var args []string
	switch def.Profile {
	case "codex":
		sandbox = orDefault(sandbox, "strict")
		approval = orDefault(approval, "never")
		if !validCodexSandboxes[sandbox] {
			return nil, fmt.Errorf("agent %q: invalid codex sandbox %q (valid: strict, permissive, danger-full-access)", def.Name, sandbox)
		}
		if !validCodexApprovals[approval] {
			return nil, fmt.Errorf("agent %q: invalid codex approval_policy %q (valid: never, on-failure, on-request, untrusted)", def.Name, approval)
		}
		args = []string{"exec", "--sandbox", sandbox, "--approval-policy", approval}
		if model != "" {
			args = append(args, "--model", model)
		}
		if maxTurns > 0 {
			args = append(args, "--max-turns", fmt.Sprintf("%d", clampTurns(maxTurns)))
		}
		if cwd != "" {
			args = append(args, "--cwd", cwd)
		}
		args = append(args, "--", prompt)
	case "claude":
		approval = orDefault(approval, "never")
		if approval != "never" {
			return nil, fmt.Errorf("agent %q: claude profile only supports approval_policy=never in non-interactive runs", def.Name)
		}
		args = []string{"-p", prompt, "--output-format", "text"}
		if model != "" {
			args = append(args, "--model", model)
		}
		if maxTurns > 0 {
			args = append(args, "--max-turns", fmt.Sprintf("%d", clampTurns(maxTurns)))
		}
	case "gemini":
		args = []string{"-p", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
	case "generic":
		args = append(args, def.Args...)
		args = append(args, "--", prompt)
	default:
		return nil, fmt.Errorf("agent %q: unknown cli profile %q", def.Name, def.Profile)
	}
	return args, nil
}

var validCodexSandboxes = map[string]bool{
	"strict":             true,
	"permissive":         true,
	"danger-full-access": true,
}

var validCodexApprovals = map[string]bool{
	"never":      true,
	"on-failure": true,
	"on-request": true,
	"untrusted":  true,
}

func clampTurns(maxTurns int) int {
	if maxTurns < 0 {
		return 0
	}
	if maxTurns > 1000 {
		return 1000
	}
	return maxTurns
}

// isValidModelName accepts only the identifier charset real model names
// use (gpt-5.6, openai/o4-mini, claude-sonnet-4, ...): this blocks
// argv-level tricks such as a model value that starts with "--" being
// re-parsed as another flag by the CLI.
func isValidModelName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	if s[0] == '-' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '/':
		default:
			return false
		}
	}
	return true
}

// canonicalWorkdir validates a user-supplied working directory: it makes
// the path absolute, lexical-clean (folding any ".." traversal segments
// under the root), resolves symlinks and requires it to be an existing
// directory, so the value forwarded to the subprocess is fully
// normalized before spawn.
func canonicalWorkdir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid cwd %q: %w", dir, err)
	}
	clean := filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("cwd %q does not resolve: %w", dir, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("cwd %q not accessible: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cwd %q is not a directory", dir)
	}
	return resolved, nil
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func formatDuration(d time.Duration) string { return d.String() }
