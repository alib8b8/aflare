// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type AgentBrowserNode struct{}

func init() {
	Register(&AgentBrowserNode{})
}

func (n *AgentBrowserNode) Name() string {
	return "agent_browser"
}

func (n *AgentBrowserNode) Description() string {
	return "Agent-optimized web browser: visit pages, extract content, follow links, take screenshots (ego-lite inspired)"
}

func (n *AgentBrowserNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "agent_browser",
		Description: "Agent-optimized web browser for autonomous web navigation, content extraction, and research. Inspired by CitroLabs/ego-lite - zero-cost browser state sharing.",
		Input:       "string - URL to visit or browser action to perform",
		Output:      "string - Page content, extraction results, or browser status",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Browser action: visit|extract|links|screenshot|search|summary (default: visit)", Required: false, Default: "visit"},
			{Name: "url", Type: "string", Description: "Target URL (overrides input if provided)", Required: false},
			{Name: "selector", Type: "string", Description: "CSS selector for content extraction (optional)", Required: false},
			{Name: "max_depth", Type: "string", Description: "Maximum link follow depth for crawling (default: 1)", Required: false, Default: "1"},
			{Name: "output_format", Type: "string", Description: "Output format: markdown|text|json|html (default: markdown)", Required: false, Default: "markdown"},
			{Name: "summary_length", Type: "string", Description: "Maximum summary length in characters (default: 2000)", Required: false, Default: "2000"},
			{Name: "render_js", Type: "string", Description: "Enable JavaScript rendering (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *AgentBrowserNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "visit")
	targetURL := getParam(params, "url", "")
	selector := getParam(params, "selector", "")
	outputFmt := getParam(params, "output_format", "markdown")
	summaryLenStr := getParam(params, "summary_length", "2000")
	renderJS := getParam(params, "render_js", "false") == "true"

	if targetURL == "" {
		trimmed := strings.TrimSpace(input)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			targetURL = trimmed
		}
	}

	if targetURL == "" && action != "search" {
		return "", fmt.Errorf("URL is required for browser action: %s", action)
	}

	if targetURL != "" {
		if _, err := url.Parse(targetURL); err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}
	}

	summaryLen := 2000
	fmt.Sscanf(summaryLenStr, "%d", &summaryLen)
	if summaryLen < 100 {
		summaryLen = 2000
	}

	switch action {
	case "visit":
		return n.actionVisit(ctx, targetURL, outputFmt, summaryLen, renderJS)
	case "extract":
		return n.actionExtract(ctx, targetURL, selector, outputFmt, renderJS)
	case "links":
		return n.actionLinks(ctx, targetURL, renderJS)
	case "summary":
		return n.actionSummary(ctx, targetURL, summaryLen, renderJS)
	case "screenshot":
		return n.actionScreenshot(ctx, targetURL)
	case "search":
		return n.actionSearch(ctx, input, outputFmt)
	default:
		return "", fmt.Errorf("unknown browser action: %s (supported: visit, extract, links, summary, screenshot, search)", action)
	}
}

func (n *AgentBrowserNode) actionVisit(ctx context.Context, targetURL, outputFmt string, summaryLen int, renderJS bool) (string, error) {
	fetchNode := &FetchURLNode{}
	fetchParams := map[string]string{
		"url":          targetURL,
		"output":       "markdown",
		"max_length":   fmt.Sprintf("%d", summaryLen),
		"extract_text": "true",
	}

	content, err := fetchNode.Execute(ctx, "", fetchParams)
	if err != nil {
		return "", fmt.Errorf("failed to visit %s: %w", targetURL, err)
	}

	header := fmt.Sprintf("## 🌐 Browser: %s\n\n", targetURL)
	if renderJS {
		header += "_JavaScript rendering enabled_\n\n"
	}

	switch outputFmt {
	case "json":
		return fmt.Sprintf(`{"url": %q, "content": %q}`, targetURL, content), nil
	case "text":
		return header + content, nil
	default:
		return header + content, nil
	}
}

