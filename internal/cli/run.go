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
	"encoding/json"
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
	"github.com/alib8b8/aflare/internal/policy"
	"github.com/alib8b8/aflare/internal/tui"
	"github.com/alib8b8/aflare/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
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
//
// Supported flags (断点12):
//
//	--resume [path] <file>            enable checkpoint resume
//	--resume=/path <file>             explicit checkpoint path
//	--set key=value <file>            pass a single parameter (repeatable)
//	--set=key=value <file>            single-token form
//	--params-file <path> <file>      load parameters from JSON/YAML file
//	--params-file=<path> <file>      single-token form
//	--params k=v [k2=v2 ...] <file>  [deprecated] legacy params, use --set
//	--params="k=v k2=v2" <file>      [deprecated] single-token legacy form
//
// Merge priority (later overrides earlier): --params < --set < --params-file
func HandleRun(args []string, dryRun bool, safeMode bool) {
	resumeEnabled := false
	resumePath := ""
	var legacyParams []string // from --params (deprecated)
	var setParams []string    // from --set
	var paramsFile string     // from --params-file
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
		} else if arg == "--set" {
			// Consume following key=value tokens until the next flag or end.
			for j := i + 1; j < len(args); j++ {
				if strings.HasPrefix(args[j], "-") {
					break
				}
				setParams = append(setParams, args[j])
				i = j
			}
		} else if strings.HasPrefix(arg, "--set=") {
			setParams = append(setParams, strings.TrimPrefix(arg, "--set="))
		} else if arg == "--params-file" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				paramsFile = args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "--params-file=") {
			paramsFile = strings.TrimPrefix(arg, "--params-file=")
		} else if arg == "--params" {
			// Deprecated: warn but keep working.
			fmt.Fprintln(os.Stderr, "⚠️  --params 已弃用，请改用 --set key=value（可重复）或 --params-file")
			for j := i + 1; j < len(args); j++ {
				if strings.HasPrefix(args[j], "-") {
					break
				}
				legacyParams = append(legacyParams, args[j])
				i = j
			}
		} else if strings.HasPrefix(arg, "--params=") {
			fmt.Fprintln(os.Stderr, "⚠️  --params 已弃用，请改用 --set key=value（可重复）或 --params-file")
			legacyParams = append(legacyParams, strings.TrimPrefix(arg, "--params="))
		} else {
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) < 1 {
		fmt.Println(i18n.T("run.usage"))
		os.Exit(1)
	}

	// Merge params: --params (lowest) < --set < --params-file (highest).
	merged := make(map[string]string)
	for k, v := range parseParams(legacyParams) {
		merged[k] = v
	}
	for k, v := range parseParams(setParams) {
		merged[k] = v
	}
	if paramsFile != "" {
		fileParams, err := loadParamsFile(paramsFile)
		if err != nil {
			fmt.Printf("读取参数文件失败：%v\n", err)
			os.Exit(1)
		}
		for k, v := range fileParams {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		HandleRunFile(filtered[0], dryRun, resumeEnabled, resumePath, safeMode, nil)
	} else {
		HandleRunFile(filtered[0], dryRun, resumeEnabled, resumePath, safeMode, merged)
	}
}

// printInputSchemaHelp prints a human-readable description of a workflow's
// input_schema along with a copy-pasteable example command pre-filled with
// defaults/placeholders. Used when the user runs a parameterized template
// without --params (断点8).
func printInputSchemaHelp(wfPath string, wf *workflow.Workflow) {
	fmt.Println("此模板需要以下参数：")
	fmt.Println()
	for _, field := range wf.InputSchema {
		req := "必填"
		if !field.Required {
			req = "选填"
		}
		typeStr := field.Type
		if typeStr == "" {
			typeStr = "string"
		}
		def := ""
		if field.Default != "" {
			def = fmt.Sprintf("（默认: %s）", field.Default)
		}
		fmt.Printf("  %-12s (%s) %s %s\n", field.Name, typeStr, req, def)
	}
	fmt.Println()
	fmt.Println("示例：")
	// Build an example command using --set (断点12推荐方式).
	for _, field := range wf.InputSchema {
		val := field.Default
		if val == "" {
			val = "your_" + field.Name
		}
		fmt.Printf("  aflare run %s --set %s=%s\n", wfPath, field.Name, val)
	}
	fmt.Println()
	fmt.Println("提示：使用 --set key=value 传参（可重复），或 --params-file params.json 从文件读取。")
}

