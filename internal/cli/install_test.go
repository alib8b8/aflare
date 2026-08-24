// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌‌​‌‌‌​‌‌‌​​​​‌​‌​​‌​​‌‌​‌​​‌​​‌‌‌​​​‌‌​​‌​‌‌‌​​​​​​​​​​​​​​​​​​​‌​‌‌​‌‌‌‌​‌‌‌​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/i18n"
)

// isolateUserConfig points the user config dir (where the registry cache and
// installed nodes live) at a fresh temp dir so install/uninstall tests never
// touch the real user config and never reach the network: with no cached
// registry.json, InstallNode fails in LoadRegistry before any HTTP call.
func isolateUserConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

// installedNodesDir returns the aflare nodes dir inside an isolated config
// dir, creating it.
func installedNodesDir(t *testing.T, configDir string) string {
	t.Helper()
	nodesDir := filepath.Join(configDir, "aflare", "nodes")
	if err := os.MkdirAll(nodesDir, 0o750); err != nil {
		t.Fatalf("creating nodes dir: %v", err)
	}
	return nodesDir
}

func TestInstallArg_NoArgs(t *testing.T) {
	i18n.Init("en")
	var err error
	out := captureOutput(func() {
		err = HandleInstall(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare install") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

// TestInstallArg_UnsyncedRegistry covers the failure path of HandleInstall
// without touching the network: with an isolated (empty) config dir the
// registry cache is missing, so InstallNode fails in LoadRegistry — well
// before downloadNode's HTTP request.
func TestInstallArg_UnsyncedRegistry(t *testing.T) {
	i18n.Init("en")
	isolateUserConfig(t)

	var err error
	out := captureOutput(func() {
		err = HandleInstall([]string{"some-node"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unsynced registry, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Failed to install node 'some-node'") {
		t.Errorf("expected install failure output, got: %s", out)
	}
	if !strings.Contains(out, "registry sync") {
		t.Errorf("expected registry sync hint in failure output, got: %s", out)
	}
}

func TestInstallArg_UninstallNoArgs(t *testing.T) {
	i18n.Init("en")
	var err error
	out := captureOutput(func() {
		err = HandleUninstall(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare uninstall") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

// TestInstallArg_UninstallNotInstalled covers the local-only failure path:
// the node file is absent from the (isolated) nodes dir, so UninstallNode
// errors without any registry or network access.
func TestInstallArg_UninstallNotInstalled(t *testing.T) {
	i18n.Init("en")
	isolateUserConfig(t)

	var err error
	out := captureOutput(func() {
		err = HandleUninstall([]string{"ghost-node"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for not-installed node, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Failed to uninstall node 'ghost-node'") {
		t.Errorf("expected uninstall failure output, got: %s", out)
	}
}

// TestInstallArg_UninstallSuccess covers the happy path: a node yaml present
// in the isolated nodes dir is removed and the command reports success.
func TestInstallArg_UninstallSuccess(t *testing.T) {
	i18n.Init("en")
	configDir := isolateUserConfig(t)
	nodesDir := installedNodesDir(t, configDir)
	nodeFile := filepath.Join(nodesDir, "demo-node.yaml")
	if err := os.WriteFile(nodeFile, []byte("name: demo-node\n"), 0o600); err != nil {
		t.Fatalf("writing node file: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = HandleUninstall([]string{"demo-node"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for installed node, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Node 'demo-node' uninstalled successfully!") {
		t.Errorf("expected uninstall success output, got: %s", out)
	}
	if _, statErr := os.Stat(nodeFile); !os.IsNotExist(statErr) {
		t.Errorf("expected node file removed after uninstall, stat err = %v", statErr)
	}
}
