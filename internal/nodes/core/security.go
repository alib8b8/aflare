// Copyright (c) 2026 aflare Contributors
//
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

package core

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/httpclient"
	"github.com/alib8b8/aflare/internal/logger"
)

var (
	workDir string
)

const (
	// DefaultLLMTimeout is the default timeout for LLM HTTP requests.
	DefaultLLMTimeout = 120 * time.Second
)

func init() {
	wd, err := os.Getwd()
	if err == nil {
		workDir = wd
	}
}

// HTTPRedirectValidator returns an http.Client CheckRedirect function that
// validates each redirect target with the given validator (ValidateURL for
// general HTTP, ValidateLMLEndpoint for LLM endpoints). It also caps the
// number of redirects to prevent redirect loops.
func HTTPRedirectValidator(validator func(string) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return validator(req.URL.String())
	}
}

// SafeJoinPath joins userPath onto baseDir after validating that the
// resulting path stays within baseDir. It blocks absolute paths, parent
// traversal, and (at L2+) symlink escapes.
func SafeJoinPath(baseDir, userPath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}

	cleanPath := filepath.Clean(userPath)
	if strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		GetSecurityStats().RecordBlock(BlockPathTraversal, "absolute path: "+userPath, "")
		return "", fmt.Errorf("absolute paths are not allowed, use relative paths within the working directory")
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	fullPath := filepath.Join(absBase, cleanPath)
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve full path: %w", err)
	}

	relPath, err := filepath.Rel(absBase, absFullPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}
	if strings.HasPrefix(relPath, "..") || relPath == ".." {
		GetSecurityStats().RecordBlock(BlockPathTraversal, "path escapes: "+userPath, "")
		return "", fmt.Errorf("path escapes the allowed directory")
	}

	// Resolve symlinks to prevent symlink-based bypass (L2+ only)
	if config.SecurityLevelAtLeast(config.SecurityLevelL2) {
		resolvedPath, err := filepath.EvalSymlinks(absFullPath)
		if err == nil {
			resolvedRel, err := filepath.Rel(absBase, resolvedPath)
			if err != nil || strings.HasPrefix(resolvedRel, "..") {
				GetSecurityStats().RecordBlock(BlockSymlinkBypass, "symlink escape: "+userPath, "")
				return "", fmt.Errorf("path escapes the allowed directory (symlink)")
			}
			return resolvedPath, nil
		}
	}

	return absFullPath, nil
}

// ValidateReadPath validates that path is safe to read from within the
// current working directory.
func ValidateReadPath(path string) (string, error) {
	return ValidateReadPathIn(workDir, path)
}

// ValidateReadPathIn is like ValidateReadPath but uses the supplied base
// directory instead of the cached workDir. Sub-packages that maintain
// their own (possibly test-overridden) workDir copy should call this so
// their overrides take effect.
func ValidateReadPathIn(baseDir, path string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("working directory not available")
	}
	return SafeJoinPath(baseDir, path)
}

// ValidateWritePath validates that path is safe to write to within the
// current working directory, applying extension/dotfile restrictions
// based on the active security level.
func ValidateWritePath(path string) (string, error) {
	return ValidateWritePathIn(workDir, path)
}