// printExtractedParamsHelp prints a parameter hint for templates that lack an
// explicit input_schema but reference {{var.xxx}} / {{ .params.xxx }} in their
// YAML (断点E). All extracted names are treated as required strings since the
// generator cannot infer type/default/required from a bare template reference.
func printExtractedParamsHelp(wfPath string, refs []string) {
	fmt.Println("此模板引用了以下参数（未声明 input_schema，已从 YAML 自动提取）：")
	fmt.Println()
	for _, name := range refs {
		fmt.Printf("  %-16s (string) 必填\n", name)
	}
	fmt.Println()
	fmt.Println("示例：")
	for _, name := range refs {
		fmt.Printf("  aflare run %s --set %s=your_%s\n", wfPath, name, name)
	}
	fmt.Println()
	fmt.Println("提示：使用 --set key=value 传参（可重复），或 --params-file params.json 从文件读取。")
}

// parseParams converts a list of "key=value" tokens into a map.
// Tokens without "=" are ignored.
func parseParams(tokens []string) map[string]string {
	if len(tokens) == 0 {
		return nil
	}
	m := make(map[string]string)
	for _, t := range tokens {
		// A single --params token may contain space-separated pairs.
		for _, pair := range strings.Fields(t) {
			if idx := strings.IndexByte(pair, '='); idx > 0 {
				m[pair[:idx]] = pair[idx+1:]
			}
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// loadParamsFile reads workflow parameters from a JSON or YAML file (断点12).
//
// Supported formats:
//   - JSON:  {"key": "value", ...}  (values are stringified if non-string)
//   - YAML:  key: value             (flat map only)
//
// The file extension determines the parser: .json → JSON, .yaml/.yml → YAML.
// When the extension is ambiguous, the content is tried as JSON first, then YAML.
func loadParamsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取参数文件 %s: %w", path, err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, fmt.Errorf("参数文件 %s 为空", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var raw map[string]interface{}

	switch ext {
	case ".json":
		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			return nil, fmt.Errorf("解析 JSON 失败: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
			return nil, fmt.Errorf("解析 YAML 失败: %w", err)
		}
	default:
		// Auto-detect: try JSON first, then YAML.
		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
				return nil, fmt.Errorf("无法解析参数文件（尝试 JSON 和 YAML 均失败）: %w", err)
			}
		}
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

// HandleRunFile runs a workflow file with optional resume support.
// params (from --params) are injected into wf.Vars; when the workflow declares
// an input_schema but no params are supplied, the schema is printed as guidance
// and the process exits (断点8: 模板参数不透明).
func HandleRunFile(wfPath string, dryRun bool, resumeEnabled bool, resumePath string, safeMode bool, params map[string]string) {
	wf, reg, err := PrepareWorkflow(wfPath)
	if err != nil {
		fmt.Printf("Error preparing workflow: %v\n", err)
		os.Exit(1)
	}

	// 断点8: 当工作流声明了 input_schema 但未通过 --params 传参时，打印参数
	// 说明并给出可复制的示例命令，而不是让用户对着空参数报错发呆。
	if len(wf.InputSchema) > 0 && len(params) == 0 {
		printInputSchemaHelp(wfPath, wf)
		os.Exit(1)
	}

	// 断点E: 对没有 input_schema 但 YAML 里引用了 {{var.xxx}} / {{ .params.xxx }}
	// 的模板，自动提取参数列表并提示，避免拿到空值后在 http_request / execute
	// 等节点报晦涩的 "variable not found" 错误。
	if len(wf.InputSchema) == 0 && len(params) == 0 {
		if refs := workflow.ExtractReferencedVars(wf); len(refs) > 0 {
			printExtractedParamsHelp(wfPath, refs)
			os.Exit(1)
		}
	}

	// 注入 --params 到 wf.Vars，使其可通过 {{var.name}} 在工作流中引用。
	if len(params) > 0 {
		if wf.Vars == nil {
			wf.Vars = make(map[string]string)
		}
		for k, v := range params {
			wf.Vars[k] = v
		}
	}

	// 断点A: 若工作流需要 LLM 但未配置任何 LLM provider，提前提示并退出，
	// 而不是跑到 LLM 节点才报错。仅当完全没有配置 provider 时触发
	// （ollama 已配置但未运行的情况由运行时的 humanizeError 处理）。
	if !detectLLMConfig() && workflow.RequiresLLM(wf) {
		fmt.Println("此工作流需要 LLM，但尚未配置 LLM provider。")
		fmt.Println("运行 aflare init 完成首次配置（推荐 Ollama 本地，或配置 DeepSeek/OpenAI API Key）。")
		fmt.Printf("配置后重试：aflare run %s\n", wfPath)
		os.Exit(1)
	}

	if suggestions := workflow.ValidateWorkflow(wf); len(suggestions) > 0 {
		fmt.Println("⚠️ Workflow validation warnings:")
		for _, suggestion := range suggestions {
			fmt.Printf("  - %s\n", suggestion)
		}
	}

	// 遗留修复: 工作流携带 schedule 配置时，引擎本身不会自动定时执行（调度由
	// `aflare schedule add` 外部管理）。这里打印一行激活提示，避免用户以为
	// 写了 schedule: cron 就会自动周期运行。
	if wf.Schedule != nil && wf.Schedule.Cron != "" {
		fmt.Printf("⏰ 检测到调度配置 (cron: %s)。引擎不会自动定时执行，激活请运行：\n", wf.Schedule.Cron)
		fmt.Printf("   aflare schedule add --cron \"%s\" %s\n", wf.Schedule.Cron, wfPath)
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
		RunTUI(wfPath, wf, reg, statePath, safeMode)
	} else {
		RunCLI(wf, reg, statePath, safeMode)
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
		if err := f.Close(); err != nil {
			logger.Error("audit lock file close failed", "err", err)
		}
		_ = os.Remove(lockPath) // best-effort cleanup
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
func newAuditEnabledExecutor(auditDir string, safeMode bool) (*workflow.PolicyExecutor, func()) {
	var exec *workflow.Executor
	resolved := resolveAuditDir(auditDir)
	if resolved == "" {
		exec = workflow.NewExecutor().WithAuditLog(true, "")
	} else {
		release, err := acquireAuditLock(resolved)
		if err != nil {
			logger.Warn("audit lock failed; disabling audit for this process to avoid cross-process hash-chain corruption",
				"dir", resolved, "error", err)
			exec = workflow.NewExecutor().WithAuditLog(false, "")
		} else {
			exec = workflow.NewExecutor().WithAuditLog(true, auditDir)
			policyEngine := newPolicyEngine(safeMode)
			return workflow.NewPolicyExecutor(exec, policyEngine), release
		}
	}
	policyEngine := newPolicyEngine(safeMode)
	return workflow.NewPolicyExecutor(exec, policyEngine), func() {}
}

// newPolicyEngine returns a policy engine based on the safeMode flag.
// In safe mode, StrictPolicy is used (shell disabled, network allowlist, delete denied).
// In normal mode, DefaultPolicy is used (permissive for development).
func newPolicyEngine(safeMode bool) *policy.Engine {
	if safeMode {
		return policy.NewEngine(policy.StrictPolicy(), nil)
	}
	return policy.NewEngine(policy.DefaultPolicy(), nil)
}

// RunTUI runs a workflow in interactive TUI mode.
func RunTUI(wfPath string, wf *workflow.Workflow, reg *nodes.Registry, statePath string, safeMode bool) {
	model := tui.NewModel(wf.Name, wfPath, len(wf.Steps))
	program := tea.NewProgram(model, tea.WithAltScreen())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		exec, releaseAudit := newAuditEnabledExecutor("", safeMode)
		defer releaseAudit()
		if statePath != "" {
			exec = exec.WithCheckpoint(statePath)
		}
		if err := exec.ValidateWorkflow(ctx, wf); err != nil {
			log.Printf("Policy validation failed: %v", err)
			return
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

// RunCLI runs a workflow in CLI (non-interactive) mode with real-time progress
// output (断点13). Each step prints a progress line as it starts/completes/fails:
//
//	[1/5] ✓ http_request  → CoinGecko API          (0.3s)
//	[2/5] ✗ agent          → FAILED
//	      错误：LLM 调用超时（30s）
//	      排查建议：
//	        1. 检查 Ollama 是否运行：ollama list
//	        ...
//
// Errors are translated to user-friendly messages via humanizeError (断点11),
// while the raw error is logged at debug level for troubleshooting.
func RunCLI(wf *workflow.Workflow, reg *nodes.Registry, statePath string, safeMode bool) {
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

	// 断点13: 实时进度回调。在 step 开始/完成/失败/跳过时立即打印进度行。
	progressCB := func(ev workflow.StepProgressEvent) {
		label := ev.StepName
		if label == "" {
			label = ev.NodeName
		}
		switch ev.Status {
		case workflow.StepProgressStarted:
			fmt.Printf("[%d/%d] ⏳ %-14s → %s\n", ev.Index+1, ev.Total, ev.NodeName, label)
		case workflow.StepProgressCompleted:
			fmt.Printf("[%d/%d] ✓ %-14s → %-20s (%s)\n", ev.Index+1, ev.Total, ev.NodeName, label, formatDuration(ev.Duration))
		case workflow.StepProgressFailed:
			human, debug := humanizeError(ev.Error, ev.NodeName)
			fmt.Printf("[%d/%d] ✗ %-14s → %s FAILED\n", ev.Index+1, ev.Total, ev.NodeName, label)
			fmt.Printf("      错误：%s\n", human)
			// 底层错误保留在日志中供 debug (断点11).
			logger.Error("step failed", "index", ev.Index, "node", ev.NodeName, "raw_error", debug, "duration", ev.Duration)
			if hint := troubleshootHint(ev.NodeName, ev.Error); hint != "" {
				fmt.Printf("      %s\n", hint)
			}
		case workflow.StepProgressSkipped:
			fmt.Printf("[%d/%d] ⊘ %-14s → skipped (condition not met)\n", ev.Index+1, ev.Total, ev.NodeName)
		}
	}

	var finalOutput string
	var execErr error
	exec, releaseAudit := newAuditEnabledExecutor("", safeMode)
	defer releaseAudit()
	if statePath != "" {
		exec = exec.WithCheckpoint(statePath)
	}
	exec = exec.WithProgress(progressCB)
	if err := exec.ValidateWorkflow(context.Background(), wf); err != nil {
		fmt.Printf("Policy validation failed: %v\n", err)
		os.Exit(1)
	}
	finalOutput, _, execErr = exec.Execute(context.Background(), wf, reg)

	if execErr != nil {
		// 工作流级错误也走翻译层 (断点11).
		human, debug := humanizeError(execErr, "")
		fmt.Printf("\n%s\n", i18n.T("workflow.failed", human))
		logger.Error("workflow failed", "raw_error", debug)
		os.Exit(1)
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.final_output"))
	fmt.Println(finalOutput)
	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ "+i18n.T("workflow.completed")))
}
