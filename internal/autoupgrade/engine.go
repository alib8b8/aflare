package autoupgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/version"
)

type UpgradeMode string

const (
	ModeAuto    UpgradeMode = "auto"
	ModeMonitor UpgradeMode = "monitor"
	ModeManual  UpgradeMode = "manual"
)

type UpgradeConfig struct {
	Mode                UpgradeMode `yaml:"mode,omitempty"`
	AutoUpdateEnabled   bool        `yaml:"auto_update_enabled,omitempty"`
	AutoMergeEnabled    bool        `yaml:"auto_merge_enabled,omitempty"`
	CheckInterval       string      `yaml:"check_interval,omitempty"`
	BackupBeforeUpgrade bool        `yaml:"backup_before_upgrade,omitempty"`
	RollbackOnFailure   bool        `yaml:"rollback_on_failure,omitempty"`
	RepositoryURL       string      `yaml:"repository_url,omitempty"`
}

type UpgradeState struct {
	LastCheck         time.Time `yaml:"last_check"`
	LastUpgrade       time.Time `yaml:"last_upgrade"`
	CurrentVersion    string    `yaml:"current_version"`
	LatestVersion     string    `yaml:"latest_version"`
	UpgradeInProgress bool      `yaml:"upgrade_in_progress"`
	UpgradeStatus     string    `yaml:"upgrade_status"`
}

type UpgradeEngine struct {
	config   *UpgradeConfig
	state    *UpgradeState
	stopChan chan struct{}
}

func NewUpgradeEngine(config *UpgradeConfig) *UpgradeEngine {
	return &UpgradeEngine{
		config:   config,
		state:    &UpgradeState{CurrentVersion: version.GetVersion()},
		stopChan: make(chan struct{}),
	}
}

func (e *UpgradeEngine) Start(ctx context.Context) error {
	if e.config.Mode == ModeManual {
		logger.Info("auto-upgrade engine disabled (manual mode)")
		return nil
	}

	interval, err := time.ParseDuration(e.config.CheckInterval)
	if err != nil {
		interval = 24 * time.Hour
	}

	logger.Info("auto-upgrade engine started", "mode", e.config.Mode, "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("auto-upgrade engine stopped")
			return nil
		case <-ticker.C:
			e.CheckAndUpgrade(ctx)
		}
	}
}

func (e *UpgradeEngine) Stop() {
	close(e.stopChan)
}

func (e *UpgradeEngine) CheckAndUpgrade(ctx context.Context) {
	if e.state.UpgradeInProgress {
		logger.Warn("upgrade already in progress, skipping check")
		return
	}

	logger.Info("checking for updates...")

	release, err := version.CheckLatestRelease("alib8b8/llm-box")
	if err != nil {
		logger.Error("failed to check for updates", "error", err)
		return
	}

	e.state.LastCheck = time.Now()
	e.state.LatestVersion = release.TagName

	if !version.HasUpdate(version.GetVersion(), release) {
		logger.Info("no update available", "current", version.GetVersion(), "latest", release.TagName)
		return
	}

	logger.Info("update available", "current", version.GetVersion(), "latest", release.TagName)

	if e.config.Mode == ModeMonitor {
		logger.Info("monitor mode: notification only, not upgrading")
		return
	}

	if !e.config.AutoUpdateEnabled {
		logger.Info("auto-update disabled, skipping upgrade")
		return
	}

	e.PerformUpgrade(ctx, release)
}

func (e *UpgradeEngine) PerformUpgrade(ctx context.Context, release *version.GitHubRelease) {
	e.state.UpgradeInProgress = true
	e.state.UpgradeStatus = "in_progress"
	logger.Info("starting upgrade", "version", release.TagName)

	var backupPath string
	if e.config.BackupBeforeUpgrade {
		exePath, err := os.Executable()
		if err == nil {
			backupPath = exePath + ".backup." + time.Now().Format("20060102-150405")
			if err := copyFile(exePath, backupPath); err != nil {
				logger.Warn("failed to create backup, proceeding without", "error", err)
				backupPath = ""
			} else {
				logger.Info("backup created", "path", backupPath)
			}
		}
	}

	result, err := version.SelfUpdate("alib8b8/llm-box")
	if err != nil {
		e.state.UpgradeStatus = "failed"
		e.state.UpgradeInProgress = false
		logger.Error("upgrade failed", "error", err)

		if e.config.RollbackOnFailure && backupPath != "" {
			if err := rollback(backupPath); err != nil {
				logger.Error("rollback failed", "error", err)
			} else {
				logger.Info("rollback successful")
			}
		}
		return
	}

	e.state.UpgradeStatus = "success"
	e.state.UpgradeInProgress = false
	e.state.LastUpgrade = time.Now()
	e.state.CurrentVersion = release.TagName

	logger.Info("upgrade successful", "result", result)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func rollback(backupPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Rename(backupPath, exePath); err != nil {
		return err
	}
	return nil
}

func (e *UpgradeEngine) GetState() *UpgradeState {
	return e.state
}

func (e *UpgradeEngine) SetConfig(config *UpgradeConfig) {
	e.config = config
}

func (e *UpgradeEngine) RunSelfUpdate() (string, error) {
	release, err := version.CheckLatestRelease("alib8b8/llm-box")
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}

	if !version.HasUpdate(version.GetVersion(), release) {
		return fmt.Sprintf("Already up to date (%s)", version.GetVersion()), nil
	}

	result, err := version.SelfUpdate("alib8b8/llm-box")
	if err != nil {
		return "", err
	}

	return result, nil
}

func (e *UpgradeEngine) RunAutoMerge() (string, error) {
	if !e.config.AutoMergeEnabled {
		return "", fmt.Errorf("auto-merge is not enabled")
	}

	logger.Info("running auto-merge workflow")

	branches, err := getLocalBranches()
	if err != nil {
		return "", fmt.Errorf("failed to get local branches: %w", err)
	}

	for _, branch := range branches {
		if branch == "main" || branch == "master" {
			continue
		}

		if err := attemptAutoMerge(branch); err != nil {
			logger.Warn("auto-merge failed for branch", "branch", branch, "error", err)
		} else {
			logger.Info("auto-merge successful", "branch", branch)
		}
	}

	return "Auto-merge completed", nil
}

func getRepoDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func getLocalBranches() ([]string, error) {
	repoDir, err := getRepoDir()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && isValidBranchName(line) {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func isValidBranchName(name string) bool {
	if len(name) > 255 || name == "" {
		return false
	}
	// Reject branch names starting with '-' to prevent git option injection
	if name[0] == '-' {
		return false
	}
	for _, ch := range name {
		if ch == '\000' || ch == '\n' || ch == '\r' {
			return false
		}
	}
	return true
}

func attemptAutoMerge(branch string) error {
	repoDir, err := getRepoDir()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	cmd = exec.Command("git", "pull", "--rebase")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull main: %w", err)
	}

	cmd = exec.Command("git", "checkout", "--", branch)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}

	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		cmd := exec.Command("git", "rebase", "--abort")
		cmd.Dir = repoDir
		cmd.Run()
		return fmt.Errorf("rebase failed: %w", err)
	}

	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout main after rebase: %w", err)
	}

	cmd = exec.Command("git", "merge", "--no-ff", "--", branch)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	cmd = exec.Command("git", "push")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}
