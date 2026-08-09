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

package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/tui"
	"github.com/alib8b8/aflare/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var sensitiveKeyPrefixes = []string{"api", "token", "bearer", "password", "passwd", "secret", "auth"}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range sensitiveKeyPrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, "_"+prefix) || strings.Contains(lower, "-"+prefix) {
			return true
		}
	}
	return false
}

func redactParams(params map[string]string) map[string]string {
	redacted := make(map[string]string)
	for k, v := range params {
		if isSensitiveKey(k) {
			redacted[k] = "***"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// HandleRun handles the "run" command.
func HandleRun(args []string, dryRun bool) {
	// Parse --resume flag. Two forms are supported:
	//   aflare run --resume my-workflow.yaml
	//     → boolean flag; checkpoint defaults to ~/.aflare/checkpoints/<name>.json
	//   aflare run --resume /path/to/state.json my-workflow.yaml
	//   aflare run --resume=/path/to/state.json my-workflow.yaml
	//     → explicit checkpoint path
	resumeEnabled := false
	resumePath := ""
	var filtered []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--resume" {
			resumeEnabled = true
			remaining := len(args) - i - 1
			if remaining >= 2 {
				resumePath = args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "--resume=") {
			resumeEnabled = true
			resumePath = strings.TrimPrefix(arg, "--resume=")
		} else {
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) < 1 {
		fmt.Println(i18n.T("run.usage"))
		os.Exit(1)
	}
	HandleRunFile(filtered[0], dryRun, resumeEnabled, resumePath)
}

// HandleRunFile runs a workflow file with optional resume support.
func HandleRunFile(wfPath string, dryRun bool, resumeEnabled bool, resumePath string) {
	wf, reg, err := PrepareWorkflow(wfPath)
	if err != nil {
		fmt.Printf("Error preparing workflow: %v\n", err)
		os.Exit(1)
	}

	if suggestions := workflow.ValidateWorkflow(wf); len(suggestions) > 0 {
		fmt.Println("⚠️ Workflow validation warnings:")
		for _, suggestion := range suggestions {
			fmt.Printf("  - %s\n", suggestion)
		}
	}

	if dryRun {
		fmt.Println("\n✅ Dry run completed - workflow is valid")
		return
	}

	// Compute the checkpoint state path.
	statePath := ""
	if resumeEnabled {
		if resumePath != "" {
			statePath = resumePath
		} else {
			name := wf.Name
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(wfPath), filepath.Ext(wfPath))
			}
			statePath = filepath.Join(meta.DataDir(), "checkpoints", name+".json")
		}
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		RunTUI(wfPath, wf, reg, statePath)
	} else {
		RunCLI(wf, reg, statePath)
	}
}

// resolveAuditDir returns the directory the audit log will land in for a given
// configured dir. When dir is non-empty it is used as-is; otherwise the history
// package's default audit directory is derived from GetAuditLogPath. Returns
// "" when no audit directory is available (e.g. HOME unset), in which case
// audit logging no-ops inside history.AppendAuditLog and no lock is needed.
func resolveAuditDir(dir string) string {
	if dir != "" {
		return dir
	}
	p := history.GetAuditLogPath()
	if p == "" {
		return ""
	}
	return filepath.Dir(p)
}

// acquireAuditLock takes an exclusive lock on the audit directory to prevent
// two aflare processes from concurrently appending to the same HMAC
// hash-chained audit log (H-6). The history package's auditWriteMu only
// serializes appends within a single process; without this lock, two
// `aflare run` invocations sharing one audit directory would interleave
// appends and fork the hash chain, making VerifyAuditChain fail and breaking
// tamper-evidence — the core guarantee for the financial audit scenario.
//
// The lock is a .audit.lock file created atomically with O_CREATE|O_EXCL. On
// success a release function is returned that closes and removes the lock;
// the caller MUST defer it. A stale lock left by a crashed process blocks
// subsequent runs — in that case the caller disables audit for the new
// process (with a warning) rather than corrupting the chain; the operator
// removes the stale lock manually. Pass dir="" to skip locking entirely
// (audit no-ops anyway when no directory is configured).
func acquireAuditLock(dir string) (func(), error) {
	if dir == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("audit: create dir %s: %w", dir, err)
	}
	lockPath := filepath.Join(dir, ".audit.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another aflare process is writing audit logs to %s; only one process may run with audit enabled at a time (set AFLARE_AUDIT_DIR to isolate, or remove a stale %s)", dir, lockPath)
		}
		return nil, err
	}
	fmt.Fprintf(f, "pid=%d started=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	return func() {
		_ = f.Close()
		_ = os.Remove(lockPath)
	}, nil
}

