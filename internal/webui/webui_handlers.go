// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​​‌​‌‌‌‌​‌‌​‌‌​​​​‌‌‌‌​‌‌​​​​‌‌​‌‌​‌‌‌​‌​​​​​​‌​​​​​​​​​​​​​​​​​​​‌‌​​​​‌​‌‌‌​​‌​⁠
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
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/visualizer"
	"github.com/alib8b8/aflare/internal/workflow"
)

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
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated webui asset path
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