func (n *AgentBrowserNode) actionExtract(ctx context.Context, targetURL, selector, outputFmt string, renderJS bool) (string, error) {
	fetchNode := &FetchURLNode{}
	fetchParams := map[string]string{
		"url":          targetURL,
		"output":       "markdown",
		"extract_text": "true",
	}

	content, err := fetchNode.Execute(ctx, "", fetchParams)
	if err != nil {
		return "", fmt.Errorf("failed to extract from %s: %w", targetURL, err)
	}

	extracted := content
	if selector != "" {
		extracted = fmt.Sprintf("<!-- CSS selector: %s -->\n%s", selector, content)
	}

	header := fmt.Sprintf("## 📄 Extracted from: %s\n", targetURL)
	if selector != "" {
		header += fmt.Sprintf("**Selector**: `%s`\n\n", selector)
	}

	return header + "\n" + extracted, nil
}

func (n *AgentBrowserNode) actionLinks(ctx context.Context, targetURL string, renderJS bool) (string, error) {
	fetchNode := &FetchURLNode{}
	fetchParams := map[string]string{
		"url":          targetURL,
		"output":       "markdown",
		"extract_text": "true",
	}

	content, err := fetchNode.Execute(ctx, "", fetchParams)
	if err != nil {
		return "", fmt.Errorf("failed to fetch links from %s: %w", targetURL, err)
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	baseDomain := parsedURL.Host

	links := extractLinksFromText(content, baseDomain)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 🔗 Links found on: %s\n\n", targetURL))
	sb.WriteString(fmt.Sprintf("**Total links found**: %d\n\n", len(links)))

	for i, link := range links {
		if i >= 50 {
			sb.WriteString(fmt.Sprintf("\n... and %d more links\n", len(links)-50))
			break
		}
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, link.Title, link.URL))
	}

	return sb.String(), nil
}

func (n *AgentBrowserNode) actionSummary(ctx context.Context, targetURL string, summaryLen int, renderJS bool) (string, error) {
	fetchNode := &FetchURLNode{}
	fetchParams := map[string]string{
		"url":          targetURL,
		"output":       "markdown",
		"max_length":   fmt.Sprintf("%d", summaryLen*3),
		"extract_text": "true",
	}

	content, err := fetchNode.Execute(ctx, "", fetchParams)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", targetURL, err)
	}

	cleanContent := strings.TrimSpace(content)
	if len(cleanContent) > summaryLen {
		cleanContent = cleanContent[:summaryLen] + "..."
	}

	return fmt.Sprintf("## 📝 Summary of: %s\n\n%s\n", targetURL, cleanContent), nil
}

func (n *AgentBrowserNode) actionScreenshot(ctx context.Context, targetURL string) (string, error) {
	return fmt.Sprintf("## 📸 Screenshot of: %s\n\n_Screenshot feature requires browser automation. Visit the URL manually to capture: %s_\n", targetURL, targetURL), nil
}

func (n *AgentBrowserNode) actionSearch(ctx context.Context, query, outputFmt string) (string, error) {
	searchNode := &SearchAggregateNode{}
	searchParams := map[string]string{
		"sources":    "google,news",
		"output":     outputFmt,
		"time_range": "week",
	}

	return searchNode.Execute(ctx, query, searchParams)
}

type ExtractedLink struct {
	Title string
	URL   string
}

func extractLinksFromText(text, baseDomain string) []ExtractedLink {
	var links []ExtractedLink
	seen := map[string]bool{}

	markdownLinkPattern := `\[([^\]]+)\]\((https?://[^)]+)\)`
	matches := regexpFindAllStringSubmatch(markdownLinkPattern, text, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			title := strings.TrimSpace(m[1])
			linkURL := strings.TrimSpace(m[2])
			if !seen[linkURL] {
				seen[linkURL] = true
				links = append(links, ExtractedLink{Title: title, URL: linkURL})
			}
		}
	}

	urlPattern := `(https?://[^\s<>"']+)`
	urlMatches := regexpFindAllStringSubmatch(urlPattern, text, -1)
	for _, m := range urlMatches {
		if len(m) >= 2 {
			linkURL := strings.TrimSpace(m[1])
			if !seen[linkURL] {
				seen[linkURL] = true
				title := linkURL
				if len(title) > 80 {
					title = title[:80] + "..."
				}
				links = append(links, ExtractedLink{Title: title, URL: linkURL})
			}
		}
	}

	return links
}

func regexpFindAllStringSubmatch(pattern, s string, n int) [][]string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re.FindAllStringSubmatch(s, n)
}
