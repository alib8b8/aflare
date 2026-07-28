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

package webui

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/visualizer"
	"github.com/alib8b8/llm-box/internal/workflow"
)

const (
	defaultHost           = "127.0.0.1"
	defaultPort           = "8081"
	serverReadTimeout     = 30 * time.Second
	serverWriteTimeout    = 30 * time.Second
	serverShutdownTimeout = 10 * time.Second
	maxWorkflowFileSize   = 5 * 1024 * 1024 // 5MB
)

type WebUIServer struct {
	host         string
	port         string
	workflowsDir string
	authToken    string

	mu     sync.RWMutex
	server *http.Server
	stopCh chan struct{}
}

func NewWebUIServer(host, port string) *WebUIServer {
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
	}
	return &WebUIServer{
		host:   host,
		port:   port,
		stopCh: make(chan struct{}),
	}
}

// SetAuthToken enables token-based authentication for the WebUI.
// When set, all API requests must include an "X-Auth-Token" header.
func (s *WebUIServer) SetAuthToken(token string) {
	s.authToken = token
}

func (s *WebUIServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken != "" {
			token := r.Header.Get("X-Auth-Token")
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *WebUIServer) SetWorkflowsDir(dir string) {
	s.workflowsDir = dir
}

func (s *WebUIServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/visualize", s.authMiddleware(s.handleVisualize))
	mux.HandleFunc("/api/workflows", s.authMiddleware(s.handleListWorkflows))
	mux.HandleFunc("/api/workflow", s.authMiddleware(s.handleWorkflow))
	mux.HandleFunc("/api/validate", s.authMiddleware(s.handleValidate))

	// pprof 调试端点:默认关闭,仅当环境变量 LLM_BOX_PPROF=1 时启用。
	// 生产环境保持关闭以避免安全暴露;需要在线性能剖析时显式开启。
	// 端点受 authMiddleware 保护,访问需带 X-Auth-Token(若设置了 token)。
	if os.Getenv("LLM_BOX_PPROF") == "1" {
		s.registerPprof(mux)
		logger.Info("pprof endpoints enabled at /debug/pprof/ (LLM_BOX_PPROF=1)")
	}

	srv := &http.Server{
		Addr:         s.host + ":" + s.port,
		Handler:      mux,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}

	s.mu.Lock()
	s.server = srv
	s.mu.Unlock()

	logger.Info("WebUI server started", "port", s.port)
	return srv.ListenAndServe()
}

// registerPprof 在 mux 上注册 net/http/pprof 调试端点。
// 所有端点经 authMiddleware 保护(若设置了 authToken 则需带 X-Auth-Token)。
func (s *WebUIServer) registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", s.authMiddleware(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", s.authMiddleware(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", s.authMiddleware(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", s.authMiddleware(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", s.authMiddleware(pprof.Trace))
}

func (s *WebUIServer) Stop() error {
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
	logger.Info("WebUI server stopped")
	return nil
}

func (s *WebUIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(indexHTML))
}

func (s *WebUIServer) handleVisualize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	var yamlStr string

	if strings.HasPrefix(contentType, "application/json") {
		var req struct {
			Workflow string `json:"workflow"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, maxWorkflowFileSize)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		yamlStr = req.Workflow
	} else {
		body := make([]byte, maxWorkflowFileSize+1)
		n, err := r.Body.Read(body)
		if err != nil && err != io.EOF {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if n > maxWorkflowFileSize {
			http.Error(w, "workflow too large", http.StatusRequestEntityTooLarge)
			return
		}
		yamlStr = string(body[:n])
	}

	format := r.URL.Query().Get("format")
	switch format {
	case "mermaid":
		result := visualizer.GenerateMermaid(yamlStr)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(result))
	case "dot":
		result := visualizer.GenerateDOT(yamlStr)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(result))
	case "ascii":
		result := visualizer.GenerateASCII(yamlStr)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(result))
	case "json":
		fallthrough
	default:
		result := visualizer.GenerateJSON(yamlStr)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func (s *WebUIServer) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := s.getWorkflowsDir()
	files, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "failed to read workflows directory", http.StatusInternalServerError)
		return
	}

	var workflows []string
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				workflows = append(workflows, strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml"))
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": workflows,
		"directory": dir,
	})
}

func (s *WebUIServer) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetWorkflow(w, r)
	case http.MethodPost:
		s.handleSaveWorkflow(w, r)
	case http.MethodDelete:
		s.handleDeleteWorkflow(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *WebUIServer) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing workflow name", http.StatusBadRequest)
		return
	}

	if !isValidWorkflowName(name) {
		http.Error(w, "invalid workflow name", http.StatusBadRequest)
		return
	}

	path := s.getWorkflowPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	w.Write(data)
}

func (s *WebUIServer) handleSaveWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxWorkflowFileSize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "missing workflow name", http.StatusBadRequest)
		return
	}

	if !isValidWorkflowName(req.Name) {
		http.Error(w, "invalid workflow name", http.StatusBadRequest)
		return
	}

	if len(req.Content) > maxWorkflowFileSize {
		http.Error(w, "workflow too large", http.StatusRequestEntityTooLarge)
		return
	}

	dir := s.getWorkflowsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	path := filepath.Join(dir, req.Name+".yaml")
	if err := os.WriteFile(path, []byte(req.Content), 0600); err != nil {
		http.Error(w, "failed to save workflow", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved", "path": path})
}

func (s *WebUIServer) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing workflow name", http.StatusBadRequest)
		return
	}

	if !isValidWorkflowName(name) {
		http.Error(w, "invalid workflow name", http.StatusBadRequest)
		return
	}

	path := s.getWorkflowPath(name)
	if err := os.Remove(path); err != nil {
		http.Error(w, "failed to delete workflow", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *WebUIServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Workflow string `json:"workflow"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxWorkflowFileSize)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	wf, err := workflow.ParseWorkflowFromContent(req.Workflow)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":    false,
			"error":    err.Error(),
			"warnings": []string{},
		})
		return
	}

	warnings := workflow.ValidateWorkflow(wf)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":    true,
		"error":    "",
		"warnings": warnings,
		"name":     wf.Name,
		"steps":    len(wf.Steps),
	})
}

