// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌‌​‌​‌​‌‌‌​‌​​​‌​​‌‌​​‌​‌​‌‌‌‌‌‌​‌‌​‌​‌‌‌‌‌​‌‌‌​​​​​​​​​​​​​​​​​​​‌​‌‌‌​‌‌‌‌​‌‌​‌⁠
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

package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/meta"
)

// envRegistryFile overrides the connectors file location (useful for
// enterprise deployments pointing at mounted config and for tests).
const envRegistryFile = "AFLARE_CONNECTORS_FILE"

const registryFileName = "connectors.yaml"

// registryFile is the on-disk representation.
type registryFile struct {
	Version    int    `yaml:"version"`
	Connectors []Spec `yaml:"connectors"`
}

// Registry is the thread-safe set of registered connector specs, persisted
// as YAML. The file contains endpoint data and credential references but
// never credential values, so leaking the file does not leak credentials.
type Registry struct {
	mu    sync.RWMutex
	path  string
	specs map[string]Spec
}

// DefaultRegistryPath returns the connectors file path: the
// AFLARE_CONNECTORS_FILE env var when set, otherwise
// <data-dir>/config/connectors.yaml (~/.aflare/config/connectors.yaml).
func DefaultRegistryPath() string {
	if p := os.Getenv(envRegistryFile); p != "" {
		return p
	}
	return meta.ConfigFile(registryFileName)
}

// LoadRegistry loads the registry from path. A missing file yields an
// empty registry (registering connectors is a first-run action); a
// malformed file is an error so silent data loss never happens.
func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{
		path:  path,
		specs: make(map[string]Spec),
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the registry file chosen by the operator
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("failed to read connectors file %s: %w", path, err)
	}
	var rf registryFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("failed to parse connectors file %s: %w", path, err)
	}
	for _, spec := range rf.Connectors {
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("connectors file %s: %w", path, err)
		}
		r.specs[spec.Name] = spec
	}
	return r, nil
}

// LoadDefaultRegistry loads the registry from DefaultRegistryPath.
func LoadDefaultRegistry() (*Registry, error) {
	return LoadRegistry(DefaultRegistryPath())
}

// Save persists the registry atomically (tmp file + rename, mode 0600).
func (r *Registry) Save() error {
	r.mu.RLock()
	specs := make([]Spec, 0, len(r.specs))
	for _, spec := range r.specs {
		specs = append(specs, spec)
	}
	path := r.path
	r.mu.RUnlock()

	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	rf := registryFile{Version: 1, Connectors: specs}
	data, err := yaml.Marshal(&rf)
	if err != nil {
		return fmt.Errorf("failed to marshal connectors: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	// Clear a pre-existing tmp path first: in a writable shared directory
	// an attacker could plant connectors.yaml.tmp as a symlink and have the
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

// Get returns a copy of the spec registered under name.
func (r *Registry) Get(name string) (Spec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.specs[name]
	return spec, ok
}

// List returns all specs sorted by name.
func (r *Registry) List() []Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]Spec, 0, len(r.specs))
	for _, spec := range r.specs {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// Upsert validates the spec and registers it, replacing any existing spec
// with the same name.
func (r *Registry) Upsert(spec Spec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs[spec.Name] = spec
	return nil
}

// Remove deletes the spec registered under name.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.specs[name]; !ok {
		return fmt.Errorf("connector %q does not exist", name)
	}
	delete(r.specs, name)
	return nil
}

// Path returns the persistence path of this registry.
func (r *Registry) Path() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.path
}
