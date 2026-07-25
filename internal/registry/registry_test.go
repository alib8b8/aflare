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
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTempConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	return tmpDir
}

func TestIsValidNodeName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"my-node", true},
		{"my_node", true},
		{"MyNode123", true},
		{"", false},
		{"node with spaces", false},
		{"node/../../../etc", false},
		{"node.tar.gz", false},
	}

	for _, tt := range tests {
		result := isValidNodeName(tt.name)
		if result != tt.expected {
			t.Errorf("isValidNodeName(%q) = %v, want %v", tt.name, result, tt.expected)
		}
	}

	// Test too long name
	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}
	if isValidNodeName(string(longName)) {
		t.Error("expected false for name > 100 chars")
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	if !containsIgnoreCase("Hello World", "hello") {
		t.Error("expected case-insensitive match")
	}
	if containsIgnoreCase("Hello", "xyz") {
		t.Error("expected no match")
	}
	if containsIgnoreCase("", "test") {
		t.Error("expected false for empty string")
	}
	if containsIgnoreCase("test", "") {
		t.Error("expected false for empty substring")
	}
	// Unicode support (was broken with old ASCII-only lowercase)
	if !containsIgnoreCase("ÉMoji Café", "émoji") {
		t.Error("expected Unicode case-insensitive match")
	}
}

func TestStringsToLower(t *testing.T) {
	// Verify standard library handles Unicode correctly
	if strings.ToLower("HÉLLO") != "héllo" {
		t.Error("strings.ToLower should handle Unicode")
	}
}

func TestContainsTag(t *testing.T) {
	tags := []string{"LLM", "AI", "NLP"}
	if !containsTag(tags, "llm") {
		t.Error("expected case-insensitive tag match")
	}
	if containsTag(tags, "ml") {
		t.Error("expected no match for 'ml'")
	}
	if containsTag(nil, "test") {
		t.Error("expected no match for nil tags")
	}
	if containsTag([]string{}, "test") {
		t.Error("expected no match for empty tags")
	}
}

func TestLoadRegistryNotFound(t *testing.T) {
	setupTempConfig(t)
	_, err := LoadRegistry()
	if err == nil {
		t.Error("expected error for missing registry")
	}
}

func TestLoadRegistry_InvalidJSON(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	os.WriteFile(filepath.Join(regDir, "registry.json"), []byte("invalid"), 0600)

	_, err := LoadRegistry()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadRegistry_Success(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{
		Nodes: []NodeInfo{
			{Name: "test-node", Description: "test", Version: "1.0.0"},
		},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	loaded, err := LoadRegistry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(loaded.Nodes))
	}
	if loaded.Nodes[0].Name != "test-node" {
		t.Errorf("expected test-node, got %s", loaded.Nodes[0].Name)
	}
}

