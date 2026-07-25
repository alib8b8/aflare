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
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/workflow"
)

const (
	defaultCoordinatorPort = "8090"
	defaultWorkerPort      = "8091"
	heartbeatInterval      = 10 * time.Second
	heartbeatTimeout       = 30 * time.Second
	maxRequestBodySize     = 10 * 1024 * 1024 // 10MB
)

var safeHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ip := net.ParseIP(host)
			if ip != nil {
				if ip.IsLoopback() || ip.IsPrivate() {
					return (&net.Dialer{}).DialContext(ctx, network, addr)
				}
				return nil, fmt.Errorf("only loopback and private network addresses are allowed for distributed communication")
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	},
}

type NodeStatus string

const (
	NodeStatusIdle    NodeStatus = "idle"
	NodeStatusBusy    NodeStatus = "busy"
	NodeStatusOffline NodeStatus = "offline"
)

type NodeInfo struct {
	ID            string     `json:"id"`
	Host          string     `json:"host"`
	Port          string     `json:"port"`
	Status        NodeStatus `json:"status"`
	LastHeartbeat time.Time  `json:"last_heartbeat"`
	Capacity      int        `json:"capacity"`
	CurrentLoad   int        `json:"current_load"`
}

type Task struct {
	ID         string                `json:"id"`
	Workflow   string                `json:"workflow"`
	StepIndex  int                   `json:"step_index"`
	Step       workflow.WorkflowStep `json:"step"`
	Status     TaskStatus            `json:"status"`
	Output     string                `json:"output"`
	Error      string                `json:"error"`
	AssignedTo string                `json:"assigned_to"`
	StartTime  *time.Time            `json:"start_time"`
	EndTime    *time.Time            `json:"end_time"`
}

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type ExecutionResult struct {
	TaskID    string `json:"task_id"`
	StepIndex int    `json:"step_index"`
	Output    string `json:"output"`
	Error     string `json:"error"`
	Success   bool   `json:"success"`
}

type Coordinator struct {
	port          string
	authToken     string
	nodes         map[string]*NodeInfo
	tasks         map[string]*Task
	mu            sync.RWMutex
	httpServer    *http.Server
	nodeIDCounter int
	stopCh        chan struct{}
	stopOnce      sync.Once
	breakers      *BreakerRegistry // 节点级熔断器
}

func NewCoordinator(port, authToken string) *Coordinator {
	if port == "" {
		port = defaultCoordinatorPort
	}
	return &Coordinator{
		port:      port,
		authToken: authToken,
		nodes:     make(map[string]*NodeInfo),
		tasks:     make(map[string]*Task),
		stopCh:    make(chan struct{}),
		breakers:  NewBreakerRegistry(),
	}
}

func (c *Coordinator) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", c.authMiddleware(c.handleRegister))
	mux.HandleFunc("/api/heartbeat", c.authMiddleware(c.handleHeartbeat))
	mux.HandleFunc("/api/nodes", c.authMiddleware(c.handleListNodes))
	mux.HandleFunc("/api/task", c.authMiddleware(c.handleTask))
	mux.HandleFunc("/api/tasks", c.authMiddleware(c.handleListTasks))
	mux.HandleFunc("/api/execute", c.authMiddleware(c.handleExecute))
	mux.HandleFunc("/api/breakers", c.authMiddleware(c.handleBreakers))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	c.httpServer = &http.Server{
		Addr:    ":" + c.port,
		Handler: mux,
	}

	go c.cleanupOfflineNodes()

	logger.Info("Coordinator started", "port", c.port)
	return c.httpServer.ListenAndServe()
}

func (c *Coordinator) Stop() error {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	if c.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return c.httpServer.Shutdown(ctx)
	}
	return nil
}

func (c *Coordinator) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		Capacity int    `json:"capacity"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Host == "" || !isValidPort(req.Port) {
		http.Error(w, "invalid host or port", http.StatusBadRequest)
		return
	}
	if req.Capacity <= 0 || req.Capacity > 1000 {
		http.Error(w, "capacity must be between 1 and 1000", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.nodeIDCounter++
	nodeID := fmt.Sprintf("node-%d", c.nodeIDCounter)
	node := &NodeInfo{
		ID:            nodeID,
		Host:          req.Host,
		Port:          req.Port,
		Status:        NodeStatusIdle,
		LastHeartbeat: time.Now(),
		Capacity:      req.Capacity,
		CurrentLoad:   0,
	}
	c.nodes[nodeID] = node
	c.mu.Unlock()

	logger.Info("Node registered", "id", nodeID, "host", req.Host)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"node_id": nodeID})
}

func (c *Coordinator) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID      string `json:"node_id"`
		CurrentLoad int    `json:"current_load"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	node, ok := c.nodes[req.NodeID]
	if ok {
		node.LastHeartbeat = time.Now()
		node.Status = NodeStatusIdle
		node.CurrentLoad = req.CurrentLoad
	}
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (c *Coordinator) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.mu.RLock()
	nodes := make([]NodeInfo, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, *node)
	}
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

