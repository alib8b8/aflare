#!/usr/bin/env python3
"""Mock bank API for the idempotent-transfer demo.

模拟一个支持 Idempotency-Key 去重的银行转账 API：
  - POST /transfer  处理转账请求
  - GET  /health    健康检查

行为：
  - 同一 Idempotency-Key 的重复请求返回首次结果（服务端去重）
  - amount > 10000 时返回失败（演示失败审计路径）
  - 约 10% 概率返回 503（演示 HTTP 重试）
  - 无外部依赖，仅用标准库

运行：python3 mock-server.py
默认监听 :17800
"""

import json
import random
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

LISTEN_ADDR = ("0.0.0.0", 17800)

# 服务端幂等缓存：Idempotency-Key -> 首次响应
# 真实银行会用数据库 + 唯一索引实现，这里用内存 map 演示
_idem_store = {}
_idem_lock = threading.Lock()


class BankHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        # 简洁日志，避免默认的 stderr 噪音格式
        print(f"[mock-bank] {self.address_string()} - {fmt % args}")

    def _send_json(self, status_code, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self._send_json(200, {"status": "ok"})
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/transfer":
            self._send_json(404, {"error": "not found"})
            return

        # 读取请求体
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            req = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            self._send_json(400, {"status": "failed", "error": "invalid JSON body"})
            return

        idem_key = self.headers.get("Idempotency-Key", "")

        # 服务端去重：同一 Idempotency-Key 返回首次结果
        if idem_key:
            with _idem_lock:
                if idem_key in _idem_store:
                    cached = _idem_store[idem_key]
                    print(f"[mock-bank] idempotency hit key={idem_key} -> cached")
                    self._send_json(cached["code"], cached["body"])
                    return

        # 模拟瞬时故障（约 10% 概率返回 503，触发客户端重试）
        if random.random() < 0.1:
            print(f"[mock-bank] transient 503 (retryable) key={idem_key}")
            self._send_json(503, {"status": "failed", "error": "transient bank outage"})
            return

        # 业务逻辑：金额超过 10000 返回失败（演示失败审计路径）
        amount = float(req.get("amount", 0))
        if amount > 10000:
            resp_code, resp_body = 200, {
                "status": "failed",
                "error": "amount exceeds single-transfer limit (10000)",
                "transaction_id": "",
            }
        else:
            resp_code, resp_body = 200, {
                "status": "success",
                "transaction_id": f"TXN-{random.randint(100000, 999999)}",
                "amount": amount,
                "currency": req.get("currency", "CNY"),
            }

        # 缓存首次响应（服务端去重）
        if idem_key:
            with _idem_lock:
                _idem_store[idem_key] = {"code": resp_code, "body": resp_body}

        print(f"[mock-bank] transfer {req.get('from')}->{req.get('to')} "
              f"amount={amount} status={resp_body['status']} key={idem_key}")
        self._send_json(resp_code, resp_body)


def main():
    server = HTTPServer(LISTEN_ADDR, BankHandler)
    print(f"[mock-bank] listening on http://{LISTEN_ADDR[0]}:{LISTEN_ADDR[1]}")
    print("[mock-bank] endpoints: POST /transfer, GET /health")
    print("[mock-bank] idempotency: cached by Idempotency-Key header")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock-bank] shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()
