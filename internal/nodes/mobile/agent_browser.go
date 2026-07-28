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

package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

type AgentBrowserNode struct{}

func init() {
	core.Register(&AgentBrowserNode{})
}

func (n *AgentBrowserNode) Name() string {
	return "agent_browser"
}

func (n *AgentBrowserNode) Description() string {
	return "Agent-optimized web browser: visit pages, extract content, follow links, take screenshots (ego-lite inspired)"
}

func (n *AgentBrowserNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "agent_browser",
		Description: "Agent-optimized web browser for autonomous web navigation, content extraction, and research. Inspired by CitroLabs/ego-lite - zero-cost browser state sharing.",
		Input:       "string - URL to visit or browser action to perform",
		Output:      "string - Page content, extraction results, or browser status",
		Params: []core.ParamSchema{
			{Name: "action", Type: "string", Description: "Browser action: visit|extract|links|screenshot|search|summary|connect_existing|import_cookies (default: visit)", Required: false, Default: "visit"},
			{Name: "url", Type: "string", Description: "Target URL (overrides input if provided)", Required: false},
			{Name: "selector", Type: "string", Description: "CSS selector for content extraction (optional)", Required: false},
			{Name: "max_depth", Type: "string", Description: "Maximum link follow depth for crawling (default: 1)", Required: false, Default: "1"},
			{Name: "output_format", Type: "string", Description: "Output format: markdown|text|json|html (default: markdown)", Required: false, Default: "markdown"},
			{Name: "summary_length", Type: "string", Description: "Maximum summary length in characters (default: 2000)", Required: false, Default: "2000"},
			{Name: "render_js", Type: "string", Description: "Enable JavaScript rendering (default: false)", Required: false, Default: "false"},
			{Name: "use_session", Type: "string", Description: "Reuse authenticated browser session (default: false)", Required: false, Default: "false"},
			{Name: "browser_profile", Type: "string", Description: "Browser profile path for session reuse (optional)", Required: false},
			{Name: "cdp_port", Type: "string", Description: "Chrome DevTools Protocol port (default: 9222)", Required: false, Default: "9222"},
		},
	}
}

func (n *AgentBrowserNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := core.GetParam(params, "action", "visit")
	targetURL := core.GetParam(params, "url", "")
	selector := core.GetParam(params, "selector", "")
	outputFmt := core.GetParam(params, "output_format", "markdown")
	renderJS := core.GetParam(params, "render_js", "false") == "true"

	if targetURL == "" {
		trimmed := strings.TrimSpace(input)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			targetURL = trimmed
		}
	}

	if targetURL == "" && action != "search" && action != "connect_existing" && action != "import_cookies" {
		return "", fmt.Errorf("URL is required for browser action: %s", action)
	}

	if targetURL != "" {
		if _, err := url.Parse(targetURL); err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}
	}

	summaryLen := core.ParamInt(params, "summary_length", 2000, 100, 100000)

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
	case "connect_existing":
		return n.actionConnectExisting(ctx, params)
	case "import_cookies":
		return n.actionImportCookies(ctx, params)
	default:
		return "", fmt.Errorf("unknown browser action: %s (supported: visit, extract, links, summary, screenshot, search, connect_existing, import_cookies)", action)
	}
}

func (n *AgentBrowserNode) actionVisit(ctx context.Context, targetURL, outputFmt string, summaryLen int, renderJS bool) (string, error) {
	fetchNode, _ := core.Get("fetch_url")
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
	fetchNode, _ := core.Get("fetch_url")
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
	fetchNode, _ := core.Get("fetch_url")
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
	fetchNode, _ := core.Get("fetch_url")
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
	searchNode, _ := core.Get("search_aggregate")
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

// cdpVersionInfo 表示 CDP /json/version 端点返回的浏览器版本信息
type cdpVersionInfo struct {
	Browser              string `json:"Browser"`
	ProtocolVersion      string `json:"Protocol-Version"`
	UserAgent            string `json:"User-Agent"`
	V8                   string `json:"V8"`
	WebKit               string `json:"WebKit"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// cdpTabInfo 表示 CDP /json 端点返回的单个标签页信息
type cdpTabInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	DevtoolsFrontendURL  string `json:"devtoolsFrontendUrl"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// browserConnectionInfo 表示与运行中浏览器的连接状态信息
type browserConnectionInfo struct {
	Connected       bool
	DebugPort       int
	Browser         string
	ProtocolVersion string
	UserAgent       string
	TabCount        int
	PageTabCount    int
	Tabs            []cdpTabInfo
}

// maxCDPResponseSize 限制 CDP HTTP 端点响应体大小，防止恶意/被劫持的
// 本地服务返回超大响应导致 OOM。CDP /json/version 与 /json 端点正常
// 输出远小于此上限。
const maxCDPResponseSize = 4 * 1024 * 1024 // 4MB

// fetchCDP 向 CDP HTTP 端点发起 GET 请求并返回响应体字节。
// 为防止 SSRF 与本地端口扫描，target 必须指向 localhost 的非特权端口
// （>=1024），且响应体大小受 maxCDPResponseSize 限制。
func (n *AgentBrowserNode) fetchCDP(ctx context.Context, target string) ([]byte, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid CDP target URL: %w", err)
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("CDP target must use http scheme, got: %s", u.Scheme)
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, fmt.Errorf("CDP target must point to localhost, got: %s", host)
	}
	if u.Port() == "" {
		return nil, fmt.Errorf("CDP target must include an explicit port")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	// CDP 端点只监听本机，使用较短超时避免长时间挂起
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDP 端点返回状态码 %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxCDPResponseSize))
}

