package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
)

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
	nodes         map[string]*NodeInfo
	tasks         map[string]*Task
	mu            sync.RWMutex
	httpServer    *http.Server
	nodeIDCounter int
}

func NewCoordinator(port string) *Coordinator {
	if port == "" {
		port = defaultCoordinatorPort
	}
	return &Coordinator{
		port:  port,
		nodes: make(map[string]*NodeInfo),
		tasks: make(map[string]*Task),
	}
}

func (c *Coordinator) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", c.handleRegister)
	mux.HandleFunc("/api/heartbeat", c.handleHeartbeat)
	mux.HandleFunc("/api/nodes", c.handleListNodes)
	mux.HandleFunc("/api/task", c.handleTask)
	mux.HandleFunc("/api/tasks", c.handleListTasks)
	mux.HandleFunc("/api/execute", c.handleExecute)

	c.httpServer = &http.Server{
		Addr:    ":" + c.port,
		Handler: mux,
	}

	go c.cleanupOfflineNodes()

	logger.Info("Coordinator started", "port", c.port)
	return c.httpServer.ListenAndServe()
}

func (c *Coordinator) Stop() error {
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	nodeID := c.selectBestNode()
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
				node.CurrentLoad--
			}
		} else if req.Status == TaskStatusRunning {
			startTime := time.Now()
			task.StartTime = &startTime
		}
	}
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		nodeID := c.selectBestNode()
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
			t := c.tasks[taskID]
			c.mu.RUnlock()

			if t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed {
				if t.Status == TaskStatusFailed {
					logger.Error("Step failed", "step", i, "error", t.Error)
				} else {
					logger.Info("Step completed", "step", i)
				}
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		if task.Status == TaskStatusFailed {
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
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			logger.Error("Failed to dispatch task", "error", err)
			return
		}
		defer resp.Body.Close()
	}()
}

func (c *Coordinator) selectBestNode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var bestNodeID string
	bestLoad := -1

	for id, node := range c.nodes {
		if node.Status != NodeStatusOffline &&
			time.Since(node.LastHeartbeat) < heartbeatTimeout &&
			node.CurrentLoad < node.Capacity {
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
		}
	}
}

type Worker struct {
	port           string
	coordinatorURL string
	nodeID         string
	capacity       int
	currentLoad    int
	mu             sync.Mutex
	httpServer     *http.Server
	stopCh         chan struct{}
	stopOnce       sync.Once
}

func NewWorker(port, coordinatorURL string, capacity int) (*Worker, error) {
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

func (w *Worker) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/execute-step", w.handleExecuteStep)

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

	resp, err := http.Post(w.coordinatorURL+"/api/register", "application/json", bytes.NewBuffer(reqBody))
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

			http.Post(w.coordinatorURL+"/api/heartbeat", "application/json", bytes.NewBuffer(reqBody))
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	req, _ := http.NewRequest(http.MethodPut, w.coordinatorURL+"/api/task", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

func (w *Worker) updateTaskResult(taskID, output, errorStr string) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID,
		"status":  TaskStatusCompleted,
		"output":  output,
		"error":   errorStr,
	})
	req, _ := http.NewRequest(http.MethodPut, w.coordinatorURL+"/api/task", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
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
