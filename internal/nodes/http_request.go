// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​‌​‌​​‌​‌‌‌‌​​​‌‌‌​‌‌​​‌​‌​‌​‌​​​​​‌‌​​​‌‌‌​​‌​​​​​​​​​​​​​​​​​‌​‌‌​​​‌​‌​​‌‌‌⁠
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

package nodes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

type HTTPRequestNode struct{}

func init() {
	Register(&HTTPRequestNode{})
}

func (n *HTTPRequestNode) Name() string {
	return "http_request"
}

func (n *HTTPRequestNode) Description() string {
	return "Make HTTP requests with custom method, headers, and body"
}

func (n *HTTPRequestNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "http_request",
		Description: "Make HTTP requests with custom method, headers, and body",
		Input:       "string - request body (overrides body param)",
		Output:      "string - response body",
		Params: []ParamSchema{
			{Name: "url", Type: "string", Description: "Target URL", Required: true},
			{Name: "method", Type: "string", Description: "HTTP method: GET, POST, PUT, DELETE, PATCH", Required: false, Default: "GET"},
			{Name: "headers", Type: "string", Description: "JSON-encoded headers", Required: false},
			{Name: "body", Type: "string", Description: "Request body", Required: false},
			{Name: "timeout", Type: "int", Description: "Request timeout in seconds", Required: false, Default: "30"},
			{Name: "rate_limit_rps", Type: "float", Description: "Max requests per second per host (0=unlimited)", Required: false, Default: "0"},
			{Name: "rate_limit_burst", Type: "int", Description: "Token-bucket burst size (default=ceil(rate_limit_rps))", Required: false},
			{Name: "rate_limit_key", Type: "string", Description: "Explicit bucket key overriding URL.Host; set when multiple domain aliases resolve to the same backend so they share one bucket (M-9)", Required: false},
			{Name: "max_retries", Type: "int", Description: "Max retry attempts on transient failures (default 0=no retry)", Required: false, Default: "0"},
			{Name: "retry_backoff_ms", Type: "int", Description: "Initial retry backoff in ms (default 100)", Required: false, Default: "100"},
			{Name: "retry_max_backoff_ms", Type: "int", Description: "Max retry backoff cap in ms (default 5000)", Required: false, Default: "5000"},
			{Name: "retry_on_status", Type: "string", Description: "Comma-separated retryable status codes (default 429,500,502,503,504)", Required: false, Default: "429,500,502,503,504"},
		},
	}
}

// sensitiveHeaders that should not be set by workflow params for security reasons.
// Host header is blocked to prevent Host header attacks.
// Authorization and other auth headers are allowed because users need to call
// authenticated APIs (GitHub, OpenAI, etc.) - it's their own workflow.
var sensitiveHeaders = map[string]bool{
	"host": true,
}

