// Copyright (c) 2026 aflare Contributors
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

// This file is the version information portion of the meta package: it
// exposes the program version/build info and implements GitHub-based
// self-update (release querying, asset download, checksum verification).

package meta

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/httpclient"
	"github.com/alib8b8/aflare/internal/logger"
)

// Three shared clients for the self-update flow, built once via the
// httpclient factory so they share connection-pool tuning and SSRF
// dial-time validation with every other outbound client in aflare.
//
// We deliberately do NOT use nodes/core.ValidateURL here — that would
// create a circular dependency from the version package into nodes/core.
// httpclient is a leaf package (stdlib only), so it is safe to import.
//
// GitHub release assets are served only from the public hosts in
// allowedGitHubHosts; loopback/private ranges are never legitimate
// targets, so by default we use ValidatePublic. validateGitHubURL still
// enforces the host allow-list (a superset of SSRF: even a public IP
// we'd reject if it weren't on the list, because the release JSON could
// otherwise redirect us to an arbitrary public host).
//
// Escape hatch: when AFLARE_SELF_UPDATE_ALLOW_PRIVATE is set, the
// validator switches to ValidateAllowAll. This is for environments where
// split-horizon DNS / a corporate GitHub mirror / a zero-trust gateway
// resolves a whitelisted host (e.g. github.com) to a private IP. The host
// allow-list above still applies, so only api.github.com / github.com /
// objects.githubusercontent.com may be contacted — only the IP check is
// relaxed, not the destination.
var (
	githubAPIClient  = httpclient.NewClient(httpclient.Options{Timeout: 10 * time.Second, Validator: selfUpdateValidator()})
	githubFileClient = httpclient.NewClient(httpclient.Options{Timeout: 30 * time.Second, Validator: selfUpdateValidator()})
	githubDLClient   = httpclient.NewClient(httpclient.Options{Timeout: 5 * time.Minute, Validator: selfUpdateValidator()})
)

// selfUpdateValidator picks the dial-time IP validator for the self-update
// clients. ValidatePublic by default; ValidateAllowAll when the user opts in
// via AFLARE_SELF_UPDATE_ALLOW_PRIVATE (any non-empty value).
func selfUpdateValidator() httpclient.Validator {
	if os.Getenv("AFLARE_SELF_UPDATE_ALLOW_PRIVATE") != "" {
		return httpclient.ValidateAllowAll
	}
	return httpclient.ValidatePublic
}

var (
	Version   = "0.7.0"
	BuildDate = "2026-07-20"
)

const (
	// maxAPIResponseSize bounds the GitHub API JSON response read by
	// CheckLatestRelease. 1MB is far more than any release JSON.
	maxAPIResponseSize = 1 << 20 // 1MB
	// maxChecksumsSize bounds the checksums file downloaded by
	// downloadChecksums. Checksums files are tiny.
	maxChecksumsSize = 1 << 20 // 1MB
	// maxBinaryDownloadSize bounds the binary downloaded by SelfUpdate.
	// 200MB is a generous ceiling for the self-update payload.
	maxBinaryDownloadSize = 200 << 20 // 200MB
)

// allowedGitHubHosts are the only hosts permitted for self-update HTTP
// requests. GitHub release assets are only served from these hosts, so any
// other host in a release response indicates either a malformed release or an
// attempt to redirect the self-update flow to an attacker-controlled server.
var allowedGitHubHosts = map[string]bool{
	"api.github.com":                true,
	"github.com":                    true,
	"objects.githubusercontent.com": true,
}

// validateGitHubURL validates that rawURL is an https URL pointing at one of
// the allowed GitHub hosts and carries no userinfo. It is a lightweight,
// self-update-scoped SSRF defense; we deliberately do not import
// internal/nodes/core (which has a richer ValidateURL) to avoid creating a
// circular dependency from the version package.
func validateGitHubURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https URLs are allowed for self-update, got scheme %q", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("URLs with userinfo are not allowed for self-update")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	if !allowedGitHubHosts[host] {
		return fmt.Errorf("host %q is not allowed for self-update", host)
	}
	return nil
}

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

// GetVersion 返回当前版本号字符串。
func GetVersion() string {
	return Version
}

