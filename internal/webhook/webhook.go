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

package webhook

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
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

func (s *WebhookServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimiter.Allow(getClientIP(r)) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	if !s.requireSecret(w, r) {
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
		s.completeTask(task.ID, TaskFailed, "", err.Error())
		return
	}

	wf, err := workflow.ParseWorkflow(wfPath)
	if err != nil {
		s.completeTask(task.ID, TaskFailed, "", err.Error())
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

	// Use the immutable default timeout constant rather than the deprecated
	// mutable WorkflowTimeout global, which is unsafe under parallel tests.
	ctx, cancel := context.WithTimeout(context.Background(), workflow.DefaultWorkflowTimeout)
	defer cancel()

	output, _, err := workflow.ExecuteWorkflow(ctx, wf, s.registry)
	if err != nil {
		s.completeTask(task.ID, TaskFailed, "", err.Error())
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
		if _, err := os.Stat(absPath); err == nil {
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

func getClientIP(r *http.Request) string {
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