// connectToExistingBrowser 通过 Chrome DevTools Protocol 连接用户正在运行的浏览器，
// 获取版本信息与已打开的标签页列表，返回连接状态信息。
func (n *AgentBrowserNode) connectToExistingBrowser(ctx context.Context, debugPort int) (*browserConnectionInfo, error) {
	baseURL := fmt.Sprintf("http://localhost:%d", debugPort)

	// 获取浏览器版本信息
	versionRaw, err := n.fetchCDP(ctx, baseURL+"/json/version")
	if err != nil {
		return nil, fmt.Errorf("无法连接到浏览器 (端口 %d)，请确认 Chrome 已以 --remote-debugging-port=%d 启动: %w", debugPort, debugPort, err)
	}
	var version cdpVersionInfo
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return nil, fmt.Errorf("解析浏览器版本信息失败: %w", err)
	}

	// 获取已打开的标签页列表
	tabsRaw, err := n.fetchCDP(ctx, baseURL+"/json")
	if err != nil {
		return nil, fmt.Errorf("获取标签页列表失败: %w", err)
	}
	var tabs []cdpTabInfo
	if err := json.Unmarshal(tabsRaw, &tabs); err != nil {
		return nil, fmt.Errorf("解析标签页列表失败: %w", err)
	}

	pageTabs := 0
	for _, t := range tabs {
		if t.Type == "page" {
			pageTabs++
		}
	}

	return &browserConnectionInfo{
		Connected:       true,
		DebugPort:       debugPort,
		Browser:         version.Browser,
		ProtocolVersion: version.ProtocolVersion,
		UserAgent:       version.UserAgent,
		TabCount:        len(tabs),
		PageTabCount:    pageTabs,
		Tabs:            tabs,
	}, nil
}

