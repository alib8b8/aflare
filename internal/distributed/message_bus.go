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
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
)

// ErrBackpressure 当待发送消息超过背压阈值时返回。
// 生产者应降速或丢弃非关键消息。
var ErrBackpressure = errors.New("message bus backpressure: too many pending messages")

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
	To        string    `json:"to"`    // 空表示广播
	Topic     string    `json:"topic"` // 频道/主题
	Content   string    `json:"content"`
	Type      string    `json:"type"` // text|task|result|status|vote
	Timestamp time.Time `json:"timestamp"`
	Signature string    `json:"signature"`
	HopLimit  int       `json:"hop_limit"` // 消息剩余跳数,0 表示不限,>0 时每转发一次减 1,到 0 不再转发
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
	nodeID          string
	port            string
	host            string                       // 可选的绑定 host;为空时按 resolveAddr 规则推导
	authToken       string                       // 可选的 auth token
	subscribers     map[string][]chan BusMessage // topic -> subscribers
	mu              sync.RWMutex
	httpServer      *http.Server
	peers           map[string]string            // nodeID -> address
	peerSeen        map[string]time.Time         // nodeID -> 最后见到的时间
	votes           map[string]map[string]string // topic -> (nodeID -> choice)
	stopCh          chan struct{}
	stopOnce        sync.Once
	warnOnce        sync.Once // 一次性提示无认证模式
	defaultHopLimit int       // 默认跳数限制,0 表示不限;Broadcast 时如果 msg.HopLimit==0 则用此值
	maxPendingMsgs  int       // Publish 背压阈值:待发送消息总数超过此值时拒绝新消息
}

// NewMessageBus 创建消息总线
func NewMessageBus(nodeID, port string) *MessageBus {
	return &MessageBus{
		nodeID:          nodeID,
		port:            port,
		subscribers:     make(map[string][]chan BusMessage),
		peers:           make(map[string]string),
		peerSeen:        make(map[string]time.Time),
		votes:           make(map[string]map[string]string),
		stopCh:          make(chan struct{}),
		defaultHopLimit: 3,    // 默认最多 3 跳,防止消息风暴
		maxPendingMsgs:  1000, // 默认背压阈值:所有 topic 待发送消息总数上限 1000
	}
}

// SetHopLimit 设置默认跳数限制(0 表示不限)。
// Broadcast 时若消息未指定 HopLimit,则使用此默认值。
// 建议值 2-5:过小则消息传播不全,过大则可能产生风暴。
func (b *MessageBus) SetHopLimit(limit int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.defaultHopLimit = limit
}

// SetMaxPending 设置 Publish 背压阈值。
// 当所有 topic 的待发送消息总数超过此值时,Publish 返回 ErrBackpressure。
// 0 表示不限制(退化为原有行为)。
func (b *MessageBus) SetMaxPending(max int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxPendingMsgs = max
}

// SetAuthToken 设置可选的 auth token，用于 HTTP 端点鉴权。
// 设置后所有 /api/bus/* 端点（除 /health）都需要 X-Auth-Token 头。
func (b *MessageBus) SetAuthToken(token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.authToken = token
}

// SetHost 设置绑定的 host。为空时按 resolveAddr 规则推导:
// 无 auth token 时默认绑 127.0.0.1(安全默认),有 auth token 时绑全接口。
// 显式设置 host 总是优先(向后兼容用户意图)。
func (b *MessageBus) SetHost(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.host = host
}

// resolveAddr 返回 HTTP 服务器应绑定的地址。
//   - 若 host 非空(用户显式设置),使用 host:port(用户意图优先)。
//   - 若 host 为空且 authToken 为空(无认证),默认绑 127.0.0.1:port(安全默认,
//     避免无认证暴露公网)。
//   - 若 host 为空但 authToken 已配置,使用 :port(全接口,由认证保护)。
func (b *MessageBus) resolveAddr() string {
	b.mu.RLock()
	host, token := b.host, b.authToken
	b.mu.RUnlock()
	if host != "" {
		return host + ":" + b.port
	}
	if token == "" {
		return "127.0.0.1:" + b.port
	}
	return ":" + b.port
}

