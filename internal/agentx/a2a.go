// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌‌‌​​‌‌​​​‌​‌​​‌‌​​‌​‌​‌​‌​‌​‌‌​​‌​‌​​​​‌​‌‌‌‌​‌‌​​​​​​​​​​​​​​​​‌​​‌​‌​‌‌‌‌​‌​​​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package agentx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/httpclient"
)

// a2aHTTPClient is the A2A client's shared HTTP client, built on the
// httpclient factory for SSRF defense (re-resolve + validate at dial
// time). A2A agents commonly run on localhost or intranet hosts, so
// loopback is allowed like in the MCP client.
var a2aHTTPClient = httpclient.NewClient(httpclient.Options{
	Timeout:   60 * time.Second,
	Validator: httpclient.ValidateAllowLoopback,
})

// maxA2AResponseSize bounds how much of an A2A server's response body is
// read, mirroring the MCP client's cap.
const maxA2AResponseSize = 10 * 1024 * 1024

// a2aPollInterval is the delay between tasks/get polls.
const a2aPollInterval = 2 * time.Second

// a2aRetryDelays are the backoff delays between retry attempts for
// transient transport failures.
var a2aRetryDelays = []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}

// isA2ADialError reports whether err is a connection-establishment
// failure. The request never reached the server, so retrying even a
// non-idempotent submit cannot duplicate work.
func isA2ADialError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// isA2ARetryableReadError reports whether err is transient and safe to
// retry for an idempotent read (tasks/get): any transport failure, or
// an HTTP 5xx from the server side.
func isA2ARetryableReadError(err error) bool {
	if isA2ADialError(err) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var statusErr *a2aHTTPStatusError
	return errors.As(err, &statusErr) && statusErr.Status >= 500
}

// a2aCallRetried performs one JSON-RPC call with bounded retries for
// transient failures. retryable decides which errors may be retried:
// submits pass isA2ADialError (pre-delivery only), idempotent reads
// pass isA2ARetryableReadError.
func a2aCallRetried[T any](ctx context.Context, def AgentDef, base, method string, params map[string]any, retryable func(error) bool) (*T, error) {
	for attempt := 0; ; attempt++ {
		resp, err := a2aCall[T](ctx, def, base, method, params)
		if err == nil {
			return resp, nil
		}
		if attempt >= len(a2aRetryDelays) || !retryable(err) {
			return nil, err
		}
		select {
		case <-time.After(a2aRetryDelays[attempt]):
		case <-ctx.Done():
			return nil, err
		}
	}
}

// AgentCard is the subset of the A2A agent card aflare consumes.
type AgentCard struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	Version     string       `json:"version,omitempty"`
	Skills      []AgentSkill `json:"skills,omitempty"`
}

