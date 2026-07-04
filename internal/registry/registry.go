package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
)

// safeHTTPClient is a shared HTTP client with a timeout to prevent
// slowloris-style hangs against the registry sync / node download endpoints.
var safeHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

type NodeInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
	Category    string   `json:"category"`
}

type Registry struct {
	Nodes []NodeInfo `json:"nodes"`
}

const defaultRegistryURL = "https://raw.githubusercontent.com/alib8b8/llm-box/main/nodes-registry.json"

func GetRegistryPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "llm-box", "registry.json")
	}
	return filepath.Join(configDir, "llm-box", "registry.json")
}

func LoadRegistry() (*Registry, error) {
	path := GetRegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("registry not found, run 'llm-box registry sync' first")
		}
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	return &reg, nil
}

func SyncRegistry() error {
	logger.Info("syncing node registry")

	if err := validateRegistryURL(defaultRegistryURL); err != nil {
		return fmt.Errorf("invalid registry URL: %w", err)
	}

	resp, err := safeHTTPClient.Get(defaultRegistryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	// Limit registry size to 5MB to prevent OOM
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return fmt.Errorf("failed to read registry data: %w", err)
	}

	path := GetRegistryPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write registry: %w", err)
	}

	logger.Info("registry synced successfully")
	return nil
}

func ListNodes() ([]NodeInfo, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Nodes, nil
}

func SearchNodes(query string) ([]NodeInfo, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	var results []NodeInfo
	for _, node := range reg.Nodes {
		if containsIgnoreCase(node.Name, query) ||
			containsIgnoreCase(node.Description, query) ||
			containsTag(node.Tags, query) {
			results = append(results, node)
		}
	}

	return results, nil
}

func containsIgnoreCase(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	s = lowercase(s)
	substr = lowercase(substr)
	return contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func lowercase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func containsTag(tags []string, query string) bool {
	q := lowercase(query)
	for _, tag := range tags {
		if lowercase(tag) == q {
			return true
		}
	}
	return false
}

func GetNode(name string) (*NodeInfo, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	for _, node := range reg.Nodes {
		if node.Name == name {
			return &node, nil
		}
	}

	return nil, fmt.Errorf("node '%s' not found in registry", name)
}

func InstallNode(name string) error {
	node, err := GetNode(name)
	if err != nil {
		return err
	}

	return downloadNode(node)
}

func downloadNode(node *NodeInfo) error {
	// Validate node name to prevent path traversal
	if !isValidNodeName(node.Name) {
		return fmt.Errorf("invalid node name: %q (only alphanumeric, hyphens, underscores allowed)", node.Name)
	}

	// Validate URL: enforce HTTPS and SSRF protection
	if err := validateRegistryURL(node.URL); err != nil {
		return fmt.Errorf("invalid node URL: %w", err)
	}

	logger.Info("installing node", "name", node.Name, "url", node.URL)

	resp, err := safeHTTPClient.Get(node.URL)
	if err != nil {
		return fmt.Errorf("failed to download node: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Limit download size to prevent OOM
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 1MB max node file
	if err != nil {
		return fmt.Errorf("failed to read node data: %w", err)
	}

	nodesDir, err := GetNodesDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(nodesDir, node.Name+".yaml")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write node file: %w", err)
	}

	logger.Info("node installed", "name", node.Name, "path", filePath)
	return nil
}

// isValidNodeName checks if a node name is safe for filesystem use
func isValidNodeName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// validateRegistryURL enforces HTTPS and SSRF protection for any URL used to
// sync the registry or download node files. Plain HTTP is rejected to prevent
// man-in-the-middle tampering of downloaded workflow definitions.
func validateRegistryURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Enforce HTTPS only - registry/node downloads are trusted code, so plain
	// HTTP is not acceptable.
	if u.Scheme != "https" {
		return fmt.Errorf("only https URLs are allowed, got: %s", u.Scheme)
	}

	// Block userinfo to prevent credential leakage
	if u.User != nil {
		return fmt.Errorf("URLs with userinfo (credentials) are not allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Block localhost variants
	if isLocalhost(host) {
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Validate resolved IPs to prevent SSRF
	if ip := net.ParseIP(host); ip != nil {
		if err := validatePublicIP(ip, host); err != nil {
			return err
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve host %s: %w", host, err)
		}
		for _, resolvedIP := range ips {
			if err := validatePublicIP(resolvedIP, host); err != nil {
				return err
			}
		}
	}

	return nil
}

func isLocalhost(host string) bool {
	switch host {
	case "localhost", "localhost.localdomain", "ip6-localhost", "ip6-loopback":
		return true
	}
	return false
}

// validatePublicIP rejects loopback / private / link-local / unspecified /
// multicast / reserved IPs to prevent SSRF against internal services.
func validatePublicIP(ip net.IP, displayHost string) error {
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
	return nil
}

func GetNodesDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config dir: %w", err)
	}
	nodesDir := filepath.Join(configDir, "llm-box", "nodes")
	if err := os.MkdirAll(nodesDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create nodes directory: %w", err)
	}
	return nodesDir, nil
}

func ListInstalledNodes() ([]string, error) {
	nodesDir, err := GetNodesDir()
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(nodesDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeNames []string
	for _, file := range files {
		name := filepath.Base(file)
		name = name[:len(name)-5]
		nodeNames = append(nodeNames, name)
	}

	return nodeNames, nil
}

func UninstallNode(name string) error {
	nodesDir, err := GetNodesDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(nodesDir, name+".yaml")
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("node '%s' is not installed", name)
		}
		return fmt.Errorf("failed to uninstall node: %w", err)
	}

	logger.Info("node uninstalled", "name", name)
	return nil
}
