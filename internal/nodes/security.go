package nodes

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	workDir string
)

const (
	DefaultLLMTimeout = 120 * time.Second
)

func init() {
	wd, err := os.Getwd()
	if err == nil {
		workDir = wd
	}
}

// httpRedirectValidator returns an http.Client CheckRedirect function that
// validates each redirect target with the given validator (validateURL for
// general HTTP, validateLMLEndpoint for LLM endpoints). It also caps the
// number of redirects to prevent redirect loops.
func httpRedirectValidator(validator func(string) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return validator(req.URL.String())
	}
}

func safeJoinPath(baseDir, userPath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}

	cleanPath := filepath.Clean(userPath)
	if strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
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
		return "", fmt.Errorf("path escapes the allowed directory")
	}

	// Resolve symlinks to prevent symlink-based bypass
	resolvedPath, err := filepath.EvalSymlinks(absFullPath)
	if err == nil {
		resolvedRel, err := filepath.Rel(absBase, resolvedPath)
		if err != nil || strings.HasPrefix(resolvedRel, "..") {
			return "", fmt.Errorf("path escapes the allowed directory (symlink)")
		}
		return resolvedPath, nil
	}

	return absFullPath, nil
}

func validateReadPath(path string) (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("working directory not available")
	}
	return safeJoinPath(workDir, path)
}

func validateWritePath(path string) (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("working directory not available")
	}
	safePath, err := safeJoinPath(workDir, path)
	if err != nil {
		return "", err
	}

	// Reject dotfiles (e.g. .bashrc, .ssh/authorized_keys)
	baseName := filepath.Base(safePath)
	if strings.HasPrefix(baseName, ".") {
		return "", fmt.Errorf("dotfiles are not allowed for writing")
	}

	ext := strings.ToLower(filepath.Ext(safePath))
	allowedExts := map[string]bool{
		".txt":        true,
		".md":         true,
		".yaml":       true,
		".yml":        true,
		".json":       true,
		".csv":        true,
		".xml":        true,
		".log":        true,
		".html":       true,
		".htm":        true,
		".css":        true,
		".js":         true,
		".ts":         true,
		".py":         true,
		".go":         true,
		".rs":         true,
		".java":       true,
		".c":          true,
		".cpp":        true,
		".h":          true,
		".hpp":        true,
		".rb":         true,
		".php":        true,
		".sh":         true,
		".bash":       true,
		".zsh":        true,
		".fish":       true,
		".bat":        true,
		".ps1":        true,
		".sql":        true,
		".toml":       true,
		".ini":        true,
		".conf":       true,
		".cfg":        true,
		".env":        true,
		".dockerfile": true,
		".makefile":   true,
		".svg":        true,
		".png":        true,
		".jpg":        true,
		".jpeg":       true,
		".gif":        true,
		".webp":       true,
		".pdf":        true,
		".epub":       true,
		".mobi":       true,
		".mp3":        true,
		".wav":        true,
		".mp4":        true,
		".mov":        true,
		".avi":        true,
		".zip":        true,
		".tar":        true,
		".gz":         true,
		".7z":         true,
	}

	if ext != "" && !allowedExts[ext] {
		return "", fmt.Errorf("file extension '%s' is not allowed for writing", ext)
	}

	return safePath, nil
}

func redactAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	sensitivePrefixes := []string{"api", "token", "bearer", "password", "passwd", "secret", "auth"}
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, "_"+prefix) || strings.Contains(lower, "-"+prefix) {
			return true
		}
	}
	return false
}

// validateURL checks if a URL is safe to request (SSRF protection)
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
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

	// Block localhost variants
	lowerHost := strings.ToLower(host)
	localhostVariants := map[string]bool{
		"localhost":             true,
		"localhost.localdomain": true,
		"ip6-localhost":         true,
		"ip6-loopback":          true,
	}
	if localhostVariants[lowerHost] {
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip != nil {
		if err := validateIP(ip, host); err != nil {
			return err
		}
	} else {
		// DNS-resolve the hostname to prevent domain-based SSRF
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve host %s: %w", host, err)
		}
		for _, resolvedIP := range ips {
			if err := validateIP(resolvedIP, host); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateIP(ip net.IP, displayHost string) error {
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
	if isReservedIP(ip) {
		return fmt.Errorf("access to reserved address %s is not allowed", displayHost)
	}
	return nil
}

// validateLMLEndpoint validates an LLM API endpoint URL. It is similar to
// validateURL but allows loopback/localhost addresses, because LLM servers
// (e.g. Ollama, llama.cpp) commonly run on http://localhost:11434.
// Non-loopback private addresses and other dangerous ranges remain blocked.
func validateLMLEndpoint(rawURL string) error {
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
		return validateLMLEndpointIP(ip, host)
	}

	// DNS-resolve the hostname to prevent domain-based SSRF
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", host, err)
	}
	for _, resolvedIP := range ips {
		if err := validateLMLEndpointIP(resolvedIP, host); err != nil {
			return err
		}
	}

	return nil
}

// validateLMLEndpointIP validates an IP for LLM endpoints. Loopback is allowed,
// but other private/reserved ranges are still blocked.
func validateLMLEndpointIP(ip net.IP, displayHost string) error {
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
	if isReservedIP(ip) {
		return fmt.Errorf("access to reserved address %s is not allowed", displayHost)
	}
	return nil
}

func isReservedIP(ip net.IP) bool {
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