// GetBuildInfo 返回包含版本、构建时间与运行时环境的构建信息。
func GetBuildInfo() string {
	return fmt.Sprintf("aflare version %s\n  built: %s\n  go:    %s\n  os:    %s/%s",
		Version, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// CheckLatestRelease 查询指定仓库的最新 GitHub release 信息。
// repo 参数格式为 "owner/name"，若设置了 GITHUB_TOKEN 环境变量则会用于鉴权。
func CheckLatestRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	if err := validateGitHubURL(url); err != nil {
		return nil, fmt.Errorf("invalid release URL: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := githubAPIClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
		bodyStr := string(body)
		if readErr != nil {
			bodyStr = fmt.Sprintf("(failed to read response body: %v)", readErr)
		}
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, bodyStr)
	}

	var release GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAPIResponseSize)).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	return &release, nil
}

// HasUpdate 比较当前版本与最新 release 的 tag，不同则视为有更新。
func HasUpdate(currentVersion string, latest *GitHubRelease) bool {
	if latest == nil || latest.TagName == "" {
		return false
	}
	current := strings.TrimPrefix(currentVersion, "v")
	latestTag := strings.TrimPrefix(latest.TagName, "v")
	return current != latestTag
}

// FindAsset 在 release 资源中查找匹配 goos/goarch 的二进制下载地址。
// 返回值为下载 URL 与资源文件名；未找到时返回空字符串。
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
	data, err := os.ReadFile(filePath) // #nosec G304 -- internally generated version path
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
	if err := validateGitHubURL(url); err != nil {
		return nil, fmt.Errorf("invalid checksums URL: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := githubFileClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsSize))
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

// SelfUpdate 检查并下载最新版本替换当前可执行文件。
// 流程：查询最新 release、下载匹配资源、校验 checksum、备份并替换二进制。
// 返回更新结果描述；若已是最新则返回 "Already up to date"。
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
	if err := validateGitHubURL(downloadURL); err != nil {
		return "", fmt.Errorf("invalid download URL: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	tmpPath := exePath + ".tmp"

	ctx, cancel := context.WithTimeout(context.Background(), 5*60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := githubDLClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // #nosec G304 -- internally generated version path
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Read at most maxBinaryDownloadSize+1 bytes so we can detect payloads
	// that exceed the limit and reject them explicitly instead of silently
	// truncating (which would only surface later as a checksum mismatch).
	written, err := io.Copy(out, io.LimitReader(resp.Body, maxBinaryDownloadSize+1))
	if err != nil {
		if cerr := out.Close(); cerr != nil {
			logger.Error("temp file close failed", "err", cerr)
		}
		removeBestEffort(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if written > maxBinaryDownloadSize {
		if cerr := out.Close(); cerr != nil {
			logger.Error("temp file close failed", "err", cerr)
		}
		removeBestEffort(tmpPath)
		return "", fmt.Errorf("downloaded binary exceeds max size of %d bytes", maxBinaryDownloadSize)
	}
	if err := out.Close(); err != nil {
		removeBestEffort(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	if checksumsURL := findChecksumsURL(release); checksumsURL != "" {
		checksums, err := downloadChecksums(checksumsURL)
		if err != nil {
			removeBestEffort(tmpPath)
			return "", fmt.Errorf("failed to download checksums: %w (refusing to install without verification)", err)
		}
		if len(checksums) == 0 {
			removeBestEffort(tmpPath)
			return "", fmt.Errorf("checksums file is empty (refusing to install without verification)")
		}
		expected, ok := checksums[assetName]
		if !ok {
			removeBestEffort(tmpPath)
			return "", fmt.Errorf("no checksum found for asset %q (refusing to install without verification)", assetName)
		}
		if err := verifyChecksum(tmpPath, expected); err != nil {
			removeBestEffort(tmpPath)
			return "", fmt.Errorf("checksum verification failed: %w", err)
		}
	} else {
		removeBestEffort(tmpPath)
		return "", fmt.Errorf("release has no checksums file (refusing to install without verification)")
	}

	backupPath := exePath + ".bak"
	if err := os.Rename(exePath, backupPath); err != nil {
		return "", fmt.Errorf("failed to back up current binary: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		if rerr := os.Rename(backupPath, exePath); rerr != nil {
			logger.Error("failed to restore backup binary after update failure", "backup", backupPath, "err", rerr)
		}
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	removeBestEffort(backupPath)

	return fmt.Sprintf("Updated to %s (was %s)", release.TagName, Version), nil
}

// removeBestEffort removes a file, logging a warning if the removal fails for
// reasons other than the file not existing (expected in cleanup paths).
func removeBestEffort(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Warn("best-effort cleanup failed", "path", path, "err", err)
	}
}
