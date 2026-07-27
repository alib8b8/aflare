// Copyright (c) 2026 llm-box Contributors
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

package distributed

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMessageBus_PublishSubscribe 测试本地发布订阅
func TestMessageBus_PublishSubscribe(t *testing.T) {
	bus := NewMessageBus("node-1", "0")

	ch := bus.Subscribe("test-topic")
	msg := BusMessage{
		Topic:   "test-topic",
		Content: "hello world",
		Type:    "text",
	}
	if err := bus.Publish(msg); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case received := <-ch:
		if received.Topic != "test-topic" {
			t.Errorf("expected topic test-topic, got %s", received.Topic)
		}
		if received.Content != "hello world" {
			t.Errorf("expected content 'hello world', got %s", received.Content)
		}
		if received.From != "node-1" {
			t.Errorf("expected from node-1, got %s", received.From)
		}
		if received.ID == "" {
			t.Error("expected non-empty ID")
		}
		if received.Signature == "" {
			t.Error("expected non-empty signature")
		}
		if received.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
		if !verifyBusMessage(received) {
			t.Error("signature verification failed")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for message")
	}
}

// TestMessageBus_MultipleSubscribers 测试同一主题的多个订阅者都能收到消息
func TestMessageBus_MultipleSubscribers(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	ch1 := bus.Subscribe("topic-a")
	ch2 := bus.Subscribe("topic-a")

	msg := BusMessage{Topic: "topic-a", Content: "broadcast", Type: "text"}
	if err := bus.Publish(msg); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	for i, ch := range []<-chan BusMessage{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Content != "broadcast" {
				t.Errorf("subscriber %d: expected 'broadcast', got %s", i, received.Content)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d timed out waiting for message", i)
		}
	}
}

// TestMessageBus_TopicIsolation 测试不同主题之间消息互不干扰
func TestMessageBus_TopicIsolation(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	chA := bus.Subscribe("topic-a")
	chB := bus.Subscribe("topic-b")

	if err := bus.Publish(BusMessage{Topic: "topic-a", Content: "a", Type: "text"}); err != nil {
		t.Fatalf("Publish topic-a failed: %v", err)
	}
	if err := bus.Publish(BusMessage{Topic: "topic-b", Content: "b", Type: "text"}); err != nil {
		t.Fatalf("Publish topic-b failed: %v", err)
	}

	select {
	case r := <-chA:
		if r.Content != "a" {
			t.Errorf("expected 'a' on topic-a, got %s", r.Content)
		}
	case <-time.After(time.Second):
		t.Error("timed out on topic-a")
	}

	select {
	case r := <-chB:
		if r.Content != "b" {
			t.Errorf("expected 'b' on topic-b, got %s", r.Content)
		}
	case <-time.After(time.Second):
		t.Error("timed out on topic-b")
	}
}

// TestMessageBus_VoteTally 测试投票统计
func TestMessageBus_VoteTally(t *testing.T) {
	bus := NewMessageBus("node-1", "0")

	// 本节点投票（无 peer，Broadcast 退化为本地分发）
	if err := bus.Vote("proposal-1", "yes"); err != nil {
		t.Fatalf("Vote failed: %v", err)
	}

	// 模拟来自其他 peer 的投票（通过 recordVote，模拟 HTTP /api/bus/vote 接收到的投票）
	bus.recordVote("proposal-1", "node-2", "yes")
	bus.recordVote("proposal-1", "node-3", "no")
	bus.recordVote("proposal-1", "node-4", "yes")

	tally := bus.TallyVotes("proposal-1")
	if tally["yes"] != 3 {
		t.Errorf("expected 3 yes votes, got %d", tally["yes"])
	}
	if tally["no"] != 1 {
		t.Errorf("expected 1 no vote, got %d", tally["no"])
	}

	// 验证空主题返回空 map
	empty := bus.TallyVotes("nonexistent")
	if len(empty) != 0 {
		t.Errorf("expected empty tally for unknown topic, got %v", empty)
	}
}

// TestMessageBus_VoteOverwrite 测试同一节点重复投票会覆盖
func TestMessageBus_VoteOverwrite(t *testing.T) {
	bus := NewMessageBus("node-1", "0")

	// 同一节点对同一主题先投 yes 再投 no，应该只算 no 一票
	bus.recordVote("proposal-2", "node-1", "yes")
	bus.recordVote("proposal-2", "node-1", "no")

	tally := bus.TallyVotes("proposal-2")
	if tally["yes"] != 0 {
		t.Errorf("expected 0 yes votes after overwrite, got %d", tally["yes"])
	}
	if tally["no"] != 1 {
		t.Errorf("expected 1 no vote after overwrite, got %d", tally["no"])
	}
}