func (c *Coordinator) handleTask(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.handleGetTask(w, r)
	case http.MethodPost:
		c.handleAssignTask(w, r)
	case http.MethodPut:
		c.handleUpdateTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *Coordinator) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}

	c.mu.RLock()
	task, ok := c.tasks[taskID]
	c.mu.RUnlock()

	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (c *Coordinator) handleAssignTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StepIndex int                   `json:"step_index"`
		Step      workflow.WorkflowStep `json:"step"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	nodeID := c.selectBestNodeLocked()
	if nodeID == "" {
		c.mu.Unlock()
		http.Error(w, "no available nodes", http.StatusServiceUnavailable)
		return
	}

	taskID := fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), req.StepIndex)
	task := &Task{
		ID:         taskID,
		StepIndex:  req.StepIndex,
		Step:       req.Step,
		Status:     TaskStatusPending,
		AssignedTo: nodeID,
	}
	c.tasks[taskID] = task

	node, _ := c.nodes[nodeID]
	node.CurrentLoad++
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id":     taskID,
		"assigned_to": nodeID,
	})
}

func (c *Coordinator) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string     `json:"task_id"`
		Status TaskStatus `json:"status"`
		Output string     `json:"output"`
		Error  string     `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	task, ok := c.tasks[req.TaskID]
	if ok {
		task.Status = req.Status
		task.Output = req.Output
		task.Error = req.Error

		if req.Status == TaskStatusCompleted || req.Status == TaskStatusFailed {
			endTime := time.Now()
			task.EndTime = &endTime

			if node, ok := c.nodes[task.AssignedTo]; ok {
				if node.CurrentLoad > 0 {
					node.CurrentLoad--
				}
			}
		} else if req.Status == TaskStatusRunning {
			startTime := time.Now()
			task.StartTime = &startTime
		}
	}
	// 在锁内捕获，锁外记录熔断（避免与 breaker 内部锁形成嵌套）
	assignedTo := ""
	finalStatus := TaskStatusPending
	if ok {
		assignedTo = task.AssignedTo
		finalStatus = req.Status
	}
	c.mu.Unlock()

	// 熔断器：根据任务结果记录成功/失败
	if assignedTo != "" {
		switch finalStatus {
		case TaskStatusCompleted:
			c.breakers.RecordSuccess(assignedTo)
		case TaskStatusFailed:
			if tripped := c.breakers.RecordFailure(assignedTo); tripped {
				logger.Warn("Circuit breaker tripped for node", "node_id", assignedTo)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// handleBreakers 返回所有节点熔断器状态
func (c *Coordinator) handleBreakers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := c.breakers.StatsAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"breakers": stats,
		"count":    len(stats),
	})
}

func (c *Coordinator) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")

	c.mu.RLock()
	tasks := make([]Task, 0)
	for _, task := range c.tasks {
		if status == "" || string(task.Status) == status {
			tasks = append(tasks, *task)
		}
	}
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

