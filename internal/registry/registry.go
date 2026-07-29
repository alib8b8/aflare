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
	"sort"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/httpclient"
	"github.com/alib8b8/llm-box/internal/logger"
)

// safeHTTPClient is the registry's shared HTTP client. It uses the
// httpclient factory so that SSRF defense (re-resolve + validate at dial
// time, closing the DNS-rebinding TOCTOU window) and connection-pool
// tuning (MaxIdleConns / MaxIdleConnsPerHost / IdleConnTimeout) are
// shared with every other outbound client in llm-box rather than
// re-implemented here. Registry endpoints are public-only, so we use
// ValidatePublic (loopback blocked).
var safeHTTPClient = httpclient.NewClient(httpclient.Options{
	Timeout:   30 * time.Second,
	Validator: httpclient.ValidatePublic,
})

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
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated config path
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
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsTag(tags []string, query string) bool {
	q := strings.ToLower(query)
	for _, tag := range tags {
		if strings.ToLower(tag) == q {
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

	// If the host is already an IP literal, validate it now. Hostname-based
	// resolution is deferred to the custom DialContext on safeHTTPClient so
	// that there is no TOCTOU window for DNS rebinding attacks.
	if ip := net.ParseIP(host); ip != nil {
		if err := validatePublicIP(ip, host); err != nil {
			return err
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

// validatePublicIP delegates to the shared httpclient validator so the
// IP-range policy (loopback/private/link-local/unspecified/multicast/
// reserved) lives in exactly one place. validateRegistryURL still does
// the URL-scheme/host checks that httpclient cannot (HTTPS-only, block
// userinfo, block localhost-by-name); the IP-literal fast path here
// covers the case where the URL host is already an IP, while the
// DialContext on safeHTTPClient covers the hostname case at connect time.
func validatePublicIP(ip net.IP, displayHost string) error {
	return httpclient.ValidatePublic(ip, displayHost)
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

	yamlFiles, err := filepath.Glob(filepath.Join(nodesDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	ymlFiles, err := filepath.Glob(filepath.Join(nodesDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	files := append(yamlFiles, ymlFiles...)

	var nodeNames []string
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		nodeNames = append(nodeNames, name)
	}

	sort.Strings(nodeNames)

	return nodeNames, nil
}

func UninstallNode(name string) error {
	if !isValidNodeName(name) {
		return fmt.Errorf("invalid node name: %q", name)
	}

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
