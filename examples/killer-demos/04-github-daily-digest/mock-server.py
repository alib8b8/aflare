#!/usr/bin/env python3
"""Mock GitHub + Telegram API for the GitHub Daily Digest demo.

模拟 GitHub REST API 和 Telegram Bot API，用于演示个人自动化场景：

GitHub 端点：
  GET /users/{username}/events  用户动态（Push/PR/Issue/Star/Release）
  GET /trending                 Trending 仓库（模拟）
  GET /search/issues            搜索 Issue/PR
  GET /health                   健康检查

Telegram 端点：
  POST /bot{token}/sendMessage  发送消息（模拟）

故障注入：
  - 约 10% 概率返回 503（演示 HTTP 重试 + 退避）
  - 环境变量 MOCK_FORCE_503_COUNT=N 强制前 N 次返回 503

运行：python3 mock-server.py
默认监听 :17902
"""

import json
import os
import random
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse

LISTEN_ADDR = ("0.0.0.0", 17902)

# ── 确定性 503 注入 ──
_force_503_remaining = 0
_force_503_lock = threading.Lock()
try:
    _force_503_remaining = int(os.environ.get("MOCK_FORCE_503_COUNT", "0"))
except ValueError:
    _force_503_remaining = 0

# ── 模拟数据 ──
MOCK_EVENTS = [
    {
        "id": "1", "type": "PushEvent",
        "repo": {"name": "your-username/awesome-project", "url": "https://github.com/your-username/awesome-project"},
        "payload": {"commits": [{"message": "feat: add real-time notification system"}, {"message": "fix: resolve race condition in worker pool"}]},
        "created_at": "2026-08-09T08:30:00Z"
    },
    {
        "id": "2", "type": "PullRequestEvent",
        "repo": {"name": "your-username/api-gateway", "url": "https://github.com/your-username/api-gateway"},
        "payload": {"action": "opened", "pull_request": {"title": "Add rate limiting middleware", "state": "open", "html_url": "https://github.com/your-username/api-gateway/pull/42"}},
        "created_at": "2026-08-09T07:15:00Z"
    },
    {
        "id": "3", "type": "IssuesEvent",
        "repo": {"name": "your-username/cli-tool", "url": "https://github.com/your-username/cli-tool"},
        "payload": {"action": "opened", "issue": {"title": "Support custom config path via environment variable", "state": "open", "html_url": "https://github.com/your-username/cli-tool/issues/128"}},
        "created_at": "2026-08-09T06:00:00Z"
    },
    {
        "id": "4", "type": "WatchEvent",
        "repo": {"name": "torvalds/linux", "url": "https://github.com/torvalds/linux"},
        "payload": {"action": "started"},
        "created_at": "2026-08-08T22:00:00Z"
    },
    {
        "id": "5", "type": "ReleaseEvent",
        "repo": {"name": "your-username/awesome-project", "url": "https://github.com/your-username/awesome-project"},
        "payload": {"release": {"tag_name": "v2.1.0", "name": "v2.1.0 - Performance Boost", "html_url": "https://github.com/your-username/awesome-project/releases/tag/v2.1.0"}},
        "created_at": "2026-08-08T18:00:00Z"
    },
    {
        "id": "6", "type": "PullRequestReviewEvent",
        "repo": {"name": "your-username/awesome-project", "url": "https://github.com/your-username/awesome-project"},
        "payload": {"action": "submitted", "review": {"state": "changes_requested"}, "pull_request": {"title": "Refactor database layer", "html_url": "https://github.com/your-username/awesome-project/pull/87"}},
        "created_at": "2026-08-08T16:00:00Z"
    },
    {
        "id": "7", "type": "PushEvent",
        "repo": {"name": "your-username/dotfiles", "url": "https://github.com/your-username/dotfiles"},
        "payload": {"commits": [{"message": "Update zsh config for new M4 Mac"}]},
        "created_at": "2026-08-08T14:00:00Z"
    },
]

