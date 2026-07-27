// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package distributed

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── handleHealth ───

func TestMessageBus_HandleHealth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		method   string
		wantCode int
	}{
		{"WrongMethod", http.MethodPost, http.StatusMethodNotAllowed},
		{"GetOK", http.MethodGet, http.StatusOK},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bus := NewMessageBus("health-node", "0")
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()
			bus.handleHealth(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}
			if tt.wantCode == http.StatusOK {
				var resp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["status"] != "ok" {
					t.Errorf("expected status ok, got %s", resp["status"])
				}
				if resp["node_id"] != "health-node" {
					t.Errorf("expected node_id health-node, got %s", resp["node_id"])
				}
			}
		})
	}
}

// ─── handleListPeers ───

func TestMessageBus_HandleListPeers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		method    string
		addPeers  bool
		wantCode  int
		wantCount int
	}{
		{"WrongMethod", http.MethodPost, false, http.StatusMethodNotAllowed, 0},
		{"EmptyPeers", http.MethodGet, false, http.StatusOK, 0},
		{"WithPeers", http.MethodGet, true, http.StatusOK, 2},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bus := NewMessageBus("list-node", "0")
			if tt.addPeers {
				bus.AddPeer("p1", "127.0.0.1:9001")
				bus.AddPeer("p2", "127.0.0.1:9002")
			}
			req := httptest.NewRequest(tt.method, "/api/bus/peers", nil)
			rec := httptest.NewRecorder()
			bus.handleListPeers(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}
			if tt.wantCode == http.StatusOK {
				var resp struct {
					Peers []PeerInfo `json:"peers"`
					Count int        `json:"count"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Count != tt.wantCount {
					t.Errorf("expected count %d, got %d", tt.wantCount, resp.Count)
				}
				if len(resp.Peers) != tt.wantCount {
					t.Errorf("expected %d peers, got %d", tt.wantCount, len(resp.Peers))
				}
			}
		})
	}
}

// ─── handleRegisterPeer ───

func TestMessageBus_HandleRegisterPeer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		method   string
		body     string
		wantCode int
	}{
		{"WrongMethod", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"InvalidJSON", http.MethodPost, "not-json", http.StatusBadRequest},
		{"MissingNodeID", http.MethodPost, `{"node_id":"","address":"127.0.0.1:9001"}`, http.StatusBadRequest},
		{"MissingAddress", http.MethodPost, `{"node_id":"p1","address":""}`, http.StatusBadRequest},
		{"Valid", http.MethodPost, `{"node_id":"new-peer","address":"127.0.0.1:9005"}`, http.StatusCreated},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bus := NewMessageBus("reg-node", "0")
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/api/bus/peer", body)
			rec := httptest.NewRecorder()
			bus.handleRegisterPeer(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d (body: %s)", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantCode == http.StatusCreated {
				// 验证 peer 确实被添加
				peers := bus.ListPeers()
				if len(peers) != 1 {
					t.Errorf("expected 1 peer after register, got %d", len(peers))
				}
				var resp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["status"] != "ok" {
					t.Errorf("expected status ok, got %s", resp["status"])
				}
			}
		})
	}
}

// ─── handleVote ───

func TestMessageBus_HandleVote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		method   string
		body     string
		wantCode int
	}{
		{"WrongMethod", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"InvalidJSON", http.MethodPost, "not-json", http.StatusBadRequest},
		{"MissingTopic", http.MethodPost, `{"topic":"","choice":"yes","node_id":"n1"}`, http.StatusBadRequest},
		{"MissingNodeID", http.MethodPost, `{"topic":"t1","choice":"yes","node_id":""}`, http.StatusBadRequest},
		{"Valid", http.MethodPost, `{"topic":"vote-topic","choice":"yes","node_id":"voter-1"}`, http.StatusOK},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bus := NewMessageBus("vote-node", "0")
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/api/bus/vote", body)
			rec := httptest.NewRecorder()
			bus.handleVote(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d (body: %s)", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantCode == http.StatusOK {
				tally := bus.TallyVotes("vote-topic")
				if tally["yes"] != 1 {
					t.Errorf("expected 1 yes vote recorded, got %d", tally["yes"])
				}
				var resp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["status"] != "ok" {
					t.Errorf("expected status ok, got %s", resp["status"])
				}
			}
		})
	}
}

// ─── handleMessage ───

func TestMessageBus_HandleMessage(t *testing.T) {
	t.Parallel()
	t.Run("WrongMethod", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		req := httptest.NewRequest(http.MethodGet, "/api/bus/message", nil)
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", strings.NewReader("not-json"))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		// 提供错误的签名
		body := `{"topic":"t","content":"c","type":"text","from":"peer-1","signature":"bad-sig"}`
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", strings.NewReader(body))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d for invalid signature, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("ValidMessageDeliversLocally", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		ch := bus.Subscribe("http-topic")
		msg := BusMessage{
			Topic:   "http-topic",
			Content: "from-http",
			Type:    "text",
			From:    "remote-peer",
		}
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", bytes.NewReader(data))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
		}
		select {
		case received := <-ch:
			if received.Content != "from-http" {
				t.Errorf("expected 'from-http', got %s", received.Content)
			}
		case <-time.After(time.Second):
			t.Error("timed out waiting for delivered message")
		}
	})

	t.Run("ValidSignatureAccepted", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		bus.Subscribe("sig-topic")
		// 构造带正确签名的消息
		msg := BusMessage{
			Topic:     "sig-topic",
			Content:   "signed",
			Type:      "text",
			From:      "remote-peer",
			Timestamp: time.Now(),
		}
		msg.Signature = signBusMessage(msg)
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", bytes.NewReader(data))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d for valid signature, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("UpdatesPeerLastSeen", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("msg-node", "0")
		// 预先注册一个 peer,设置较早的 LastSeen
		bus.AddPeer("remote-peer", "127.0.0.1:9999")
		// 强制将 LastSeen 调到很久以前
		bus.mu.Lock()
		bus.peerSeen["remote-peer"] = time.Now().Add(-1 * time.Hour)
		oldSeen := bus.peerSeen["remote-peer"]
		bus.mu.Unlock()

		msg := BusMessage{
			Topic:   "seen-topic",
			Content: "hi",
			Type:    "text",
			From:    "remote-peer",
		}
		data, _ := json.Marshal(msg)
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", bytes.NewReader(data))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		bus.mu.RLock()
		newSeen := bus.peerSeen["remote-peer"]
		bus.mu.RUnlock()
		if !newSeen.After(oldSeen) {
			t.Error("expected LastSeen to be updated after receiving message from peer")
		}
	})
}

// ─── forwardReceivedMessage ───

func TestMessageBus_ForwardReceivedMessage(t *testing.T) {
	t.Parallel()

	t.Run("SelfMessageNotForwarded", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("self-node", "0")
		peerSrv, peerReceived := newTestPeerServer(t)
		bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

		// From == self.nodeID:不应转发
		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "self",
			From:     "self-node",
			HopLimit: 5,
		})
		// 给 HTTP 一点时间
		time.Sleep(50 * time.Millisecond)
		if got := peerReceived(); len(got) != 0 {
			t.Errorf("expected 0 forwarded messages for self-message, got %d", len(got))
		}
	})

	t.Run("HopLimitOneNotForwarded", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("fwd-node", "0")
		peerSrv, peerReceived := newTestPeerServer(t)
		bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "hop1",
			From:     "remote",
			HopLimit: 1,
		})
		time.Sleep(50 * time.Millisecond)
		if got := peerReceived(); len(got) != 0 {
			t.Errorf("expected 0 forwarded messages with HopLimit=1, got %d", len(got))
		}
	})

	t.Run("HopLimitGtOneForwardedWithDecrement", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("fwd-node", "0")
		peerSrv, peerReceived := newTestPeerServer(t)
		bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "fwd",
			From:     "remote",
			HopLimit: 3,
		})
		// 等待 HTTP 转发完成
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if len(peerReceived()) > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		got := peerReceived()
		if len(got) != 1 {
			t.Fatalf("expected 1 forwarded message, got %d", len(got))
		}
		if got[0].HopLimit != 2 {
			t.Errorf("expected HopLimit 2 after decrement, got %d", got[0].HopLimit)
		}
	})

	t.Run("HopLimitZeroUnlimitedForward", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("fwd-node", "0")
		peerSrv, peerReceived := newTestPeerServer(t)
		bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "unlimited",
			From:     "remote",
			HopLimit: 0,
		})
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if len(peerReceived()) > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		got := peerReceived()
		if len(got) != 1 {
			t.Fatalf("expected 1 forwarded message, got %d", len(got))
		}
		if got[0].HopLimit != 0 {
			t.Errorf("expected HopLimit 0 (unlimited), got %d", got[0].HopLimit)
		}
	})

	t.Run("NoPeersNoop", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("fwd-node", "0")
		// 无 peer,不应 panic 也不应出错
		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "no-peers",
			From:     "remote",
			HopLimit: 5,
		})
	})

	t.Run("SelfPeerSkipped", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("self-node", "0")
		peerSrv, peerReceived := newTestPeerServer(t)
		// 将自己也加为 peer(地址指向 peerSrv 也没关系,因为应该被跳过)
		bus.AddPeer("self-node", peerAddressFromURL(peerSrv.URL))
		bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "skip-self",
			From:     "remote",
			HopLimit: 5,
		})
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if len(peerReceived()) > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		got := peerReceived()
		// peer-1 应收到 1 条;self-node 应被跳过
		if len(got) != 1 {
			t.Errorf("expected 1 forwarded message (self skipped), got %d", len(got))
		}
	})

	t.Run("ForwardFailureDoesNotPanic", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("fwd-node", "0")
		// 指向一个不可达的地址,触发 postMessage 失败
		bus.AddPeer("dead-peer", "127.0.0.1:1")
		// 不应 panic
		bus.forwardReceivedMessage(BusMessage{
			Topic:    "t",
			Content:  "dead",
			From:     "remote",
			HopLimit: 5,
		})
	})
}

// ─── authMiddleware ───

func TestMessageBus_AuthMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("NoTokenAllowsAll", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("auth-node", "0")
		called := false
		h := bus.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/bus/peers", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if !called {
			t.Error("expected handler to be called when no auth token set")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("ValidTokenAccepted", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("auth-node", "0")
		bus.SetAuthToken("secret")
		called := false
		h := bus.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/bus/peers", nil)
		req.Header.Set("X-Auth-Token", "secret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if !called {
			t.Error("expected handler to be called with valid token")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("InvalidTokenRejected", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("auth-node", "0")
		bus.SetAuthToken("secret")
		called := false
		h := bus.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/bus/peers", nil)
		req.Header.Set("X-Auth-Token", "wrong-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if called {
			t.Error("expected handler NOT to be called with invalid token")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("MissingTokenRejected", func(t *testing.T) {
		t.Parallel()
		bus := NewMessageBus("auth-node", "0")
		bus.SetAuthToken("secret")
		called := false
		h := bus.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/bus/peers", nil)
		// 不设置 X-Auth-Token
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if called {
			t.Error("expected handler NOT to be called with missing token")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("HealthEndpointBypassesAuth", func(t *testing.T) {
		t.Parallel()
		// /health 端点不经过 authMiddleware,即使设置了 token 也应可访问
		bus := NewMessageBus("auth-node", "0")
		bus.SetAuthToken("secret")
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		// 不带 token
		rec := httptest.NewRecorder()
		bus.handleHealth(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected /health to bypass auth and return 200, got %d", rec.Code)
		}
	})
}

// ─── Start (集成测试:启动真实 HTTP 服务器) ───

func TestMessageBus_StartAndServe(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("start-node", freeTestPort(t))

	// 启动服务器(在 goroutine 中,因为 ListenAndServe 阻塞)
	errCh := make(chan error, 1)
	go func() {
		errCh <- bus.Start()
	}()
	t.Cleanup(func() {
		_ = bus.Stop()
	})

	// 等待服务器就绪
	waitForHTTPReady(t, "http://127.0.0.1:"+bus.port+"/health", 2*time.Second)

	// 测试 /health 端点
	resp, err := http.Get("http://127.0.0.1:" + bus.port + "/health")
	if err != nil {
		t.Fatalf("failed to GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /health, got %d", resp.StatusCode)
	}
	var healthResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if healthResp["node_id"] != "start-node" {
		t.Errorf("expected node_id start-node, got %s", healthResp["node_id"])
	}

	// 测试 /api/bus/peers 端点
	resp2, err := http.Get("http://127.0.0.1:" + bus.port + "/api/bus/peers")
	if err != nil {
		t.Fatalf("failed to GET /api/bus/peers: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /api/bus/peers, got %d", resp2.StatusCode)
	}

	// 测试 /api/bus/peer 注册端点
	peerBody := strings.NewReader(`{"node_id":"new-peer","address":"127.0.0.1:9999"}`)
	resp3, err := http.Post("http://127.0.0.1:"+bus.port+"/api/bus/peer", "application/json", peerBody)
	if err != nil {
		t.Fatalf("failed to POST /api/bus/peer: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 for /api/bus/peer, got %d", resp3.StatusCode)
	}
	// 验证 peer 已注册
	if peers := bus.ListPeers(); len(peers) != 1 {
		t.Errorf("expected 1 peer after register, got %d", len(peers))
	}

	// 测试 /api/bus/message 端点(端到端:HTTP -> 本地订阅者)
	ch := bus.Subscribe("start-topic")
	msg := BusMessage{Topic: "start-topic", Content: "via-http", Type: "text"}
	msgData, _ := json.Marshal(msg)
	resp4, err := http.Post("http://127.0.0.1:"+bus.port+"/api/bus/message", "application/json", bytes.NewReader(msgData))
	if err != nil {
		t.Fatalf("failed to POST /api/bus/message: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /api/bus/message, got %d", resp4.StatusCode)
	}
	select {
	case received := <-ch:
		if received.Content != "via-http" {
			t.Errorf("expected 'via-http', got %s", received.Content)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for message via HTTP")
	}
}

func TestMessageBus_StartWithAuth(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("auth-start-node", freeTestPort(t))
	bus.SetAuthToken("secret-token")

	errCh := make(chan error, 1)
	go func() {
		errCh <- bus.Start()
	}()
	t.Cleanup(func() {
		_ = bus.Stop()
	})
	waitForHTTPReady(t, "http://127.0.0.1:"+bus.port+"/health", 2*time.Second)

	// /health 无需鉴权(应可访问)
	resp, err := http.Get("http://127.0.0.1:" + bus.port + "/health")
	if err != nil {
		t.Fatalf("failed to GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /health without token, got %d", resp.StatusCode)
	}

	// /api/bus/peers 无 token 应被拒绝
	resp2, err := http.Get("http://127.0.0.1:" + bus.port + "/api/bus/peers")
	if err != nil {
		t.Fatalf("failed to GET /api/bus/peers: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp2.StatusCode)
	}

	// /api/bus/peers 带正确 token 应通过
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+bus.port+"/api/bus/peers", nil)
	req.Header.Set("X-Auth-Token", "secret-token")
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to GET /api/bus/peers with token: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", resp3.StatusCode)
	}
}

// ─── Stop 幂等性与无服务器场景 ───

func TestMessageBus_StopWithoutStart(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("stop-node", "0")
	// 从未 Start 过 httpServer,直接 Stop 不应 panic
	if err := bus.Stop(); err != nil {
		t.Errorf("Stop without Start should not error, got %v", err)
	}
}

func TestMessageBus_StopConcurrentIdempotent(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("stop-node", "0")
	bus.Subscribe("topic-1")
	bus.Subscribe("topic-2")

	// 并发 Stop 多次,所有调用都应成功且不 panic
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = bus.Stop()
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Stop returned error: %v", i, err)
		}
	}
}

// ─── postMessage 错误路径 ───

func TestMessageBus_PostMessageFailure(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("post-node", "0")
	// 指向一个不可达地址(端口 1 通常被拒绝或连接失败)
	err := bus.postMessage("127.0.0.1:1", []byte(`{}`))
	if err == nil {
		t.Error("expected error when posting to unreachable address")
	}
}

func TestMessageBus_PostMessageWithAuth(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("post-node", "0")
	bus.SetAuthToken("secret")
	// 启动一个 mock peer,验证请求带上了 auth token
	var gotToken string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotToken = r.Header.Get("X-Auth-Token")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := bus.postMessage(peerAddressFromURL(srv.URL), []byte(`{}`))
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotToken != "secret" {
		t.Errorf("expected auth token 'secret' in request header, got %q", gotToken)
	}
}

func TestMessageBus_PostMessagePeerError(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("post-node", "0")
	// mock peer 返回 4xx 错误
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()
	err := bus.postMessage(peerAddressFromURL(srv.URL), []byte(`{}`))
	if err == nil {
		t.Error("expected error when peer returns 4xx")
	}
}

// ─── Broadcast 错误路径 ───

func TestMessageBus_BroadcastPeerErrorContinues(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("bc-node", "0")
	bus.SetHopLimit(3)
	// 一个不可达 peer + 一个可达 peer
	bus.AddPeer("dead-peer", "127.0.0.1:1")
	peerSrv, peerReceived := newTestPeerServer(t)
	bus.AddPeer("live-peer", peerAddressFromURL(peerSrv.URL))

	msg := BusMessage{Topic: "bc-err-topic", Content: "broadcast", Type: "text"}
	err := bus.Broadcast(msg)
	// 应返回 lastErr(不可达 peer 的错误),但 live peer 仍应收到
	if err == nil {
		t.Error("expected error from dead peer")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(peerReceived()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := peerReceived()
	if len(got) != 1 {
		t.Errorf("expected live peer to still receive message despite dead peer error, got %d", len(got))
	}
}

// ─── handleMessage 背压触发 500 ───

func TestMessageBus_HandleMessageBackpressure(t *testing.T) {
	t.Parallel()
	bus := NewMessageBus("bp-node", "0")
	bus.SetMaxPending(1)
	// 订阅但不消费,让消息堆积
	bus.Subscribe("bp-topic")

	// 背压检查在发布前:pending > maxPendingMsgs 才拒绝。
	// maxPending=1 时:第 1 条(pending 0->1,0>1 false)和第 2 条(pending 1->2,1>1 false)都成功,
	// 第 3 条(pending=2,2>1 true)触发背压返回 500。
	for i := 0; i < 2; i++ {
		msg := BusMessage{Topic: "bp-topic", Content: "msg", Type: "text"}
		data, _ := json.Marshal(msg)
		req := httptest.NewRequest(http.MethodPost, "/api/bus/message", bytes.NewReader(data))
		rec := httptest.NewRecorder()
		bus.handleMessage(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected message %d to succeed, got %d", i+1, rec.Code)
		}
	}

	// 第三条应触发背压(pending=2 > maxPending=1),返回 500
	thirdMsg := BusMessage{Topic: "bp-topic", Content: "msg-3", Type: "text"}
	data3, _ := json.Marshal(thirdMsg)
	req3 := httptest.NewRequest(http.MethodPost, "/api/bus/message", bytes.NewReader(data3))
	rec3 := httptest.NewRecorder()
	bus.handleMessage(rec3, req3)
	if rec3.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 due to backpressure, got %d", rec3.Code)
	}
}
