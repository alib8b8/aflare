// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​​​‌‌‌​​‌​‌​​​‌​​‌​‌​‌‌​​​​‌‌​​‌‌‌​​​‌‌​‌‌‌​​​​​​​​​​​​​​​​​‌​‌‌​​‌‌​‌​‌​‌​‌⁠
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
	"fmt"
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

// sseEvent is the JSON payload carried inside each SSE data: line.
type sseEvent struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Response  string `json:"response,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// writeSSE encodes ev as a single SSE data: line and flushes it to the client.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, ev sseEvent) {
	data, _ := json.Marshal(ev)
	fmt.Fprintf(w, "data: %s\n\n", data) //nolint:errcheck // best-effort SSE write
	flusher.Flush()
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

// handleChatStream processes a chat message with SSE streaming.
// POST /api/chat/stream
//
// Emits Server-Sent Events:
//
//	data: {"type":"session","session_id":"..."}
//	data: {"type":"chunk","content":"token text"}
//	... (more chunks)
//	data: {"type":"done","response":"full response if not streamed","session_id":"..."}
//
// If the provider suppresses streaming, no "chunk" events are emitted and
// the full response arrives in the "done" event. On error, an "error" event
// is emitted instead of "done".
func (s *WebUIServer) handleChatStream(w http.ResponseWriter, r *http.Request) {
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

	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if sessionID == "" {
		sessionID = "default"
	}

	if req.Reset {
		s.sessions.Reset(sessionID)
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx/proxy buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send session_id so the client can store it before chunks arrive.
	writeSSE(w, flusher, sseEvent{Type: "session", SessionID: sessionID})

	session := s.sessions.GetOrCreate(sessionID)

	var streamed bool
	onChunk := func(chunk string) {
		streamed = true
		writeSSE(w, flusher, sseEvent{Type: "chunk", Content: chunk})
	}

	// r.Context() is cancelled when the client disconnects, which cancels
	// the agent's LLM call via SendMessageStream's derived context.
	response, err := session.SendMessageStream(r.Context(), req.Message, onChunk)

	if err != nil {
		writeSSE(w, flusher, sseEvent{Type: "error", Error: err.Error(), SessionID: sessionID})
		return
	}

	// If nothing was streamed (e.g. ollama suppressed JSON), send the full
	// response as a single chunk so the client sees the answer.
	if !streamed && response != "" {
		writeSSE(w, flusher, sseEvent{Type: "chunk", Content: response})
	}

	writeSSE(w, flusher, sseEvent{Type: "done", Response: response, SessionID: sessionID})
}
