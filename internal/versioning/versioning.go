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

package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// WorkflowVersion represents a single version snapshot of a workflow.
type WorkflowVersion struct {
	ID           string    `json:"id"`
	WorkflowName string    `json:"workflow_name"`
	Content      string    `json:"content"`
	Author       string    `json:"author"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
	Tags         []string  `json:"tags"`
}

// VersionManager manages workflow versions on local filesystem.
type VersionManager struct {
	storageDir string
}

// NewVersionManager creates a new VersionManager with the given storage directory.
func NewVersionManager(storageDir string) *VersionManager {
	return &VersionManager{
		storageDir: storageDir,
	}
}

// validateWorkflowName prevents directory traversal.
func validateWorkflowName(name string) error {
	if name == "" {
		return fmt.Errorf("workflow name cannot be empty")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid workflow name: %s", name)
	}
	return nil
}

// workflowDir returns the directory path for a workflow.
func (vm *VersionManager) workflowDir(workflowName string) string {
	return filepath.Join(vm.storageDir, workflowName)
}

// generateVersionID generates a version ID from timestamp and content hash.
func generateVersionID(content string) string {
	timestamp := time.Now().UTC().Format("20060102150405")
	hash := sha256.Sum256([]byte(content))
	shortHash := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("%s-%s", timestamp, shortHash)
}

// SaveVersion saves a new version of a workflow.
func (vm *VersionManager) SaveVersion(workflowName, content, author, message string) (*WorkflowVersion, error) {
	if err := validateWorkflowName(workflowName); err != nil {
		return nil, err
	}

	version := &WorkflowVersion{
		WorkflowName: workflowName,
		Content:      content,
		Author:       author,
		Message:      message,
		CreatedAt:    time.Now().UTC(),
		Tags:         []string{},
	}

	dir := vm.workflowDir(workflowName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create workflow directory: %w", err)
	}

	// Ensure unique version ID to avoid overwriting existing files.
	for {
		version.ID = generateVersionID(content)
		yamlPath := filepath.Join(dir, version.ID+".yaml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Write YAML content.
	yamlPath := filepath.Join(dir, version.ID+".yaml")
	if err := os.WriteFile(yamlPath, []byte(content), 0600); err != nil {
		return nil, fmt.Errorf("failed to write version yaml: %w", err)
	}

	// Write metadata JSON.
	metaPath := filepath.Join(dir, version.ID+".json")
	metaData, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal version metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write version metadata: %w", err)
	}

	return version, nil
}

// ListVersions returns all versions for a workflow, sorted by creation time (oldest first).
func (vm *VersionManager) ListVersions(workflowName string) ([]*WorkflowVersion, error) {
	if err := validateWorkflowName(workflowName); err != nil {
		return nil, err
	}

	dir := vm.workflowDir(workflowName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*WorkflowVersion{}, nil
		}
		return nil, fmt.Errorf("failed to read workflow directory: %w", err)
	}

	var versions []*WorkflowVersion
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "tags.json" {
			continue
		}

		metaPath := filepath.Join(dir, name)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var version WorkflowVersion
		if err := json.Unmarshal(data, &version); err != nil {
			continue
		}
		versions = append(versions, &version)
	}

	// Sort by CreatedAt ascending.
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.Before(versions[j].CreatedAt)
	})

	return versions, nil
}

// GetVersion retrieves a specific version by ID.
func (vm *VersionManager) GetVersion(workflowName, versionID string) (*WorkflowVersion, error) {
	if err := validateWorkflowName(workflowName); err != nil {
		return nil, err
	}
	if versionID == "" {
		return nil, fmt.Errorf("version ID cannot be empty")
	}

	dir := vm.workflowDir(workflowName)
	metaPath := filepath.Join(dir, versionID+".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("version not found: %s", versionID)
		}
		return nil, fmt.Errorf("failed to read version metadata: %w", err)
	}

	var version WorkflowVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version metadata: %w", err)
	}

	// Ensure content is loaded from yaml file.
	yamlPath := filepath.Join(dir, versionID+".yaml")
	content, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read version content: %w", err)
	}
	version.Content = string(content)

	return &version, nil
}

// GetLatestVersion returns the most recent version of a workflow.
func (vm *VersionManager) GetLatestVersion(workflowName string) (*WorkflowVersion, error) {
	versions, err := vm.ListVersions(workflowName)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for workflow: %s", workflowName)
	}
	return vm.GetVersion(workflowName, versions[len(versions)-1].ID)
}

// CompareVersions returns a diff string between two versions.
func (vm *VersionManager) CompareVersions(workflowName, versionID1, versionID2 string) (string, error) {
	v1, err := vm.GetVersion(workflowName, versionID1)
	if err != nil {
		return "", err
	}
	v2, err := vm.GetVersion(workflowName, versionID2)
	if err != nil {
		return "", err
	}

	return computeDiff(v1.Content, v2.Content, versionID1, versionID2), nil
}

// computeDiff computes a simple line-based diff.
func computeDiff(content1, content2, label1, label2 string) string {
	lines1 := strings.Split(content1, "\n")
	lines2 := strings.Split(content2, "\n")

	// Build LCS table.
	m, n := len(lines1), len(lines2)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if lines1[i-1] == lines2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to generate diff.
	var diff []string
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && lines1[i-1] == lines2[j-1] {
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			diff = append(diff, fmt.Sprintf("+%s\t%s", label2, lines2[j-1]))
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			diff = append(diff, fmt.Sprintf("-%s\t%s", label1, lines1[i-1]))
			i--
		}
	}

	// Reverse diff.
	for k := 0; k < len(diff)/2; k++ {
		diff[k], diff[len(diff)-1-k] = diff[len(diff)-1-k], diff[k]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s\n", label1))
	sb.WriteString(fmt.Sprintf("+++ %s\n", label2))
	for _, line := range diff {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// Rollback creates a new version with the content of an old version.
func (vm *VersionManager) Rollback(workflowName, versionID string) (*WorkflowVersion, error) {
	oldVersion, err := vm.GetVersion(workflowName, versionID)
	if err != nil {
		return nil, err
	}

	newVersion, err := vm.SaveVersion(workflowName, oldVersion.Content, oldVersion.Author, fmt.Sprintf("Rollback to %s: %s", versionID, oldVersion.Message))
	if err != nil {
		return nil, err
	}
	return newVersion, nil
}

// loadTags loads the tags mapping for a workflow.
func (vm *VersionManager) loadTags(workflowName string) (map[string]string, error) {
	if err := validateWorkflowName(workflowName); err != nil {
		return nil, err
	}

	dir := vm.workflowDir(workflowName)
	tagsPath := filepath.Join(dir, "tags.json")
	data, err := os.ReadFile(tagsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read tags file: %w", err)
	}

	var tags map[string]string
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}
	return tags, nil
}

// saveTags saves the tags mapping for a workflow.
func (vm *VersionManager) saveTags(workflowName string, tags map[string]string) error {
	dir := vm.workflowDir(workflowName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	tagsPath := filepath.Join(dir, "tags.json")
	data, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}
	if err := os.WriteFile(tagsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tags file: %w", err)
	}
	return nil
}

// TagVersion tags a version with a label.
func (vm *VersionManager) TagVersion(workflowName, versionID, tag string) error {
	if err := validateWorkflowName(workflowName); err != nil {
		return err
	}
	if versionID == "" {
		return fmt.Errorf("version ID cannot be empty")
	}
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}

	// Verify version exists.
	if _, err := vm.GetVersion(workflowName, versionID); err != nil {
		return err
	}

	tags, err := vm.loadTags(workflowName)
	if err != nil {
		return err
	}

	tags[tag] = versionID
	return vm.saveTags(workflowName, tags)
}

// GetVersionByTag retrieves a version by its tag.
func (vm *VersionManager) GetVersionByTag(workflowName, tag string) (*WorkflowVersion, error) {
	if err := validateWorkflowName(workflowName); err != nil {
		return nil, err
	}
	if tag == "" {
		return nil, fmt.Errorf("tag cannot be empty")
	}

	tags, err := vm.loadTags(workflowName)
	if err != nil {
		return nil, err
	}

	versionID, ok := tags[tag]
	if !ok {
		return nil, fmt.Errorf("tag not found: %s", tag)
	}

	return vm.GetVersion(workflowName, versionID)
}

// DeleteVersion removes a version. At least one version must remain.
func (vm *VersionManager) DeleteVersion(workflowName, versionID string) error {
	if err := validateWorkflowName(workflowName); err != nil {
		return err
	}
	if versionID == "" {
		return fmt.Errorf("version ID cannot be empty")
	}

	versions, err := vm.ListVersions(workflowName)
	if err != nil {
		return err
	}
	if len(versions) <= 1 {
		return fmt.Errorf("cannot delete the last version of workflow: %s", workflowName)
	}

	// Check version exists.
	found := false
	for _, v := range versions {
		if v.ID == versionID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("version not found: %s", versionID)
	}

	dir := vm.workflowDir(workflowName)
	yamlPath := filepath.Join(dir, versionID+".yaml")
	metaPath := filepath.Join(dir, versionID+".json")

	if err := os.Remove(yamlPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove version yaml: %w", err)
	}
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove version metadata: %w", err)
	}

	// Remove tag references.
	tags, err := vm.loadTags(workflowName)
	if err != nil {
		return err
	}
	changed := false
	for t, vid := range tags {
		if vid == versionID {
			delete(tags, t)
			changed = true
		}
	}
	if changed {
		if err := vm.saveTags(workflowName, tags); err != nil {
			return err
		}
	}

	return nil
}

// ValidateVersionID checks if a version ID has the correct format.
func ValidateVersionID(versionID string) error {
	if versionID == "" {
		return fmt.Errorf("version ID cannot be empty")
	}
	parts := strings.Split(versionID, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid version ID format: expected timestamp-hash")
	}
	timestampStr := parts[0]
	if len(timestampStr) != 14 {
		return fmt.Errorf("invalid timestamp in version ID: expected 14 digits")
	}
	if _, err := strconv.ParseInt(timestampStr, 10, 64); err != nil {
		return fmt.Errorf("invalid timestamp in version ID: %w", err)
	}
	hash := parts[1]
	if len(hash) != 8 {
		return fmt.Errorf("invalid hash in version ID: expected 8 characters")
	}
	return nil
}