// ValidateWritePathIn is like ValidateWritePath but uses the supplied
// base directory instead of the cached workDir. Sub-packages with their
// own workDir override should call this so test reassignments propagate.
func ValidateWritePathIn(baseDir, path string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("working directory not available")
	}
	safePath, err := SafeJoinPath(baseDir, path)
	if err != nil {
		return "", err
	}

	// Reject dotfiles (L1+ only)
	if config.SecurityLevelAtLeast(config.SecurityLevelL1) {
		baseName := filepath.Base(safePath)
		if strings.HasPrefix(baseName, ".") {
			GetSecurityStats().RecordBlock(BlockSensitiveFile, "dotfile: "+path, "")
			return "", fmt.Errorf("dotfiles are not allowed for writing")
		}
	}

	ext := strings.ToLower(filepath.Ext(safePath))

	// Forbidden extensions (enforced at all security levels; L3 additionally blocks script extensions below)
	forbiddenExts := map[string]bool{
		".env":   true,
		".sh":    true,
		".bash":  true,
		".zsh":   true,
		".fish":  true,
		".bat":   true,
		".ps1":   true,
		".exe":   true,
		".dll":   true,
		".so":    true,
		".dylib": true,
		".msi":   true,
		".apk":   true,
		".ipa":   true,
		".deb":   true,
		".rpm":   true,
		".pkg":   true,
	}
	if config.SecurityLevelAtLeast(config.SecurityLevelL3) {
		forbiddenExts[".py"] = true
		forbiddenExts[".rb"] = true
		forbiddenExts[".php"] = true
		forbiddenExts[".pl"] = true
	}
	if forbiddenExts[ext] {
		GetSecurityStats().RecordBlock(BlockUnsafeExtension, "forbidden ext: "+ext, "")
		return "", fmt.Errorf("writing to %s files is not allowed (security risk)", ext)
	}

	// Allowed extensions (L3 only - strict allowlist)
	if config.SecurityLevelAtLeast(config.SecurityLevelL3) {
		allowedExts := map[string]bool{
			".txt":  true,
			".md":   true,
			".yaml": true,
			".yml":  true,
			".json": true,
			".csv":  true,
			".xml":  true,
			".log":  true,
			".html": true,
			".htm":  true,
			".css":  true,
			".js":   true,
			".ts":   true,
			".go":   true,
			".rs":   true,
			".java": true,
			".c":    true,
			".cpp":  true,
			".h":    true,
			".sql":  true,
			".toml": true,
			".ini":  true,
			".svg":  true,
			".png":  true,
			".jpg":  true,
			".jpeg": true,
			".gif":  true,
			".pdf":  true,
			".zip":  true,
			".tar":  true,
			".gz":   true,
		}
		if ext != "" && !allowedExts[ext] {
			GetSecurityStats().RecordBlock(BlockUnsafeExtension, "ext not allowed: "+ext, "")
			return "", fmt.Errorf("file extension '%s' is not allowed for writing at L3 security level", ext)
		}
	}

	return safePath, nil
}

// RedactAPIKey masks an API key, showing only the first 4 and last 4 chars.
func RedactAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// IsSensitiveKey reports whether key looks like it holds a secret
// (api key, token, bearer, password, etc.).
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	sensitivePrefixes := []string{"api", "token", "bearer", "password", "passwd", "secret", "auth"}
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, "_"+prefix) || strings.Contains(lower, "-"+prefix) {
			return true
		}
	}
	return false
}

var sensitivePatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)(bearer\s+)([^\s"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(authorization[:\s]+)([^\s"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(api[_-]?key=)([^\s&"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(password=)([^\s&"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(passwd=)([^\s&"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(token=)([^\s&"']+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(secret=)([^\s&"']+)`), "${1}****"},
	{regexp.MustCompile(`(https?://[^/\s:@]+:)([^@\s/@]+)(@)`), "${1}****${3}"},
	{regexp.MustCompile(`(?i)ghp_[A-Za-z0-9]{20,}`), "ghp_****"},
	{regexp.MustCompile(`(?i)sk-[A-Za-z0-9]{20,}`), "sk-****"},
	{regexp.MustCompile(`(?i)xox[baprs]-[A-Za-z0-9-]{10,}`), "xoxb-****"},
}

// RedactSensitive masks known secret patterns (Bearer tokens, API keys,
// GitHub/Slack tokens, URL credentials, etc.) in s.
func RedactSensitive(s string) string {
	if s == "" {
		return s
	}
	result := s
	for _, sp := range sensitivePatterns {
		result = sp.pattern.ReplaceAllString(result, sp.replace)
	}
	if len(result) > 1000 {
		result = result[:1000] + "... (truncated)"
	}
	return result
}

// ValidateURL checks if a URL is safe to request (SSRF protection).
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		GetSecurityStats().RecordBlock(BlockNetwork, "non-http scheme: "+u.Scheme, "")
		return fmt.Errorf("only http and https URLs are allowed, got: %s", u.Scheme)
	}

	// Block userinfo to prevent credential injection
	if u.User != nil {
		return fmt.Errorf("URLs with userinfo (credentials) are not allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Block localhost variants (unless explicitly allowed for local dev/demo)
	lowerHost := strings.ToLower(host)
	localhostVariants := map[string]bool{
		"localhost":             true,
		"localhost.localdomain": true,
		"ip6-localhost":         true,
		"ip6-loopback":          true,
	}
	if localhostVariants[lowerHost] && !loopbackAllowed() {
		GetSecurityStats().RecordBlock(BlockSSRF, "localhost: "+host, "")
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip != nil {
		// When loopback is explicitly allowed (local dev/demo), use the
		// LLM-endpoint IP validator which permits loopback but still
		// blocks link-local/unspecified/multicast/reserved ranges.
		validator := ValidateIP
		if loopbackAllowed() {
			validator = ValidateLMLEndpointIP
		}
		if err := validator(ip, host); err != nil {
			GetSecurityStats().RecordBlock(BlockSSRF, "blocked IP: "+host, "")
			return err
		}
	} else {
		// DNS-resolve the hostname to prevent domain-based SSRF
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve host %s: %w", host, err)
		}
		dnsValidator := ValidateIP
		if loopbackAllowed() {
			dnsValidator = ValidateLMLEndpointIP
		}
		for _, resolvedIP := range ips {
			if err := dnsValidator(resolvedIP, host); err != nil {
				GetSecurityStats().RecordBlock(BlockSSRF, "blocked IP: "+resolvedIP.String(), "")
				return err
			}
		}
	}

	return nil
}

// loopbackAllowed reports whether loopback/localhost URLs are permitted by
// the http_request node. This is OFF by default (strict SSRF protection) and
// only enabled via the AFLARE_ALLOW_LOOPBACK=1 env var, intended solely for
// local development and demos that run mock services on localhost. Production
// deployments must leave this unset.
func loopbackAllowed() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AFLARE_ALLOW_LOOPBACK")))
	return v == "1" || v == "true" || v == "yes"
}

// ValidateIP reports whether ip is safe to connect to (blocks loopback,
// private, link-local, unspecified, multicast, and reserved ranges). It
// delegates to httpclient.ValidatePublic so the IP-range policy lives in
// exactly one place. Kept as a thin wrapper for backward compatibility
// with callers that already import nodes/core.
func ValidateIP(ip net.IP, displayHost string) error {
	return httpclient.ValidatePublic(ip, displayHost)
}

// SafeHTTPClient is the shared HTTP client for general outbound traffic
// (fetch_url, http_request, webhooks, etc.). It is built via the
// httpclient factory so the dial-time SSRF re-validation and the
// connection-pool tuning (MaxIdleConns / MaxIdleConnsPerHost /
// IdleConnTimeout) live in one place rather than being copy-pasted.
//
// The validator is a closure (not a pre-built httpclient.Validator) on
// purpose: SafeHTTPClient honors AFLARE_ALLOW_LOOPBACK at *dial* time,
// not at init time, so flipping the env var takes effect for the next
// connection without restarting the process. The closure dispatches to
// ValidateIP (loopback blocked) or ValidateLMLEndpointIPAllowLoopback
// (loopback allowed) accordingly, keeping DNS-rebinding protection
// intact in both modes.
var SafeHTTPClient = httpclient.NewClient(httpclient.Options{
	Timeout: 30 * time.Second,
	Validator: func(ip net.IP, displayHost string) error {
		if loopbackAllowed() {
			return ValidateLMLEndpointIPAllowLoopback(ip, displayHost)
		}
		return ValidateIP(ip, displayHost)
	},
})

// SafeLLMHTTPClient is like SafeHTTPClient but allows loopback and
// private addresses (for local/LAN LLM servers like Ollama on
// 127.0.0.1 or a self-hosted model on 10.0.0.5). It still blocks
// link-local, unspecified, multicast, and reserved ranges at dial time.
// Built via the httpclient factory for the same pool-tuning and
// dial-time-SSRF reasons as SafeHTTPClient.
var SafeLLMHTTPClient = httpclient.NewClient(httpclient.Options{
	Timeout:   120 * time.Second,
	Validator: ValidateLMLEndpointIPAllowLoopback,
})

// ValidateLMLEndpointIPAllowLoopback validates an IP for LLM endpoints, allowing loopback.
func ValidateLMLEndpointIPAllowLoopback(ip net.IP, displayHost string) error {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("access to link-local address %s is not allowed", displayHost)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("access to unspecified address %s is not allowed", displayHost)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("access to multicast address %s is not allowed", displayHost)
	}
	if IsReservedIP(ip) {
		return fmt.Errorf("access to reserved address %s is not allowed", displayHost)
	}
	return nil
}

// ValidateLMLEndpoint validates an LLM API endpoint URL. It is similar to
// ValidateURL but allows loopback/localhost addresses, because LLM servers
// (e.g. Ollama, llama.cpp) commonly run on http://localhost:11434.
// Non-loopback private addresses and other dangerous ranges remain blocked.
func ValidateLMLEndpoint(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed, got: %s", u.Scheme)
	}

	// Block userinfo to prevent credential leakage
	if u.User != nil {
		return fmt.Errorf("URLs with userinfo (credentials) are not allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Allow loopback and localhost variants for LLM endpoints
	lowerHost := strings.ToLower(host)
	localhostVariants := map[string]bool{
		"localhost":             true,
		"localhost.localdomain": true,
		"ip6-localhost":         true,
		"ip6-loopback":          true,
	}
	if localhostVariants[lowerHost] {
		return nil
	}

	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip != nil {
		return ValidateLMLEndpointIP(ip, host)
	}

	// DNS-resolve the hostname to prevent domain-based SSRF
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", host, err)
	}
	for _, resolvedIP := range ips {
		if err := ValidateLMLEndpointIP(resolvedIP, host); err != nil {
			return err
		}
	}

	return nil
}

// ValidateLMLEndpointIP validates an IP for LLM endpoints. Loopback is allowed,
// but other private/reserved ranges are still blocked.
func ValidateLMLEndpointIP(ip net.IP, displayHost string) error {
	if ip.IsLoopback() {
		return nil
	}
	if ip.IsPrivate() {
		return fmt.Errorf("access to private address %s is not allowed", displayHost)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("access to link-local address %s is not allowed", displayHost)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("access to unspecified address %s is not allowed", displayHost)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("access to multicast address %s is not allowed", displayHost)
	}
	if IsReservedIP(ip) {
		return fmt.Errorf("access to reserved address %s is not allowed", displayHost)
	}
	return nil
}

// IsReservedIP reports whether ip falls in a reserved range that should
// not be reachable from production code (TEST-NET, CGNAT, 0.0.0.0/8,
// IPv6 ULA, etc.). It delegates to httpclient.IsReservedIP so the range
// table lives in exactly one place. Kept as a thin wrapper for backward
// compatibility with callers that already import nodes/core.
func IsReservedIP(ip net.IP) bool {
	return httpclient.IsReservedIP(ip)
}

// MaxHTTPResponseSize bounds how much of an HTTP response body the nodes
// are willing to read. It mirrors the constant previously defined in
// http_request.go and is shared by fetch_url, notify, and the LLM base.
const MaxHTTPResponseSize = 10 * 1024 * 1024 // 10MB

// GetWorkDir returns the cached working directory used for path validation.
func GetWorkDir() string {
	return workDir
}

// ------------------------------------------------------------
// LLM 出口密钥脱敏（opt-in）
// ------------------------------------------------------------
//
// 这部分是 nodes.RedactSecrets 的核心逻辑下沉：LLM 出口路径（package core
// 以及子包 providers）需要对 prompt 做密钥脱敏，但 core 不能 import nodes
// （nodes 已 import core，会形成循环依赖）。因此把密钥模式与 RedactSecrets
// 实现放在 core，nodes.RedactSecrets 改为委托此处，避免重复代码。

// MaxRedactInputSize limits redact input length to avoid regex backtracking.
const MaxRedactInputSize = 10 * 1024 * 1024 // 10MB

// secretPattern 描述一类密钥的识别规则。
type secretPattern struct {
	name    string
	pattern *regexp.Regexp
	mask    string // 脱敏后的占位符
}

// secretPatterns 常见密钥格式（高置信度，避免误伤普通文本）。
var secretPatterns = []secretPattern{
	{name: "aws_access_key", pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), mask: "AKIA[REDACTED]"},
	{name: "aws_secret", pattern: regexp.MustCompile(`(?i)aws_secret_access_key["'\s:=]+([A-Za-z0-9/+=]{40})`), mask: "aws_secret_access_key=[REDACTED]"},
	{name: "github_token", pattern: regexp.MustCompile(`gh[posur]_[A-Za-z0-9]{36}`), mask: "ghp_[REDACTED]"},
	{name: "gitlab_token", pattern: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20}`), mask: "glpat-[REDACTED]"},
	{name: "slack_token", pattern: regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`), mask: "xox-[REDACTED]"},
	{name: "generic_api_key", pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key)["'\s:=]+([A-Za-z0-9+/=_-]{32,})`), mask: "${1}=[REDACTED]"},
	{name: "bearer_token", pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9+/=_-]{20,}`), mask: "bearer [REDACTED]"},
	{name: "private_key", pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`), mask: "-----BEGIN [REDACTED PRIVATE KEY]-----"},
	{name: "jwt", pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), mask: "eyJ[REDACTED].eyJ[REDACTED].[REDACTED]"},
	{name: "db_password", pattern: regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:([^@]{3,})@`), mask: "${1}://[REDACTED]:[REDACTED]@"},
}

// RedactSecrets 对文本内容进行密钥脱敏，返回脱敏后的内容与命中的密钥数量。
//   - 若 wholeFile 为 true，则整文件标记为 [REDACTED]（适用于 .env / 私钥等）
//   - 否则逐条密钥模式匹配替换为占位符
//
// 这是 nodes.RedactSecrets 的实现：core 与 providers 通过它脱敏 LLM 出口
// prompt，nodes.RedactSecrets 委托到此以避免重复代码。
func RedactSecrets(content string, wholeFile bool) (string, int) {
	if len(content) > MaxRedactInputSize {
		// 超长内容截断后再脱敏，防止正则 DoS
		content = content[:MaxRedactInputSize]
	}

	if wholeFile {
		return fmt.Sprintf("[REDACTED: 敏感文件内容已隐藏，共 %d 字节]", len(content)), 1
	}

	masked := content
	totalHits := 0
	for _, sp := range secretPatterns {
		// 限制单模式替换次数，防止异常输入导致过度回溯
		matches := sp.pattern.FindAllStringSubmatchIndex(masked, 64)
		if len(matches) == 0 {
			continue
		}
		// 从后往前替换，避免索引偏移
		mask := sp.mask
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			// 若 mask 含 ${1} 占位（保留前缀），用分组替换
			if strings.Contains(mask, "${1}") && len(m) >= 4 {
				replacement := strings.ReplaceAll(mask, "${1}", masked[m[2]:m[3]])
				masked = masked[:m[0]] + replacement + masked[m[1]:]
			} else {
				masked = masked[:m[0]] + mask + masked[m[1]:]
			}
			totalHits++
		}
	}
	return masked, totalHits
}

// LLMRedactEnv 是开启 LLM 出口密钥脱敏的环境变量。
const LLMRedactEnv = "AFLARE_LLM_REDACT_SECRETS"

// llmRedactEnabled 报告是否开启了 LLM 出口密钥脱敏（opt-in，默认关闭）。
func llmRedactEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(LLMRedactEnv))) {
	case "1", "true":
		return true
	}
	return false
}

// MaybeRedactLLMSecrets 在 AFLARE_LLM_REDACT_SECRETS=1 时对 input 与
// systemPrompt 调用 RedactSecrets 进行脱敏，返回（可能已脱敏的）input 与
// systemPrompt。未开启时原样返回。发生脱敏时打一条 Info 日志，包含 provider
// 名称与命中计数，绝不记录原始 secret 内容。
//
// 供 core/llm_base.go 与 providers（ollama/fastgpt）在构造 messages 之前
// 调用，确保 prompt 里的 secret 不会原样发给 LLM 服务商。
func MaybeRedactLLMSecrets(providerName, input, systemPrompt string) (string, string) {
	if !llmRedactEnabled() {
		return input, systemPrompt
	}
	ri, hi := RedactSecrets(input, false)
	rs, hs := RedactSecrets(systemPrompt, false)
	if total := hi + hs; total > 0 {
		logger.Info("[security] LLM egress secrets redacted",
			"provider", providerName,
			"redacted_count", total,
		)
	}
	return ri, rs
}
