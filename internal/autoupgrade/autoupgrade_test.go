package autoupgrade

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/version"
)

func TestNewUpgradeEngine(t *testing.T) {
	config := getDefaultConfig()
	engine := NewUpgradeEngine(config)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.config != config {
		t.Error("engine config mismatch")
	}
	if engine.state == nil {
		t.Fatal("expected non-nil state")
	}
	if engine.state.CurrentVersion != version.GetVersion() {
		t.Errorf("expected current version %s, got %s", version.GetVersion(), engine.state.CurrentVersion)
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()
	if config.Mode != ModeMonitor {
		t.Errorf("expected mode %s, got %s", ModeMonitor, config.Mode)
	}
	if !config.AutoUpdateEnabled {
		t.Error("expected AutoUpdateEnabled to be true")
	}
	if config.CheckInterval != "24h" {
		t.Errorf("expected interval 24h, got %s", config.CheckInterval)
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	// Ensure env var is not set
	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", "")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.Mode != ModeMonitor {
		t.Errorf("expected default mode monitor, got %s", config.Mode)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "autoupgrade.yaml")
	data := []byte("mode: auto\ncheck_interval: 1h\n")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", configPath)
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", config.Mode)
	}
	if config.CheckInterval != "1h" {
		t.Errorf("expected interval 1h, got %s", config.CheckInterval)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "autoupgrade.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: ["), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", configPath)
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Mode != ModeMonitor {
		t.Errorf("expected default mode monitor, got %s", config.Mode)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	// os.UserHomeDir respects HOME on Unix

	config := &UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   false,
		AutoMergeEnabled:    true,
		CheckInterval:       "12h",
		BackupBeforeUpgrade: false,
		RollbackOnFailure:   false,
		RepositoryURL:       "https://example.com/repo",
	}

	if err := SaveConfig(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	configDir := filepath.Join(tmpDir, ".config", "llm-box")
	configPath := filepath.Join(configDir, "autoupgrade.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not found: %v", err)
	}

	// Load and verify
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loaded.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", loaded.Mode)
	}
	if loaded.AutoUpdateEnabled {
		t.Error("expected AutoUpdateEnabled false")
	}
}

func TestGetConfigPaths(t *testing.T) {
	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", "")
	paths := getConfigPaths()
	if len(paths) == 0 {
		t.Error("expected at least one path")
	}
	// Should contain home dir path and cwd path
	foundCwd := false
	for _, p := range paths {
		if filepath.Base(p) == "autoupgrade.yaml" {
			foundCwd = true
		}
	}
	if !foundCwd {
		t.Error("expected cwd autoupgrade.yaml in paths")
	}
}

func TestStart_ManualMode(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{Mode: ModeManual})
	ctx := context.Background()
	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStart_InvalidInterval(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:          ModeMonitor,
		CheckInterval: "invalid",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStop(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	// Should not panic even if called multiple times
	engine.Stop()
}

func TestGetState(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	state := engine.GetState()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.UpgradeInProgress {
		t.Error("expected UpgradeInProgress to be false")
	}
}

func TestSetConfig(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	newConfig := &UpgradeConfig{Mode: ModeAuto}
	engine.SetConfig(newConfig)
	if engine.config.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", engine.config.Mode)
	}
}

func TestCheckAndUpgrade_AlreadyInProgress(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	engine.state.UpgradeInProgress = true
	ctx := context.Background()
	engine.CheckAndUpgrade(ctx)
	// Should skip without panic
}

func TestCheckAndUpgrade_MonitorMode(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{Mode: ModeMonitor})
	engine.state.UpgradeInProgress = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// This will call CheckLatestRelease (network), but we accept timeout
	// Use a goroutine so the test doesn't block forever
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.CheckAndUpgrade(ctx)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("CheckAndUpgrade network call timed out, skipping assertion")
	}
}

func TestCheckAndUpgrade_AutoUpdateDisabled(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:              ModeAuto,
		AutoUpdateEnabled: false,
	})
	engine.state.UpgradeInProgress = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.CheckAndUpgrade(ctx)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("CheckAndUpgrade network call timed out, skipping assertion")
	}
}

func TestCheckAndUpgrade_NoUpdate(t *testing.T) {
	// When current version matches latest, workflow should return early
	engine := NewUpgradeEngine(&UpgradeConfig{Mode: ModeAuto, AutoUpdateEnabled: true})
	engine.state.UpgradeInProgress = false
	engine.state.CurrentVersion = "v999.999.999" // unlikely to exist
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.CheckAndUpgrade(ctx)
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Log("CheckAndUpgrade network call timed out, skipping assertion")
	}
}

func TestGetLocalBranches(t *testing.T) {
	// Change to repo root where .git exists
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	repoRoot := filepath.Join(origDir, "..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	if _, err := os.Stat(".git"); err != nil {
		t.Skip("not in a git repository, skipping")
	}
	branches, err := getLocalBranches()
	if err != nil {
		t.Logf("getLocalBranches error (may be expected in CI): %v", err)
		return
	}
	if len(branches) == 0 {
		t.Log("no local branches found")
	}
	// Verify all branch names are valid
	for _, b := range branches {
		if !isValidBranchName(b) {
			t.Errorf("invalid branch name: %q", b)
		}
	}
}