MOCK_TRENDING = [
    {"name": "microsoft/garnet", "description": "A remote cache-store from Microsoft Research", "stars": 12340, "stars_today": 342, "language": "C#", "url": "https://github.com/microsoft/garnet"},
    {"name": "exo-lang/exo", "description": "A new systems programming language with compile-time memory safety", "stars": 8920, "stars_today": 567, "language": "Rust", "url": "https://github.com/exo-lang/exo"},
    {"name": "anthropics/claude-code", "description": "Claude Code — an AI coding agent in your terminal", "stars": 25600, "stars_today": 1203, "language": "TypeScript", "url": "https://github.com/anthropics/claude-code"},
    {"name": "vercel/ai", "description": "Build AI-powered applications with React and Svelte", "stars": 18900, "stars_today": 234, "language": "TypeScript", "url": "https://github.com/vercel/ai"},
    {"name": "tursodatabase/limbo", "description": "Limbo: A complete rewrite of SQLite in Rust", "stars": 11200, "stars_today": 891, "language": "Rust", "url": "https://github.com/tursodatabase/limbo"},
    {"name": "huggingface/transformers", "description": "State-of-the-art Machine Learning for JAX, PyTorch and TensorFlow", "stars": 145000, "stars_today": 156, "language": "Python", "url": "https://github.com/huggingface/transformers"},
    {"name": "astral-sh/uv", "description": "An extremely fast Python package and project manager, written in Rust", "stars": 45200, "stars_today": 678, "language": "Rust", "url": "https://github.com/astral-sh/uv"},
]

MOCK_PR_ISSUES = {
    "total_count": 3,
    "items": [
        {"title": "Add rate limiting middleware", "state": "open", "html_url": "https://github.com/your-username/api-gateway/pull/42", "created_at": "2026-08-09T07:15:00Z", "labels": [{"name": "enhancement"}, {"name": "needs-review"}]},
        {"title": "Refactor database layer", "state": "open", "html_url": "https://github.com/your-username/awesome-project/pull/87", "created_at": "2026-08-07T10:00:00Z", "labels": [{"name": "refactor"}, {"name": "changes-requested"}]},
        {"title": "Support custom config path via environment variable", "state": "open", "html_url": "https://github.com/your-username/cli-tool/issues/128", "created_at": "2026-08-09T06:00:00Z", "labels": [{"name": "enhancement"}, {"name": "good-first-issue"}]},
    ]
}


class MockHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[mock] {self.address_string()} - {fmt % args}")

    def _send_json(self, status_code, payload):
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _maybe_flaky(self):
        """模拟瞬时故障：支持确定性 503 注入和随机 10% 概率。"""
        global _force_503_remaining
        with _force_503_lock:
            if _force_503_remaining > 0:
                _force_503_remaining -= 1
                print(f"[mock] forced 503 remaining={_force_503_remaining}")
                return True
        return random.random() < 0.1

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path

        # 健康检查
        if path == "/health":
            self._send_json(200, {"status": "ok"})
            return

        # 模拟瞬时故障
        if self._maybe_flaky():
            self._send_json(503, {"error": "temporary service unavailable"})
            return

        # 用户动态
        if path.startswith("/users/") and path.endswith("/events"):
            # 提取用户名：/users/{username}/events
            username = path.split("/")[2]
            # 非演示用户返回空列表（演示限流/降级场景）
            if username not in ("your-username", "demo-user"):
                self._send_json(200, [])
                return
            self._send_json(200, MOCK_EVENTS)
            return

        # Trending
        if path == "/trending":
            self._send_json(200, MOCK_TRENDING)
            return

        # 搜索 Issue/PR
        if path == "/search/issues":
            # 检查是否有查询参数
            query = parsed.query
            if not query:
                self._send_json(200, {"total_count": 0, "items": []})
                return
            self._send_json(200, MOCK_PR_ISSUES)
            return

        self._send_json(404, {"error": "not found"})

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path

        # Telegram sendMessage（模拟）
        if "/sendMessage" in path:
            # 读取请求体
            length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(length) if length > 0 else b"{}"
            try:
                req = json.loads(body)
            except json.JSONDecodeError:
                req = {}
            chat_id = req.get("chat_id", "unknown")
            text = (req.get("text", "") or "")[:80]
            print(f"[mock] telegram message to chat={chat_id}: {text}...")
            self._send_json(200, {"ok": True, "result": {"message_id": random.randint(1000, 9999), "chat": {"id": chat_id}}})
            return

        self._send_json(404, {"error": "not found"})


def main():
    print("=" * 60)
    print("  🐙  Mock GitHub + Telegram API — GitHub Daily Digest Demo")
    print("=" * 60)
    server = HTTPServer(LISTEN_ADDR, MockHandler)
    print(f"  Listening on http://{LISTEN_ADDR[0]}:{LISTEN_ADDR[1]}")
    print()
    print("  GitHub Endpoints:")
    print("    GET /users/{username}/events  — 用户动态")
    print("    GET /trending                 — Trending 仓库")
    print("    GET /search/issues            — 搜索 Issue/PR")
    print("    GET /health                   — 健康检查")
    print()
    print("  Telegram Endpoints:")
    print("    POST /bot{token}/sendMessage  — 发送消息")
    print("=" * 60)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock] shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()