// AgentSkill is one capability advertised by an A2A agent.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// FetchAgentCard retrieves the agent card from the well-known endpoints.
// Newer servers serve agent-card.json; the older draft used agent.json.
func FetchAgentCard(ctx context.Context, def AgentDef) (*AgentCard, error) {
	def, err := def.Resolve()
	if err != nil {
		return nil, err
	}
	base, err := parseA2AURL(def.URL)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, path := range []string{"/.well-known/agent-card.json", "/.well-known/agent.json"} {
		card, err := a2aGetJSON[AgentCard](ctx, def, base+path)
		if err == nil {
			return card, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("agent %q: no agent card found: %w", def.Name, lastErr)
}

// a2aTask mirrors the A2A task object across its lifecycle.
type a2aTask struct {
	ID        string        `json:"id"`
	ContextID string        `json:"contextId"`
	Status    a2aStatus     `json:"status"`
	Artifacts []a2aArtifact `json:"artifacts,omitempty"`
}

type a2aStatus struct {
	State   string      `json:"state"`
	Message *a2aMessage `json:"message,omitempty"`
}

type a2aArtifact struct {
	Name  string    `json:"name,omitempty"`
	Parts []a2aPart `json:"parts,omitempty"`
}

type a2aMessage struct {
	Role  string    `json:"role"`
	Parts []a2aPart `json:"parts,omitempty"`
}

type a2aPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// terminal states per the A2A task lifecycle.
var a2aTerminalStates = map[string]bool{
	"completed": true,
	"failed":    true,
	"canceled":  true,
	"rejected":  true,
}

// SendMessage delegates one task to an A2A agent and blocks until the
// task reaches a terminal state or the task timeout elapses. It first
// tries the current method name (message/send) and falls back to the
// older draft name (tasks/send), then polls tasks/get.
func SendMessage(ctx context.Context, def AgentDef, t Task) (string, error) {
	def, err := def.Resolve()
	if err != nil {
		return "", err
	}

	prompt := strings.TrimSpace(t.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("agent %q requires a non-empty prompt", def.Name)
	}
	if len(prompt) > maxPromptChars {
		return "", fmt.Errorf("prompt too long (%d chars, max %d)", len(prompt), maxPromptChars)
	}

	base, err := parseA2AURL(def.URL)
	if err != nil {
		return "", err
	}

	timeout := t.resolveTimeout()
	timeoutStr := formatDuration(timeout)

	if t.Audit != nil {
		apiKeyEnv := def.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = "(none)"
		}
		entry := fmt.Sprintf("agentx a2a agent=%s url=%s api_key_env=%s timeout=%s", def.Name, base, apiKeyEnv, timeoutStr)
		if err := t.Audit(entry); err != nil {
			return "", fmt.Errorf("agent %q: failed to write audit log: %w", def.Name, err)
		}
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	task, err := a2aSendTask(taskCtx, def, base, prompt)
	if err != nil {
		return "", err
	}

	// Some servers return the final state directly; otherwise poll.
	task, err = a2aAwaitTerminal(taskCtx, def, base, task)
	if err != nil {
		return "", err
	}

	switch task.Status.State {
	case "completed":
		return extractTaskText(task), nil
	case "canceled":
		return "", fmt.Errorf("agent %q: task %s was canceled", def.Name, task.ID)
	case "rejected":
		return "", fmt.Errorf("agent %q: task %s was rejected", def.Name, task.ID)
	default: // failed and anything unknown
		msg := ""
		if task.Status.Message != nil {
			msg = extractMessageText(*task.Status.Message)
		}
		if msg != "" {
			return "", fmt.Errorf("agent %q: task %s failed: %s", def.Name, task.ID, msg)
		}
		return "", fmt.Errorf("agent %q: task %s failed (state %q)", def.Name, task.ID, task.Status.State)
	}
}

// a2aSendTask posts the delegation using JSON-RPC 2.0, trying the
// current method (message/send) first and the older draft (tasks/send)
// second.
func a2aSendTask(ctx context.Context, def AgentDef, base, prompt string) (*a2aTask, error) {
	params := map[string]any{
		"message": map[string]any{
			"role": "user",
			"parts": []map[string]any{
				{"kind": "text", "text": prompt},
			},
		},
	}
	var lastErr error
	for _, method := range []string{"message/send", "tasks/send"} {
		// Submit retry policy: only pre-delivery dial errors — the
		// request never reached the agent, so a retry cannot duplicate
		// the delegated task.
		resp, err := a2aCallRetried[a2aTask](ctx, def, base, method, params, isA2ADialError)
		if err == nil {
			if resp.ID == "" {
				return nil, fmt.Errorf("agent %q: %s returned a task without id", def.Name, method)
			}
			return resp, nil
		}
		lastErr = err
		// Only retry the fallback method on method-not-found style
		// errors; other failures (auth, network) won't improve.
		if !isMethodNotFoundError(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// a2aAwaitTerminal polls tasks/get until the task reaches a terminal
// state, the context deadline hits, or the server errors.
func a2aAwaitTerminal(ctx context.Context, def AgentDef, base string, task *a2aTask) (*a2aTask, error) {
	for {
		if a2aTerminalStates[task.Status.State] {
			return task, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("agent %q: timed out waiting for task %s (last state %q)", def.Name, task.ID, task.Status.State)
		case <-time.After(a2aPollInterval):
		}
		// tasks/get is an idempotent read, so any transient transport
		// failure or server-side 5xx may be retried without risking
		// duplicate side effects — a single failed poll used to kill
		// the whole delegation.
		updated, err := a2aCallRetried[a2aTask](ctx, def, base, "tasks/get", map[string]any{"id": task.ID}, isA2ARetryableReadError)
		if err != nil {
			return nil, fmt.Errorf("agent %q: tasks/get for %s failed: %w", def.Name, task.ID, err)
		}
		if updated.ID == "" {
			updated.ID = task.ID
		}
		task = updated
	}
}

// extractTaskText pulls the delegation result out of a terminal task:
// artifacts first (the agent's work products), then the status message.
func extractTaskText(task *a2aTask) string {
	var texts []string
	for _, artifact := range task.Artifacts {
		if s := strings.TrimSpace(extractPartsText(artifact.Parts)); s != "" {
			if artifact.Name != "" {
				texts = append(texts, fmt.Sprintf("[%s]\n%s", artifact.Name, s))
			} else {
				texts = append(texts, s)
			}
		}
	}
	if len(texts) > 0 {
		return strings.Join(texts, "\n\n")
	}
	if task.Status.Message != nil {
		if s := strings.TrimSpace(extractMessageText(*task.Status.Message)); s != "" {
			return s
		}
	}
	return ""
}

func extractMessageText(msg a2aMessage) string {
	return extractPartsText(msg.Parts)
}

func extractPartsText(parts []a2aPart) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Kind == "text" && p.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// a2aCall performs one JSON-RPC 2.0 call against the A2A service
// endpoint and decodes the result field of the response.
func a2aCall[T any](ctx context.Context, def AgentDef, base, method string, params map[string]any) (*T, error) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if def.APIKeyEnv != "" {
		if key := strings.TrimSpace(os.Getenv(def.APIKeyEnv)); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	resp, err := a2aHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxA2AResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &a2aHTTPStatusError{Method: method, Status: resp.StatusCode, Body: truncateForError(respBody)}
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		err := fmt.Errorf("jsonrpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		if isA2AMethodNotFound(rpcResp.Error.Code, rpcResp.Error.Message) {
			return nil, &methodNotFoundError{err}
		}
		return nil, err
	}
	if len(rpcResp.Result) == 0 {
		return nil, fmt.Errorf("%s returned no result", method)
	}
	var result T
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode %s result: %w", method, err)
	}
	return &result, nil
}

// methodNotFoundError marks a JSON-RPC -32601 so the caller can fall
// back to the older A2A method name.
type methodNotFoundError struct{ err error }

func (e *methodNotFoundError) Error() string { return e.err.Error() }
func (e *methodNotFoundError) Unwrap() error { return e.err }

// a2aHTTPStatusError marks a non-200 A2A response so transient server
// errors (5xx) can be recognized and retried on idempotent reads.
type a2aHTTPStatusError struct {
	Method string
	Status int
	Body   string
}

func (e *a2aHTTPStatusError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d: %s", e.Method, e.Status, e.Body)
}

func isMethodNotFoundError(err error) bool {
	var mnf *methodNotFoundError
	return errors.As(err, &mnf)
}

func isA2AMethodNotFound(code int, message string) bool {
	if code == -32601 {
		return true
	}
	return strings.Contains(strings.ToLower(message), "method not found")
}

// a2aGetJSON fetches and decodes a JSON document (agent card endpoints).
func a2aGetJSON[T any](ctx context.Context, def AgentDef, urlStr string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if def.APIKeyEnv != "" {
		if key := strings.TrimSpace(os.Getenv(def.APIKeyEnv)); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := a2aHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxA2AResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("invalid agent card JSON: %w", err)
	}
	return &out, nil
}

// parseA2AURL validates the A2A service endpoint: http/https only, a
// host must be present, no userinfo, no fragment.
func parseA2AURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid a2a url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid a2a url %q: scheme must be http or https", raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid a2a url %q: host is required", raw)
	}
	if u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("invalid a2a url %q: userinfo and fragments are not allowed", raw)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func truncateForError(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
