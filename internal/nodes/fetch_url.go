package nodes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxFetchURLSize = 10 * 1024 * 1024 // 10MB max response body for fetch_url

// Pre-compiled regexps to avoid recompilation on every call
var (
	htmlTagRe      = regexp.MustCompile(`<[^>]+>`)
	htmlScriptRe   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStyleRe    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlNavRe      = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	htmlFooterRe   = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	htmlHeaderRe   = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	htmlNbspRe     = regexp.MustCompile(`&nbsp;`)
	htmlAmpRe      = regexp.MustCompile(`&amp;`)
	htmlLtRe       = regexp.MustCompile(`&lt;`)
	htmlGtRe       = regexp.MustCompile(`&gt;`)
	htmlQuotRe     = regexp.MustCompile(`&quot;`)
	htmlAposRe     = regexp.MustCompile(`&#39;`)
	htmlNewlinesRe = regexp.MustCompile(`\n\s*\n\s*\n+`)
	htmlSpacesRe   = regexp.MustCompile(`[ \t]+`)

	mainContentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`),
		regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`),
		regexp.MustCompile(`(?is)<div[^>]*id=["']content["'][^>]*>(.*?)</div>`),
		regexp.MustCompile(`(?is)<div[^>]*class=["'][^"']*content[^"']*["'][^>]*>(.*?)</div>`),
		regexp.MustCompile(`(?is)<div[^>]*id=["']main["'][^>]*>(.*?)</div>`),
		regexp.MustCompile(`(?is)<div[^>]*class=["'][^"']*main[^"']*["'][^>]*>(.*?)</div>`),
	}
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
		url = params["url"]
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

	// Parse timeout from params
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

	// Read response body with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchURLSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	content := string(body)

	mode, _ := params["mode"]
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "text"
	}

	switch mode {
	case "html":
		return content, nil
	case "text":
		return extractTextFromHTML(content), nil
	case "markdown":
		return htmlToMarkdown(content), nil
	case "main_content":
		return extractMainContent(content), nil
	default:
		return extractTextFromHTML(content), nil
	}
}

func extractTextFromHTML(html string) string {
	html = htmlScriptRe.ReplaceAllString(html, "")
	html = htmlStyleRe.ReplaceAllString(html, "")
	html = htmlNavRe.ReplaceAllString(html, "")
	html = htmlFooterRe.ReplaceAllString(html, "")
	html = htmlHeaderRe.ReplaceAllString(html, "")

	text := htmlTagRe.ReplaceAllString(html, " ")

	text = htmlNbspRe.ReplaceAllString(text, " ")
	text = htmlAmpRe.ReplaceAllString(text, "&")
	text = htmlLtRe.ReplaceAllString(text, "<")
	text = htmlGtRe.ReplaceAllString(text, ">")
	text = htmlQuotRe.ReplaceAllString(text, "\"")
	text = htmlAposRe.ReplaceAllString(text, "'")

	text = htmlNewlinesRe.ReplaceAllString(text, "\n\n")
	text = htmlSpacesRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

func extractMainContent(html string) string {
	content := ""

	for _, re := range mainContentPatterns {
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 100 {
			content = matches[1]
			break
		}
	}

	if content == "" {
		content = html
	}

	return extractTextFromHTML(content)
}

func htmlToMarkdown(html string) string {
	text := html

	text = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`).ReplaceAllString(text, "# $1\n\n")
	text = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`).ReplaceAllString(text, "## $1\n\n")
	text = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`).ReplaceAllString(text, "### $1\n\n")
	text = regexp.MustCompile(`(?is)<h4[^>]*>(.*?)</h4>`).ReplaceAllString(text, "#### $1\n\n")
	text = regexp.MustCompile(`(?is)<h5[^>]*>(.*?)</h5>`).ReplaceAllString(text, "##### $1\n\n")
	text = regexp.MustCompile(`(?is)<h6[^>]*>(.*?)</h6>`).ReplaceAllString(text, "###### $1\n\n")

	text = regexp.MustCompile(`(?is)<strong[^>]*>(.*?)</strong>`).ReplaceAllString(text, "**$1**")
	text = regexp.MustCompile(`(?is)<b[^>]*>(.*?)</b>`).ReplaceAllString(text, "**$1**")
	text = regexp.MustCompile(`(?is)<em[^>]*>(.*?)</em>`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`(?is)<i[^>]*>(.*?)</i>`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`).ReplaceAllString(text, "`$1`")

	text = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`).ReplaceAllString(text, "[$2]($1)")

	text = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`).ReplaceAllString(text, "- $1\n")
	text = regexp.MustCompile(`(?is)<ul[^>]*>.*?</ul>`).ReplaceAllStringFunc(text, func(m string) string {
		return m + "\n"
	})

	text = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`).ReplaceAllString(text, "$1\n\n")
	text = regexp.MustCompile(`(?is)<br\s*/?>`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`(?is)<hr\s*/?>`).ReplaceAllString(text, "\n---\n\n")

	text = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.*?)</code></pre>`).ReplaceAllString(text, "```\n$1\n```\n\n")
	text = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`).ReplaceAllString(text, "```\n$1\n```\n\n")

	return extractTextFromHTML(text)
}
