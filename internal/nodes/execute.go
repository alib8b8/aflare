package nodes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// shellMetachars detects shell metacharacters that can be used for command injection
var shellMetachars = regexp.MustCompile("[;|&$`(){ }]")

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

	auditLog(command)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// redactCommandForLog removes potential secrets from command before logging
func redactCommandForLog(command string) string {
	// Redact common patterns: Bearer tokens, API keys, passwords
	tokenPattern := regexp.MustCompile(`(?i)(bearer\s+|api[_-]?key=|password=|token=|secret=)([^\s]+)`)
	return tokenPattern.ReplaceAllString(command, "${1}****")
}

// escapeLogContent prevents log injection by escaping control characters
func escapeLogContent(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

func auditLog(command string) {
	if auditLogFile == "" {
		return
	}
	dir := filepath.Dir(auditLogFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Use 0600 to prevent other users from reading potentially sensitive commands
	f, err := os.OpenFile(auditLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format(time.RFC3339)
	redacted := redactCommandForLog(command)
	escaped := escapeLogContent(redacted)
	fmt.Fprintf(f, "[%s] %s\n", timestamp, escaped)
}
