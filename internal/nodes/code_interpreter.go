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

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

type CodeInterpreterNode struct{}

func init() {
	Register(&CodeInterpreterNode{})
}

func (n *CodeInterpreterNode) Name() string {
	return "code_interpreter"
}

func (n *CodeInterpreterNode) Description() string {
	return "Execute Python/Node.js/Rust code in a sandboxed environment with file I/O"
}

func (n *CodeInterpreterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "code_interpreter",
		Description: "Execute Python/Node.js/Rust code in a sandboxed environment with file I/O",
		Input:       "string - stdin for the code (optional)",
		Output:      "string - stdout, stderr, and generated files",
		Params: []ParamSchema{
			{Name: "code", Type: "string", Description: "Code to execute", Required: true},
			{Name: "language", Type: "string", Description: "Programming language: python, nodejs, rust (default: python)", Required: false, Default: "python"},
			{Name: "timeout", Type: "string", Description: "Execution timeout (default based on security level)", Required: false},
			{Name: "work_dir", Type: "string", Description: "Working directory for code execution (default: temp dir)", Required: false},
			{Name: "save_outputs", Type: "string", Description: "If true, save output files to work_dir (default: true)", Required: false, Default: "true"},
			{Name: "network", Type: "string", Description: "Allow network access during execution (L0/L1 only, default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *CodeInterpreterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	secLevel := config.GetSecurityLevel()
	core.GetSecurityStats().RecordSecurityLevel(secLevel)

	if secLevel == config.SecurityLevelL3 {
		core.GetSecurityStats().RecordBlock(core.BlockSafeMode, "code_interpreter blocked at L3", "code_interpreter")
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
		defer func() { _ = os.RemoveAll(tempDir) }()
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
	case "node", "nodejs", "javascript", "js":
		result, err = runNode(ctx, code, workDir, timeout, input, allowNetwork)
		if err != nil && strings.Contains(err.Error(), "DeadlineExceeded") {
			timedOut = true
		}
	case "rust", "rs":
		result, err = runRust(ctx, code, workDir, timeout, input, allowNetwork)
		if err != nil && strings.Contains(err.Error(), "DeadlineExceeded") {
			timedOut = true
		}
	default:
		return "", fmt.Errorf("unsupported language: %s (supported: python, nodejs, rust)", language)
	}

	durationMs := time.Since(startTime).Milliseconds()
	core.GetSecurityStats().RecordCodeInterpreterRun(durationMs, false, timedOut)

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
		cmd = exec.CommandContext(ctx, bwrap, args...) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	} else {
		cmd = exec.CommandContext(ctx, "python3", scriptPath) // #nosec G204 -- sandboxed/audited execution with internally generated paths
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

func runNode(ctx context.Context, code, workDir string, timeout time.Duration, stdin string, allowNetwork bool) (string, error) {
	scriptPath := filepath.Join(workDir, "script.js")

	networkDisabledCode := ""
	if !allowNetwork {
		networkDisabledCode = `
const _origFetch = globalThis.fetch;
const _origHttp = require('http');
const _origHttps = require('https');
function _blockNet(...a) { throw new Error('Network access disabled by security policy'); }
globalThis.fetch = _blockNet;
require('http').request = _blockNet;
require('http').get = _blockNet;
require('https').request = _blockNet;
require('https').get = _blockNet;
`
	}

	finalCode := networkDisabledCode + code
	if err := os.WriteFile(scriptPath, []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
		args = append(args, "--", "node", scriptPath)
		cmd = exec.CommandContext(ctx, bwrap, args...) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	} else {
		cmd = exec.CommandContext(ctx, "node", scriptPath) // #nosec G204 -- sandboxed/audited execution with internally generated paths
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

func runRust(ctx context.Context, code, workDir string, timeout time.Duration, stdin string, allowNetwork bool) (string, error) {
	srcPath := filepath.Join(workDir, "main.rs")
	binPath := filepath.Join(workDir, "program")

	networkCrateBlock := ""
	if !allowNetwork {
		networkCrateBlock = `
#[allow(dead_code)]
mod net_block {
    use std::io;
    fn blocked() -> io::Result<std::net::TcpStream> {
        Err(io::Error::new(io::ErrorKind::PermissionDenied, "Network access disabled by security policy"))
    }
}
`
	}

	finalCode := networkCrateBlock + code
	if err := os.WriteFile(srcPath, []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write source: %w", err)
	}

	compileTimeout := timeout
	compileCtx, compileCancel := context.WithTimeout(ctx, compileTimeout)
	defer compileCancel()

	var compileCmd *exec.Cmd
	if bwrap, err := exec.LookPath("bwrap"); err == nil {
		roBinds := []string{}
		for _, p := range []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc/ld.so.cache", "/etc/alternatives", filepath.Dir(srcPath)} {
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
		)
		if allowNetwork {
			args = append(args, "--share-net")
		} else {
			args = append(args, "--unshare-net")
		}
		args = append(args, "--", "rustc", "-o", binPath, srcPath)
		compileCmd = exec.CommandContext(compileCtx, bwrap, args...) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	} else {
		compileCmd = exec.CommandContext(compileCtx, "rustc", "-o", binPath, srcPath) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	}
	compileCmd.Dir = workDir

	var compileStderr strings.Builder
	compileCmd.Stderr = &compileStderr
	if err := compileCmd.Run(); err != nil {
		msg := compileStderr.String()
		if compileCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("rust compilation timed out after %s", compileTimeout)
		}
		return "", fmt.Errorf("rust compilation failed: %w\n%s", err, msg)
	}

	runCtx, runCancel := context.WithTimeout(ctx, timeout)
	defer runCancel()

	var runCmd *exec.Cmd
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
		)
		if allowNetwork {
			args = append(args, "--share-net")
		} else {
			args = append(args, "--unshare-net")
		}
		args = append(args, "--", binPath)
		runCmd = exec.CommandContext(runCtx, bwrap, args...) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	} else {
		runCmd = exec.CommandContext(runCtx, binPath) // #nosec G204 -- sandboxed/audited execution with internally generated paths
	}
	runCmd.Dir = workDir
	runCmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr strings.Builder
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	err := runCmd.Run()

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
		if runCtx.Err() == context.DeadlineExceeded {
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
		if name == "script.py" || name == "script.js" || name == "main.rs" || name == "program" {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}
