package nodes

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	workDir string
)

func init() {
	wd, err := os.Getwd()
	if err == nil {
		workDir = wd
	}
}

func safeJoinPath(baseDir, userPath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}

	if strings.Contains(userPath, "..") {
		return "", fmt.Errorf("path traversal detected: '..' is not allowed in path")
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
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path escapes the allowed directory")
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

	ext := strings.ToLower(filepath.Ext(safePath))
	allowedExts := map[string]bool{
		".txt":  true,
		".md":   true,
		".yaml": true,
		".yml":  true,
		".json": true,
		".html": true,
		".csv":  true,
		".xml":  true,
		".log":  true,
		".py":   true,
		".sh":   true,
		".go":   true,
		".js":   true,
		".ts":   true,
		".css":  true,
		".svg":  true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".pdf":  true,
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

// validateURL checks if a URL is safe to request (SSRF protection)
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed, got: %s", u.Scheme)
	}

	host := u.Hostname()

	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Block localhost variants
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "localhost.localdomain" {
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Try to parse as IP
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() {
			return fmt.Errorf("access to loopback address %s is not allowed", host)
		}
		if ip.IsPrivate() {
			return fmt.Errorf("access to private address %s is not allowed", host)
		}
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("access to link-local address %s is not allowed", host)
		}
		if isReservedIP(ip) {
			return fmt.Errorf("access to reserved address %s is not allowed", host)
		}
	}

	return nil
}

func isReservedIP(ip net.IP) bool {
	// 0.0.0.0/8
	if len(ip) == 4 && ip[0] == 0 {
		return true
	}
	// 169.254.0.0/16 (already handled by IsLinkLocalUnicast but double-check)
	if len(ip) == 4 && ip[0] == 169 && ip[1] == 254 {
		return true
	}
	// 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 (TEST-NET)
	if len(ip) == 4 && ip[0] == 192 && ip[1] == 0 && ip[2] == 2 {
		return true
	}
	if len(ip) == 4 && ip[0] == 198 && ip[1] == 51 && ip[2] == 100 {
		return true
	}
	if len(ip) == 4 && ip[0] == 203 && ip[1] == 0 && ip[2] == 113 {
		return true
	}
	return false
}
