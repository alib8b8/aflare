// Package main implements a Remote MCP server for Grok integration.
// It wraps the existing llm-box stdio MCP server with HTTP/SSE transport,
// making it accessible from grok.com/connectors as a Remote MCP.
//
// Usage:
//
//	go run ./grok-mcp-server [--port 8080]
//	llm-box --mcp-remote --port 8080
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Port defaults to 8080
var port = 8080

func main() {
	if p := os.Getenv("LLM_BOX_MCP_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			port = v
		}
	}

	mux := http.NewServeMux()

	// SSE endpoint for Grok Remote MCP
	mux.HandleFunc("/sse", handleSSE)

	// Streamable HTTP endpoint (MCP spec 2025-03-26)
	mux.HandleFunc("/mcp", handleStreamableHTTP)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			log.Printf("Error encoding health response: %v", err)
		}
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("llm-box Remote MCP server listening on %s", addr)
	log.Printf("SSE endpoint:      http://localhost%s/sse", addr)
	log.Printf("Streamable HTTP:   http://localhost%s/mcp", addr)
	log.Printf("Health check:      http://localhost%s/health", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleSSE implements the SSE transport for Remote MCP.
// The client connects via GET /sse to receive events, and sends
// messages via POST /sse with a session ID.
func handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		handleSSEConnect(w, r)
	} else if r.Method == http.MethodPost {
		handleSSEMessage(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type sseSession struct {
	id      string
	events  chan string
	created time.Time
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	cmdMu   sync.Mutex
}

var (
	sessions   = make(map[string]*sseSession)
	sessionsMu sync.RWMutex
)

func handleSSEConnect(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	events := make(chan string, 100)

	session := &sseSession{
		id:      sessionID,
		events:  events,
		created: time.Now(),
	}

	sessionsMu.Lock()
	sessions[sessionID] = session
	sessionsMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		sessionsMu.Lock()
		delete(sessions, sessionID)
		sessionsMu.Unlock()
		session.cmdMu.Lock()
		if session.cmd != nil && session.cmd.Process != nil {
			if err := session.cmd.Process.Kill(); err != nil {
				log.Printf("Error killing MCP process: %v", err)
			}
		}
		session.cmdMu.Unlock()
	}()

	// Start the stdio MCP server process
	cmd := exec.Command("llm-box", "--mcp-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stdin pipe: %v", err), http.StatusInternalServerError)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stdout pipe: %v", err), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start llm-box: %v", err), http.StatusInternalServerError)
		return
	}

	session.cmdMu.Lock()
	session.cmd = cmd
	session.stdin = stdin
	session.cmdMu.Unlock()

	// Read stdout from llm-box and forward as SSE events
	go func() {
		decoder := json.NewDecoder(stdout)
		for {
			var msg json.RawMessage
			if err := decoder.Decode(&msg); err != nil {
				events <- fmt.Sprintf("event: error\ndata: %s\n\n", quoteJSON(err.Error()))
				return
			}
			events <- fmt.Sprintf("event: message\ndata: %s\n\n", string(msg))
		}
	}()

	// Send the endpoint event with session ID
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	fmt.Fprintf(w, "event: endpoint\ndata: /sse?sessionId=%s\n\n", sessionID)
	flusher.Flush()

	// Forward events to client
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			fmt.Fprint(w, event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			// Keepalive
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing sessionId", http.StatusBadRequest)
		return
	}

	sessionsMu.RLock()
	session, ok := sessions[sessionID]
	sessionsMu.RUnlock()

	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session.cmdMu.Lock()
	stdin := session.stdin
	session.cmdMu.Unlock()

	if stdin == nil {
		http.Error(w, "MCP server not running", http.StatusInternalServerError)
		return
	}

	var msg json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Forward to llm-box stdin
	if _, err := stdin.Write(append(msg, '\n')); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write to MCP server: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleStreamableHTTP implements the Streamable HTTP transport for MCP.
// A single /mcp endpoint accepts POST requests with JSON-RPC messages.
func handleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the JSON-RPC request
	var req json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"error":   map[string]interface{}{"code": -32700, "message": "Parse error"},
		}); err != nil {
			log.Printf("Error encoding error response: %v", err)
		}
		return
	}

	// Start llm-box stdio MCP server for this request
	cmd := exec.Command("llm-box", "--mcp-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stdin pipe: %v", err), http.StatusInternalServerError)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stdout pipe: %v", err), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start llm-box: %v", err), http.StatusInternalServerError)
		return
	}
	defer cmd.Wait()

	// Check if this is a batch request
	reqStr := string(req)
	var messages []json.RawMessage

	if strings.HasPrefix(strings.TrimSpace(reqStr), "[") {
		if err := json.Unmarshal(req, &messages); err != nil {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"error":   map[string]interface{}{"code": -32600, "message": "Invalid request"},
			}); err != nil {
				log.Printf("Error encoding error response: %v", err)
			}
			return
		}
	} else {
		messages = []json.RawMessage{req}
	}

	// Send initialize if needed, then process messages
	// For simplicity, we send each message and collect responses
	var responses []json.RawMessage

	for _, msg := range messages {
		if _, err := stdin.Write(append(msg, '\n')); err != nil {
			log.Printf("Error writing to stdin: %v", err)
			continue
		}
	}

	// Close stdin to signal we're done sending
	if err := stdin.Close(); err != nil {
		log.Printf("Error closing stdin: %v", err)
	}

	// Read all responses
	decoder := json.NewDecoder(stdout)
	for {
		var resp json.RawMessage
		if err := decoder.Decode(&resp); err != nil {
			break
		}
		responses = append(responses, resp)
	}

	// Return response(s)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if len(responses) == 1 {
		if _, err := w.Write(responses[0]); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	} else if len(responses) > 1 {
		batch, _ := json.Marshal(responses)
		if _, err := w.Write(batch); err != nil {
			log.Printf("Error writing batch response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
