package nodes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
	"github.com/alib8b8/llm-box/internal/stats"
)

type CodeInterpreterNode struct{}

func init() {
	Register(&CodeInterpreterNode{})
}

func (n *CodeInterpreterNode) Name() string {
	return "code_interpreter"
}

func (n *CodeInterpreterNode) Description() string {
	return "Execute Python code in a sandboxed environment with file I/O"
}

func (n *CodeInterpreterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "code_interpreter",
		Description: "Execute Python code in a sandboxed environment with file I/O",
		Input:       "string - stdin for the Python code (optional)",
		Output:      "string - stdout, stderr, and generated files",
		Params: []ParamSchema{
			{Name: "code", Type: "string", Description: "Python code to execute", Required: true},
			{Name: "language", Type: "string", Description: "Programming language (default: python)", Required: false, Default: "python"},
			{Name: "timeout", Type: "string", Description: "Execution timeout (default based on security level)", Required: false},
			{Name: "work_dir", Type: "string", Description: "Working directory for code execution (default: temp dir)", Required: false},
			{Name: "save_outputs", Type: "string", Description: "If true, save output files to work_dir (default: true)", Required: false, Default: "true"},
			{Name: "network", Type: "string", Description: "Allow network access during execution (L0/L1 only, default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *CodeInterpreterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	secLevel := config.GetSecurityLevel()
	stats.GetSecurityStats().RecordSecurityLevel(secLevel)

	if secLevel == config.SecurityLevelL3 {
		stats.GetSecurityStats().RecordBlock(stats.BlockSafeMode, "code_interpreter blocked at L3", "code_interpreter")
		return "", fmt.Errorf("code_interpreter node is disabled at L3 security level (max security)")
	}

	code := params["code"]
	if code == "" {
		return "", fmt.Errorf("code parameter is required")
	}

	language := getParam(params, "language", "python")
	timeoutStr := getParam(params, "timeout", "")
	workDir := params["work_dir"]
	saveOutputs := getParam(params, "save_outputs", "true") == "true"
	allowNetwork := getParam(params, "network", "false") == "true"

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil || timeout <= 0 {
		switch secLevel {
		case config.SecurityLevelL0:
			timeout = 120 * time.Second
		case config.SecurityLevelL1:
			timeout = 30 * time.Second
		case config.SecurityLevelL2:
			timeout = 15 * time.Second
		default:
			timeout = 5 * time.Second
		}
	}

	if allowNetwork && !config.SecurityLevelAtLeast(config.SecurityLevelL2) == false {
		if secLevel == config.SecurityLevelL2 || secLevel == config.SecurityLevelL3 {
			allowNetwork = false
		}
	}

	if workDir == "" {
		tempDir, err := os.MkdirTemp("", "code-interp-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tempDir)
		workDir = tempDir
	} else {
		safeWorkDir, err := validateWritePath(workDir)
		if err != nil {
			return "", fmt.Errorf("invalid work_dir: %w", err)
		}
		workDir = safeWorkDir
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work dir: %w", err)
	}

	var result string
	startTime := time.Now()
	var timedOut bool

	switch language {
	case "python", "python3", "py":
		result, err = runPython(ctx, code, workDir, timeout, input, allowNetwork)
		if err != nil && strings.Contains(err.Error(), "DeadlineExceeded") {
			timedOut = true
		}
	default:
		return "", fmt.Errorf("unsupported language: %s (currently only python is supported)", language)
	}

	durationMs := time.Since(startTime).Milliseconds()
	stats.GetSecurityStats().RecordCodeInterpreterRun(durationMs, false, timedOut)

	if err != nil && result == "" {
		return result, err
	}

	if saveOutputs {
		files, _ := listGeneratedFiles(workDir)
		if len(files) > 0 {
			result += fmt.Sprintf("\n\n--- Generated Files ---\n%s", strings.Join(files, "\n"))
		}
	}

	return result, nil
}

func runPython(ctx context.Context, code, workDir string, timeout time.Duration, stdin string, allowNetwork bool) (string, error) {
	scriptPath := filepath.Join(workDir, "script.py")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	networkDisabledCode := ""
	if !allowNetwork {
		networkDisabledCode = `
import socket
import urllib.request
_orig_connect = socket.socket.connect
_orig_create = socket.create_connection
_orig_urlopen = urllib.request.urlopen
def _block_connect(*a, **k): raise OSError("Network access disabled by security policy")
def _block_urlopen(*a, **k): raise OSError("Network access disabled by security policy")
socket.socket.connect = _block_connect
socket.create_connection = _block_connect
urllib.request.urlopen = _block_urlopen
`
	}

	finalCode := networkDisabledCode + code
	if err := os.WriteFile(scriptPath, []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	var cmd *exec.Cmd
	if bwrap, err := exec.LookPath("bwrap"); err == nil {
		roBinds := []string{}
		for _, p := range []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc/ld.so.cache", "/etc/alternatives"} {
			if _, err := os.Stat(p); err == nil {
				roBinds = append(roBinds, "--ro-bind", p, p)
			}
		}
		args := []string{}
		args = append(args, roBinds...)
		args = append(args,
			"--dev", "/dev",
			"--proc", "/proc",
			"--bind", workDir, workDir,
			"--chdir", workDir,
			"--unshare-all",
			"--share-net",
		)
		if !allowNetwork {
			args[len(args)-1] = "--unshare-net"
		}
		args = append(args, "--", "python3", scriptPath)
		cmd = exec.CommandContext(ctx, bwrap, args...)
	} else {
		cmd = exec.CommandContext(ctx, "python3", scriptPath)
	}
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ""
	if stdoutStr := stdout.String(); stdoutStr != "" {
		result += stdoutStr
	}
	if stderrStr := stderr.String(); stderrStr != "" {
		if result != "" {
			result += "\n"
		}
		result += "[stderr] " + stderrStr
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result += fmt.Sprintf("\n[Error] Execution timed out after %s", timeout)
		} else if result == "" {
			return "", fmt.Errorf("code execution failed: %w", err)
		} else {
			result += fmt.Sprintf("\n[Error] %v", err)
		}
	}

	return result, nil
}

func listGeneratedFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "script.py" {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}
