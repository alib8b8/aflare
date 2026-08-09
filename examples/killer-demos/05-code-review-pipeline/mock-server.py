#!/usr/bin/env python3
"""Mock LLM API + sample source file for the Code Review Pipeline demo.

模拟 OpenAI-compatible Chat Completion API，返回代码审查结果。
配合 mock-source.go 示例文件，展示完整的代码审查流水线。

LLM 端点：
  POST /v1/chat/completions   Chat Completion（返回审查结果）
  GET  /health                 健康检查

故障注入：
  - 约 10% 概率返回 503（演示 HTTP 重试）
  - 环境变量 MOCK_FORCE_503_COUNT=N 强制前 N 次返回 503

运行：python3 mock-server.py
默认监听 :17903
"""

import json
import os
import random
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

LISTEN_ADDR = ("0.0.0.0", 17903)

# ── 确定性 503 注入 ──
_force_503_remaining = 0
_force_503_lock = threading.Lock()
try:
    _force_503_remaining = int(os.environ.get("MOCK_FORCE_503_COUNT", "0"))
except ValueError:
    _force_503_remaining = 0

# ── 模拟代码审查结果 ──
MOCK_REVIEW_RESULT = """## Issues Found

### 🔴 Critical
- **Line 42**: Possible nil pointer dereference — `resp.Body` accessed without nil check after `http.Post` call. The `http.Post` function can return a non-nil response with an error. Add a nil check before accessing `resp.Body`.
- **Line 87**: Hardcoded API key — `apiKey := "sk-test-12345"` exposed in source code. Use environment variables or a secrets manager instead.

### 🟡 High
- **Line 56**: Missing context propagation — `http.NewRequest` used without `context.Context`. All outgoing HTTP requests should carry a context for cancellation and timeout.
- **Line 128**: Race condition in `counter` variable — concurrent goroutines increment `counter` without synchronization. Use `sync/atomic` or a `sync.Mutex`.

### 🟢 Medium
- **Line 15**: Error ignored — `json.Unmarshal` return value not checked. Silent data corruption risk if JSON is malformed.
- **Line 73**: Large allocation in hot path — `make([]byte, 1024*1024)` allocated inside a loop. Consider using a buffer pool (`sync.Pool`).

### ⚪ Low
- **Line 102**: Magic number `8080` — extract to a named constant for readability.
- **Line 155**: Function `processData` exceeds 80 lines — consider splitting into smaller functions.

### 📊 Summary
- **Total issues**: 8
- **Critical**: 2 | **High**: 2 | **Medium**: 2 | **Low**: 2
- **Security**: 1 found (hardcoded API key)
- **Thread Safety**: 1 found (unsynchronized counter)
- **Error Handling**: 2 found (nil dereference, ignored error)
- **Performance**: 1 found (large allocation in loop)
- **Code Style**: 2 found (magic number, long function)
"""

MOCK_SECOND_OPINION = """## Second Opinion — Cross-Validation

### CONFIRMED Findings
- **Line 42 (nil pointer dereference)**: CONFIRMED. This is a real bug — `http.Post` with a non-2xx response returns both a response and an error. The body should be checked for nil before reading.
- **Line 87 (hardcoded API key)**: CONFIRMED. Critical security issue. Even in test code, never hardcode credentials.
- **Line 128 (race condition)**: CONFIRMED. The `counter` variable is accessed from multiple goroutines. Use `sync/atomic.AddInt64`.

### DISPUTED Findings
- **Line 73 (large allocation)**: DISPUTED. The `make([]byte, 1024*1024)` is inside a loop that runs at most 3 times. A `sync.Pool` would add unnecessary complexity here. The current approach is acceptable.

### MISSED Issues (Primary Reviewer Overlooked)
- **Line 34**: Missing `defer resp.Body.Close()` — resource leak. The response body is never closed.
- **Line 66**: `time.Sleep` in test — makes tests flaky. Use `time.Ticker` or mock the clock instead.

### 📊 Second Opinion Summary
- **Agreement**: 3/5 findings confirmed
- **Disputed**: 1 finding (premature optimization)
- **New Issues Found**: 2 (resource leak, flaky test)
- **Overall**: Primary review is thorough. The disputed finding is a style preference, not a bug.
"""

# 请求计数器（用于区分主审查和第二意见）
_request_count = 0
_request_lock = threading.Lock()


class MockLLMHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[mock-llm] {self.address_string()} - {fmt % args}")

    def _send_json(self, status_code, payload):
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _maybe_flaky(self):
        global _force_503_remaining
        with _force_503_lock:
            if _force_503_remaining > 0:
                _force_503_remaining -= 1
                print(f"[mock-llm] forced 503 remaining={_force_503_remaining}")
                return True
        return random.random() < 0.1

    def do_GET(self):
        if self.path == "/health":
            self._send_json(200, {"status": "ok"})
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self._send_json(404, {"error": "not found"})
            return

        # 模拟瞬时故障
        if self._maybe_flaky():
            self._send_json(503, {"error": "temporary service unavailable"})
            return

        # 读取请求体
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length > 0 else b"{}"
        try:
            req = json.loads(body)
        except json.JSONDecodeError:
            req = {}

        model = req.get("model", "unknown")
        messages = req.get("messages", [])
        system_msg = ""
        for m in messages:
            if m.get("role") == "system":
                system_msg = m.get("content", "")
                break

        # 根据请求类型返回不同的审查结果
        global _request_count
        with _request_lock:
            _request_count += 1
            req_num = _request_count

        # 判断是主审查还是第二意见：第二意见的 system prompt 包含 "second opinion"
        if "second opinion" in system_msg.lower() or "second opinion" in str(messages).lower():
            content = MOCK_SECOND_OPINION
            print(f"[mock-llm] request #{req_num}: second opinion review (model={model})")
        else:
            content = MOCK_REVIEW_RESULT
            print(f"[mock-llm] request #{req_num}: primary review (model={model})")

        # 模拟延迟（LLM 推理需要时间）
        time.sleep(random.uniform(0.5, 1.5))

        resp = {
            "id": f"chatcmpl-{random.randint(10000, 99999)}",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": content
                    },
                    "finish_reason": "stop"
                }
            ],
            "usage": {
                "prompt_tokens": random.randint(500, 2000),
                "completion_tokens": random.randint(300, 1500),
                "total_tokens": random.randint(800, 3500)
            }
        }
        self._send_json(200, resp)


def create_sample_source():
    """创建示例源文件供演示使用。"""
    content = """package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

// processData fetches and processes data from the given URL.
func processData(url string) (*Response, error) {
    resp, err := http.Post(url, "application/json", nil)
    // BUG: resp.Body accessed without nil check — http.Post can return
    // a non-nil response with an error (e.g. non-2xx status).
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()

    var result Response
    // BUG: json.Unmarshal error ignored — silent data corruption.
    json.Unmarshal(body, &result)

    return &result, nil
}

// Counter tracks concurrent operations.
type Counter struct {
    value int64
    // BUG: value accessed without synchronization in concurrent goroutines.
}

func (c *Counter) Increment() {
    c.value++
    // BUG: Race condition — multiple goroutines can read-modify-write
    // simultaneously without atomic operations or mutex.
}

func (c *Counter) Value() int64 {
    return c.value
}

// APIKey used for external service authentication.
// BUG: Hardcoded API key — exposed in source code and version control.
const apiKey = "sk-test-12345"

// ServerConfig holds the server configuration.
type ServerConfig struct {
    Port int
    Host string
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() ServerConfig {
    return ServerConfig{
        // BUG: Magic number — extract to named constant.
        Port: 8080,
        Host: "localhost",
    }
}

// fetchBatch fetches a batch of URLs concurrently.
func fetchBatch(urls []string) []string {
    var wg sync.WaitGroup
    results := make([]string, len(urls))

    for i, url := range urls {
        wg.Add(1)
        go func(idx int, u string) {
            defer wg.Done()
            // BUG: Large allocation inside loop — make([]byte, 1MB) per iteration.
            buf := make([]byte, 1024*1024)
            // Simulate fetching...
            _ = buf
            results[idx] = fmt.Sprintf("fetched: %s", u)
        }(i, url)
    }

    wg.Wait()
    return results
}

// waitForService polls until the service is ready.
func waitForService(url string) error {
    for i := 0; i < 30; i++ {
        if ok := checkHealth(url); ok {
            return nil
        }
        // BUG: time.Sleep in test/production code — can cause flaky tests.
        time.Sleep(1 * time.Second)
    }
    return fmt.Errorf("service not ready after 30 attempts")
}

func checkHealth(url string) bool {
    // Stub implementation
    return true
}

type Response struct {
    Status  string `json:"status"`
    Message string `json:"message"`
}

func main() {
    fmt.Println("Starting server...")
    // BUG: Missing context propagation — no cancellation/timeout.
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    http.ListenAndServe(":8080", nil)
}
"""
    path = "mock-source.go"
    if not os.path.exists(path):
        with open(path, "w") as f:
            f.write(content)
        print(f"[mock] created sample source file: {path}")
    else:
        print(f"[mock] sample source file already exists: {path}")


def main():
    print("=" * 60)
    print("  🤖  Mock LLM API — Code Review Pipeline Demo")
    print("=" * 60)

    # 创建示例源文件
    create_sample_source()

    server = HTTPServer(LISTEN_ADDR, MockLLMHandler)
    print(f"  Listening on http://{LISTEN_ADDR[0]}:{LISTEN_ADDR[1]}")
    print()
    print("  Endpoints:")
    print("    POST /v1/chat/completions  — Chat Completion（返回审查结果）")
    print("    GET  /health               — 健康检查")
    print()
    print("  Sample source file:")
    print("    mock-source.go             — 包含故意引入的 bug 的示例代码")
    print("=" * 60)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock-llm] shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()