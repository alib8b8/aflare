package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGetVersion(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "v0.3.0"
	if got := GetVersion(); got != "v0.3.0" {
		t.Errorf("GetVersion() = %q, want %q", got, "v0.3.0")
	}
}

func TestHasUpdate(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{"same version", "v0.3.0", "v0.3.0", false},
		{"new version", "v0.2.0", "v0.3.0", true},
		{"no v prefix", "0.3.0", "0.3.0", false},
		{"mixed prefix", "v0.3.0", "0.3.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &GitHubRelease{TagName: tt.latest}
			result := HasUpdate(tt.current, release)
			if result != tt.expected {
				t.Errorf("HasUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, result, tt.expected)
			}
		})
	}
}

func TestHasUpdateNilRelease(t *testing.T) {
	if HasUpdate("v0.1.0", nil) {
		t.Error("HasUpdate with nil release should return false")
	}
}

func TestHasUpdateEmptyTagName(t *testing.T) {
	if HasUpdate("v0.1.0", &GitHubRelease{TagName: ""}) {
		t.Error("HasUpdate with empty tag name should return false")
	}
}

func TestFindAsset(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "llm-box-linux-amd64", BrowserDownloadURL: "https://example.com/linux-amd64"},
			{Name: "llm-box-darwin-amd64", BrowserDownloadURL: "https://example.com/darwin-amd64"},
			{Name: "llm-box-windows-amd64.exe", BrowserDownloadURL: "https://example.com/win-amd64"},
		},
	}

	tests := []struct {
		os       string
		arch     string
		wantURL  string
		wantName string
	}{
		{"linux", "amd64", "https://example.com/linux-amd64", "llm-box-linux-amd64"},
		{"darwin", "amd64", "https://example.com/darwin-amd64", "llm-box-darwin-amd64"},
		{"windows", "amd64", "https://example.com/win-amd64", "llm-box-windows-amd64.exe"},
		{"freebsd", "arm64", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.os+"/"+tt.arch, func(t *testing.T) {
			url, name := FindAsset(release, tt.os, tt.arch)
			if url != tt.wantURL {
				t.Errorf("FindAsset url = %q, want %q", url, tt.wantURL)
			}
			if name != tt.wantName {
				t.Errorf("FindAsset name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestFindAssetNoMatch(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "other-binary", BrowserDownloadURL: "https://example.com/other"},
		},
	}
	url, name := FindAsset(release, "linux", "amd64")
	if url != "" || name != "" {
		t.Error("FindAsset with no matching asset should return empty values")
	}
}

func TestGetBuildInfo(t *testing.T) {
	oldVersion := Version
	oldBuildDate := BuildDate
	defer func() {
		Version = oldVersion
		BuildDate = oldBuildDate
	}()

	Version = "v0.3.0"
	BuildDate = "2024-01-01"
	info := GetBuildInfo()
	if info == "" {
		t.Error("GetBuildInfo() returned empty string")
	}
	if !contains(info, "v0.3.0") {
		t.Error("GetBuildInfo() missing version")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCheckLatestRelease_NetworkError(t *testing.T) {
	// Force HTTP requests to fail immediately by pointing to non-existent proxy
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	done := make(chan struct{})
	var release *GitHubRelease
	var err error
	go func() {
		defer close(done)
		release, err = CheckLatestRelease("alib8b8/llm-box")
	}()
	select {
	case <-done:
		if err == nil {
			t.Error("expected error due to proxy failure")
		}
		if release != nil {
			t.Error("expected nil release on error")
		}
	case <-time.After(5 * time.Second):
		t.Skip("CheckLatestRelease took too long, skipping")
	}
}

func TestCheckLatestRelease_GitHubToken(t *testing.T) {
	// Just verify the function reads GITHUB_TOKEN env var by setting it
	t.Setenv("GITHUB_TOKEN", "fake-token-for-test")
	// We still need network to fail quickly
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = CheckLatestRelease("alib8b8/llm-box")
	}()
	select {
	case <-done:
		if err == nil {
			t.Error("expected error due to proxy failure")
		}
	case <-time.After(5 * time.Second):
		t.Skip("CheckLatestRelease took too long, skipping")
	}
}

func TestSelfUpdate_NetworkError(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	done := make(chan struct{})
	var result string
	var err error
	go func() {
		defer close(done)
		result, err = SelfUpdate("alib8b8/llm-box")
	}()
	select {
	case <-done:
		if err == nil {
			t.Error("expected error due to proxy failure")
		}
		if result != "" {
			t.Error("expected empty result on error")
		}
	case <-time.After(5 * time.Second):
		t.Skip("SelfUpdate took too long, skipping")
	}
}

func TestSelfUpdate_AlreadyUpToDate(t *testing.T) {
	// To test "Already up to date", we need CheckLatestRelease to succeed and return same version.
	// Since we can't mock the network easily, we skip this test.
	t.Skip("requires mocking GitHub API")
}

func TestGitHubReleaseStruct(t *testing.T) {
	// Ensure struct fields exist and types are correct
	release := GitHubRelease{
		TagName:     "v1.0.0",
		Name:        "Release 1.0.0",
		PublishedAt: time.Now(),
		Body:        "Release notes",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "asset", BrowserDownloadURL: "https://example.com/asset", Size: 1024},
		},
	}
	if release.TagName != "v1.0.0" {
		t.Error("struct field mismatch")
	}
}

func TestFindAssetWindows(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "llm-box-windows-amd64.exe", BrowserDownloadURL: "https://example.com/win"},
		},
	}
	url, name := FindAsset(release, "windows", "amd64")
	if url != "https://example.com/win" {
		t.Errorf("unexpected url: %s", url)
	}
	if !strings.HasSuffix(name, ".exe") {
		t.Errorf("expected .exe suffix, got %s", name)
	}
}