// TestMessageBus_PeerManagement 测试 peer 管理
func TestMessageBus_PeerManagement(t *testing.T) {
	bus := NewMessageBus("node-1", "0")

	// 初始无 peer
	if peers := bus.ListPeers(); len(peers) != 0 {
		t.Errorf("expected 0 peers initially, got %d", len(peers))
	}

	// 添加两个 peer
	bus.AddPeer("node-2", "localhost:9001")
	bus.AddPeer("node-3", "localhost:9002")

	peers := bus.ListPeers()
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}

	// 验证 peer 信息
	found := make(map[string]bool)
	for _, p := range peers {
		found[p.NodeID] = true
		if p.Status != "online" {
			t.Errorf("expected peer %s to be online, got %s", p.NodeID, p.Status)
		}
		if p.LastSeen.IsZero() {
			t.Errorf("expected non-zero LastSeen for peer %s", p.NodeID)
		}
	}
	if !found["node-2"] || !found["node-3"] {
		t.Errorf("expected to find node-2 and node-3, got %v", found)
	}

	// 移除一个 peer
	bus.RemovePeer("node-2")
	peers = bus.ListPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer after removal, got %d", len(peers))
	}
	if peers[0].NodeID != "node-3" {
		t.Errorf("expected remaining peer node-3, got %s", peers[0].NodeID)
	}

	// 移除不存在的 peer 不应 panic
	bus.RemovePeer("nonexistent")

	// 重复添加会更新地址
	bus.AddPeer("node-3", "localhost:9999")
	peers = bus.ListPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer after re-add, got %d", len(peers))
	}
	if peers[0].Address != "localhost:9999" {
		t.Errorf("expected updated address localhost:9999, got %s", peers[0].Address)
	}
}

// TestMessageBus_SendToSelf 测试点对点发送给自己时退化为本地 Publish
func TestMessageBus_SendToSelf(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	ch := bus.Subscribe("dm-topic")

	msg := BusMessage{Topic: "dm-topic", Content: "self-msg", Type: "text"}
	if err := bus.SendTo("node-1", msg); err != nil {
		t.Fatalf("SendTo self failed: %v", err)
	}

	select {
	case received := <-ch:
		if received.Content != "self-msg" {
			t.Errorf("expected 'self-msg', got %s", received.Content)
		}
		if received.To != "node-1" {
			t.Errorf("expected To 'node-1', got %s", received.To)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for self-sent message")
	}
}

// TestMessageBus_SendToUnknownPeer 测试向未知 peer 发送返回错误
func TestMessageBus_SendToUnknownPeer(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	err := bus.SendTo("unknown-peer", BusMessage{Topic: "x", Type: "text"})
	if err == nil {
		t.Error("expected error when sending to unknown peer")
	}
}

// TestMessageBus_BroadcastNoPeers 测试无 peer 时 Broadcast 仍能本地分发
func TestMessageBus_BroadcastNoPeers(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	ch := bus.Subscribe("bc-topic")

	err := bus.Broadcast(BusMessage{Topic: "bc-topic", Content: "hi", Type: "text"})
	if err != nil {
		t.Errorf("Broadcast with no peers should not return error, got %v", err)
	}

	select {
	case received := <-ch:
		if received.Content != "hi" {
			t.Errorf("expected 'hi', got %s", received.Content)
		}
		if received.To != "" {
			t.Errorf("broadcast message should have empty To, got %s", received.To)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for broadcast message")
	}
}

// TestMessageBus_SignatureRoundTrip 测试消息签名经过 JSON 序列化往返后仍可验证
func TestMessageBus_SignatureRoundTrip(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	ch := bus.Subscribe("sig-topic")

	original := BusMessage{Topic: "sig-topic", Content: "signed", Type: "text"}
	if err := bus.Publish(original); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case received := <-ch:
		// 模拟 JSON 往返：序列化后再反序列化
		data, err := json.Marshal(received)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var roundtripped BusMessage
		if err := json.Unmarshal(data, &roundtripped); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if !verifyBusMessage(roundtripped) {
			t.Error("signature verification failed after JSON round-trip")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for message")
	}
}

// TestMessageBus_SetAuthToken 测试设置 auth token
func TestMessageBus_SetAuthToken(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	bus.SetAuthToken("secret-token")
	// 简单验证设置后能读取（通过内部字段）
	bus.mu.RLock()
	token := bus.authToken
	bus.mu.RUnlock()
	if token != "secret-token" {
		t.Errorf("expected auth token 'secret-token', got %s", token)
	}
}

// TestMessageBus_StopClosesChannels 测试 Stop 关闭订阅 channel
func TestMessageBus_StopClosesChannels(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	ch := bus.Subscribe("stop-topic")

	if err := bus.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// channel 应该被关闭，读取应立即返回零值
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after Stop")
		}
	default:
		t.Error("expected channel read to return immediately after Stop")
	}

	// 重复 Stop 应该是幂等的
	if err := bus.Stop(); err != nil {
		t.Errorf("repeated Stop should be idempotent, got %v", err)
	}
}

// TestMessageBus_BusMessageMaxSize 测试消息大小常量
func TestMessageBus_BusMessageMaxSize(t *testing.T) {
	if busMessageMaxSize != 1*1024*1024 {
		t.Errorf("expected busMessageMaxSize to be 1MB, got %d", busMessageMaxSize)
	}
}

