// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​‌​​‌‌​​​‌​​‌​​​‌‌​​​‌​‌‌‌​‌‌‌‌‌‌‌‌‌​‌​​‌‌​‌​‌​‌​​‌‌‌‌‌‌​​​​​‌​‌​​​​​​​​​​​​​​​​​‌‌​​​​​​​‌​‌‌​‌⁠
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

package agentx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/meta"
)

// envAgentStoreFile overrides the agents file location (same contract as
// AFLARE_CONNECTORS_FILE: enterprise deployments, tests).
const envAgentStoreFile = "AFLARE_AGENTS_FILE"

const agentStoreFileName = "agents.yaml"

// agentStoreFile is the on-disk representation of the CLI-managed agent
// registry (`aflare agent add`). It mirrors the `agents:` section of the
// main config: the same AgentDef schema, never any secret material.
type agentStoreFile struct {
	Version int                 `yaml:"version"`
	Agents  map[string]AgentDef `yaml:"agents"`
}

// DefaultAgentStorePath returns the agents file path: the
// AFLARE_AGENTS_FILE env var when set, otherwise
// <data-dir>/config/agents.yaml (~/.aflare/config/agents.yaml).
func DefaultAgentStorePath() string {
	if p := os.Getenv(envAgentStoreFile); p != "" {
		return p
	}
	return meta.ConfigFile(agentStoreFileName)
}

// LoadAgentStore reads the CLI-managed agent definitions from path. A
// missing file yields an empty map (`aflare agent add` is a first-run
// action); a malformed file is an error so silent data loss never
// happens — the same contract as the connector registry.
func LoadAgentStore(path string) (map[string]AgentDef, error) {
	agents := make(map[string]AgentDef)
	data, err := os.ReadFile(path) // #nosec G304 -- path is the store file chosen by the operator
	if err != nil {
		if os.IsNotExist(err) {
			return agents, nil
		}
		return nil, fmt.Errorf("failed to read agents file %s: %w", path, err)
	}
	var sf agentStoreFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("failed to parse agents file %s: %w", path, err)
	}
	for name, def := range sf.Agents {
		if name == "" {
			continue
		}
		def.Name = name
		agents[name] = def
	}
	return agents, nil
}

// SaveAgentStore persists agent definitions to path atomically (tmp file
// + rename, mode 0600, with the same symlink-planting defense as the
// connector registry). Entries are written sorted for stable diffs.
func SaveAgentStore(path string, agents map[string]AgentDef) error {
	sf := agentStoreFile{Version: 1, Agents: make(map[string]AgentDef, len(agents))}
	for name, def := range agents {
		def.Name = name
		sf.Agents[name] = def
	}
	data, err := yaml.Marshal(&sf)
	if err != nil {
		return fmt.Errorf("failed to marshal agents: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	// Clear a pre-existing tmp path first: in a writable shared directory
	// an attacker could plant agents.yaml.tmp as a symlink and have the
	// rename clobber its target. os.Remove never follows symlinks.
	if fi, err := os.Lstat(tmpPath); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("refusing to write temporary file %s: a directory already exists there", tmpPath)
		}
		if err := os.Remove(tmpPath); err != nil {
			return fmt.Errorf("failed to clear stale temporary file: %w", err)
		}
	}
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	return nil
}

// SortedAgentNames returns the store's keys sorted by name — for stable
// iteration in CLI output and tests.
func SortedAgentNames(agents map[string]AgentDef) []string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
