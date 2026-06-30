package nodes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPRequestNode struct{}

func init() {
	Register(&HTTPRequestNode{})
}

func (n *HTTPRequestNode) Name() string {
	return "http_request"
}

func (n *HTTPRequestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	url, ok := params["url"]
	if !ok || url == "" {
		return "", fmt.Errorf("url parameter is required")
	}

	method, ok := params["method"]
	if !ok || method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	body := input
	if customBody, ok := params["body"]; ok && customBody != "" {
		body = customBody
	}

	var reqBody io.Reader
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		reqBody = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	contentType, ok := params["content_type"]
	if ok && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		req.Header.Set("Content-Type", "application/json")
	}

	headers, ok := params["headers"]
	if ok && headers != "" {
		for _, headerLine := range strings.Split(headers, "\n") {
			headerLine = strings.TrimSpace(headerLine)
			if headerLine == "" {
				continue
			}
			parts := strings.SplitN(headerLine, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				req.Header.Set(key, value)
			}
		}
	}

	timeout := 60 * time.Second
	if timeoutStr, ok := params["timeout"]; ok && timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = t
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(respBody), nil
}