func TestRegistryJSONRoundTrip(t *testing.T) {
	original := Registry{
		Nodes: []NodeInfo{
			{
				Name:        "ollama",
				Description: "Run Ollama models locally",
				Version:     "1.0.0",
				Author:      "test",
				URL:         "https://example.com/ollama.yaml",
				Tags:        []string{"llm", "local"},
				Category:    "ai",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed Registry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(parsed.Nodes))
	}
	if parsed.Nodes[0].Name != "ollama" {
		t.Errorf("expected name 'ollama', got '%s'", parsed.Nodes[0].Name)
	}
	if len(parsed.Nodes[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(parsed.Nodes[0].Tags))
	}
}

func TestSearchNodesWithFile(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{
		Nodes: []NodeInfo{
			{Name: "test-node", Description: "A test node", Tags: []string{"test"}},
			{Name: "other-node", Description: "Another node", Tags: []string{"other"}},
		},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	results, err := SearchNodes("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "test-node" {
		t.Errorf("expected test-node, got %s", results[0].Name)
	}
}

func TestSearchNodes_NoMatch(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{Nodes: []NodeInfo{{Name: "node1", Description: "desc", Tags: []string{"tag1"}}}}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	results, err := SearchNodes("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestListNodes(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{
		Nodes: []NodeInfo{
			{Name: "node1", Description: "first"},
			{Name: "node2", Description: "second"},
		},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	nodes, err := ListNodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestGetNode_Found(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{
		Nodes: []NodeInfo{{Name: "found-node", Description: "found"}},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	node, err := GetNode("found-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Name != "found-node" {
		t.Errorf("expected found-node, got %s", node.Name)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{Nodes: []NodeInfo{}}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	_, err := GetNode("missing")
	if err == nil {
		t.Error("expected error for missing node")
	}
}

func TestGetNodesDir(t *testing.T) {
	setupTempConfig(t)
	dir, err := GetNodesDir()
	if err != nil {
		t.Fatalf("GetNodesDir failed: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty directory path")
	}
}

func TestGetRegistryPath(t *testing.T) {
	path := GetRegistryPath()
	if path == "" {
		t.Error("expected non-empty registry path")
	}
	if filepath.Base(path) != "registry.json" {
		t.Errorf("expected path ending in registry.json, got %s", path)
	}
}

func TestListInstalledNodes(t *testing.T) {
	tmpDir := setupTempConfig(t)
	nodesDir := filepath.Join(tmpDir, "llm-box", "nodes")
	os.MkdirAll(nodesDir, 0750)
	os.WriteFile(filepath.Join(nodesDir, "node1.yaml"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(nodesDir, "node2.yml"), []byte("test"), 0644)

	names, err := ListInstalledNodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(names))
	}
	if names[0] != "node1" || names[1] != "node2" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestListInstalledNodes_Empty(t *testing.T) {
	tmpDir := setupTempConfig(t)
	nodesDir := filepath.Join(tmpDir, "llm-box", "nodes")
	os.MkdirAll(nodesDir, 0750)

	names, err := ListInstalledNodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(names))
	}
}

func TestUninstallNode(t *testing.T) {
	tmpDir := setupTempConfig(t)
	nodesDir := filepath.Join(tmpDir, "llm-box", "nodes")
	os.MkdirAll(nodesDir, 0750)
	os.WriteFile(filepath.Join(nodesDir, "testnode.yaml"), []byte("test"), 0644)

	if err := UninstallNode("testnode"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := os.Stat(filepath.Join(nodesDir, "testnode.yaml"))
	if !os.IsNotExist(err) {
		t.Error("expected file to be removed")
	}
}

func TestUninstallNode_NotInstalled(t *testing.T) {
	setupTempConfig(t)
	if err := UninstallNode("notinstalled"); err == nil {
		t.Error("expected error for not-installed node")
	}
}

func TestUninstallNode_InvalidName(t *testing.T) {
	if err := UninstallNode("../etc/passwd"); err == nil {
		t.Error("expected error for invalid node name")
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"localhost.localdomain", true},
		{"ip6-localhost", true},
		{"ip6-loopback", true},
		{"example.com", false},
		{"127.0.0.1", false}, // isLocalhost only checks strings
		{"", false},
	}

	for _, tt := range tests {
		result := isLocalhost(tt.host)
		if result != tt.expected {
			t.Errorf("isLocalhost(%q) = %v, want %v", tt.host, result, tt.expected)
		}
	}
}

func TestValidatePublicIP(t *testing.T) {
	tests := []struct {
		ip      string
		wantErr bool
	}{
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", tt.ip)
		}
		err := validatePublicIP(ip, tt.ip)
		if (err != nil) != tt.wantErr {
			t.Errorf("validatePublicIP(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
		}
	}
}

func TestValidateRegistryURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/registry.json", false},
		{"https://raw.githubusercontent.com/alib8b8/llm-box/main/nodes-registry.json", false},
		{"http://example.com/registry.json", true},
		{"ftp://example.com/registry.json", true},
		{"https://localhost/registry.json", true},
		{"https://127.0.0.1/registry.json", true},
		{"https://10.0.0.1/registry.json", true},
		{"https://user:pass@example.com/registry.json", true},
		{"not-a-url", true},
		{"", true},
	}

	for _, tt := range tests {
		err := validateRegistryURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateRegistryURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}

func TestInstallNode_InvalidName(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{
		Nodes: []NodeInfo{{Name: "bad../name", URL: "https://example.com/node.yaml"}},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	if err := InstallNode("bad../name"); err == nil {
		t.Error("expected error for invalid node name")
	}
}

func TestInstallNode_NotFound(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	reg := Registry{Nodes: []NodeInfo{}}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(regDir, "registry.json"), data, 0600)

	if err := InstallNode("missing"); err == nil {
		t.Error("expected error for missing node")
	}
}

func TestDownloadNode_InvalidURL(t *testing.T) {
	// HTTP URL should be rejected by validateRegistryURL
	node := &NodeInfo{Name: "testnode", URL: "http://example.com/node.yaml"}
	if err := downloadNode(node); err == nil {
		t.Error("expected error for HTTP URL")
	}
}

func TestDownloadNode_LocalhostURL(t *testing.T) {
	node := &NodeInfo{Name: "testnode", URL: "https://localhost/node.yaml"}
	if err := downloadNode(node); err == nil {
		t.Error("expected error for localhost URL")
	}
}

func TestDownloadNode_InvalidName(t *testing.T) {
	node := &NodeInfo{Name: "bad/name", URL: "https://example.com/node.yaml"}
	if err := downloadNode(node); err == nil {
		t.Error("expected error for invalid node name")
	}
}

func TestSyncRegistry_Network(t *testing.T) {
	setupTempConfig(t)
	// SyncRegistry will attempt to fetch from GitHub. In CI it may succeed or fail quickly.
	// We use a short timeout via context is not supported, so just call and log result.
	done := make(chan error, 1)
	go func() { done <- SyncRegistry() }()
	select {
	case err := <-done:
		if err != nil {
			t.Logf("SyncRegistry returned error (network may be unavailable): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Skip("SyncRegistry network call timed out")
	}
}

func TestDownloadNode_NetworkError(t *testing.T) {
	// Use a valid-looking HTTPS URL to a non-existent host so DNS fails quickly
	node := &NodeInfo{Name: "testnode", URL: "https://this-host-definitely-does-not-exist-12345.example.com/node.yaml"}
	done := make(chan error, 1)
	go func() { done <- downloadNode(node) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error for unreachable host")
		}
	case <-time.After(10 * time.Second):
		t.Skip("downloadNode network call timed out")
	}
}

func TestValidateRegistryURL_IPLiteral(t *testing.T) {
	// Public IP should pass
	if err := validateRegistryURL("https://8.8.8.8/path"); err != nil {
		t.Errorf("unexpected error for public IP: %v", err)
	}
}

func TestGetNodesDir_ConfigDirError(t *testing.T) {
	// Unset HOME and XDG_CONFIG_HOME to force os.UserConfigDir error
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	_, err := GetNodesDir()
	if err == nil {
		t.Error("expected error when config dir cannot be determined")
	}
}

func TestGetRegistryPath_NoConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	path := GetRegistryPath()
	if path == "" {
		t.Error("expected fallback path")
	}
	if !strings.Contains(path, os.TempDir()) {
		t.Errorf("expected temp dir fallback, got %s", path)
	}
}

func TestLoadRegistry_ReadError(t *testing.T) {
	tmpDir := setupTempConfig(t)
	regDir := filepath.Join(tmpDir, "llm-box")
	os.MkdirAll(regDir, 0750)
	regPath := filepath.Join(regDir, "registry.json")
	os.WriteFile(regPath, []byte("valid but not json"), 0000) // no read permission
	// On some systems (e.g. running as root) this may still be readable.
	// Skip if we can still read it.
	if _, err := os.ReadFile(regPath); err == nil {
		t.Skip("running as root or file still readable, skipping")
	}
	_, err := LoadRegistry()
	if err == nil {
		t.Error("expected error when file is unreadable")
	}
}

func TestUninstallNode_RemoveError(t *testing.T) {
	// Create a directory instead of a file so os.Remove fails (or use a read-only parent)
	tmpDir := setupTempConfig(t)
	nodesDir := filepath.Join(tmpDir, "llm-box", "nodes")
	os.MkdirAll(nodesDir, 0750)
	// Create a directory with the target name so os.Remove on a non-empty dir may fail
	os.MkdirAll(filepath.Join(nodesDir, "testnode.yaml"), 0750)
	// Write a nested file so the directory is non-empty and remove fails on some OS
	os.WriteFile(filepath.Join(nodesDir, "testnode.yaml", "inner"), []byte("x"), 0644)
	err := UninstallNode("testnode")
	if err == nil {
		t.Error("expected error when removing non-empty directory posing as node file")
	}
}
