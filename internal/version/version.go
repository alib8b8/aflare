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

package version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Version   = "0.6.0"
	BuildDate = "2026-07-20"
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
		body, readErr := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if readErr != nil {
			bodyStr = fmt.Sprintf("(failed to read response body: %v)", readErr)
		}
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, bodyStr)
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

func findChecksumsURL(release *GitHubRelease) string {
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "checksums") || strings.Contains(asset.Name, "sha256") {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func verifyChecksum(filePath, expectedChecksum string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for checksum: %w", err)
	}
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])
	if actual != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actual)
	}
	return nil
}

func downloadChecksums(url string) (map[string]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			checksums[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
		}
	}
	return checksums, nil
}

func SelfUpdate(repo string) (string, error) {
	release, err := CheckLatestRelease(repo)
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}

	if !HasUpdate(Version, release) {
		return "Already up to date (" + Version + ")", nil
	}

	downloadURL, assetName := FindAsset(release, runtime.GOOS, runtime.GOARCH)
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

	if checksumsURL := findChecksumsURL(release); checksumsURL != "" {
		checksums, err := downloadChecksums(checksumsURL)
		if err == nil && len(checksums) > 0 {
			if expected, ok := checksums[assetName]; ok {
				if err := verifyChecksum(tmpPath, expected); err != nil {
					os.Remove(tmpPath)
					return "", fmt.Errorf("checksum verification failed: %w", err)
				}
			}
		}
	}

	backupPath := exePath + ".bak"
	os.Rename(exePath, backupPath)

	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Rename(backupPath, exePath)
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	os.Remove(backupPath)

	return fmt.Sprintf("Updated to %s (was %s)", release.TagName, Version), nil
}

// Small change for PR
