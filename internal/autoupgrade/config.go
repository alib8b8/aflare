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
			data, err := os.ReadFile(path) // #nosec G304 -- path from internal config search list
			if err != nil {
				continue
			}
			var config UpgradeConfig
			if err := yaml.Unmarshal(data, &config); err != nil {
				continue
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
		Channel:             ChannelStable,
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
