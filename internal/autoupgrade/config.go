package autoupgrade

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadConfig() (*UpgradeConfig, error) {
	paths := getConfigPaths()
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var config UpgradeConfig
			if err := yaml.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
			return &config, nil
		}
	}
	return getDefaultConfig(), nil
}

func getConfigPaths() []string {
	var paths []string

	if envPath := os.Getenv("LLM_BOX_AUTOUPGRADE_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "llm-box", "autoupgrade.yaml"))
	}

	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, "autoupgrade.yaml"))
	}

	return paths
}

func getDefaultConfig() *UpgradeConfig {
	return &UpgradeConfig{
		Mode:                ModeMonitor,
		AutoUpdateEnabled:   true,
		AutoMergeEnabled:    false,
		CheckInterval:       "24h",
		BackupBeforeUpgrade: true,
		RollbackOnFailure:   true,
		RepositoryURL:       "https://github.com/alib8b8/llm-box",
	}
}

func SaveConfig(config *UpgradeConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "llm-box")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path := filepath.Join(configDir, "autoupgrade.yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
