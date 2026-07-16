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

var allowedCommands = map[string]bool{
	"echo": true, "cat": true, "ls": true, "pwd": true,
	"grep": true, "sed": true, "awk": true, "sort": true,
	"wc": true, "head": true, "tail": true, "cut": true,
	"tr": true, "uniq": true, "find": true, "date": true,
	"curl": true, "wget": true, "jq": true, "python3": true,
	"python": true, "node": true, "git": true, "go": true,
	"npm": true, "npx": true, "yarn": true, "pnpm": true,
	"docker": true, "kubectl": true, "kubectx": true,
}

// shellMetachars detects shell metacharacters that can be used for command
// injection. Note: space is intentionally NOT included here so that commands
// with arguments (e.g. "ls -la", "echo hello") still work under allowlist mode.
var shellMetachars = regexp.MustCompile("[;|&$`(){}<>\\\\*?!~='\"]")

var allowListEnabled = false
var auditLogFile string

type ExecuteNode struct{}

func init() {
	Register(&ExecuteNode{})
	if os.Getenv("LLM_BOX_EXECUTE_ALLOWLIST") == "1" {
		allowListEnabled = true
	}
	home, err := os.UserHomeDir()
	if err == nil {
		auditLogFile = filepath.Join(home, ".config", "llm-box", "audit.log")
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

	dryRun := getParam(params, "dry_run", "false") == "true"
	timeoutStr := getParam(params, "timeout", "5m")

	// When allowlist is enabled, block shell metacharacters to prevent injection
	if allowListEnabled {
		if shellMetachars.MatchString(command) {
			return "", fmt.Errorf("shell metacharacters (;|&$`{} etc.) are not allowed when allowlist is enabled")
		}
		firstWord := strings.Fields(command)
		if len(firstWord) > 0 {
			cmdName := filepath.Base(firstWord[0])
			if !allowedCommands[cmdName] {
				return "", fmt.Errorf("command %q is not in the allowed list (enable via LLM_BOX_EXECUTE_ALLOWLIST=1)", cmdName)
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
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
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
