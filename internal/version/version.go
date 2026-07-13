package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
)

type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func GetVersion() string {
	return Version
}

func GetBuildInfo() string {
	return fmt.Sprintf("llm-box version %s\n  built: %s\n  go:    %s\n  os:    %s/%s",
		Version, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func CheckLatestRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	client := &http.Client{Timeout: 10 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	return &release, nil
}

func HasUpdate(currentVersion string, latest *GitHubRelease) bool {
	if latest == nil || latest.TagName == "" {
		return false
	}
	current := strings.TrimPrefix(currentVersion, "v")
	latestTag := strings.TrimPrefix(latest.TagName, "v")
	return current != latestTag
}

func FindAsset(release *GitHubRelease, goos, goarch string) (string, string) {
	suffix := fmt.Sprintf("%s-%s", goos, goarch)
	if goos == "windows" {
		suffix += ".exe"
	}
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, suffix) {
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	return "", ""
}

func SelfUpdate(repo string) (string, error) {
	release, err := CheckLatestRelease(repo)
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}

	if !HasUpdate(Version, release) {
		return "Already up to date (" + Version + ")", nil
	}

	downloadURL, _ := FindAsset(release, runtime.GOOS, runtime.GOARCH)
	if downloadURL == "" {
		return "", fmt.Errorf("no binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	tmpPath := exePath + ".tmp"

	ctx, cancel := context.WithTimeout(context.Background(), 5*60*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 5 * 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	out.Close()

	backupPath := exePath + ".bak"
	os.Rename(exePath, backupPath)

	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Rename(backupPath, exePath)
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	os.Remove(backupPath)

	return fmt.Sprintf("Updated to %s (was %s)", release.TagName, Version), nil
}
