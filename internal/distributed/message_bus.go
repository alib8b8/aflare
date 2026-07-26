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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
)

// message_bus.go — 蜂群通信消息总线
//
// 为多个 llm-box 实例 / Agent 提供轻量级进程间 / 节点间消息总线。
// 基于 HTTP 实现，复用 distributed.go 中的 safeHTTPClient（仅允许 loopback 与私有网络）。
// 支持主题订阅、点对点发送、广播、群体投票共识。
//
// 拓扑：全连接 mesh（每个节点显式 AddPeer 所有其他节点）。
// 消息流向：
//
//	Publish      -> 本地订阅者
//	Broadcast    -> 本地订阅者 + 所有 peer（HTTP）
//	SendTo       -> 指定 peer（HTTP），或本地（peer==self）

// 消息总线相关常量
const (
	busMessageMaxSize = 1 * 1024 * 1024 // 1MB 消息大小限制
	busSubscribeBuf   = 64              // 订阅者 channel 缓冲大小
)

// BusMessage 消息总线中传递的消息
type BusMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`        // 空表示广播
	Topic     string    `json:"topic"`     // 频道/主题
	Content   string    `json:"content"`
	Type      string    `json:"type"`      // text|task|result|status|vote
	Timestamp time.Time `json:"timestamp"`
	Signature string    `json:"signature"`
}

// PeerInfo 节点信息
type PeerInfo struct {
	NodeID   string    `json:"node_id"`
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"` // online|offline
}

// MessageBus 蜂群通信消息总线
type MessageBus struct {
	nodeID      string
	port        string
	authToken   string                        // 可选的 auth token
	subscribers map[string][]chan BusMessage  // topic -> subscribers
	mu          sync.RWMutex
	httpServer  *http.Server
	peers       map[string]string            // nodeID -> address
	peerSeen    map[string]time.Time         // nodeID -> 最后见到的时间
	votes       map[string]map[string]string // topic -> (nodeID -> choice)
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// NewMessageBus 创建消息总线
func NewMessageBus(nodeID, port string) *MessageBus {
	return &MessageBus{
		nodeID:      nodeID,
		port:        port,
		subscribers: make(map[string][]chan BusMessage),
		peers:       make(map[string]string),
		peerSeen:    make(map[string]time.Time),
		votes:       make(map[string]map[string]string),
		stopCh:      make(chan struct{}),
	}
}

// SetAuthToken 设置可选的 auth token，用于 HTTP 端点鉴权。
// 设置后所有 /api/bus/* 端点（除 /health）都需要 X-Auth-Token 头。
func (b *MessageBus) SetAuthToken(token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.authToken = token
}

// Start 启动 HTTP 服务器，监听消息
func (b *MessageBus) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/bus/message", b.authMiddleware(b.handleMessage))
	mux.HandleFunc("/api/bus/vote", b.authMiddleware(b.handleVote))
	mux.HandleFunc("/api/bus/peers", b.authMiddleware(b.handleListPeers))
	mux.HandleFunc("/api/bus/peer", b.authMiddleware(b.handleRegisterPeer))
	mux.HandleFunc("/health", b.handleHealth)

	b.httpServer = &http.Server{
		Addr:    ":" + b.port,
		Handler: mux,
	}

	logger.Info("MessageBus started", "node_id", b.nodeID, "port", b.port)
	return b.httpServer.ListenAndServe()
}

// Stop 停止消息总线
func (b *MessageBus) Stop() error {
	var err error
	b.stopOnce.Do(func() {
		close(b.stopCh)
		if b.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = b.httpServer.Shutdown(ctx)
		}
		// 关闭所有订阅 channel，让接收方知道总线已停止
		b.mu.Lock()
		for topic, subs := range b.subscribers {
			for _, ch := range subs {
				close(ch)
			}
			delete(b.subscribers, topic)
		}
		b.mu.Unlock()
	})
	return err
}

// Subscribe 订阅主题，返回只读 channel。
// 当总线 Stop 时，channel 会被关闭。
func (b *MessageBus) Subscribe(topic string) <-chan BusMessage {
	ch := make(chan BusMessage, busSubscribeBuf)
	b.mu.Lock()
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	b.mu.Unlock()
	return ch
}

