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

package webui

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/alib8b8/aflare/internal/agent"
)

// chatHandler manages the chat session for the WebUI.
type chatHandler struct {
	mu           sync.Mutex
	session      *agent.ChatSession
	capabilities []string
}

// setCapabilities sets the capability names to enable for the chat session.
func (h *chatHandler) setCapabilities(caps []string) {
	h.capabilities = caps
}

// getOrCreateSession returns the existing chat session or creates a new one.
func (h *chatHandler) getOrCreateSession() *agent.ChatSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.session == nil {
		cfg := agent.DefaultConfig()
		cfg.Capabilities = h.capabilities
		h.session = agent.NewChatSession(cfg)
	}
	return h.session
}

// resetSession clears the current chat session.
func (h *chatHandler) resetSession() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.session != nil {
		h.session.ResetSession()
	}
}

// chatRequest is the JSON request body for the chat endpoint.
type chatRequest struct {
	Message string `json:"message"`
	Reset   bool   `json:"reset,omitempty"`
}

// chatResponse is the JSON response for the chat endpoint.
type chatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// handleChat processes a chat message through the ReActAgent.
// POST /api/chat
func (s *WebUIServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	// Reset session if requested
	if req.Reset {
		s.chat.resetSession()
	}

	session := s.chat.getOrCreateSession()
	response, err := session.SendMessage(req.Message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(chatResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{Response: response})
}