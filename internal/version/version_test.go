package version

import "testing"

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
