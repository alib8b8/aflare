// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​‌​​‌‌‌​‌‌‌‌‌‌‌‌​​‌‌​​​​​‌​​​​‌​‌​​‌‌​​‌‌‌‌​​​​​​​​​​​​​​​​​‌‌‌​‌​‌​​​​​‌‌​​⁠
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

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/policy"
	"github.com/alib8b8/aflare/internal/workflow"
)

const (
	defaultPort           = "8080"
	maxBodySize           = 1 * 1024 * 1024 // 1MB
	rateLimitPerMin       = 60
	taskCleanupInterval   = 10 * time.Minute
	taskMaxAge            = 1 * time.Hour
	serverReadTimeout     = 10 * time.Second
	serverWriteTimeout    = 10 * time.Second
	serverShutdownTimeout = 5 * time.Second
	maxConcurrentTasks    = 100 // Limit concurrent webhook executions to prevent goroutine leaks

	// Replay protection: an HMAC proves integrity, not freshness — a
	// captured delivery would otherwise trigger the workflow on every
	// replay, forever. Timestamped signatures must fall within this
	// window of server time (± to tolerate clock drift).
	signatureMaxAge = 5 * time.Minute
	// Platform deliveries (GitHub/Gitea/Forgejo) carry no timestamp in
	// their signature, so their delivery IDs are deduplicated for this
	// long instead: a verbatim replay of the same delivery is rejected.
	deliveryDedupWindow = 24 * time.Hour
	deliveryCacheLimit  = 8192
)

