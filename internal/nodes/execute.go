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
	"regexp"
	"runtime"
	"strings"
	"time"
)

// shellMetachars detects shell metacharacters that can be used for command
// injection. Note: space is intentionally NOT included here so that commands
// with arguments (e.g. "ls -la", "echo hello") still work under allowlist mode.
//
// Newlines (\n, \r) are included because in `sh -c` they act as command
// separators — without them an attacker could write `ls\ncurl evil` and the
// first-word allowlist check only inspects "ls", letting the second line
// execute an arbitrary non-whitelisted command.
var shellMetachars = regexp.MustCompile(`[;|&$`(){}<>\\*?!~='"\n\r]`)

var allowListEnabled = true // 默认开启白名单
var auditLogFile string

type ExecuteNode struct{}

func init() {
	Register(&ExecuteNode{})
	// 用户主动关闭安全模式时才关白名单
	if os.Getenv("AFLARE_EXECUTE_UNSAFE") == "1" {
		allowListEnabled = false
		fmt.Fprintln(os.Stderr, "AFLARE_EXECUTE_UNSAFE=1: command allowlist disabled, arbitrary commands can be executed")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		auditLogFile = filepath.Join(home, ".config", "aflare", "audit.log")
	}
}

func (n *ExecuteNode) Name() string {
	return "execute"
}

func (n *ExecuteNode) Description() string {
	return "Execute shell commands (disabled in safe mode)"
}

func (n *ExecuteNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "execute",
		Description: "Execute shell commands (disabled in safe mode)",
		Input:       "string - stdin for the command",
		Output:      "string - stdout and stderr of the command",
		Params: []ParamSchema{
			{Name: "command", Type: "string", Description: "Shell command to execute", Required: true},
			{Name: "dry_run", Type: "string", Description: "If true, preview the command without executing (default: false)", Required: false, Default: "false"},
			{Name: "timeout", Type: "string", Description: "Command timeout, e.g. 30s, 5m, 1h (default: 5m)", Required: false, Default: "5m"},
		},
		Notes: "WARNING: Setting AFLARE_EXECUTE_UNSAFE=1 removes all command execution security restrictions and may expose your system to risks.",
	}
}

func (n *ExecuteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("execute node is disabled in safe mode")
	}

	command, ok := params["command"]
	if !ok || command == "" {
		return "", fmt.Errorf("command parameter is required")
	}

	if len(command) > 4096 {
		return "", fmt.Errorf("command too long (max 4096 characters)")
	}

	dryRun := getParam(params, "dry_run", "false") == "true"
	timeoutStr := getParam(params, "timeout", "5m")

	if len(params) > 10 {
		return "", fmt.Errorf("too many parameters (max 10)")
	}
	for k, v := range params {
		if len(k) > 50 || len(v) > 1000 {
			return "", fmt.Errorf("parameter %s too long", k)
		}
	}

	// When allowlist is enabled, block shell metacharacters to prevent injection
	if allowListEnabled {
		if shellMetachars.MatchString(command) {
			return "", fmt.Errorf("shell metacharacters (;|&$`{} etc.) are not allowed when allowlist is enabled")
		}
		firstWord := strings.Fields(command)
		if len(firstWord) > 0 {
			cmdName := filepath.Base(firstWord[0])
			if !SafeCommandWhitelist[cmdName] {
				return "", fmt.Errorf("command %q is not in the allowed list (set AFLARE_EXECUTE_UNSAFE=1 to disable allowlist)", cmdName)
			}
			// sed and awk are read-only commands in the whitelist, but their
			// -i flag enables in-place file modification and -f loads a script
			// file that can contain arbitrary shell escapes (e.g. awk's
			// system("...")). Block both explicitly to prevent allowlist bypass.
			if cmdName == "sed" || cmdName == "awk" {
				for _, arg := range firstWord[1:] {
					if arg == "-i" || strings.HasPrefix(arg, "-i") {
						return "", fmt.Errorf("in-place edit (-i) is not allowed for %s under allowlist mode", cmdName)
					}
					if arg == "-f" || arg == "--file" || strings.HasPrefix(arg, "-f") {
						return "", fmt.Errorf("script file loading (-f) is not allowed for %s under allowlist mode (can execute arbitrary commands)", cmdName)
					}
				}
			}
		}
	}

	if err := auditLog(command); err != nil {
		// If audit logging fails, refuse to execute the command so that
		// commands cannot run without an audit trail (fail-closed).
		return "", fmt.Errorf("failed to write audit log: %w", err)
	}

	if dryRun {
		return fmt.Sprintf("[DRY RUN] Command that would execute:\n%s\n\nTo execute, set dry_run=false", command), nil
	}

	// Apply timeout
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 5 * time.Minute
	}
	if timeout > 30*time.Minute {
		timeout = 30 * time.Minute // cap at 30 minutes
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command) // #nosec G204 -- command is audited and timeout-capped
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command) // #nosec G204 -- command is audited and timeout-capped
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %s", timeoutStr)
		}
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// redactCommandForLog removes potential secrets from command before logging.
// It covers Bearer tokens, api_key/password/token/secret assignments,
// Authorization headers, and URL-embedded credentials.
func redactCommandForLog(command string) string {
	redacted := command
	redacted = regexp.MustCompile(`(?i)(bearer\s+)([^\s]+)`).ReplaceAllString(redacted, "${1}****")
	redacted = regexp.MustCompile(`(?i)(authorization[:\s]+)([^\s'"]+)`).ReplaceAllString(redacted, "${1}****")
	redacted = regexp.MustCompile(`(?i)(api[_-]?key=|password=|passwd=|token=|secret=)([^\s]+)`).ReplaceAllString(redacted, "${1}****")
	redacted = regexp.MustCompile(`(https?://[^/\s:@]+:)[^@\s/@]+(@)`).ReplaceAllString(redacted, "${1}****${2}")
	return redacted
}

// escapeLogContent prevents log injection by escaping control characters
func escapeLogContent(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\x00", "\\0")
	for i := 1; i < 32; i++ {
		s = strings.ReplaceAll(s, string(rune(i)), fmt.Sprintf("\\x%02x", i))
	}
	for i := 127; i < 160; i++ {
		s = strings.ReplaceAll(s, string(rune(i)), fmt.Sprintf("\\x%02x", i))
	}
	return s
}

func auditLog(command string) error {
	if auditLogFile == "" {
		return nil
	}
	dir := filepath.Dir(auditLogFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}
	// Use 0600 to prevent other users from reading potentially sensitive commands
	f, err := os.OpenFile(auditLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer f.Close()
	timestamp := time.Now().Format(time.RFC3339)
	redacted := redactCommandForLog(command)
	escaped := escapeLogContent(redacted)
	if _, err := fmt.Fprintf(f, "[%s] %s\n", timestamp, escaped); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}
	return nil
}