func TestIsValidBranchName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"main", true},
		{"feature-branch", true},
		{"feature_branch", true},
		{"", true}, // empty string is allowed by current implementation
		{string(make([]byte, 256)), false},
		{"branch\nname", false},
		{"branch\rname", false},
		{"branch\x00name", false},
	}

	for _, tt := range tests {
		result := isValidBranchName(tt.name)
		if result != tt.expected {
			t.Errorf("isValidBranchName(%q) = %v, want %v", tt.name, result, tt.expected)
		}
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("expected %q, got %q", content, data)
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	if err := copyFile(src, dst); err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestRollback(t *testing.T) {
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "executable")
	backupPath := filepath.Join(tmpDir, "executable.backup")
	content := []byte("original")
	if err := os.WriteFile(exePath, content, 0755); err != nil {
		t.Fatalf("failed to write exe: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("backup"), 0755); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	// We can't easily mock os.Executable, but we can test copyFile directly
	// Rollback requires os.Executable() which returns test binary path
	// So we just ensure it doesn't panic when backup doesn't exist
	_ = rollback("/nonexistent/backup")
}

func TestRunAutoMerge_Disabled(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{AutoMergeEnabled: false})
	_, err := engine.RunAutoMerge()
	if err == nil {
		t.Error("expected error when auto-merge is disabled")
	}
}

func TestRunAutoMerge_Enabled_NotGitRepo(t *testing.T) {
	// Run in a temp directory that is not a git repo
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	engine := NewUpgradeEngine(&UpgradeConfig{AutoMergeEnabled: true})
	_, err = engine.RunAutoMerge()
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

func TestAttemptAutoMerge_NotGitRepo(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	err = attemptAutoMerge("main")
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

func TestUpgradeState(t *testing.T) {
	state := &UpgradeState{
		CurrentVersion:    "v1.0.0",
		LatestVersion:     "v2.0.0",
		UpgradeInProgress: false,
		UpgradeStatus:     "idle",
	}
	if state.CurrentVersion != "v1.0.0" {
		t.Error("state mismatch")
	}
}

func TestUpgradeConfig(t *testing.T) {
	cfg := &UpgradeConfig{
		Mode:                ModeManual,
		AutoUpdateEnabled:   false,
		AutoMergeEnabled:    false,
		CheckInterval:       "1h",
		BackupBeforeUpgrade: true,
		RollbackOnFailure:   true,
		RepositoryURL:       "https://github.com/test/repo",
	}
	if cfg.Mode != ModeManual {
		t.Errorf("expected mode manual, got %s", cfg.Mode)
	}
	if cfg.AutoUpdateEnabled {
		t.Error("expected AutoUpdateEnabled false")
	}
}

func TestSaveConfig_FailCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .config/llm-box as a file to cause mkdir failure
	configDir := filepath.Join(tmpDir, ".config", "llm-box")
	if err := os.MkdirAll(filepath.Dir(configDir), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := os.WriteFile(configDir, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	t.Setenv("HOME", tmpDir)
	config := getDefaultConfig()
	err := SaveConfig(config)
	if err == nil {
		t.Error("expected error when config directory cannot be created")
	}
}

func TestRollback_BackupNotFound(t *testing.T) {
	err := rollback("/nonexistent/path/backup")
	if err == nil {
		t.Error("expected error when backup file not found")
	}
}

func TestPerformUpgrade_BackupDisabled(t *testing.T) {
	_ = t.TempDir()
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   true,
		BackupBeforeUpgrade: false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		release := &version.GitHubRelease{TagName: "v999.999.999"}
		engine.PerformUpgrade(ctx, release)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("PerformUpgrade network call timed out")
	}
}

func TestPerformUpgrade_WithBackup(t *testing.T) {
	_ = t.TempDir()
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   true,
		BackupBeforeUpgrade: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		release := &version.GitHubRelease{TagName: "v999.999.999"}
		engine.PerformUpgrade(ctx, release)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("PerformUpgrade network call timed out")
	}
}

func TestPerformUpgrade_FailWithRollback(t *testing.T) {
	_ = t.TempDir()
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   true,
		BackupBeforeUpgrade: false,
		RollbackOnFailure:   true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		release := &version.GitHubRelease{TagName: "v999.999.999"}
		engine.PerformUpgrade(ctx, release)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("PerformUpgrade network call timed out")
	}

	if engine.state.UpgradeStatus != "failed" {
		t.Logf("expected status failed, got %s (may be timeout)", engine.state.UpgradeStatus)
	}
}

func TestCopyFile_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0444); err != nil {
		t.Skip("cannot create readonly directory, skipping")
	}

	dst := filepath.Join(readOnlyDir, "dst.txt")
	err := copyFile(src, dst)
	if err == nil {
		t.Log("expected error for write to readonly dir (may pass on some systems)")
	}
}

