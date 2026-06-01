package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchURLNode(t *testing.T) {
	// Test that fetch_url node is registered
	node, ok := Get("fetch_url")
	if !ok {
		t.Fatal("fetch_url node not found in registry")
	}
	if node.Name() != "fetch_url" {
		t.Errorf("expected node name 'fetch_url', got '%s'", node.Name())
	}
}

func TestFetchURLExecute(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello from mock server!"))
	}))
	defer mockServer.Close()

	// Get fetch_url node
	node, ok := Get("fetch_url")
	if !ok {
		t.Fatal("fetch_url node not found")
	}

	// Test with URL in params
	ctx := context.Background()
	input := ""
	params := map[string]string{
		"url": mockServer.URL,
	}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "Hello from mock server!"
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}
}

func TestFetchURLFromInput(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("URL from input!"))
	}))
	defer mockServer.Close()

	node, ok := Get("fetch_url")
	if !ok {
		t.Fatal("fetch_url node not found")
	}

	ctx := context.Background()
	input := mockServer.URL
	params := map[string]string{}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "URL from input!"
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}
}

func TestFileWriteNode(t *testing.T) {
	// Test that file_write node is registered
	node, ok := Get("file_write")
	if !ok {
		t.Fatal("file_write node not found in registry")
	}
	if node.Name() != "file_write" {
		t.Errorf("expected node name 'file_write', got '%s'", node.Name())
	}
}

func TestFileWriteExecute(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "test-file-write-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "test.txt")

	// Get file_write node
	node, ok := Get("file_write")
	if !ok {
		t.Fatal("file_write node not found")
	}

	// Execute
	ctx := context.Background()
	input := "This is test content to write!"
	params := map[string]string{
		"path": tmpFile,
	}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check output message
	expectedMsg := "written to " + tmpFile
	if output != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, output)
	}

	// Read back and verify
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(content) != input {
		t.Errorf("expected content '%s', got '%s'", input, string(content))
	}
}