func (n *HTTPRequestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	url, ok := params["url"]
	if !ok || url == "" {
		return "", fmt.Errorf("url parameter is required")
	}

	if err := validateURL(url); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	method, ok := params["method"]
	if !ok || method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	// Validate HTTP method
	allowedMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true,
		"DELETE": true, "PATCH": true, "HEAD": true,
	}
	if !allowedMethods[method] {
		return "", fmt.Errorf("HTTP method %q is not allowed", method)
	}

	body := input
	if customBody, ok := params["body"]; ok && customBody != "" {
		body = customBody
	}

	// Build headers once up front. This validates CRLF injection and the
	// sensitive-header policy a single time rather than on every retry
	// attempt, and produces an immutable http.Header we clone onto each
	// rebuilt request (the request body reader is consumed by client.Do,
	// so a fresh *http.Request must be constructed per attempt).
	headers, err := buildRequestHeaders(params, method, body)
	if err != nil {
		return "", err
	}

	// Parse timeout: accept both "30" (seconds) and "30s" (duration)
	timeout := 30 * time.Second
	if timeoutStr, ok := params["timeout"]; ok && timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil && t > 0 && t <= 5*time.Minute {
			timeout = t
		}
	}

	// Use safeHTTPClient with DNS rebinding protection, custom timeout and redirect validation
	client := &http.Client{
		Timeout:       timeout,
		Transport:     safeHTTPClient.Transport,
		CheckRedirect: httpRedirectValidator(validateURL),
	}

	// Optional rate-limit + retry policy. The zero value (no params set)
	// reproduces the original single-shot behavior exactly: no token-bucket
	// wait and a single attempt with no retry.
	rlCfg := parseRateLimitConfig(params)
	reqURL, err := neturl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	// M-9: pass rlCfg.rateLimitKey so an explicit bucket key (when set)
	// overrides URL.Host, merging domain aliases that resolve to the same
	// backend into one token bucket.
	limiter := getHTTPRateLimiter(reqURL, rlCfg.rps, rlCfg.burst, rlCfg.rateLimitKey)
	// host is used for log lines; the limiter bucket key (which may differ
	// when rate_limit_key is set) is internal to getHTTPRateLimiter.
	host := reqURL.Host

	hasBody := body != "" && (method == "POST" || method == "PUT" || method == "PATCH")

	var lastErr error
	var lastStatus int
	for attempt := 0; attempt <= rlCfg.maxRetries; attempt++ {
		// Token-bucket rate limit (per host). Wait() honors ctx so a
		// cancelled workflow returns immediately instead of parking on a
		// token.
		if err := rateLimitedWait(ctx, limiter, host); err != nil {
			return "", fmt.Errorf("rate limit wait interrupted: %w", err)
		}

		var reqBody io.Reader
		if hasBody {
			reqBody = bytes.NewBufferString(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header = headers.Clone()

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			lastStatus = 0
			if attempt < rlCfg.maxRetries {
				backoff := rlCfg.backoffForAttempt(attempt + 1)
				logger.Warn("HTTP request failed, retrying",
					"host", host, "attempt", attempt+1, "max_retries", rlCfg.maxRetries,
					"backoff_ms", backoff.Milliseconds(), "error", err)
				if err := sleepCtx(ctx, backoff); err != nil {
					return "", fmt.Errorf("retry interrupted: %w", err)
				}
				continue
			}
			return "", fmt.Errorf("HTTP request failed: %w", err)
		}

		lastStatus = resp.StatusCode
		// Retryable status with a retry policy configured. Two sub-cases:
		//   - retries remaining -> back off and retry;
		//   - retries exhausted -> fail with an error that records how many
		//     retries were attempted (distinct from the plain non-2xx error
		//     so callers can tell a transient storm from a hard failure).
		// The maxRetries>0 guard preserves the original single-shot behavior
		// when no retry policy is set: a 500/503 with max_retries=0 falls
		// through to readHTTPResponse and yields the plain
		// "HTTP request failed with status %d" message.
		if rlCfg.maxRetries > 0 && rlCfg.shouldRetryStatus(resp.StatusCode) {
			if attempt < rlCfg.maxRetries {
				resp.Body.Close()
				backoff := rlCfg.backoffForAttempt(attempt + 1)
				logger.Warn("HTTP request returned retryable status, retrying",
					"host", host, "status", resp.StatusCode,
					"attempt", attempt+1, "max_retries", rlCfg.maxRetries,
					"backoff_ms", backoff.Milliseconds())
				if err := sleepCtx(ctx, backoff); err != nil {
					return "", fmt.Errorf("retry interrupted: %w", err)
				}
				continue
			}
			resp.Body.Close()
			return "", fmt.Errorf("HTTP request failed with status %d (after %d retries)", resp.StatusCode, rlCfg.maxRetries)
		}

		// Non-retryable outcome (2xx success or a 4xx/other client error).
		// Identical to the original single-shot path, preserving backward
		// compatibility for the no-config case.
		result, err := readHTTPResponse(resp)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		// http_request 也是出站流量：成功返回前上报响应体大小（best-effort，
		// 监控器为 nil 即关闭时 no-op）。
		if m := GetGlobalOutboundMonitor(); m != nil {
			m.Record(len(result))
		}
		return result, nil
	}

	// Unreachable in practice: every loop iteration either returns or
	// continues, and the final iteration (attempt == maxRetries) cannot
	// continue. Kept as a defensive fallback so the function has a total
	// return.
	if lastStatus != 0 {
		return "", fmt.Errorf("HTTP request failed with status %d (after %d retries)", lastStatus, rlCfg.maxRetries)
	}
	if lastErr != nil {
		return "", fmt.Errorf("HTTP request failed: %w", lastErr)
	}
	return "", fmt.Errorf("HTTP request failed without a response")
}

// buildRequestHeaders parses the optional "headers" and "content_type"
// params into a validated http.Header. It applies the same CRLF-injection
// and sensitive-header policy that was previously inlined in Execute, so
// the security boundary is unchanged; it is factored out so the result can
// be reused across retry attempts without re-parsing.
func buildRequestHeaders(params map[string]string, method, body string) (http.Header, error) {
	h := http.Header{}

	if contentType, ok := params["content_type"]; ok && contentType != "" {
		h.Set("Content-Type", contentType)
	} else if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		h.Set("Content-Type", "application/json")
	}

	raw, ok := params["headers"]
	if !ok || raw == "" {
		return h, nil
	}
	for _, headerLine := range strings.Split(raw, "\n") {
		headerLine = strings.TrimSpace(headerLine)
		if headerLine == "" {
			continue
		}
		parts := strings.SplitN(headerLine, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Reject CRLF injection and sensitive headers
			if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
				return nil, fmt.Errorf("CRLF characters are not allowed in headers")
			}
			if sensitiveHeaders[strings.ToLower(key)] {
				return nil, fmt.Errorf("setting sensitive header %q is not allowed", key)
			}
			h.Set(key, value)
		}
	}
	return h, nil
}

// readHTTPResponse reads the response body (capped at maxHTTPResponseSize to
// prevent OOM) and returns the formatted "HTTP <code>\n<body>" string on a
// 2xx response, or an error mirroring the original non-2xx failure message
// for backward compatibility.
func readHTTPResponse(resp *http.Response) (string, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}
	// Limit response body size to prevent OOM
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody)), nil
}

// sleepCtx sleeps for d while honoring ctx cancellation. Returns ctx.Err()
// if the context is cancelled before the timer fires. A non-positive d is a
// no-op. Used to back off between retry attempts without blocking a
// cancelled workflow.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
