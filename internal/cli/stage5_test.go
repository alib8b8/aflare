// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌​​‌​‌​‌​​​‌​‌‌​​‌​‌​‌​​​​‌​​​​‌​‌‌​​‌​​‌‌​​​​​​​​​​​​​​​​​‌‌​‌​​‌​​‌​​‌​​​⁠
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
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/workflow"
)

// --- 断点11: humanizeError tests ---

func TestHumanizeError_DNSLookupFailure(t *testing.T) {
	err := fmt.Errorf(`Get "https://api.example.com": dial tcp: lookup api.example.com: no such host`)
	human, debug := humanizeError(err, "http_request")
	if !strings.Contains(human, "网络错误") {
		t.Errorf("expected network error message, got: %s", human)
	}
	if !strings.Contains(human, "api.example.com") {
		t.Errorf("expected hostname in message, got: %s", human)
	}
	if debug == "" {
		t.Error("debug message should not be empty")
	}
}

func TestHumanizeError_ConnectionRefused(t *testing.T) {
	err := fmt.Errorf("dial tcp 127.0.0.1:8080: connect: connection refused")
	human, _ := humanizeError(err, "http_request")
	if !strings.Contains(human, "拒绝连接") {
		t.Errorf("expected 'refused' message, got: %s", human)
	}
}

func TestHumanizeError_Unauthorized401(t *testing.T) {
	err := fmt.Errorf("HTTP request failed with status 401")
	human, _ := humanizeError(err, "http_request")
	if !strings.Contains(human, "认证失败") {
		t.Errorf("expected auth failure message, got: %s", human)
	}
}

