package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client implements an MCP client that communicates with a remote MCP server
// using JSON-RPC 2.0 over HTTP SSE or Streamable HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client

	// SSE-specific fields
	sseEndpoint   string
	sseReader     io.ReadCloser
	sseScanner    *bufio.Scanner
	pending       map[int]chan *rpcResponse
	pendingMu     sync.RWMutex
	sseDone       chan struct{}
	sseWg         sync.WaitGroup
	idCounter     atomic.Int32
	isStreamable  bool
	isInitialized bool
}

// Connect establishes a connection to an MCP server at the given URL.
// The URL should point to either an SSE endpoint (e.g., http://host/sse)
// or a Streamable HTTP endpoint (e.g., http://host/mcp).
func Connect(url string) (*Client, error) {
	c := &Client{
		baseURL:    strings.TrimRight(url, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		pending:    make(map[int]chan *rpcResponse),
		sseDone:    make(chan struct{}),
	}

	if strings.HasSuffix(c.baseURL, "/sse") {
		if err := c.connectSSE(); err != nil {
			return nil, fmt.Errorf("failed to connect to SSE endpoint: %w", err)
		}
	} else {
		c.isStreamable = true
	}

	if err := c.initialize(); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return c, nil
}

// connectSSE establishes an SSE connection and reads the endpoint event.
func (c *Client) connectSSE() error {
	resp, err := c.httpClient.Get(c.baseURL)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection returned status %d", resp.StatusCode)
	}

	c.sseReader = resp.Body
	c.sseScanner = bufio.NewScanner(resp.Body)

	// Parse the first events to find the endpoint
	foundEndpoint := false
	for c.sseScanner.Scan() {
		line := c.sseScanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event: endpoint") {
			// Next line should be data: ...
			if c.sseScanner.Scan() {
				dataLine := c.sseScanner.Text()
				if strings.HasPrefix(dataLine, "data: ") {
					endpoint := strings.TrimPrefix(dataLine, "data: ")
					if !strings.HasPrefix(endpoint, "http") {
						endpoint = resolveURL(c.baseURL, endpoint)
					}
					c.sseEndpoint = endpoint
					foundEndpoint = true
					break
				}
			}
		}
	}

	if !foundEndpoint {
		c.sseReader.Close()
		return fmt.Errorf("did not receive endpoint event from SSE stream")
	}

	// Start background reader for SSE messages
	c.sseWg.Add(1)
	go c.sseReaderLoop()

	return nil
}

// sseReaderLoop continuously reads SSE events and dispatches responses.
func (c *Client) sseReaderLoop() {
	defer c.sseWg.Done()
	var currentData strings.Builder

	for {
		select {
		case <-c.sseDone:
			return
		default:
		}

		if !c.sseScanner.Scan() {
			return
		}

		line := c.sseScanner.Text()
		if line == "" {
			// End of event
			if currentData.Len() > 0 {
				c.handleSSEEvent(currentData.String())
				currentData.Reset()
			}
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			if currentData.Len() > 0 {
				currentData.WriteByte('\n')
			}
			currentData.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}
}

// handleSSEEvent parses a single SSE event data payload as JSON-RPC response.
func (c *Client) handleSSEEvent(data string) {
	var resp rpcResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return
	}

	var id int
	if err := json.Unmarshal(resp.ID, &id); err != nil {
		return
	}

	c.pendingMu.RLock()
	ch, ok := c.pending[id]
	c.pendingMu.RUnlock()
	if ok {
		select {
		case ch <- &resp:
		default:
		}
	}
}

// initialize sends the initialize request to the server.
func (c *Client) initialize() error {
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	resp, err := c.sendRequest(&req)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}
	c.isInitialized = true

	// Send initialized notification
	_ = c.sendNotification("notifications/initialized")
	return nil
}

// sendNotification sends a JSON-RPC notification (no response expected).
func (c *Client) sendNotification(method string) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
	}
	if c.isStreamable {
		return c.postStreamable(&req)
	}
	return c.postSSE(&req)
}

// sendRequest sends a JSON-RPC request and waits for the matching response.
func (c *Client) sendRequest(req *rpcRequest) (*rpcResponse, error) {
	if req.ID == nil {
		req.ID = json.RawMessage(fmt.Sprintf(`%d`, c.idCounter.Add(1)))
	}

	var id int
	if err := json.Unmarshal(req.ID, &id); err != nil {
		return nil, fmt.Errorf("invalid request ID: %w", err)
	}

	ch := make(chan *rpcResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if c.isStreamable {
		if err := c.postStreamable(req); err != nil {
			return nil, err
		}
	} else {
		if err := c.postSSE(req); err != nil {
			return nil, err
		}
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timed out after 30s")
	}
}

// postStreamable sends a request via HTTP POST and reads the immediate response.
func (c *Client) postStreamable(req *rpcRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpResp, err := c.httpClient.Post(c.baseURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusAccepted && httpResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	// For streamable HTTP, if there's a response body, we need to handle it.
	// However, since initialize expects a response, we read the body for
	// id-matched requests in a separate path. For notifications we ignore.
	if req.ID == nil {
		return nil
	}

	// Read and dispatch response
	body, err := io.ReadAll(httpResp.Body)
	if err != nil || len(body) == 0 {
		return nil
	}

	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var id int
	if err := json.Unmarshal(resp.ID, &id); err != nil {
		return nil
	}

	c.pendingMu.RLock()
	ch, ok := c.pending[id]
	c.pendingMu.RUnlock()
	if ok {
		select {
		case ch <- &resp:
		default:
		}
	}
	return nil
}

// postSSE sends a request via POST to the SSE endpoint.
func (c *Client) postSSE(req *rpcRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpResp, err := c.httpClient.Post(c.sseEndpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
	}
	return nil
}

// ListTools returns the list of tools exposed by the MCP server.
func (c *Client) ListTools() ([]tool, error) {
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	resp, err := c.sendRequest(&req)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result toolsListResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools list: %w", err)
	}

	return result.Tools, nil
}

// CallTool invokes a remote tool with the given name and arguments.
func (c *Client) CallTool(name string, args map[string]interface{}) (*toolCallResult, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	params := toolCallParams{
		Name:      name,
		Arguments: args,
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp, err := c.sendRequest(&req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tool call error: %s", sanitizeErrorString(resp.Error.Message))
	}

	var result toolCallResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &result, nil
}

// Close terminates the client connection and cleans up resources.
func (c *Client) Close() error {
	if c.sseReader != nil {
		close(c.sseDone)
		c.sseReader.Close()
		c.sseWg.Wait()
	}
	return nil
}

// resolveURL resolves a relative URL against a base URL.
func resolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "/") {
		idx := strings.Index(base, "://")
		if idx == -1 {
			return ref
		}
		slashIdx := strings.Index(base[idx+3:], "/")
		if slashIdx == -1 {
			return base + ref
		}
		return base[:idx+3+slashIdx] + ref
	}
	return base + "/" + ref
}

// sanitizeErrorString redacts sensitive keywords from error messages.
func sanitizeErrorString(msg string) string {
	lower := strings.ToLower(msg)
	sensitive := []string{"token", "key", "secret", "password", "credential"}
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return "tool execution failed (sensitive details redacted)"
		}
	}
	return msg
}
