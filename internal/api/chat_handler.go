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
	"encoding/json"
	"net/http"
	"strings"
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

// extractSessionID extracts the session ID from the request. Priority:
// 1. X-Session-Id header
// 2. session_id field in JSON body
// 3. URL path segment (e.g. /api/v1/chat/{sessionId})
func extractSessionID(r *http.Request) string {
	if id := r.Header.Get("X-Session-Id"); id != "" {
		return id
	}
	// Try URL path: /api/v1/chat/{sessionId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chat")
	path = strings.TrimPrefix(path, "/")
	if path != "" {
		return path
	}
	return ""
}

// handleChat processes a chat message through the ReActAgent.
// POST /api/v1/chat[/{sessionId}]
// Session is identified by X-Session-Id header, session_id in body, or URL path.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
		return
	}

	if req.Message == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "message is required",
		})
		return
	}

	// Determine session ID
	sessionID := extractSessionID(r)
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
		s.writeJSON(w, http.StatusInternalServerError, chatResponse{
			SessionID: sessionID,
			Error:     err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, chatResponse{
		Response:  response,
		SessionID: sessionID,
	})
}