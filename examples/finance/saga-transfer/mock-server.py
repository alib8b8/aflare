#!/usr/bin/env python3
"""Mock bank API for the saga-transfer demo.

模拟跨行转账涉及的多个端点，用于演示 saga 事务补偿：
  POST /debit    A 行扣款
  POST /credit   B 行入账（对 amount=9999 强制失败，触发补偿）
  POST /refund   A 行退款（debit 的补偿）
  POST /reverse  B 行冲正（credit 的补偿）
  POST /notify   转账通知（无副作用）
  GET  /health   健康检查

故障注入（演示补偿路径）：
  - amount=9999 时 /credit 强制返回 500，触发 debit 的补偿（/refund）
  - 其他 amount 正常处理，saga 提交
  - 改 MOCK_CREDIT_FAIL_AMOUNT 环境变量可调整触发失败的金额

服务端幂等：
  - 所有写端点支持 Idempotency-Key 去重（补偿步骤可能被多次调用）
  - 无外部依赖，仅用标准库

运行：python3 mock-server.py
默认监听 :17801
"""

import json
import os
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

LISTEN_ADDR = ("0.0.0.0", 17801)

# 触发 /credit 失败的金额（演示补偿路径）
CREDIT_FAIL_AMOUNT = float(os.environ.get("MOCK_CREDIT_FAIL_AMOUNT", "9999"))

# 服务端幂等缓存：Idempotency-Key -> 首次响应
_idem_store = {}
_idem_lock = threading.Lock()

# 操作日志（供验证 forward/compensate 顺序）
_op_log = []
_op_log_lock = threading.Lock()


def log_op(endpoint, req, status):
    """记录操作到内存日志，便于验证 saga forward/compensate 执行顺序。"""
    entry = {
        "endpoint": endpoint,
        "account": req.get("account", ""),
        "amount": req.get("amount", 0),
        "status": status,
        "reason": req.get("reason", ""),
    }
    with _op_log_lock:
        _op_log.append(entry)
    print(f"[mock-bank] {endpoint} account={entry['account']} "
          f"amount={entry['amount']} status={status}")


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
        if not idem_key:
            return None
        with _idem_lock:
            if idem_key in _idem_store:
                cached = _idem_store[idem_key]
                print(f"[mock-bank] idempotency hit key={idem_key}")
                return cached
        return default_resp

    def _cache_response(self, idem_key, code, body):
        if idem_key:
            with _idem_lock:
                _idem_store[idem_key] = {"code": code, "body": body}

    def do_GET(self):
        if self.path == "/health":
            self._send_json(200, {"status": "ok"})
            return
        if self.path == "/ops":
            # 查看操作日志（验证 saga 执行顺序）
            with _op_log_lock:
                self._send_json(200, {"ops": list(_op_log)})
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

        amount = float(req.get("amount", 0))

        if self.path == "/debit":
            # A 行扣款：始终成功（演示 forward 成功后需补偿）
            resp = {"status": "success", "endpoint": "debit",
                    "account": req.get("account", ""), "amount": amount}
            log_op("debit", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/credit":
            # B 行入账：amount=9999 强制失败（触发 saga 补偿 debit）
            if amount == CREDIT_FAIL_AMOUNT:
                resp = {"status": "failed", "endpoint": "credit",
                        "error": "credit limit exceeded for demo amount",
                        "account": req.get("account", "")}
                log_op("credit", req, "failed")
                self._cache_response(idem_key, 500, resp)
                self._send_json(500, resp)
                return
            resp = {"status": "success", "endpoint": "credit",
                    "account": req.get("account", ""), "amount": amount}
            log_op("credit", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/refund":
            # A 行退款（debit 的补偿）：始终成功
            resp = {"status": "success", "endpoint": "refund",
                    "account": req.get("account", ""), "amount": amount,
                    "reason": req.get("reason", "")}
            log_op("refund", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/reverse":
            # B 行冲正（credit 的补偿）：始终成功
            resp = {"status": "success", "endpoint": "reverse",
                    "account": req.get("account", ""), "amount": amount,
                    "reason": req.get("reason", "")}
            log_op("reverse", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        if self.path == "/notify":
            # 通知：无副作用，始终成功
            resp = {"status": "success", "endpoint": "notify"}
            log_op("notify", req, "success")
            self._cache_response(idem_key, 200, resp)
            self._send_json(200, resp)
            return

        self._send_json(404, {"error": "not found"})


def main():
    server = HTTPServer(LISTEN_ADDR, BankHandler)
    print(f"[mock-bank] listening on http://{LISTEN_ADDR[0]}:{LISTEN_ADDR[1]}")
    print(f"[mock-bank] credit fail amount: {CREDIT_FAIL_AMOUNT}")
    print("[mock-bank] endpoints: POST /debit /credit /refund /reverse /notify, "
          "GET /health /ops")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock-bank] shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()