func (s *WebUIServer) getWorkflowsDir() string {
	if s.workflowsDir != "" {
		return s.workflowsDir
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func (s *WebUIServer) getWorkflowPath(name string) string {
	return filepath.Join(s.getWorkflowsDir(), name+".yaml")
}

func isValidWorkflowName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

var indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LLM Box - Workflow Visualizer</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f0f23; color: #e0e0e0; }
        .container { display: flex; height: 100vh; }
        .sidebar { width: 320px; background: #1a1a2e; border-right: 1px solid #2a2a4a; display: flex; flex-direction: column; }
        .sidebar-header { padding: 16px; border-bottom: 1px solid #2a2a4a; }
        .sidebar-header h1 { font-size: 18px; font-weight: 600; color: #00d4ff; }
        .sidebar-content { flex: 1; overflow-y: auto; padding: 12px; }
        .sidebar-footer { padding: 12px; border-top: 1px solid #2a2a4a; }
        .btn { display: block; width: 100%; padding: 10px 12px; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 500; transition: all 0.2s; }
        .btn-primary { background: #00d4ff; color: #0f0f23; }
        .btn-primary:hover { background: #00b8e6; }
        .btn-secondary { background: #2a2a4a; color: #e0e0e0; }
        .btn-secondary:hover { background: #3a3a5a; }
        .btn-danger { background: #ff4757; color: white; }
        .btn-danger:hover { background: #ff3344; }
        .workflow-list { list-style: none; }
        .workflow-item { padding: 10px; margin-bottom: 6px; background: #2a2a4a; border-radius: 8px; cursor: pointer; transition: all 0.2s; }
        .workflow-item:hover { background: #3a3a5a; }
        .workflow-item.active { background: #00d4ff; color: #0f0f23; }
        .main { flex: 1; display: flex; flex-direction: column; }
        .toolbar { padding: 12px 16px; background: #1a1a2e; border-bottom: 1px solid #2a2a4a; display: flex; gap: 12px; align-items: center; }
        .toolbar select, .toolbar input { padding: 8px 12px; background: #2a2a4a; border: 1px solid #3a3a5a; border-radius: 6px; color: #e0e0e0; font-size: 14px; }
        .toolbar select:focus, .toolbar input:focus { outline: none; border-color: #00d4ff; }
        .tabs { display: flex; border-bottom: 1px solid #2a2a4a; }
        .tab { padding: 10px 20px; cursor: pointer; font-size: 14px; font-weight: 500; color: #8080a0; border-bottom: 2px solid transparent; transition: all 0.2s; }
        .tab.active { color: #00d4ff; border-bottom-color: #00d4ff; }
        .tab-content { flex: 1; overflow: auto; padding: 16px; }
        textarea { width: 100%; height: 100%; min-height: 500px; padding: 16px; background: #0f0f23; border: 1px solid #2a2a4a; border-radius: 8px; color: #e0e0e0; font-family: 'Monaco', 'Menlo', monospace; font-size: 14px; resize: none; }
        textarea:focus { outline: none; border-color: #00d4ff; }
        .preview { background: #1a1a2e; border-radius: 8px; padding: 16px; font-family: 'Monaco', 'Menlo', monospace; font-size: 13px; white-space: pre-wrap; word-break: break-all; max-height: 100%; overflow: auto; }
        .mermaid-container { background: #1a1a2e; border-radius: 8px; padding: 16px; overflow: auto; }
        .visualization-svg { width: 100%; height: auto; }
        .status-bar { padding: 8px 16px; background: #1a1a2e; border-top: 1px solid #2a2a4a; font-size: 12px; color: #8080a0; display: flex; gap: 20px; }
        .status-bar .valid { color: #00ff88; }
        .status-bar .invalid { color: #ff4757; }
        .modal { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); justify-content: center; align-items: center; z-index: 1000; }
        .modal.show { display: flex; }
        .modal-content { background: #1a1a2e; border: 1px solid #2a2a4a; border-radius: 12px; padding: 24px; width: 400px; }
        .modal-content h2 { margin-bottom: 16px; font-size: 18px; }
        .modal-content input { width: 100%; padding: 10px; margin-bottom: 16px; background: #2a2a4a; border: 1px solid #3a3a5a; border-radius: 6px; color: #e0e0e0; }
        .modal-content input:focus { outline: none; border-color: #00d4ff; }
        .modal-actions { display: flex; gap: 12px; justify-content: flex-end; }
        .error-message { color: #ff4757; font-size: 12px; margin-top: 8px; }
        .warnings { background: #2a2a0a; border: 1px solid #4a4a1a; border-radius: 6px; padding: 10px; margin-top: 10px; font-size: 12px; color: #ffff80; }
    </style>
</head>
<body>
    <div class="container">
        <div class="sidebar">
            <div class="sidebar-header">
                <h1>LLM Box</h1>
            </div>
            <div class="sidebar-content">
                <ul class="workflow-list" id="workflowList"></ul>
            </div>
            <div class="sidebar-footer">
                <button class="btn btn-primary" onclick="showNewModal()">+ New Workflow</button>
            </div>
        </div>

        <div class="main">
            <div class="toolbar">
                <select id="outputFormat" onchange="renderVisualization()">
                    <option value="mermaid">Mermaid</option>
                    <option value="json">JSON</option>
                    <option value="dot">DOT</option>
                    <option value="ascii">ASCII</option>
                </select>
                <button class="btn btn-secondary" onclick="validateWorkflow()">Validate</button>
                <button class="btn btn-primary" onclick="saveWorkflow()">Save</button>
                <button class="btn btn-danger" onclick="deleteCurrentWorkflow()" style="display:none" id="deleteBtn">Delete</button>
                <input type="text" id="workflowName" placeholder="Workflow name..." />
            </div>

            <div class="tabs">
                <div class="tab active" onclick="switchTab('editor')">Editor</div>
                <div class="tab" onclick="switchTab('preview')">Preview</div>
            </div>

            <div class="tab-content" id="editorTab">
                <textarea id="workflowEditor" placeholder="Enter workflow YAML here..."></textarea>
            </div>

            <div class="tab-content" id="previewTab" style="display:none">
                <div id="previewContent"></div>
            </div>

            <div class="status-bar">
                <span id="validationStatus">Not validated</span>
                <span id="stepCount">0 steps</span>
            </div>
        </div>
    </div>

    <div class="modal" id="newModal">
        <div class="modal-content">
            <h2>New Workflow</h2>
            <input type="text" id="newWorkflowName" placeholder="Workflow name" />
            <div class="error-message" id="newError"></div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="hideNewModal()">Cancel</button>
                <button class="btn btn-primary" onclick="createNewWorkflow()">Create</button>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
    <script>
        let currentWorkflow = '';

        async function loadWorkflows() {
            try {
                const response = await fetch('/api/workflows');
                const data = await response.json();
                const list = document.getElementById('workflowList');
                list.innerHTML = '';
                data.workflows.forEach(name => {
                    const li = document.createElement('li');
                    li.className = 'workflow-item';
                    li.textContent = name;
                    li.onclick = () => loadWorkflow(name);
                    list.appendChild(li);
                });
            } catch (e) {
                console.error('Failed to load workflows:', e);
            }
        }

        async function loadWorkflow(name) {
            try {
                const response = await fetch('/api/workflow?name=' + encodeURIComponent(name));
                if (response.ok) {
                    const content = await response.text();
                    document.getElementById('workflowEditor').value = content;
                    document.getElementById('workflowName').value = name;
                    currentWorkflow = name;
                    document.getElementById('deleteBtn').style.display = 'block';

                    document.querySelectorAll('.workflow-item').forEach(item => {
                        item.classList.remove('active');
                        if (item.textContent === name) item.classList.add('active');
                    });

                    renderVisualization();
                }
            } catch (e) {
                console.error('Failed to load workflow:', e);
            }
        }

        async function saveWorkflow() {
            const name = document.getElementById('workflowName').value.trim();
            const content = document.getElementById('workflowEditor').value;

            if (!name) {
                alert('Please enter a workflow name');
                return;
            }

            try {
                const response = await fetch('/api/workflow', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, content })
                });

                if (response.ok) {
                    currentWorkflow = name;
                    await loadWorkflows();
                    document.querySelectorAll('.workflow-item').forEach(item => {
                        if (item.textContent === name) item.classList.add('active');
                    });
                    document.getElementById('deleteBtn').style.display = 'block';
                    alert('Workflow saved successfully');
                } else {
                    const data = await response.json();
                    alert('Failed to save: ' + (data.error || 'Unknown error'));
                }
            } catch (e) {
                console.error('Failed to save workflow:', e);
            }
        }

        async function deleteCurrentWorkflow() {
            if (!currentWorkflow) return;
            if (!confirm('Are you sure you want to delete this workflow?')) return;

            try {
                const response = await fetch('/api/workflow?name=' + encodeURIComponent(currentWorkflow), {
                    method: 'DELETE'
                });

                if (response.ok) {
                    document.getElementById('workflowEditor').value = '';
                    document.getElementById('workflowName').value = '';
                    document.getElementById('deleteBtn').style.display = 'none';
                    currentWorkflow = '';
                    document.querySelectorAll('.workflow-item').forEach(item => item.classList.remove('active'));
                    await loadWorkflows();
                }
            } catch (e) {
                console.error('Failed to delete workflow:', e);
            }
        }

        async function validateWorkflow() {
            const content = document.getElementById('workflowEditor').value;
            try {
                const response = await fetch('/api/validate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ workflow: content })
                });

                const data = await response.json();
                const status = document.getElementById('validationStatus');
                const steps = document.getElementById('stepCount');

                if (data.valid) {
                    status.textContent = '✓ Valid';
                    status.className = 'valid';
                    steps.textContent = data.steps + ' steps';
                } else {
                    status.textContent = '✗ Invalid: ' + data.error;
                    status.className = 'invalid';
                    steps.textContent = '0 steps';
                }
            } catch (e) {
                console.error('Failed to validate:', e);
            }
        }

        async function renderVisualization() {
            const content = document.getElementById('workflowEditor').value;
            const format = document.getElementById('outputFormat').value;
            const preview = document.getElementById('previewContent');

            try {
                const response = await fetch('/api/visualize?format=' + format, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ workflow: content })
                });

                const result = await response.text();

                if (format === 'mermaid') {
                    preview.innerHTML = '<div class="mermaid-container"><div class="mermaid"></div></div>';
                    preview.querySelector('.mermaid').textContent = result;
                    mermaid.init(undefined, '.mermaid');
                } else if (format === 'json') {
                    preview.innerHTML = '<div class="preview">' + syntaxHighlight(result) + '</div>';
                } else {
                    preview.innerHTML = '<div class="preview">' + escapeHtml(result) + '</div>';
                }
            } catch (e) {
                preview.innerHTML = '<div class="preview">Error: ' + escapeHtml(e.message) + '</div>';
            }
        }

        function switchTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');

            event.target.classList.add('active');
            document.getElementById(tab + 'Tab').style.display = 'block';

            if (tab === 'preview') {
                renderVisualization();
            }
        }

        function showNewModal() {
            document.getElementById('newModal').classList.add('show');
            document.getElementById('newWorkflowName').value = '';
            document.getElementById('newError').textContent = '';
        }

        function hideNewModal() {
            document.getElementById('newModal').classList.remove('show');
        }

        async function createNewWorkflow() {
            const name = document.getElementById('newWorkflowName').value.trim();
            if (!name) {
                document.getElementById('newError').textContent = 'Please enter a workflow name';
                return;
            }

            document.getElementById('workflowEditor').value = '# New Workflow\nname: ' + name + '\ndescription: \nsteps:\n';
            document.getElementById('workflowName').value = name;
            currentWorkflow = name;
            hideNewModal();
            document.getElementById('deleteBtn').style.display = 'none';
        }

        function escapeHtml(text) {
            return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
        }

        function syntaxHighlight(json) {
            json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
            return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
                let cls = 'number';
                if (/^"/.test(match)) {
                    if (/:$/.test(match)) cls = 'key';
                    else cls = 'string';
                } else if (/true|false/.test(match)) cls = 'boolean';
                else if (/null/.test(match)) cls = 'null';
                return '<span style="color:' + (cls === 'key' ? '#00d4ff' : cls === 'string' ? '#00ff88' : cls === 'number' ? '#ffaa00' : cls === 'boolean' ? '#ff4757' : '#888') + '">' + match + '</span>';
            });
        }

        document.getElementById('workflowEditor').addEventListener('input', () => {
            document.getElementById('validationStatus').textContent = 'Not validated';
            document.getElementById('validationStatus').className = '';
        });

        loadWorkflows();
    </script>
</body>
</html>`