// TaskStatus represents the execution status of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// Task represents a single workflow execution triggered by webhook.
type Task struct {
	ID           string     `json:"id"`
	WorkflowName string     `json:"workflow_name"`
	Input        string     `json:"input"`
	Status       TaskStatus `json:"status"`
	Output       string     `json:"output,omitempty"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// RateLimiter provides per-IP rate limiting.
type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*ipLimiter
	maxReq   int
	window   time.Duration
}

type ipLimiter struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter with the specified max requests per window.
func NewRateLimiter(maxReq int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		maxReq:   maxReq,
		window:   window,
	}
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	l, ok := rl.limiters[ip]
	if !ok {
		l = &ipLimiter{windowStart: time.Now()}
		rl.limiters[ip] = l
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.windowStart) > rl.window {
		l.windowStart = now
		l.count = 0
	}

	if l.count >= rl.maxReq {
		return false
	}
	l.count++
	return true
}

// WebhookServer implements an HTTP server for triggering workflows via webhooks.
type WebhookServer struct {
	port         string
	host         string
	secret       string
	registry     *nodes.Registry
	rateLimiter  *RateLimiter
	workflowsDir string
	deliveries   *deliveryCache

	mu       sync.RWMutex
	tasks    map[string]*Task
	server   *http.Server
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	sem      chan struct{} // Semaphore to limit concurrent task execution
	warnOnce sync.Once     // 一次性提示无认证模式
}

// NewWebhookServer creates a new WebhookServer.
func NewWebhookServer(port, secret string, registry *nodes.Registry) *WebhookServer {
	if port == "" {
		port = defaultPort
	}
	return &WebhookServer{
		port:        port,
		secret:      secret,
		registry:    registry,
		rateLimiter: NewRateLimiter(rateLimitPerMin, time.Minute),
		deliveries:  newDeliveryCache(deliveryDedupWindow, deliveryCacheLimit),
		tasks:       make(map[string]*Task),
		stopCh:      make(chan struct{}),
		sem:         make(chan struct{}, maxConcurrentTasks),
	}
}

// SetWorkflowsDir sets the directory to search for workflow files.
func (s *WebhookServer) SetWorkflowsDir(dir string) {
	s.workflowsDir = dir
}

// SetHost sets the host to bind to. If empty, the server binds to 127.0.0.1
// when no auth secret is configured (safe default), or to all interfaces when
// an auth secret is configured. Setting a host explicitly always takes precedence.
func (s *WebhookServer) SetHost(host string) {
	s.host = host
}

// resolveAddr returns the address the HTTP server should bind to.
//   - 若 host 非空(用户显式设置),使用 host:port(向后兼容,用户意图优先)。
//   - 若 host 为空且 secret 为空(无认证),默认绑 127.0.0.1:port(安全默认,
//     避免无认证暴露公网)。
//   - 若 host 为空但 secret 已配置,使用 :port(全接口,由认证保护)。
func (s *WebhookServer) resolveAddr() string {
	if s.host != "" {
		return s.host + ":" + s.port
	}
	if s.secret == "" {
		return "127.0.0.1:" + s.port
	}
	return ":" + s.port
}

// Addr returns the address the server will bind to (see resolveAddr for the
// host fallback rules). Safe to call before Start.
func (s *WebhookServer) Addr() string {
	return s.resolveAddr()
}

// Start starts the HTTP server.
func (s *WebhookServer) Start() error {
	srv := &http.Server{
		Addr:         s.resolveAddr(),
		Handler:      s.handler(),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}

	s.mu.Lock()
	s.server = srv
	s.mu.Unlock()

	s.wg.Add(1)
	go s.cleanupTasks()

	return srv.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *WebhookServer) Stop() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.mu.RLock()
	srv := s.server
	s.mu.RUnlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	s.wg.Wait()
	return nil
}

func (s *WebhookServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/", s.routeWebhook)
	return mux
}

func (s *WebhookServer) routeWebhook(w http.ResponseWriter, r *http.Request) {
	// 无认证模式一次性告警:已默认绑 127.0.0.1,如需公网访问请配置 auth_token
	if s.secret == "" {
		s.warnOnce.Do(func() {
			logger.Warn("webhook 运行在无认证模式,已默认绑定 127.0.0.1;如需公网访问请配置 auth_token")
		})
	}

	path := r.URL.Path

	if path == "/webhook/health" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleHealth(w, r)
		return
	}

	if strings.HasPrefix(path, "/webhook/status/") {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.requireSecret(w, r) {
			return
		}
		s.handleStatus(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.handleWebhook(w, r)
}

// requireSecret validates the X-Webhook-Secret header against s.secret when a
// secret is configured. It writes a 401 response and returns false if the
// request is unauthorized; returns true if the request is allowed (either no
// secret is configured or the header matches). Comparison uses
// subtle.ConstantTimeCompare to avoid timing side channels.
func (s *WebhookServer) requireSecret(w http.ResponseWriter, r *http.Request) bool {
	if s.secret == "" {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Webhook-Secret")), []byte(s.secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// requireAuth validates a trigger request when a secret is configured. Two
// credential forms are accepted, both verified in constant time:
//
//  1. X-Hub-Signature-256 — the GitHub webhook signature scheme (also used
//     by Gitea/Forgejo): "sha256=<hex HMAC-SHA256 ...>" keyed by the same
//     secret configured in the repository webhook settings. Proves both the
//     delivery origin and body integrity, so it works for untrusted callers
//     where a shared-secret header alone cannot. Replay protection is
//     enforced (see verifySignature).
//  2. X-Webhook-Secret — the plain shared-secret header for trusted
//     automation callers (curl, n8n, cron jobs, ...).
//
// A request presenting a signature header that FAILS verification is
// rejected without falling back to the shared-secret check: a bad signature
// is tamper evidence, not a missing credential.
func (s *WebhookServer) requireAuth(w http.ResponseWriter, r *http.Request, body []byte) bool {
	if s.secret == "" {
		return true
	}
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		return s.verifySignature(w, r, body, sig)
	}
	return s.requireSecret(w, r)
}

// verifySignature authenticates an HMAC-signed delivery WITH replay
// protection. Two freshness schemes, in order of preference:
//
//  1. X-Timestamp — for aflare's own trigger chain (curl, n8n, scripts):
//     the MAC must cover "<unix-seconds>.<body>" and the timestamp must be
//     within ±signatureMaxAge of server time, so a captured request stops
//     being valid once the window closes.
//
//  2. Platform delivery ID (X-GitHub-Delivery / X-Gitea-Delivery /
//     X-Gogs-Delivery) — forges sign only the body and cannot add a
//     timestamp, so those deliveries use the classic body-only MAC and are
//     deduplicated by delivery ID: a verbatim replay of the same delivery
//     is rejected.
//
// A signature with NEITHER is rejected: an unbound body-only MAC is valid
// forever, which is exactly the replay hole this closes.
func (s *WebhookServer) verifySignature(w http.ResponseWriter, r *http.Request, body []byte, sig string) bool {
	workflowName := strings.TrimPrefix(r.URL.Path, "/webhook/")
	client := getClientIP(r)

	if ts := r.Header.Get("X-Timestamp"); ts != "" {
		if err := verifyTimestampedSignature(body, s.secret, sig, ts); err != nil {
			logger.Warn("webhook request rejected: timestamped signature verification failed",
				"client", client,
				"workflow", workflowName,
				"err", err.Error(),
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return false
		}
		return true
	}

	if delivery := platformDeliveryID(r); delivery != "" {
		if !verifyWebhookSignature(body, s.secret, sig) {
			logger.Warn("webhook request rejected: invalid X-Hub-Signature-256",
				"client", client,
				"workflow", workflowName,
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return false
		}
		if !s.deliveries.Record(delivery) {
			logger.Warn("webhook request rejected: replayed delivery",
				"client", client,
				"workflow", workflowName,
				"delivery_id", delivery,
			)
			http.Error(w, "replayed delivery", http.StatusUnauthorized)
			return false
		}
		return true
	}

	logger.Warn("webhook request rejected: body-only signature without X-Timestamp or platform delivery ID (replayable)",
		"client", client,
		"workflow", workflowName,
	)
	http.Error(w, "unauthorized: signature requires X-Timestamp (or platform delivery headers)", http.StatusUnauthorized)
	return false
}

// verifyWebhookSignature checks a GitHub-style X-Hub-Signature-256 header
// ("sha256=<hexdigest>") against the HMAC-SHA256 of the raw request body
// keyed by secret. Malformed headers and decode failures return false
// immediately; the digest comparison is constant time.
func verifyWebhookSignature(body []byte, secret, header string) bool {
	got, ok := parseSignatureHeader(header)
	if !ok {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body) // hash.Hash.Write never returns an error
	return hmac.Equal(got, mac.Sum(nil))
}

// verifyTimestampedSignature checks the timestamped MAC scheme:
//
//	X-Timestamp: <unix seconds>
//	X-Hub-Signature-256: sha256=<hex HMAC-SHA256(secret, "<ts>." + body)>
//
// The timestamp is enforced against server time (±signatureMaxAge) AND
// bound into the MAC, so it can be neither replayed after the window nor
// swapped on a captured signature. The timestamp is re-encoded via
// FormatInt before signing so equivalent forms ("007" vs "7") cannot shift
// the MAC input.
func verifyTimestampedSignature(body []byte, secret, header, tsHeader string) error {
	ts, err := strconv.ParseInt(strings.TrimSpace(tsHeader), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid X-Timestamp %q", tsHeader)
	}
	drift := time.Now().Unix() - ts
	if drift < 0 {
		drift = -drift
	}
	if drift > int64(signatureMaxAge/time.Second) {
		return fmt.Errorf("timestamp %d is %ds from server time (max ±%s)", ts, drift, signatureMaxAge)
	}
	got, ok := parseSignatureHeader(header)
	if !ok {
		return errors.New("malformed X-Hub-Signature-256")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return errors.New("signature mismatch (MAC must cover \"<timestamp>.<body>\")")
	}
	return nil
}

// parseSignatureHeader decodes a "sha256=<hex>" signature header into the
// raw digest. False covers every malformed form (wrong prefix, bad hex,
// empty digest).
func parseSignatureHeader(header string) ([]byte, bool) {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return nil, false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil || len(got) == 0 {
		return nil, false
	}
	return got, true
}

// platformDeliveryID returns the unique delivery identifier forge platforms
// attach to every webhook delivery, or "" when the caller is not a forge.
func platformDeliveryID(r *http.Request) string {
	for _, h := range []string{"X-GitHub-Delivery", "X-Gitea-Delivery", "X-Gogs-Delivery"} {
		if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
			return v
		}
	}
	return ""
}

// deliveryCache deduplicates platform delivery IDs so a captured delivery
// cannot be replayed verbatim. IDs are only recorded AFTER the HMAC
// verified, so unauthenticated traffic can never grow the cache.
type deliveryCache struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	window time.Duration
	limit  int
}

func newDeliveryCache(window time.Duration, limit int) *deliveryCache {
	return &deliveryCache{
		seen:   make(map[string]time.Time),
		window: window,
		limit:  limit,
	}
}

// Record reports whether id is a first sighting (true, and remembered for
// the window) or a replay (false). A replay refreshes the entry so a
// sustained replay attack cannot simply wait out the window.
func (c *deliveryCache) Record(id string) bool {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if ts, ok := c.seen[id]; ok && now.Sub(ts) < c.window {
		c.seen[id] = now
		return false
	}
	if len(c.seen) >= c.limit {
		c.sweepLocked(now)
	}
	c.seen[id] = now
	return true
}

// sweepLocked drops expired entries once the cache hits its limit. Delivery
// volume from verified forges is tiny; this is a bound, not a hot path.
func (c *deliveryCache) sweepLocked(now time.Time) {
	for id, ts := range c.seen {
		if now.Sub(ts) >= c.window {
			delete(c.seen, id)
		}
	}
}

func (s *WebhookServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimiter.Allow(getClientIP(r)) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	workflowName := strings.TrimPrefix(r.URL.Path, "/webhook/")
	if workflowName == "" || !isValidWorkflowName(workflowName) {
		http.Error(w, "invalid workflow name", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(body) > maxBodySize {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Auth runs after the body is read because signature verification needs
	// the raw bytes. See requireAuth for the accepted credential forms.
	if !s.requireAuth(w, r, body) {
		return
	}

	// Resolve the workflow BEFORE accepting the task: a request for a
	// nonexistent workflow can never succeed, so answering 404 up front
	// (instead of 202 + an async "not found" task) keeps the API honest
	// and spares clients a pointless status-poll round trip.
	if _, err := s.findWorkflowPath(workflowName); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Acquire semaphore to limit concurrent executions
	select {
	case s.sem <- struct{}{}:
	default:
		http.Error(w, "server too busy, try again later", http.StatusServiceUnavailable)
		return
	}

	task := &Task{
		ID:           generateTaskID(),
		WorkflowName: workflowName,
		Input:        string(body),
		Status:       TaskPending,
		CreatedAt:    time.Now(),
	}
	s.addTask(task)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { <-s.sem }() // Release semaphore when done
		defer func() {
			if r := recover(); r != nil {
				logger.Error("webhook task panicked",
					"task_id", task.ID,
					"workflow", task.WorkflowName,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				s.completeTask(task.ID, TaskFailed, "", fmt.Sprintf("panic: %v", r))
			}
		}()
		s.runTask(task, body, r.URL.Query())
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"task_id": task.ID})
}

func (s *WebhookServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/webhook/status/")
	if taskID == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}

	task, ok := s.getTask(taskID)
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *WebhookServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *WebhookServer) runTask(task *Task, body []byte, query map[string][]string) {
	now := time.Now()
	s.updateTask(task.ID, func(t *Task) {
		t.Status = TaskRunning
		t.StartedAt = &now
	})

	wfPath, err := s.findWorkflowPath(task.WorkflowName)
	if err != nil {
		// findWorkflowPath errors are already generic (no internal paths).
		s.completeTask(task.ID, TaskFailed, "", err.Error())
		return
	}

	wf, err := workflow.ParseWorkflow(wfPath)
	if err != nil {
		// Log the detailed error server-side; expose only a generic message
		// to clients to avoid leaking absolute file paths / parser internals.
		logger.Error("webhook workflow parse failed",
			"task_id", task.ID,
			"workflow", task.WorkflowName,
			"err", err.Error(),
		)
		s.completeTask(task.ID, TaskFailed, "", "workflow parse failed")
		return
	}

	if wf.Vars == nil {
		wf.Vars = make(map[string]string)
	}
	wf.Vars["input"] = string(body)
	for key, values := range query {
		if key != "" && len(values) > 0 {
			wf.Vars[key] = values[0]
		}
	}

	// Use the immutable default timeout constant. (There is no mutable global
	// timeout anymore — it was removed to avoid racing under parallel tests.)
	ctx, cancel := context.WithTimeout(context.Background(), workflow.DefaultWorkflowTimeout)
	defer cancel()

	// Same audit + policy guarantees as the `aflare run` CLI path (self-test
	// finding: this entry point previously ran the bare package-level
	// ExecuteWorkflow — zero audit records, zero policy checks, while the
	// request body/query flow into workflow vars as untrusted input).
	// Approval-required actions are denied: no human is on this path.
	exec := workflow.NewExecutor().WithAuditLog(true, "")
	pexec := workflow.NewPolicyExecutor(exec, policy.NewEngine(policy.DefaultPolicy(), nil))
	if verr := pexec.ValidateWorkflow(ctx, wf); verr != nil {
		logger.Error("webhook workflow blocked by policy",
			"task_id", task.ID,
			"workflow", task.WorkflowName,
			"err", verr.Error(),
		)
		s.completeTask(task.ID, TaskFailed, "", "workflow blocked by policy")
		return
	}

	output, _, err := pexec.Execute(ctx, wf, s.registry)
	if err != nil {
		// ExecuteWorkflow errors may contain node internals, file paths,
		// or partial output. Log the full error server-side for debugging,
		// but return a generic message to clients to avoid information
		// disclosure through the /webhook/status endpoint.
		logger.Error("webhook workflow execution failed",
			"task_id", task.ID,
			"workflow", task.WorkflowName,
			"err", err.Error(),
		)
		s.completeTask(task.ID, TaskFailed, "", "workflow execution failed")
		return
	}
	s.completeTask(task.ID, TaskCompleted, output, "")
}

func (s *WebhookServer) findWorkflowPath(name string) (string, error) {
	if !isValidWorkflowName(name) {
		return "", fmt.Errorf("invalid workflow name")
	}

	dir := s.workflowsDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			dir = "."
		}
	}

	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absDir, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := os.Stat(absPath); err == nil { // codeql[go/path-injection] -- name is restricted to [a-zA-Z0-9_-] by isValidWorkflowName (no separators/traversal) and absPath is re-verified to stay under workflowsDir by the filepath.Rel ".." check above
			return absPath, nil
		}
	}
	return "", fmt.Errorf("workflow %q not found", name)
}

func (s *WebhookServer) addTask(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *WebhookServer) getTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	t := *task
	return &t, true
}

func (s *WebhookServer) updateTask(id string, fn func(*Task)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[id]; ok {
		fn(task)
	}
}

func (s *WebhookServer) completeTask(id string, status TaskStatus, output, errStr string) {
	now := time.Now()
	s.updateTask(id, func(t *Task) {
		t.Status = status
		t.Output = output
		t.Error = errStr
		t.CompletedAt = &now
	})
}

func (s *WebhookServer) cleanupTasks() {
	defer s.wg.Done()
	ticker := time.NewTicker(taskCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, task := range s.tasks {
				if task.Status == TaskCompleted || task.Status == TaskFailed {
					if task.CompletedAt != nil && now.Sub(*task.CompletedAt) > taskMaxAge {
						delete(s.tasks, id)
					}
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

func isValidWorkflowName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// getClientIP extracts the client IP. Proxy headers (X-Forwarded-For /
// X-Real-IP) are only trusted when AFLARE_TRUST_PROXY_HEADERS=1, since they
// are client-controlled and can be spoofed to bypass per-IP rate limiting.
func getClientIP(r *http.Request) string {
	if os.Getenv("AFLARE_TRUST_PROXY_HEADERS") == "1" {
		ip := r.Header.Get("X-Forwarded-For")
		if ip != "" {
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
		ip = r.Header.Get("X-Real-Ip")
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func generateTaskID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