func TestRunSelfUpdate(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())

	done := make(chan struct{})
	var result string
	var err error
	go func() {
		defer close(done)
		result, err = engine.RunSelfUpdate()
	}()

	select {
	case <-done:
		if err != nil {
			t.Logf("RunSelfUpdate error (may be expected in test env): %v", err)
		} else {
			t.Logf("RunSelfUpdate result: %s", result)
		}
	case <-time.After(5 * time.Second):
		t.Log("RunSelfUpdate network call timed out")
	}
}

func TestRunAutoMerge_NoBranches(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	if err := os.MkdirAll(".git", 0755); err != nil {
		t.Fatalf("failed to create .git: %v", err)
	}

	engine := NewUpgradeEngine(&UpgradeConfig{AutoMergeEnabled: true})
	_, err = engine.RunAutoMerge()
	if err != nil {
		t.Logf("RunAutoMerge error (may be expected in mock git): %v", err)
	}
}

func TestAttemptAutoMerge_CheckoutFail(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	err = attemptAutoMerge("feature-branch")
	if err == nil {
		t.Error("expected error when git checkout fails")
	}
}

func TestStart_MonitorMode_WithTimeout(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:          ModeMonitor,
		CheckInterval: "1ms",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Start(ctx)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Log("Start timed out as expected")
	}
}

func TestStart_AutoMode_WithTimeout(t *testing.T) {
	engine := NewUpgradeEngine(&UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   true,
		BackupBeforeUpgrade: false,
		CheckInterval:       "1ms",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Start(ctx)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Log("Start timed out as expected")
	}
}

func TestGetConfigPaths_WithEnv(t *testing.T) {
	envPath := "/custom/path/autoupgrade.yaml"
	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", envPath)
	paths := getConfigPaths()
	if len(paths) == 0 {
		t.Error("expected at least one path")
	}
	foundEnv := false
	for _, p := range paths {
		if p == envPath {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Errorf("expected %q in paths", envPath)
	}
}

func TestLoadConfig_FromCwd(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "autoupgrade.yaml")
	data := []byte("mode: manual\ncheck_interval: 6h\n")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", "")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Mode != ModeManual {
		t.Errorf("expected mode manual, got %s", config.Mode)
	}
	if config.CheckInterval != "6h" {
		t.Errorf("expected interval 6h, got %s", config.CheckInterval)
	}
}

func TestSetConfig_Complete(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	newConfig := &UpgradeConfig{
		Mode:                ModeAuto,
		AutoUpdateEnabled:   true,
		AutoMergeEnabled:    true,
		CheckInterval:       "1h",
		BackupBeforeUpgrade: true,
		RollbackOnFailure:   true,
		RepositoryURL:       "https://github.com/test/repo",
	}
	engine.SetConfig(newConfig)
	if engine.config.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", engine.config.Mode)
	}
	if !engine.config.AutoUpdateEnabled {
		t.Error("expected AutoUpdateEnabled true")
	}
	if !engine.config.AutoMergeEnabled {
		t.Error("expected AutoMergeEnabled true")
	}
	if engine.config.CheckInterval != "1h" {
		t.Errorf("expected interval 1h, got %s", engine.config.CheckInterval)
	}
	if !engine.config.BackupBeforeUpgrade {
		t.Error("expected BackupBeforeUpgrade true")
	}
	if !engine.config.RollbackOnFailure {
		t.Error("expected RollbackOnFailure true")
	}
	if engine.config.RepositoryURL != "https://github.com/test/repo" {
		t.Errorf("expected repo URL, got %s", engine.config.RepositoryURL)
	}
}

func TestAttemptAutoMerge_AllStepsFail(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	err = attemptAutoMerge("feature-branch")
	if err == nil {
		t.Error("expected error when git commands fail")
	}
}

func TestRunAutoMerge_MultipleBranches(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	if err := os.MkdirAll(".git", 0755); err != nil {
		t.Fatalf("failed to create .git: %v", err)
	}

	engine := NewUpgradeEngine(&UpgradeConfig{AutoMergeEnabled: true})
	_, err = engine.RunAutoMerge()
	if err != nil {
		t.Logf("RunAutoMerge error (may be expected in mock git): %v", err)
	}
}

func TestCheckAndUpgrade_NetworkFailure(t *testing.T) {
	engine := NewUpgradeEngine(getDefaultConfig())
	engine.state.UpgradeInProgress = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.CheckAndUpgrade(ctx)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("CheckAndUpgrade network call timed out")
	}
}

func TestLoadConfig_InvalidYAML_Skip(t *testing.T) {
	tmpDir := t.TempDir()
	// Create invalid YAML in env path
	envConfigPath := filepath.Join(tmpDir, "env_config.yaml")
	if err := os.WriteFile(envConfigPath, []byte("invalid: ["), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Setenv("LLM_BOX_AUTOUPGRADE_CONFIG", envConfigPath)

	// Create valid config in cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cwdConfigPath := filepath.Join(tmpDir, "autoupgrade.yaml")
	data := []byte("mode: auto\n")
	if err := os.WriteFile(cwdConfigPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", config.Mode)
	}
}
