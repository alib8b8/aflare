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
			{Name: "timeout", Type: "string", Description: "Execution timeout (default: 30s)", Required: false, Default: "30s"},
			{Name: "work_dir", Type: "string", Description: "Working directory for code execution (default: temp dir)", Required: false},
			{Name: "save_outputs", Type: "string", Description: "If true, save output files to work_dir (default: true)", Required: false, Default: "true"},
		},
	}
}

func (n *CodeInterpreterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if IsSafeMode() {
		return "", fmt.Errorf("code_interpreter node is disabled in safe mode")
	}

	code := params["code"]
	if code == "" {
		return "", fmt.Errorf("code parameter is required")
	}

	language := getParam(params, "language", "python")
	timeoutStr := getParam(params, "timeout", "30s")
	workDir := params["work_dir"]
	saveOutputs := getParam(params, "save_outputs", "true") == "true"

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Second
	}

	if workDir == "" {
		tempDir, err := os.MkdirTemp("", "code-interp-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tempDir)
		workDir = tempDir
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work dir: %w", err)
	}

	var result string
	switch language {
	case "python", "python3", "py":
		result, err = runPython(ctx, code, workDir, timeout, input)
	default:
		return "", fmt.Errorf("unsupported language: %s (currently only python is supported)", language)
	}

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

func runPython(ctx context.Context, code, workDir string, timeout time.Duration, stdin string) (string, error) {
	scriptPath := filepath.Join(workDir, "script.py")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath)
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
