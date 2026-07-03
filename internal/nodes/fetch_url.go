package nodes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchURLNode fetches content from a URL
type FetchURLNode struct{}

func init() {
	Register(&FetchURLNode{})
}

// Name returns the node name
func (n *FetchURLNode) Name() string {
	return "fetch_url"
}

func (n *FetchURLNode) Description() string {
	return "Fetch content from a URL"
}

func (n *FetchURLNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "fetch_url",
		Description: "Fetch content from a URL",
		Input:       "string - optional URL (overrides url param)",
		Output:      "string - content of the URL",
		Params: []ParamSchema{
			{Name: "url", Type: "string", Description: "URL to fetch", Required: false},
			{Name: "mode", Type: "string", Description: "Extraction mode: text, markdown, html, main_content", Required: false, Default: "text"},
			{Name: "timeout", Type: "int", Description: "Request timeout in seconds", Required: false, Default: "30"},
		},
	}
}

// looksLikeURL checks if a string looks like a URL
func looksLikeURL(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Execute implements the Node interface
func (n *FetchURLNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Get URL from input if it looks like a URL, otherwise use params
	var url string
	if input != "" && looksLikeURL(input) {
		url = input
	} else {
		url, _ = params["url"]
	}

	if url == "" {
		return "", fmt.Errorf("url parameter is required, or pass a URL as input")
	}

	if err := validateURL(url); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "llm-box/1.0")

	// Set up HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received status %d from URL", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