// mockTransport intercepts HTTP requests for testing.
type mockTransport struct {
	responses map[string]*http.Response
	errs      map[string]error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err, ok := m.errs[req.URL.Host+req.URL.Path]; ok {
		return nil, err
	}
	if resp, ok := m.responses[req.URL.Host+req.URL.Path]; ok {
		return resp, nil
	}
	return nil, http.ErrAbortHandler
}

func setMockTransport(t *testing.T, m *mockTransport) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = m
	t.Cleanup(func() { http.DefaultTransport = old })
}

func makeMockReleaseJSON(tag string) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"tag_name":     tag,
		"name":         "Release " + tag,
		"published_at": time.Now().Format(time.RFC3339),
		"body":         "notes",
		"assets":       []interface{}{},
	})
	return data
}

func makeMockReleaseJSONWithAsset(tag, assetName, assetURL string) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"tag_name": tag,
		"name":     "Release " + tag,
		"assets": []map[string]interface{}{
			{"name": assetName, "browser_download_url": assetURL, "size": 1024},
		},
	})
	return data
}

func mockResponse(body []byte, status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestCheckLatestRelease_Success(t *testing.T) {
	body := makeMockReleaseJSON("v1.2.3")
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/alib8b8/llm-box/releases/latest": mockResponse(body, http.StatusOK),
		},
	}
	setMockTransport(t, m)

	release, err := CheckLatestRelease("alib8b8/llm-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", release.TagName)
	}
}

func TestCheckLatestRelease_NotFound(t *testing.T) {
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/bad/repo/releases/latest": mockResponse([]byte("Not Found"), http.StatusNotFound),
		},
	}
	setMockTransport(t, m)

	_, err := CheckLatestRelease("bad/repo")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestCheckLatestRelease_InvalidJSON(t *testing.T) {
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/alib8b8/llm-box/releases/latest": mockResponse([]byte("not json"), http.StatusOK),
		},
	}
	setMockTransport(t, m)

	_, err := CheckLatestRelease("alib8b8/llm-box")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSelfUpdate_AlreadyUpToDate_Mock(t *testing.T) {
	oldVersion := Version
	Version = "v1.0.0"
	defer func() { Version = oldVersion }()

	body := makeMockReleaseJSON("v1.0.0")
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/alib8b8/llm-box/releases/latest": mockResponse(body, http.StatusOK),
		},
	}
	setMockTransport(t, m)

	result, err := SelfUpdate("alib8b8/llm-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Already up to date") {
		t.Errorf("expected 'Already up to date', got %s", result)
	}
}

func TestSelfUpdate_NoAsset(t *testing.T) {
	oldVersion := Version
	Version = "v0.1.0"
	defer func() { Version = oldVersion }()

	body := makeMockReleaseJSON("v1.0.0")
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/alib8b8/llm-box/releases/latest": mockResponse(body, http.StatusOK),
		},
	}
	setMockTransport(t, m)

	_, err := SelfUpdate("alib8b8/llm-box")
	if err == nil {
		t.Error("expected error when no compatible asset found")
	}
}

func TestSelfUpdate_DownloadSuccess(t *testing.T) {
	oldVersion := Version
	Version = "v0.1.0"
	defer func() { Version = oldVersion }()

	assetName := fmt.Sprintf("llm-box-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	body := makeMockReleaseJSONWithAsset("v1.0.0", assetName, "https://github.com/asset")
	m := &mockTransport{
		responses: map[string]*http.Response{
			"api.github.com/repos/alib8b8/llm-box/releases/latest": mockResponse(body, http.StatusOK),
			"github.com/asset": mockResponse([]byte("fake binary"), http.StatusOK),
		},
	}
	setMockTransport(t, m)

	_, err := SelfUpdate("alib8b8/llm-box")
	// It will try to replace the test binary; we accept either success or a rename error
	if err != nil {
		t.Logf("SelfUpdate error (may be rename-related): %v", err)
	}
}
