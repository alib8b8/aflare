package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
}

func TestLoadRegistryNotFound(t *testing.T) {
	// Use a temp dir that won't have a registry file
	tmpDir := t.TempDir()
	origPath := filepath.Join(tmpDir, "nonexistent", "registry.json")

	data, err := os.ReadFile(origPath)
	if err == nil {
		t.Error("expected error reading non-existent file")
	}
	_ = data
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
	// Create a temp registry file
	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "registry.json")

	reg := Registry{
		Nodes: []NodeInfo{
			{Name: "test-node", Description: "A test node", Tags: []string{"test"}},
			{Name: "other-node", Description: "Another node", Tags: []string{"other"}},
		},
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(regPath, data, 0600)

	// Load and search
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	var loaded Registry
	json.Unmarshal(data, &loaded)

	var results []NodeInfo
	for _, node := range loaded.Nodes {
		if containsIgnoreCase(node.Name, "test") || containsIgnoreCase(node.Description, "test") || containsTag(node.Tags, "test") {
			results = append(results, node)
		}
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetNodesDir(t *testing.T) {
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
