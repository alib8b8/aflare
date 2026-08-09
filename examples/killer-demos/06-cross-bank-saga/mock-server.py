#!/usr/bin/env python3
"""Mock Bank API for the Cross-Bank Transfer Saga demo.

模拟跨行转账涉及的多个端点，用于演示 Saga 事务补偿、限流、重试、幂等：

端点：
  POST /debit     A 行扣款（始终成功）
  POST /credit    B 行入账（amount=9999 强制失败，触发 saga 补偿）
  POST /refund    A 行退款（debit 的补偿）
  POST /reverse   B 行冲正（credit 的补偿）
  POST /notify    转账通知（无副作用）
  GET  /health    健康检查
  GET  /ops       查看操作日志（验证 saga 执行顺序）
  GET  /balances  查看账户余额

故障注入（演示补偿路径）：
  - amount=9999 时 /credit 强制返回 500，触发 debit 的补偿（/refund）
  - 其他 amount 正常处理，saga 提交
  - 改 MOCK_CREDIT_FAIL_AMOUNT 环境变量可调整触发失败的金额
  - 约 15% 概率返回 503（演示 HTTP 重试 + 退避）

服务端幂等：
  - 所有写端点支持 Idempotency-Key 去重（补偿步骤可能被多次调用）
  - 无外部依赖，仅用标准库

运行：python3 mock-server.py
默认监听 :17904
"""

import json
import os
import random
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

LISTEN_ADDR = ("0.0.0.0", 17904)

# 触发 /credit 失败的金额（演示补偿路径）
CREDIT_FAIL_AMOUNT = float(os.environ.get("MOCK_CREDIT_FAIL_AMOUNT", "9999"))

# 模拟瞬时故障概率（演示 HTTP 重试 + 退避）
FLAKY_PROBABILITY = float(os.environ.get("MOCK_FLAKY_PROBABILITY", "0.15"))

# 服务端幂等缓存：Idempotency-Key -> 首次响应
_idem_store = {}
_idem_lock = threading.Lock()

# 幂等命中计数
_idem_hits = 0
_idem_hits_lock = threading.Lock()

# 操作日志（供验证 forward/compensate 顺序）
_op_log = []
_op_log_lock = threading.Lock()

# 账户余额（模拟）
_balances = {
    "ACC-001": 1000000.00,
    "ACC-002": 500000.00,
}
_balances_lock = threading.Lock()


def log_op(endpoint, req, status):
    """记录操作到内存日志，便于验证 saga forward/compensate 执行顺序。"""
    entry = {
        "ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "endpoint": endpoint,
        "account": req.get("account", ""),
        "amount": req.get("amount", 0),
        "status": status,
        "reason": req.get("reason", ""),
        "ref": req.get("ref", ""),
    }
    with _op_log_lock:
        _op_log.append(entry)
    print(f"[mock-bank] {endpoint} account={entry['account']} "
          f"amount={entry['amount']} status={status}")


def maybe_flaky():
    """模拟瞬时故障：返回 True 表示触发 503。"""
    return random.random() < FLAKY_PROBABILITY


class BankHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[mock-bank] {self.address_string()} - {fmt % args}")

    def _send_json(self, status_code, payload):
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            return json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            return None

    def _handle_idempotent(self, idem_key, default_resp):
        """服务端去重：同一 Idempotency-Key 返回首次结果。"""
        global _idem_hits
        if not idem_key:
            return None
        with _idem_lock:
            if idem_key in _idem_store:
                cached = _idem_store[idem_key]
                with _idem_hits_lock:
                    _idem_hits += 1
                print(f"[mock-bank] idempotency hit key={idem_key} (total hits={_idem_hits})")
                return cached
        return default_resp

    def _cache_response(self, idem_key, code, body):
        if idem_key:
            with _idem_lock:
                _idem_store[idem_key] = {"code": code, "body": body}

    def _update_balance(self, account, delta):
        """更新账户余额（线程安全）。"""
        with _balances_lock:
            if account in _balances:
                _balances[account] += delta

    def do_GET(self):
        if self.path == "/health":
            self._send_json(200, {"status": "ok", "uptime": "healthy"})
            return
        if self.path == "/ops":
            with _op_log_lock:
                with _idem_hits_lock:
                    self._send_json(200, {"ops": list(_op_log), "idempotency_hits": _idem_hits})
            return
        if self.path == "/balances":
            with _balances_lock:
                self._send_json(200, {"balances": dict(_balances)})
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self):
        idem_key = self.headers.get("Idempotency-Key", "")
        req = self._read_body()
        if req is None:
            self._send_json(400, {"status": "failed", "error": "invalid JSON"})
            return

        # 幂等检查
        cached = self._handle_idempotent(idem_key, None)
        if cached is not None:
            self._send_json(cached["code"], cached["body"])
            return

        # 模拟瞬时故障（演示重试 + 退避）
        if maybe_flaky():
            resp = {"status": "failed", "error": "temporary service unavailable",
                    "retryable": True}
            self._send_json(503, resp)
            return

        amount = float(req.get("amount", 0))

        if self.path == "/debit":
            # A 行扣款：始终成功
            account = req.get("account", "")
            self._update_balance(account, -amount)
            resp = {"status": "success", "endpoint": "debit",
                    "account": account, "amount": amount,
                    "ref": req.get("ref", "")}
            log_op("debit", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/credit":
            # B 行入账：amount=9999 强制失败（触发 saga 补偿 debit）
            account = req.get("account", "")
            if amount == CREDIT_FAIL_AMOUNT:
                resp = {"status": "failed", "endpoint": "credit",
                        "error": "credit limit exceeded for demo amount",
                        "account": account}
                log_op("credit", req, "failed")
                self._cache_response(idem_key, 500, resp)
                self._send_json(500, resp)
                return
            self._update_balance(account, +amount)
            resp = {"status": "success", "endpoint": "credit",
                    "account": account, "amount": amount,
                    "ref": req.get("ref", "")}
            log_op("credit", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/refund":
            # A 行退款（debit 的补偿）：始终成功
            account = req.get("account", "")
            self._update_balance(account, +amount)
            resp = {"status": "success", "endpoint": "refund",
                    "account": account, "amount": amount,
                    "reason": req.get("reason", ""),
                    "ref": req.get("ref", "")}
            log_op("refund", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/reverse":
            # B 行冲正（credit 的补偿）：始终成功
            account = req.get("account", "")
            self._update_balance(account, -amount)
            resp = {"status": "success", "endpoint": "reverse",
                    "account": account, "amount": amount,
                    "reason": req.get("reason", ""),
                    "ref": req.get("ref", "")}
            log_op("reverse", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/notify":
            # 通知：无副作用，始终成功
            resp = {"status": "success", "endpoint": "notify",
                    "ref": req.get("ref", "")}
            log_op("notify", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        self._send_json(404, {"error": "not found"})


def main():
    print("=" * 60)
    print("  🏦  Mock Bank API — Cross-Bank Transfer Saga Demo")
    print("=" * 60)
    server = HTTPServer(LISTEN_ADDR, BankHandler)
    print(f"  Listening on http://{LISTEN_ADDR[0]}:{LISTEN_ADDR[1]}")
    print(f"  Credit fail amount: {CREDIT_FAIL_AMOUNT}")
    print(f"  Flaky probability:  {FLAKY_PROBABILITY}")
    print()
    print("  Endpoints:")
    print("    POST /debit     — A 行扣款")
    print("    POST /credit    — B 行入账（amount=9999 触发失败）")
    print("    POST /refund    — A 行退款（debit 补偿）")
    print("    POST /reverse   — B 行冲正（credit 补偿）")
    print("    POST /notify    — 转账通知")
    print("    GET  /health    — 健康检查")
    print("    GET  /ops       — 操作日志")
    print("    GET  /balances  — 账户余额")
    print()
    print("  Demo scenarios:")
    print("    amount=9999  → credit 失败 → debit 被补偿（退款）")
    print("    amount=5000  → 全部成功 → saga 提交")
    print("=" * 60)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock-bank] shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()