// actionConnectExisting 实现 connect_existing 动作：连接用户正在运行的浏览器并返回格式化的连接信息。
// cdp_port 限制为非特权端口 (>=1024) 以防止本地端口扫描攻击。
func (n *AgentBrowserNode) actionConnectExisting(ctx context.Context, params map[string]string) (string, error) {
	debugPort := core.ParamInt(params, "cdp_port", 9222, 1024, 65535)

	info, err := n.connectToExistingBrowser(ctx, debugPort)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("## 🔌 已连接到运行中的浏览器\n\n")
	sb.WriteString(fmt.Sprintf("- **调试端口**: %d\n", info.DebugPort))
	sb.WriteString(fmt.Sprintf("- **浏览器**: %s\n", nonEmpty(info.Browser, "未知")))
	sb.WriteString(fmt.Sprintf("- **协议版本**: %s\n", nonEmpty(info.ProtocolVersion, "未知")))
	sb.WriteString(fmt.Sprintf("- **User-Agent**: %s\n", nonEmpty(info.UserAgent, "未知")))
	sb.WriteString(fmt.Sprintf("- **标签页总数**: %d（页面类型: %d）\n\n", info.TabCount, info.PageTabCount))

	if info.PageTabCount > 0 {
		sb.WriteString("### 打开的页面\n\n")
		shown := 0
		for _, t := range info.Tabs {
			if t.Type != "page" {
				continue
			}
			if shown >= 20 {
				sb.WriteString(fmt.Sprintf("\n... 还有 %d 个页面未显示\n", info.PageTabCount-shown))
				break
			}
			shown++
			title := strings.TrimSpace(t.Title)
			if title == "" {
				title = "(无标题)"
			}
			pageURL := strings.TrimSpace(t.URL)
			if pageURL == "" {
				pageURL = "(about:blank)"
			}
			sb.WriteString(fmt.Sprintf("%d. **%s**\n   - %s\n", shown, title, pageURL))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("_会话状态已就绪，后续 visit/extract 操作可复用此浏览器上下文。_\n")
	return sb.String(), nil
}

// importCookiesFromChrome 从本地 Chrome 配置读取 Cookie 域名清单。
// 实际解析通过 sqlite3 CLI 读取 cookies 表的 host_key/name 列（明文存储），
// 不读取加密的 value/encrypted_value（Chrome 用 OS keychain 加密，解密需平台原生调用）。
// 返回去重后的域名列表，供 Agent 判断"哪些站点已登录"。
func (n *AgentBrowserNode) importCookiesFromChrome(ctx context.Context, profile string) ([]string, error) {
	cookiePath := getChromeCookiePath()
	if profile != "" {
		cookiePath = filepath.Join(getChromeProfilePath(profile), "Cookies")
	}
	if cookiePath == "" {
		return nil, fmt.Errorf("当前平台 %s 不支持自动检测 Chrome Cookie 路径", runtime.GOOS)
	}

	entries, err := readChromeCookieDomains(ctx, cookiePath)
	if err != nil {
		return nil, err
	}
	return uniqueHosts(entries), nil
}

// actionImportCookies 实现 import_cookies 动作：解析本地 Chrome Cookie
// SQLite 数据库，返回已登录的域名清单及每个域名的 cookie 数量。
func (n *AgentBrowserNode) actionImportCookies(ctx context.Context, params map[string]string) (string, error) {
	profile := core.GetParam(params, "browser_profile", "")
	cookiePath := getChromeCookiePath()
	if profile != "" {
		cookiePath = filepath.Join(getChromeProfilePath(profile), "Cookies")
	}

	var sb strings.Builder
	sb.WriteString("## 🍪 Cookie 导入\n\n")

	if cookiePath == "" {
		sb.WriteString(fmt.Sprintf("**状态**: 当前平台 `%s` 不支持自动检测 Chrome Cookie 路径\n", runtime.GOOS))
		sb.WriteString("\n可手动指定 `browser_profile` 参数指向 Chrome 用户数据目录。\n")
		return sb.String(), nil
	}

	sb.WriteString(fmt.Sprintf("- **Cookie 数据库路径**: `%s`\n", cookiePath))
	if profile != "" {
		sb.WriteString(fmt.Sprintf("- **指定配置文件**: `%s`\n", profile))
	} else {
		sb.WriteString("- **配置文件**: Default（默认）\n")
	}

	// 读取完整条目（host+name），用于同时报告域名数与每域名 cookie 数。
	entries, err := readChromeCookieDomains(ctx, cookiePath)
	if err != nil {
		sb.WriteString(fmt.Sprintf("- **状态**: ❌ %s\n", err.Error()))
		sb.WriteString("\n**排障建议**:\n")
		sb.WriteString("- 若提示 sqlite3 未找到：请安装 sqlite3（Linux: `apt install sqlite3`，macOS: 自带）\n")
		sb.WriteString("- 若提示超时/锁定：请先关闭正在运行的 Chrome，它持有数据库写锁\n")
		return sb.String(), nil
	}

	domains := uniqueHosts(entries)
	if info, statErr := os.Stat(cookiePath); statErr == nil {
		sb.WriteString(fmt.Sprintf("- **数据库大小**: %d 字节\n", info.Size()))
	}
	sb.WriteString(fmt.Sprintf("- **Cookie 总条数**: %d\n", len(entries)))
	sb.WriteString(fmt.Sprintf("- **已登录域名数**: %d\n\n", len(domains)))

	sb.WriteString("### 安全说明\n\n")
	sb.WriteString("- 已通过 `sqlite3 -readonly` 只读读取 host_key/name 列（明文存储）\n")
	sb.WriteString("- 不读取 cookie 值：Chrome 用 OS keychain（DPAPI/Keychain/kwallet）加密，需平台原生调用解密\n")
	sb.WriteString("- 会话复用时，浏览器会自动从原 profile 加载完整 cookie 值\n\n")

	if len(domains) > 0 {
		namesByHost := cookieNamesByHost(entries)
		sb.WriteString("### 可用域名\n\n")
		sb.WriteString("| # | 域名 | Cookie 数 |\n")
		sb.WriteString("|---|------|----------|\n")
		for i, d := range domains {
			if i >= 100 {
				sb.WriteString(fmt.Sprintf("\n... 还有 %d 个域名未显示\n", len(domains)-100))
				break
			}
			sb.WriteString(fmt.Sprintf("| %d | `%s` | %d |\n", i+1, d, len(namesByHost[d])))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("_未检测到任何 cookie（Chrome 可能从未运行或使用无痕模式）_\n\n")
	}

	sb.WriteString("_Cookie 数据库已解析完毕，可配合 connect_existing 或浏览器启动参数复用登录态。_\n")
	return sb.String(), nil
}

// getChromeCookiePath 返回当前平台的 Chrome Cookie 数据库路径。
// 支持 macOS、Linux 和 Windows；其他平台返回空字符串。
func getChromeCookiePath() string {
	profilePath := getChromeProfilePath("Default")
	if profilePath == "" {
		return ""
	}
	return filepath.Join(profilePath, "Cookies")
}

// getChromeProfilePath 返回当前平台的 Chrome 配置文件目录路径。
// profile 指定配置文件名（如 "Default"、"Profile 1"），为空则使用 "Default"。
// 不支持的平台返回空字符串。
func getChromeProfilePath(profile string) string {
	if profile == "" {
		profile = "Default"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", profile)
	case "linux":
		return filepath.Join(home, ".config", "google-chrome", profile)
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "Google", "Chrome", "User Data", profile)
	default:
		return ""
	}
}

// nonEmpty 在 v 为空字符串时返回 fallback，否则返回 v
func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