// newAuditEnabledExecutor builds an Executor with tamper-evident audit
// logging, first acquiring a process-exclusive lock on the audit directory
// (H-6). If the lock cannot be acquired (another process holds it, or a stale
// lock remains), audit is disabled for this process — with a warning — so the
// hash chain is never forked by concurrent writers. The returned release
// function must be deferred by the caller to release the lock on exit. When
// no audit directory is available, audit no-ops and the release is a no-op.
//
// auditDir is passed through to WithAuditLog unchanged ("" means "use the
// history default"); the lock is taken on the resolved directory so the
// default directory is also protected.
func newAuditEnabledExecutor(auditDir string) (*workflow.Executor, func()) {
	resolved := resolveAuditDir(auditDir)
	if resolved == "" {
		return workflow.NewExecutor().WithAuditLog(true, ""), func() {}
	}
	release, err := acquireAuditLock(resolved)
	if err != nil {
		logger.Warn("audit lock failed; disabling audit for this process to avoid cross-process hash-chain corruption",
			"dir", resolved, "error", err)
		return workflow.NewExecutor().WithAuditLog(false, ""), func() {}
	}
	return workflow.NewExecutor().WithAuditLog(true, auditDir), release
}

// RunTUI runs a workflow in interactive TUI mode.
func RunTUI(wfPath string, wf *workflow.Workflow, reg *nodes.Registry, statePath string) {
	model := tui.NewModel(wf.Name, wfPath, len(wf.Steps))
	program := tea.NewProgram(model, tea.WithAltScreen())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		exec, releaseAudit := newAuditEnabledExecutor("")
		defer releaseAudit()
		if statePath != "" {
			exec = exec.WithCheckpoint(statePath)
		}
		if _, _, _, err := exec.ExecuteWithTrace(ctx, wf, reg, program); err != nil {
			log.Printf("Workflow execution error: %v", err)
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// RunCLI runs a workflow in CLI (non-interactive) mode.
func RunCLI(wf *workflow.Workflow, reg *nodes.Registry, statePath string) {
	if wf.Name != "" {
		fmt.Printf("%s\n", i18n.T("workflow.name", wf.Name))
	}
	if wf.Description != "" {
		fmt.Printf("%s\n", i18n.T("workflow.description", wf.Description))
	}
	fmt.Printf("\n%s\n", i18n.T("workflow.steps", len(wf.Steps)))
	for i, step := range wf.Steps {
		fmt.Printf("  %d. Node: %s\n", i+1, step.Node)
		if len(step.Params) > 0 {
			fmt.Printf("     Params: %v\n", redactParams(step.Params))
		}
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.executing"))

	var finalOutput string
	var stepResults []workflow.StepResult
	var execErr error
	exec, releaseAudit := newAuditEnabledExecutor("")
	defer releaseAudit()
	if statePath != "" {
		exec = exec.WithCheckpoint(statePath)
	}
	finalOutput, stepResults, execErr = exec.Execute(context.Background(), wf, reg)

	for _, result := range stepResults {
		status := "✅"
		if result.Error != nil {
			status = "❌"
			fmt.Printf("\n%s Step %d (%s): %v\n", status, result.StepIndex+1, result.NodeName, result.Error)
		} else {
			fmt.Printf("%s Step %d (%s): %s\n", status, result.StepIndex+1, result.NodeName, i18n.T("step.duration", result.Duration))
		}
	}

	if execErr != nil {
		fmt.Printf("\n%s\n", i18n.T("workflow.failed", execErr))
		os.Exit(1)
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.final_output"))
	fmt.Println(finalOutput)
	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ "+i18n.T("workflow.completed")))
}