// Start 启动 HTTP 服务器，监听消息
func (b *MessageBus) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/bus/message", b.authMiddleware(b.handleMessage))
	mux.HandleFunc("/api/bus/vote", b.authMiddleware(b.handleVote))
	mux.HandleFunc("/api/bus/peers", b.authMiddleware(b.handleListPeers))
	mux.HandleFunc("/api/bus/peer", b.authMiddleware(b.handleRegisterPeer))
	mux.HandleFunc("/health", b.handleHealth)

	addr := b.resolveAddr()
	b.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.Info("MessageBus started", "node_id", b.nodeID, "addr", addr)
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

	// 背压检查:统计所有 topic 待发送消息总数
	if b.maxPendingMsgs > 0 {
		b.mu.RLock()
		pending := 0
		for _, subs := range b.subscribers {
			for _, ch := range subs {
				pending += len(ch)
			}
		}
		b.mu.RUnlock()
		if pending > b.maxPendingMsgs {
			return ErrBackpressure
		}
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
// HopLimit 控制转发跳数:0 表示不限(用 defaultHopLimit),>0 时每转发一次减 1,到 0 不再转发。
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

	// 应用默认 hop limit
	b.mu.RLock()
	defaultHop := b.defaultHopLimit
	b.mu.RUnlock()
	if msg.HopLimit == 0 {
		msg.HopLimit = defaultHop
	}

	msg.To = "" // 广播

	// 本地分发
	if err := b.Publish(msg); err != nil {
		return err
	}

	// 跳数为 1 时只本地分发,不再网络转发(已经要减到 0 了)
	// 跳数 > 1 时转发给 peer,转发时 HopLimit - 1
	// 跳数 == 0 表示不限,继续转发
	if msg.HopLimit > 0 && msg.HopLimit <= 1 {
		return nil // 只本地,不外发
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

	// 转发时减 1
	forwardMsg := msg
	if forwardMsg.HopLimit > 0 {
		forwardMsg.HopLimit--
	}
	data, err := json.Marshal(forwardMsg)
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

// authMiddleware 鉴权中间件。authToken 为空时跳过鉴权(无认证模式),
// 但会记录一次性 warn 日志提示已默认绑 127.0.0.1;authToken 设置时必须匹配。
func (b *MessageBus) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b.mu.RLock()
		token := b.authToken
		b.mu.RUnlock()
		if token == "" {
			b.warnOnce.Do(func() {
				logger.Warn("message_bus 运行在无认证模式,已默认绑定 127.0.0.1")
			})
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
	// 转发到其他 peer(基于 hop_limit 控制):
	// - msg.From == b.nodeID: 不转发(防止自己发的消息绕回来)
	// - msg.HopLimit == 0: 无限制继续广播(向后兼容)
	// - msg.HopLimit == 1: 只本地分发不再转发
	// - msg.HopLimit > 1: 继续转发(HopLimit-1)
	b.forwardReceivedMessage(msg)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// forwardReceivedMessage 将接收到的 peer 消息按 hop_limit 规则转发给其他 peer。
// 反循环:如果 msg.From == b.nodeID 则不转发。转发失败仅记录日志,不影响主流程。
func (b *MessageBus) forwardReceivedMessage(msg BusMessage) {
	// 反循环:自己发的消息绕回来不转发
	if msg.From == b.nodeID {
		return
	}
	// hop_limit == 1:只本地,不再转发
	if msg.HopLimit == 1 {
		return
	}
	// hop_limit > 1:转发时减 1;hop_limit == 0:无限制转发(不变)
	forwardMsg := msg
	if forwardMsg.HopLimit > 0 {
		forwardMsg.HopLimit--
	}

	b.mu.RLock()
	peers := make(map[string]string, len(b.peers))
	for k, v := range b.peers {
		peers[k] = v
	}
	b.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	data, err := json.Marshal(forwardMsg)
	if err != nil {
		logger.Warn("MessageBus forward marshal failed",
			"topic", msg.Topic, "id", msg.ID, "error", err)
		return
	}

	for peerID, addr := range peers {
		if peerID == b.nodeID {
			continue
		}
		if err := b.postMessage(addr, data); err != nil {
			logger.Warn("MessageBus forward to peer failed",
				"peer", peerID, "address", addr, "error", err)
		}
	}
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
