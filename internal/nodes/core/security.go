// Copyright (c) 2026 llm-box Contributors
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
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
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
// only enabled via the LLMBOX_ALLOW_LOOPBACK=1 env var, intended solely for
// local development and demos that run mock services on localhost. Production
// deployments must leave this unset.
func loopbackAllowed() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LLMBOX_ALLOW_LOOPBACK")))
	return v == "1" || v == "true" || v == "yes"
}

// ValidateIP reports whether ip is safe to connect to (blocks loopback,
// private, link-local, unspecified, multicast, and reserved ranges).
func ValidateIP(ip net.IP, displayHost string) error {
	if ip.IsLoopback() {
		return fmt.Errorf("access to loopback address %s is not allowed", displayHost)
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

// SafeHTTPClient is a shared HTTP client with a custom DialContext that
// re-validates the resolved IP at connect time to close the TOCTOU window
// (DNS rebinding) that exists when validation is done before the request
// and the dial happens later.
var SafeHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			// When loopback is explicitly allowed (local dev/demo via
			// LLMBOX_ALLOW_LOOPBACK=1), use the loopback-permitting validator
			// at dial time too, keeping DNS-rebinding protection intact.
			dialValidator := ValidateIP
			if loopbackAllowed() {
				dialValidator = ValidateLMLEndpointIPAllowLoopback
			}
			for _, ip := range ips {
				if err := dialValidator(ip.IP, host); err != nil {
					return nil, err
				}
			}
			if len(ips) > 0 {
				addr = net.JoinHostPort(ips[0].IP.String(), port)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	},
}

// SafeLLMHTTPClient is like SafeHTTPClient but allows loopback addresses
// (for local LLM servers like Ollama). It still blocks private non-loopback,
// link-local, and other dangerous IP ranges at dial time.
var SafeLLMHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if err := ValidateLMLEndpointIPAllowLoopback(ip.IP, host); err != nil {
					return nil, err
				}
			}
			if len(ips) > 0 {
				addr = net.JoinHostPort(ips[0].IP.String(), port)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	},
}

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

// IsReservedIP reports whether ip falls in a reserved range that should not
// be reachable from production code (TEST-NET, CGNAT, 0.0.0.0/8, etc.).
func IsReservedIP(ip net.IP) bool {
	// Use To4() to handle both IPv4 and IPv4-mapped IPv6 addresses
	ip4 := ip.To4()
	if ip4 == nil {
		// Pure IPv6 - block ULA (fc00::/7)
		if len(ip) == 16 && ip[0]&0xfe == 0xfc {
			return true
		}
		return false
	}

	// 0.0.0.0/8
	if ip4[0] == 0 {
		return true
	}
	// 169.254.0.0/16 (link-local, also caught by IsLinkLocalUnicast but double-check)
	if ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	// 192.0.2.0/24 (TEST-NET-1)
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
		return true
	}
	// 198.51.100.0/24 (TEST-NET-2)
	if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
		return true
	}
	// 203.0.113.0/24 (TEST-NET-3)
	if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}

	return false
}

// MaxHTTPResponseSize bounds how much of an HTTP response body the nodes
// are willing to read. It mirrors the constant previously defined in
// http_request.go and is shared by fetch_url, notify, and the LLM base.
const MaxHTTPResponseSize = 10 * 1024 * 1024 // 10MB

// GetWorkDir returns the cached working directory used for path validation.
func GetWorkDir() string {
	return workDir
}