// Publish 本地发布：将消息分发给本地所有订阅该主题的订阅者。
// 不会通过网络发送。补全 ID/From/Timestamp/Signature（如为空）。
func (b *MessageBus) Publish(msg BusMessage) error {
	if msg.ID == "" {
		msg.ID = b.generateMessageID()
	}
	if msg.From == "" {
		msg.From = b.nodeID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Signature == "" {
		msg.Signature = signBusMessage(msg)
	}

	// 持有读锁覆盖整个分发过程，避免与 Stop（写锁）并发导致向已关闭 channel 发送
	b.mu.RLock()
	defer b.mu.RUnlock()
	subs := b.subscribers[msg.Topic]
	for _, ch := range subs {
		// 非阻塞发送：缓冲区满则丢弃并记录告警，避免慢消费者阻塞发布者
		select {
		case ch <- msg:
		default:
			logger.Warn("MessageBus subscriber channel full, dropping message",
				"topic", msg.Topic, "id", msg.ID)
		}
	}
	return nil
}

// Broadcast 广播消息到所有 peer（并本地分发）。
// msg.To 会被强制清空以表示广播。
func (b *MessageBus) Broadcast(msg BusMessage) error {
	if msg.ID == "" {
		msg.ID = b.generateMessageID()
	}
	if msg.From == "" {
		msg.From = b.nodeID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Signature == "" {
		msg.Signature = signBusMessage(msg)
	}
	msg.To = "" // 广播

	// 本地分发
	if err := b.Publish(msg); err != nil {
		return err
	}

	// 网络分发到所有 peer
	b.mu.RLock()
	peers := make(map[string]string, len(b.peers))
	for k, v := range b.peers {
		peers[k] = v
	}
	b.mu.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	var lastErr error
	for peerID, addr := range peers {
		if peerID == b.nodeID {
			continue
		}
		if err := b.postMessage(addr, data); err != nil {
			logger.Warn("MessageBus broadcast to peer failed",
				"peer", peerID, "address", addr, "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// SendTo 点对点发送消息给指定 peer。
// peerID == self 时退化为本地 Publish。
func (b *MessageBus) SendTo(peerID string, msg BusMessage) error {
	if msg.ID == "" {
		msg.ID = b.generateMessageID()
	}
	if msg.From == "" {
		msg.From = b.nodeID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Signature == "" {
		msg.Signature = signBusMessage(msg)
	}
	msg.To = peerID

	if peerID == b.nodeID {
		return b.Publish(msg)
	}

	b.mu.RLock()
	addr, ok := b.peers[peerID]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("peer not found: %s", peerID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return b.postMessage(addr, data)
}

// AddPeer 添加 peer
func (b *MessageBus) AddPeer(nodeID, address string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.peers[nodeID] = address
	b.peerSeen[nodeID] = time.Now()
}

// RemovePeer 移除 peer
func (b *MessageBus) RemovePeer(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.peers, nodeID)
	delete(b.peerSeen, nodeID)
}

// ListPeers 列出所有 peer
func (b *MessageBus) ListPeers() []PeerInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]PeerInfo, 0, len(b.peers))
	for id, addr := range b.peers {
		seen, ok := b.peerSeen[id]
		status := "online"
		if !ok {
			status = "offline"
		}
		out = append(out, PeerInfo{
			NodeID:   id,
			Address:  addr,
			LastSeen: seen,
			Status:   status,
		})
	}
	return out
}

// Vote 投票：记录本地节点的投票，并广播给所有 peer。
// 同一 topic 下同一 nodeID 多次投票会覆盖。
func (b *MessageBus) Vote(topic string, choice string) error {
	b.mu.Lock()
	if b.votes[topic] == nil {
		b.votes[topic] = make(map[string]string)
	}
	b.votes[topic][b.nodeID] = choice
	b.mu.Unlock()

	// 广播投票给所有 peer（通过 BusMessage，Type=vote）
	msg := BusMessage{
		Topic:     topic,
		Type:      "vote",
		Content:   choice,
		Timestamp: time.Now(),
	}
	return b.Broadcast(msg)
}

// TallyVotes 统计指定主题的投票结果，返回 choice -> 票数
func (b *MessageBus) TallyVotes(topic string) map[string]int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[string]int)
	for _, choice := range b.votes[topic] {
		out[choice]++
	}
	return out
}

// recordVote 记录来自 peer 的投票
func (b *MessageBus) recordVote(topic, nodeID, choice string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.votes[topic] == nil {
		b.votes[topic] = make(map[string]string)
	}
	b.votes[topic][nodeID] = choice
}

// ------------------------------------------------------------
// HTTP 端点
// ------------------------------------------------------------

// authMiddleware 鉴权中间件。authToken 为空时跳过鉴权（开放模式）。
func (b *MessageBus) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b.mu.RLock()
		token := b.authToken
		b.mu.RUnlock()
		if token == "" {
			next(w, r)
			return
		}
		provided := r.Header.Get("X-Auth-Token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleMessage POST /api/bus/message 接收消息并本地分发
func (b *MessageBus) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var msg BusMessage
	if err := json.NewDecoder(io.LimitReader(r.Body, busMessageMaxSize)).Decode(&msg); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// 验证签名（如果提供）
	if msg.Signature != "" {
		if !verifyBusMessage(msg) {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
	}
	// 更新已知 peer 的 LastSeen
	if msg.From != "" && msg.From != b.nodeID {
		b.mu.Lock()
		if _, ok := b.peers[msg.From]; ok {
			b.peerSeen[msg.From] = time.Now()
		}
		b.mu.Unlock()
	}
	// 本地分发
	if err := b.Publish(msg); err != nil {
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleVote POST /api/bus/vote 接收投票
func (b *MessageBus) handleVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Topic  string `json:"topic"`
		Choice string `json:"choice"`
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, busMessageMaxSize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Topic == "" || req.NodeID == "" {
		http.Error(w, "missing topic or node_id", http.StatusBadRequest)
		return
	}
	b.recordVote(req.Topic, req.NodeID, req.Choice)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleListPeers GET /api/bus/peers 列出 peer
func (b *MessageBus) handleListPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peers := b.ListPeers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleRegisterPeer POST /api/bus/peer 注册 peer
func (b *MessageBus) handleRegisterPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, busMessageMaxSize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.NodeID == "" || req.Address == "" {
		http.Error(w, "missing node_id or address", http.StatusBadRequest)
		return
	}
	b.AddPeer(req.NodeID, req.Address)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleHealth GET /health 健康检查（无需鉴权）
func (b *MessageBus) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"node_id": b.nodeID,
	})
}

// ------------------------------------------------------------
// 内部辅助方法
// ------------------------------------------------------------

// postMessage 通过 HTTP 向指定地址发送消息（复用 safeHTTPClient）
func (b *MessageBus) postMessage(address string, data []byte) error {
	url := fmt.Sprintf("http://%s/api/bus/message", address)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	b.mu.RLock()
	token := b.authToken
	b.mu.RUnlock()
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	resp, err := safeHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("peer returned status %d", resp.StatusCode)
	}
	return nil
}

// generateMessageID 生成消息 ID
func (b *MessageBus) generateMessageID() string {
	return fmt.Sprintf("msg-%s-%d", b.nodeID, time.Now().UnixNano())
}

// signBusMessage 计算消息签名（基于 SHA256 的内容摘要）。
// 注意：这只是基本的完整性校验，真正的身份认证依赖 auth token。
// 使用 UnixNano 作为时间戳的规范表示，确保 JSON 往返后签名仍可验证。
func signBusMessage(msg BusMessage) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%d",
		msg.From, msg.To, msg.Topic, msg.Content, msg.Type,
		msg.Timestamp.UnixNano())
	return hex.EncodeToString(h.Sum(nil))
}

// verifyBusMessage 验证消息签名
func verifyBusMessage(msg BusMessage) bool {
	expected := signBusMessage(msg)
	return subtle.ConstantTimeCompare([]byte(msg.Signature), []byte(expected)) == 1
}
