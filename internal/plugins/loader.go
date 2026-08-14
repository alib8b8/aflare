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

package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
)

// DefaultPluginDir returns the default directory for community .so plugins.
// Uses ~/.config/aflare/plugins/ on Linux/macOS.
func DefaultPluginDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".aflare", "plugins")
	}
	return filepath.Join(home, ".config", "aflare", "plugins")
}

// EnsurePluginDirSecure creates the plugin dir with 0700 if missing and
// best-effort tightens an existing dir that is more permissive. Loading .so
// files grants in-process code execution, so the directory must be writable
// only by the owner. Returns nil if the dir is already secure or was fixed.
func EnsurePluginDirSecure(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create plugin dir %s: %w", dir, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	// Best-effort: if the dir is more permissive than 0700, tighten it.
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("tighten plugin dir perms %s: %w", dir, err)
		}
	}
	return nil
}

// LoadPlugin opens a Go plugin .so file, extracts the exported "Plugin" symbol,
// registers it with the manager, and enables it. The .so must export a symbol
// named "Plugin" that implements the Plugin interface.
//
// Example .so build command:
//
//	go build -buildmode=plugin -o myplugin.so ./myplugin
func LoadPlugin(path string, pm *PluginManager) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Go plugins are not supported on Windows: the 'plugin' package is Linux/macOS only. Use aflare's built-in MCP server support for cross-platform extensibility")
	}

	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("lookup Plugin symbol in %s: %w (did you export 'var Plugin MyPlugin'?)", path, err)
	}

	plug, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("Plugin symbol in %s does not implement plugins.Plugin interface", path)
	}

	info := plug.GetInfo()
	if info.Name == "" {
		return fmt.Errorf("plugin in %s has empty name", path)
	}

	if err := pm.Register(plug); err != nil {
		return fmt.Errorf("register plugin %s: %w", path, err)
	}

	if err := pm.Enable(info.Name); err != nil {
		return fmt.Errorf("enable plugin %s: %w", path, err)
	}

	log.Printf("[plugins] loaded %q (v%s) from %s", info.Name, info.Version, path)
	return nil
}

// LoadDir scans a directory for *.so files and loads each as a plugin.
// Non-.so files and directories are silently skipped. Loading errors are
// logged but do not prevent other plugins from loading.
//
// Returns the number of successfully loaded plugins and the first error
// encountered (if any).
func LoadDir(dir string, pm *PluginManager) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // no plugins directory is not an error
		}
		return 0, fmt.Errorf("read plugin dir %s: %w", dir, err)
	}

	var loaded int
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".so" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := LoadPlugin(path, pm); err != nil {
			log.Printf("[plugins] warning: %v", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		loaded++
	}

	return loaded, firstErr
}
