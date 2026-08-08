// Copyright (c) 2026 aflare Contributors
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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/workflow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the HTTP API server for aflare.
type Server struct {
	host         string
	port         string
	apiKey       string
	readTimeout  time.Duration
	writeTimeout time.Duration
	workflowsDir string

	// Metrics
	requestsTotal   uint64
	requestsActive  int64
	workflowsRun    uint64
	workflowsFailed uint64
}

// NewServer creates a new API server with the given configuration.
func NewServer(host, port, apiKey string) *Server {
	return &Server{
		host:         host,
		port:         port,
		apiKey:       apiKey,
		readTimeout:  30 * time.Second,
		writeTimeout: 60 * time.Second,
	}
}

// SetWorkflowsDir sets the directory to scan for example workflows.
func (s *Server) SetWorkflowsDir(dir string) {
	s.workflowsDir = dir
}

// Start begins listening and serving HTTP requests. It blocks until the
// server is stopped or an error occurs.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/workflows/run", s.handleRunWorkflow)
	mux.HandleFunc("/api/v1/workflows", s.handleListWorkflows)
	mux.HandleFunc("/api/v1/workflows/", s.handleGetWorkflow)

	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	if s.host == "" {
		addr = fmt.Sprintf(":%s", s.port)
	}

	handler := s.middlewareStack(mux)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  120 * time.Second,
	}

	return srv.ListenAndServe()
}

// middlewareStack wraps the handler with all middleware layers.
func (s *Server) middlewareStack(next http.Handler) http.Handler {
	return s.corsMiddleware(
		s.loggingMiddleware(
			s.metricsMiddleware(
				s.authMiddleware(next),
			),
		),
	)
}

// corsMiddleware adds CORS headers for frontend integration.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each incoming request.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[api] %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// metricsMiddleware tracks request counts and active connections.
func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&s.requestsTotal, 1)
		atomic.AddInt64(&s.requestsActive, 1)
		defer atomic.AddInt64(&s.requestsActive, -1)

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates the API key header if an API key is configured.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health and metrics are publicly accessible
		if r.URL.Path == "/health" || r.URL.Path == "/api/v1/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		if s.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.Header.Get("Authorization")
			key = strings.TrimPrefix(key, "Bearer ")
		}

		if key != s.apiKey {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid or missing API key",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth serves the health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"uptime":  time.Now().Format(time.RFC3339),
	})
}

// handleMetrics serves Prometheus metrics.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	promhttp.Handler().ServeHTTP(w, r)
}

// runWorkflowRequest is the JSON request body for running a workflow.
type runWorkflowRequest struct {
	Workflow string `json:"workflow"`
	Timeout  string `json:"timeout,omitempty"`
}

// runWorkflowResponse is the JSON response for a workflow run.
type runWorkflowResponse struct {
	Success     bool                 `json:"success"`
	Output      string               `json:"output,omitempty"`
	StepResults []workflowStepResult `json:"step_results,omitempty"`
	Error       string               `json:"error,omitempty"`
	Duration    string               `json:"duration"`
}

// workflowStepResult mirrors workflow.StepResult in JSON-safe form.
type workflowStepResult struct {
	StepIndex int    `json:"step_index"`
	NodeName  string `json:"node_name"`
	Input     string `json:"input,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration"`
}

