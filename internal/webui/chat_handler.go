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
)

// chatRequest is the JSON request body for the chat endpoint.
type chatRequest struct {
	Message   string `json:"message"`
	Reset     bool   `json:"reset,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// chatResponse is the JSON response for the chat endpoint.
type chatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleChat processes a chat message through the ReActAgent.
// POST /api/chat
// Session is identified by X-Session-Id header or session_id in body.
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

	// Determine session ID
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if sessionID == "" {
		sessionID = "default"
	}

	// Reset session if requested
	if req.Reset {
		s.sessions.Reset(sessionID)
	}

	session := s.sessions.GetOrCreate(sessionID)
	response, err := session.SendMessage(req.Message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(chatResponse{SessionID: sessionID, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{Response: response, SessionID: sessionID})
}