func TestHumanizeError_Timeout(t *testing.T) {
	err := fmt.Errorf("context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
	human, _ := humanizeError(err, "http_request")
	if !strings.Contains(human, "超时") {
		t.Errorf("expected timeout message, got: %s", human)
	}
}

func TestHumanizeError_FileNotFound(t *testing.T) {
	err := fmt.Errorf(`failed to stat file: stat /path/to/missing.yaml: no such file or directory`)
	human, _ := humanizeError(err, "file_read")
	if !strings.Contains(human, "文件不存在") {
		t.Errorf("expected file-not-found message, got: %s", human)
	}
}

func TestHumanizeError_FileNotFoundTyped(t *testing.T) {
	err := &fs.PathError{Op: "open", Path: "/tmp/missing.txt", Err: fs.ErrNotExist}
	human, _ := humanizeError(err, "file_read")
	if !strings.Contains(human, "文件不存在") {
		t.Errorf("expected file-not-found message, got: %s", human)
	}
	if !strings.Contains(human, "/tmp/missing.txt") {
		t.Errorf("expected path in message, got: %s", human)
	}
}

func TestHumanizeError_CommandNotFound(t *testing.T) {
	err := &exec.Error{Name: "kubectl", Err: fmt.Errorf("executable file not found in $PATH")}
	human, _ := humanizeError(err, "execute")
	if !strings.Contains(human, "未找到命令") || !strings.Contains(human, "kubectl") {
		t.Errorf("expected command-not-found with kubectl, got: %s", human)
	}
}

func TestHumanizeError_CommandNotFoundPattern(t *testing.T) {
	err := fmt.Errorf(`command failed: exec: "jq": executable file not found in $PATH`)
	human, _ := humanizeError(err, "execute")
	if !strings.Contains(human, "未找到命令") {
		t.Errorf("expected command-not-found message, got: %s", human)
	}
}

func TestHumanizeError_DNSTyped(t *testing.T) {
	err := &net.DNSError{Err: "no such host", Name: "api.missing.com", IsNotFound: true}
	human, _ := humanizeError(err, "http_request")
	if !strings.Contains(human, "api.missing.com") {
		t.Errorf("expected hostname in message, got: %s", human)
	}
}

func TestHumanizeError_RateLimited(t *testing.T) {
	err := fmt.Errorf("HTTP request failed with status 429")
	human, _ := humanizeError(err, "http_request")
	if !strings.Contains(human, "限流") {
		t.Errorf("expected rate-limit message, got: %s", human)
	}
}

func TestHumanizeError_OllamaNotRunning(t *testing.T) {
	err := fmt.Errorf("ollama not running, please start it first: connection refused")
	human, _ := humanizeError(err, "ollama")
	if !strings.Contains(human, "Ollama") {
		t.Errorf("expected Ollama hint, got: %s", human)
	}
}

func TestHumanizeError_NodeNotFound(t *testing.T) {
	err := fmt.Errorf("node 'foobar' not found in registry")
	human, _ := humanizeError(err, "foobar")
	if !strings.Contains(human, "未知节点") || !strings.Contains(human, "foobar") {
		t.Errorf("expected node-not-found message, got: %s", human)
	}
}

func TestHumanizeError_Fallback(t *testing.T) {
	err := fmt.Errorf("some completely unknown error type")
	human, debug := humanizeError(err, "custom")
	if human != debug {
		t.Errorf("fallback should return original error, got human=%q debug=%q", human, debug)
	}
}

func TestHumanizeError_NilError(t *testing.T) {
	human, debug := humanizeError(nil, "http_request")
	if human != "" || debug != "" {
		t.Errorf("nil error should return empty strings, got human=%q debug=%q", human, debug)
	}
}

// --- 断点11: troubleshootHint tests ---

func TestTroubleshootHint_Ollama(t *testing.T) {
	hint := troubleshootHint("ollama", fmt.Errorf("timeout"))
	if !strings.Contains(hint, "ollama list") {
		t.Errorf("expected ollama troubleshooting hint, got: %s", hint)
	}
}

func TestTroubleshootHint_HTTPRequest(t *testing.T) {
	hint := troubleshootHint("http_request", fmt.Errorf("dial tcp: no such host"))
	if !strings.Contains(hint, "URL") {
		t.Errorf("expected URL troubleshooting hint, got: %s", hint)
	}
}

func TestTroubleshootHint_NilError(t *testing.T) {
	if hint := troubleshootHint("http_request", nil); hint != "" {
		t.Errorf("nil error should return empty hint, got: %s", hint)
	}
}

func TestTroubleshootHint_UnknownNode(t *testing.T) {
	if hint := troubleshootHint("unknown_node", fmt.Errorf("some error")); hint != "" {
		t.Errorf("unknown node should return empty hint, got: %s", hint)
	}
}

// --- 断点11: formatDuration tests ---

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{100 * time.Millisecond, "0.1s"},
		{300 * time.Millisecond, "0.3s"},
		{1500 * time.Millisecond, "1.5s"},
		{90 * time.Second, "1m30s"},
	}
	for _, tc := range tests {
		t.Run(tc.d.String(), func(t *testing.T) {
			got := formatDuration(tc.d)
			if got != tc.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// --- 断点12: loadParamsFile tests ---

func TestLoadParamsFile_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	content := `{"coin_ids": "bitcoin", "portfolio_file": "./portfolio.json"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	params, err := loadParamsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["coin_ids"] != "bitcoin" {
		t.Errorf("coin_ids = %q, want %q", params["coin_ids"], "bitcoin")
	}
	if params["portfolio_file"] != "./portfolio.json" {
		t.Errorf("portfolio_file = %q, want %q", params["portfolio_file"], "./portfolio.json")
	}
}

func TestLoadParamsFile_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.yaml")
	content := "coin_ids: bitcoin\nportfolio_file: ./portfolio.json\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	params, err := loadParamsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["coin_ids"] != "bitcoin" {
		t.Errorf("coin_ids = %q, want %q", params["coin_ids"], "bitcoin")
	}
}

func TestLoadParamsFile_NonStringValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	content := `{"count": 42, "enabled": true}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	params, err := loadParamsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["count"] != "42" {
		t.Errorf("count = %q, want %q", params["count"], "42")
	}
	if params["enabled"] != "true" {
		t.Errorf("enabled = %q, want %q", params["enabled"], "true")
	}
}

func TestLoadParamsFile_FileNotFound(t *testing.T) {
	_, err := loadParamsFile("/nonexistent/params.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "无法读取") {
		t.Errorf("expected 'cannot read' error, got: %v", err)
	}
}

func TestLoadParamsFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadParamsFile(path)
	if err == nil || !strings.Contains(err.Error(), "为空") {
		t.Errorf("expected empty-file error, got: %v", err)
	}
}

func TestLoadParamsFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadParamsFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadParamsFile_AutoDetectYAML(t *testing.T) {
	dir := t.TempDir()
	// No extension, should auto-detect as YAML after JSON fails.
	path := filepath.Join(dir, "params")
	content := "key: value\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	params, err := loadParamsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["key"] != "value" {
		t.Errorf("key = %q, want %q", params["key"], "value")
	}
}

// --- 断点13: StepProgressEvent / workflow callback integration test ---

func TestStepProgressCallback_Invoked(t *testing.T) {
	// This is a lightweight unit test verifying that the callback type
	// compiles and can be called. Full executor integration is covered
	// by existing executor tests that now pass nil as the callback.
	var events []workflow.StepProgressEvent
	cb := workflow.StepProgressFunc(func(ev workflow.StepProgressEvent) {
		events = append(events, ev)
	})
	cb(workflow.StepProgressEvent{Index: 0, Total: 3, Status: workflow.StepProgressStarted})
	cb(workflow.StepProgressEvent{Index: 0, Total: 3, Status: workflow.StepProgressCompleted, Duration: 100 * time.Millisecond})

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Status != workflow.StepProgressStarted {
		t.Errorf("first event status = %q, want %q", events[0].Status, workflow.StepProgressStarted)
	}
	if events[1].Status != workflow.StepProgressCompleted {
		t.Errorf("second event status = %q, want %q", events[1].Status, workflow.StepProgressCompleted)
	}
}