func (c *Coordinator) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		WorkflowYAML string `json:"workflow"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	wf, err := workflow.ParseWorkflowFromContent(req.WorkflowYAML)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid workflow: %v", err), http.StatusBadRequest)
		return
	}

	c.mu.RLock()
	nodeCount := len(c.nodes)
	c.mu.RUnlock()

	if nodeCount == 0 {
		http.Error(w, "no worker nodes available", http.StatusServiceUnavailable)
		return
	}

	go c.executeWorkflowDistributed(wf)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (c *Coordinator) executeWorkflowDistributed(wf *workflow.Workflow) {
	logger.Info("Starting distributed workflow execution", "name", wf.Name, "steps", len(wf.Steps))

	for i, step := range wf.Steps {
		c.mu.Lock()
		nodeID := c.selectBestNodeLocked()
		c.mu.Unlock()

		if nodeID == "" {
			logger.Error("No available nodes for step", "step", i)
			break
		}

		taskID := fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), i)
		task := &Task{
			ID:         taskID,
			StepIndex:  i,
			Step:       step,
			Status:     TaskStatusPending,
			AssignedTo: nodeID,
		}

		c.mu.Lock()
		c.tasks[taskID] = task
		if node, ok := c.nodes[nodeID]; ok {
			node.CurrentLoad++
		}
		c.mu.Unlock()

		c.dispatchTask(nodeID, task)

		for {
			c.mu.RLock()
			t, ok := c.tasks[taskID]
			if !ok {
				c.mu.RUnlock()
				logger.Error("Task disappeared", "task_id", taskID)
				break
			}
			// Copy fields under lock to avoid data race
			status := t.Status
			errMsg := t.Error
			c.mu.RUnlock()

			if status == TaskStatusCompleted || status == TaskStatusFailed {
				if status == TaskStatusFailed {
					logger.Error("Step failed", "step", i, "error", errMsg)
				} else {
					logger.Info("Step completed", "step", i)
				}
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		c.mu.RLock()
		finalStatus := task.Status
		c.mu.RUnlock()
		if finalStatus == TaskStatusFailed {
			break
		}
	}

	logger.Info("Distributed workflow execution finished", "name", wf.Name)
}

func (c *Coordinator) dispatchTask(nodeID string, task *Task) {
	c.mu.RLock()
	node, ok := c.nodes[nodeID]
	c.mu.RUnlock()

	if !ok {
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/execute-step", node.Host, node.Port)
	data, _ := json.Marshal(map[string]interface{}{
		"task_id": task.ID,
		"step":    task.Step,
	})

	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			logger.Error("Failed to create request", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := safeHTTPClient.Do(req)
		if err != nil {
			logger.Error("Failed to dispatch task", "error", err)
			// 分派失败视为节点故障，记录熔断失败
			if tripped := c.breakers.RecordFailure(nodeID); tripped {
				logger.Warn("Circuit breaker tripped for node", "node_id", nodeID)
			}
			return
		}
		defer resp.Body.Close()
		// 非成功 HTTP 状态码也视为失败（5xx 服务端错误）
		if resp.StatusCode >= 500 {
			if tripped := c.breakers.RecordFailure(nodeID); tripped {
				logger.Warn("Circuit breaker tripped for node", "node_id", nodeID, "status", resp.StatusCode)
			}
		}
	}()
}

func (c *Coordinator) selectBestNode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.selectBestNodeLocked()
}

// selectBestNodeLocked returns the best node without acquiring the lock.
// Caller must hold c.mu (read or write).
func (c *Coordinator) selectBestNodeLocked() string {
	var bestNodeID string
	bestLoad := -1

	for id, node := range c.nodes {
		if node.Status != NodeStatusOffline &&
			time.Since(node.LastHeartbeat) < heartbeatTimeout &&
			node.CurrentLoad < node.Capacity {
			// 熔断器：跳过已熔断节点（冷却期满会自动半开放行）
			if !c.breakers.AllowRequest(id) {
				continue
			}
			if bestLoad == -1 || node.CurrentLoad < bestLoad {
				bestNodeID = id
				bestLoad = node.CurrentLoad
			}
		}
	}

	return bestNodeID
}

func (c *Coordinator) cleanupOfflineNodes() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			for id, node := range c.nodes {
				if time.Since(node.LastHeartbeat) > heartbeatTimeout {
					node.Status = NodeStatusOffline
					logger.Warn("Node marked offline", "id", id)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

type Worker struct {
	port           string
	coordinatorURL string
	authToken      string
	nodeID         string
	capacity       int
	currentLoad    int
	mu             sync.Mutex
	httpServer     *http.Server
	stopCh         chan struct{}
	stopOnce       sync.Once
}

func NewWorker(port, coordinatorURL, authToken string, capacity int) (*Worker, error) {
	if port == "" {
		port = defaultWorkerPort
	}

	if !isValidPort(port) {
		return nil, fmt.Errorf("invalid port: %s", port)
	}

	if !isValidCoordinatorURL(coordinatorURL) {
		return nil, fmt.Errorf("invalid coordinator URL: %s", coordinatorURL)
	}

	return &Worker{
		port:           port,
		coordinatorURL: coordinatorURL,
		authToken:      authToken,
		capacity:       capacity,
		currentLoad:    0,
		stopCh:         make(chan struct{}),
	}, nil
}

func isValidPort(port string) bool {
	if port == "" {
		return false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isValidCoordinatorURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (c *Coordinator) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c.authToken == "" {
			http.Error(w, "server misconfigured: auth token not set", http.StatusServiceUnavailable)
			return
		}
		token := r.Header.Get("X-Auth-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(c.authToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (w *Worker) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, r *http.Request) {
		if w.authToken == "" {
			http.Error(writer, "server misconfigured: auth token not set", http.StatusServiceUnavailable)
			return
		}
		token := r.Header.Get("X-Auth-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(w.authToken)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(writer, r)
	}
}

func (w *Worker) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/execute-step", w.authMiddleware(w.handleExecuteStep))
	mux.HandleFunc("/health", func(writer http.ResponseWriter, r *http.Request) {
		writer.WriteHeader(http.StatusOK)
		json.NewEncoder(writer).Encode(map[string]string{"status": "ok"})
	})

	w.httpServer = &http.Server{
		Addr:    ":" + w.port,
		Handler: mux,
	}

	if err := w.registerWithCoordinator(); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	go w.sendHeartbeats()

	logger.Info("Worker started", "port", w.port, "coordinator", w.coordinatorURL)
	return w.httpServer.ListenAndServe()
}

func (w *Worker) Stop() error {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	if w.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return w.httpServer.Shutdown(ctx)
	}
	return nil
}

func (w *Worker) httpPost(path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, w.coordinatorURL+path, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.authToken != "" {
		req.Header.Set("X-Auth-Token", w.authToken)
	}
	return safeHTTPClient.Do(req)
}

func (w *Worker) httpPut(path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, w.coordinatorURL+path, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.authToken != "" {
		req.Header.Set("X-Auth-Token", w.authToken)
	}
	return safeHTTPClient.Do(req)
}

func (w *Worker) registerWithCoordinator() error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"host":     hostname,
		"port":     w.port,
		"capacity": w.capacity,
	})

	resp, err := w.httpPost("/api/register", reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed: %s", resp.Status)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	w.nodeID = result["node_id"]
	logger.Info("Worker registered", "node_id", w.nodeID)
	return nil
}

func (w *Worker) sendHeartbeats() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			load := w.currentLoad
			w.mu.Unlock()

			reqBody, _ := json.Marshal(map[string]interface{}{
				"node_id":      w.nodeID,
				"current_load": load,
			})

			w.httpPost("/api/heartbeat", reqBody)
		case <-w.stopCh:
			return
		}
	}
}

func (w *Worker) handleExecuteStep(writer http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskID string                `json:"task_id"`
		Step   workflow.WorkflowStep `json:"step"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&req); err != nil {
		http.Error(writer, "invalid JSON", http.StatusBadRequest)
		return
	}

	w.mu.Lock()
	w.currentLoad++
	w.mu.Unlock()

	go w.executeStep(req.TaskID, req.Step)

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	json.NewEncoder(writer).Encode(map[string]string{"status": "accepted"})
}