// handleRunWorkflow executes a workflow from a JSON or YAML definition.
func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	atomic.AddUint64(&s.workflowsRun, 1)

	var req runWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	if req.Workflow == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "workflow definition is required",
		})
		return
	}

	// Parse the workflow from YAML/JSON content
	wf, err := workflow.ParseWorkflowFromContent(req.Workflow)
	if err != nil {
		atomic.AddUint64(&s.workflowsFailed, 1)
		s.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("failed to parse workflow: %v", err),
		})
		return
	}

	// Determine timeout (capped at 30 minutes to prevent abuse)
	timeout := workflow.DefaultWorkflowTimeout
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil && d > 0 && d <= 30*time.Minute {
			timeout = d
		}
	}

	// Build the registry
	reg := nodes.GetGlobalRegistry()

	// Execute the workflow
	exec := workflow.NewExecutor().WithTimeout(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	output, stepResults, execErr := exec.Execute(ctx, wf, reg)
	duration := time.Since(start)

	resp := runWorkflowResponse{
		Success:  execErr == nil,
		Output:   output,
		Duration: duration.String(),
	}

	if execErr != nil {
		atomic.AddUint64(&s.workflowsFailed, 1)
		resp.Error = execErr.Error()
	}

	for _, sr := range stepResults {
		wsr := workflowStepResult{
			StepIndex: sr.StepIndex,
			NodeName:  sr.NodeName,
			Input:     sr.Input,
			Output:    sr.Output,
			Duration:  sr.Duration.String(),
		}
		if sr.Error != nil {
			wsr.Error = sr.Error.Error()
		}
		resp.StepResults = append(resp.StepResults, wsr)
	}

	status := http.StatusOK
	if execErr != nil {
		status = http.StatusInternalServerError
	}

	s.writeJSON(w, status, resp)
}

// workflowInfo holds summary info for listing workflows.
type workflowInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       int    `json:"steps"`
	File        string `json:"file"`
}

// handleListWorkflows lists available example workflows from the workflows directory.
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	dir := s.workflowsDir
	if dir == "" {
		// Try default locations
		for _, candidate := range []string{"workflows", "./workflows", "../workflows"} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				dir = candidate
				break
			}
		}
	}

	if dir == "" {
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"workflows": []workflowInfo{},
		})
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to read workflows directory: %v", err),
		})
		return
	}

	var workflows []workflowInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		wf, err := workflow.ParseWorkflow(path)
		if err != nil {
			workflows = append(workflows, workflowInfo{
				Name:        strings.TrimSuffix(name, ext),
				Description: fmt.Sprintf("(parse error: %v)", err),
				File:        name,
			})
			continue
		}

		workflows = append(workflows, workflowInfo{
			Name:        wf.Name,
			Description: wf.Description,
			Steps:       len(wf.Steps),
			File:        name,
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"workflows": workflows,
	})
}

// handleGetWorkflow returns details for a specific workflow.
func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Extract workflow name from path: /api/v1/workflows/{name}
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
	if name == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workflow name is required"})
		return
	}

	// Prevent path traversal attacks: reject names containing .., /, or \
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid workflow name"})
		return
	}

	dir := s.workflowsDir
	if dir == "" {
		for _, candidate := range []string{"workflows", "./workflows", "../workflows"} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				dir = candidate
				break
			}
		}
	}

	if dir == "" {
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no workflows directory configured"})
		return
	}

	// Try both with and without extension
	var wf *workflow.Workflow
	var wfFile string
	var err error

	for _, ext := range []string{".yaml", ".yml", ""} {
		candidate := name
		if ext != "" && !strings.HasSuffix(candidate, ext) {
			candidate = name + ext
		}
		path := filepath.Join(dir, candidate)
		if _, statErr := os.Stat(path); statErr == nil {
			wf, err = workflow.ParseWorkflow(path)
			if err == nil {
				wfFile = candidate
				break
			}
		}
	}

	if wf == nil {
		s.writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("workflow '%s' not found", name),
		})
		return
	}

	type stepDetail struct {
		Node   string            `json:"node"`
		Name   string            `json:"name,omitempty"`
		Params map[string]string `json:"params,omitempty"`
	}

	steps := make([]stepDetail, len(wf.Steps))
	for i, step := range wf.Steps {
		steps[i] = stepDetail{
			Node:   step.Node,
			Name:   step.Name,
			Params: step.Params,
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        wf.Name,
		"description": wf.Description,
		"file":        wfFile,
		"steps":       steps,
		"step_count":  len(wf.Steps),
	})
}

// writeJSON writes a JSON response with the given status code.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[api] failed to write JSON response: %v", err)
	}
}