// newTestPeerServer 创建一个模拟 peer 的 httptest.Server,记录收到的消息。
// 返回 server 和获取已收消息列表的快照函数(线程安全)。
func newTestPeerServer(t *testing.T) (*httptest.Server, func() []BusMessage) {
	t.Helper()
	var mu sync.Mutex
	var received []BusMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var msg BusMessage
		if err := json.NewDecoder(io.LimitReader(r.Body, busMessageMaxSize)).Decode(&msg); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	t.Cleanup(srv.Close)
	return srv, func() []BusMessage {
		mu.Lock()
		defer mu.Unlock()
		out := make([]BusMessage, len(received))
		copy(out, received)
		return out
	}
}

// peerAddressFromURL 从 httptest.Server.URL("http://host:port")提取 "host:port"。
func peerAddressFromURL(url string) string {
	return strings.TrimPrefix(url, "http://")
}

// TestBusHopLimit 测试跳数限制:设置 hop_limit=1,Broadcast 后只有本地订阅者收到,peer 不应收到。
func TestBusHopLimit(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	bus.SetHopLimit(1)

	ch := bus.Subscribe("hop-topic")
	peerSrv, peerReceived := newTestPeerServer(t)
	bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

	msg := BusMessage{Topic: "hop-topic", Content: "local-only", Type: "text"}
	if err := bus.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// 本地订阅者应收到
	select {
	case received := <-ch:
		if received.Content != "local-only" {
			t.Errorf("expected 'local-only', got %s", received.Content)
		}
		if received.HopLimit != 1 {
			t.Errorf("expected HopLimit 1, got %d", received.HopLimit)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for local message")
	}

	// peer 不应收到(hop_limit=1 时只本地分发,不网络转发)
	if got := peerReceived(); len(got) != 0 {
		t.Errorf("expected 0 messages at peer with hop_limit=1, got %d", len(got))
	}
}

// TestBusHopLimitForward 测试转发时跳数递减:hop_limit=2 的消息被 peer 收到时,HopLimit 应为 1。
func TestBusHopLimitForward(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	bus.SetHopLimit(2)

	bus.Subscribe("hop-fwd-topic")
	peerSrv, peerReceived := newTestPeerServer(t)
	bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

	msg := BusMessage{Topic: "hop-fwd-topic", Content: "forward-me", Type: "text"}
	if err := bus.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// peer 应收到 1 条消息,HopLimit 已递减为 1
	got := peerReceived()
	if len(got) != 1 {
		t.Fatalf("expected 1 message at peer, got %d", len(got))
	}
	if got[0].HopLimit != 1 {
		t.Errorf("expected HopLimit 1 at peer, got %d", got[0].HopLimit)
	}
	if got[0].Content != "forward-me" {
		t.Errorf("expected 'forward-me', got %s", got[0].Content)
	}
}

// TestBusBackpressure 测试背压:设置 maxPendingMsgs=2,Publish 3 条后第 4 条应返回 ErrBackpressure。
func TestBusBackpressure(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	bus.SetMaxPending(2)

	// 订阅但不消费,让消息堆积在 channel 中
	bus.Subscribe("bp-topic")

	// 发布 3 条,都应成功(背压检查在发布前:pending 0→1→2→3,均未超过阈值 2)
	for i := 0; i < 3; i++ {
		msg := BusMessage{Topic: "bp-topic", Content: "msg", Type: "text"}
		if err := bus.Publish(msg); err != nil {
			t.Fatalf("Publish %d should succeed, got %v", i+1, err)
		}
	}

	// 第 4 条应返回 ErrBackpressure(pending=3 > maxPendingMsgs=2)
	err := bus.Publish(BusMessage{Topic: "bp-topic", Content: "msg", Type: "text"})
	if !errors.Is(err, ErrBackpressure) {
		t.Errorf("expected ErrBackpressure on 4th publish, got %v", err)
	}
}

// TestBusHopLimitZero 测试 hop_limit=0 不限制:默认行为下退化为无限制转发。
func TestBusHopLimitZero(t *testing.T) {
	bus := NewMessageBus("node-1", "0")
	bus.SetHopLimit(0)

	bus.Subscribe("hop-zero-topic")
	peerSrv, peerReceived := newTestPeerServer(t)
	bus.AddPeer("peer-1", peerAddressFromURL(peerSrv.URL))

	msg := BusMessage{Topic: "hop-zero-topic", Content: "unlimited", Type: "text"}
	if err := bus.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// peer 应收到消息,HopLimit 仍为 0(无限制,向后兼容)
	got := peerReceived()
	if len(got) != 1 {
		t.Fatalf("expected 1 message at peer, got %d", len(got))
	}
	if got[0].HopLimit != 0 {
		t.Errorf("expected HopLimit 0 at peer, got %d", got[0].HopLimit)
	}
	if got[0].Content != "unlimited" {
		t.Errorf("expected 'unlimited', got %s", got[0].Content)
	}
}