func (w *Worker) executeStep(taskID string, step workflow.WorkflowStep) {
	w.updateTaskStatus(taskID, TaskStatusRunning)

	output, err := executeStepLocally(step)

	if err != nil {
		w.updateTaskStatus(taskID, TaskStatusFailed)
		w.updateTaskResult(taskID, "", err.Error())
	} else {
		w.updateTaskStatus(taskID, TaskStatusCompleted)
		w.updateTaskResult(taskID, output, "")
	}

	w.mu.Lock()
	w.currentLoad--
	w.mu.Unlock()
}

func (w *Worker) updateTaskStatus(taskID string, status TaskStatus) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID,
		"status":  status,
	})
	w.httpPut("/api/task", reqBody)
}

func (w *Worker) updateTaskResult(taskID, output, errorStr string) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID,
		"status":  TaskStatusCompleted,
		"output":  output,
		"error":   errorStr,
	})
	w.httpPut("/api/task", reqBody)
}

func executeStepLocally(step workflow.WorkflowStep) (string, error) {
	return "", nil
}

func GetCoordinatorAddress() string {
	if addr := os.Getenv("LLM_BOX_COORDINATOR"); addr != "" {
		return addr
	}
	return fmt.Sprintf("http://localhost:%s", defaultCoordinatorPort)
}
