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

package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinRegistry(t *testing.T) {
	reg := NewRegistry()

	pkgs := reg.List()
	if len(pkgs) != 5 {
		t.Fatalf("expected 5 builtin packages, got %d", len(pkgs))
	}

	expected := []string{"arxiv-daily", "btc-monitor", "financial-aml", "github-alert", "habit-tracker"}
	for _, name := range expected {
		pkg, err := reg.Get(name)
		if err != nil || pkg == nil {
			t.Fatalf("package %q not found: %v", name, err)
		}
	}
}

func TestSearch(t *testing.T) {
	reg := NewRegistry()

	// Search by name
	results := reg.Search("btc")
	if len(results) != 1 || results[0].Name != "btc-monitor" {
		t.Fatalf("search 'btc' should return 1 result, got %d", len(results))
	}

	// Search by category
	results = reg.Search("finance")
	if len(results) != 2 {
		t.Fatalf("search 'finance' should return 2 results, got %d", len(results))
	}

	// Search by description (no builtin robot package remains after the
	// fictional unitree_robot node was removed; "robot" now matches none).
	results = reg.Search("robot")
	if len(results) != 0 {
		t.Fatalf("search 'robot' should return 0 results, got %d", len(results))
	}

	// No match
	results = reg.Search("nonexistent")
	if len(results) != 0 {
		t.Fatalf("search 'nonexistent' should return 0 results, got %d", len(results))
	}
}

func TestListByCategory(t *testing.T) {
	reg := NewRegistry()

	finance := reg.ListByCategory("finance")
	if len(finance) != 2 {
		t.Fatalf("expected 2 finance packages, got %d", len(finance))
	}

	robot := reg.ListByCategory("robot")
	if len(robot) != 0 {
		t.Fatalf("expected 0 robot packages (unitree-patrol removed), got %d", len(robot))
	}

	devops := reg.ListByCategory("devops")
	if len(devops) != 1 {
		t.Fatalf("expected 1 devops package, got %d", len(devops))
	}

	health := reg.ListByCategory("health")
	if len(health) != 1 {
		t.Fatalf("expected 1 health package, got %d", len(health))
	}
}

func TestInstallUninstall(t *testing.T) {
	reg := NewRegistry()

	tmpDir := t.TempDir()
	// Test using a temp dir for install
	path, err := reg.InstallTo("btc-monitor", tmpDir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "btc-monitor.yaml")
	if path != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, path)
	}

	// Verify file exists
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("installed workflow not found at %s", expectedPath)
	}

	// Cleanup
	os.Remove(expectedPath)
}

func TestInstallUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Install("unknown-package")
	if err == nil {
		t.Fatal("installing unknown package should fail")
	}
}

func TestGetUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("Get unknown should return error")
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	reg := NewRegistry()

	results := reg.Search("BTC")
	if len(results) != 1 || results[0].Name != "btc-monitor" {
		t.Fatalf("case-insensitive search 'BTC' should find btc-monitor, got %d", len(results))
	}

	results = reg.Search("ARXIV")
	if len(results) != 1 || results[0].Name != "arxiv-daily" {
		t.Fatalf("case-insensitive search 'ARXIV' should find arxiv-daily, got %d", len(results))
	}
}
