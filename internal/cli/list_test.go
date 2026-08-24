// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​‌‌‌‌‌​‌‌​​​‌‌​‌​​‌​‌​‌​​‌‌​​‌‌‌‌‌‌​​‌​‌‌‌‌‌‌​‌​​​​​​​​​​​​​​​​​‌​​​​​‌​‌‌​‌‌​‌​⁠
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

// TestListCmd_ShowsBuiltinNodes covers the happy path of HandleList with no
// community nodes installed: builtin nodes come from the local registry and
// ListInstalledNodes is a local glob, so no network is involved.
func TestListCmd_ShowsBuiltinNodes(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	out := captureOutput(func() {
		HandleList()
	})

	if !strings.Contains(out, "Available nodes:") {
		t.Errorf("expected node table header, got: %s", out)
	}
	if !strings.Contains(out, "file_write") {
		t.Errorf("expected builtin file_write node in table, got: %s", out)
	}
	if strings.Contains(out, "Installed community nodes:") {
		t.Errorf("expected no installed-nodes section for empty config dir, got: %s", out)
	}
}

// TestListCmd_ShowsInstalledNodes covers the installed-community-nodes
// section: a node yaml dropped into the isolated config dir is listed.
func TestListCmd_ShowsInstalledNodes(t *testing.T) {
	i18n.Init("en")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	nodesDir := filepath.Join(dir, "aflare", "nodes")
	if err := os.MkdirAll(nodesDir, 0o750); err != nil {
		t.Fatalf("creating nodes dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodesDir, "community-node.yaml"), []byte("name: community-node\n"), 0o600); err != nil {
		t.Fatalf("writing community node: %v", err)
	}

	out := captureOutput(func() {
		HandleList()
	})

	if !strings.Contains(out, "Available nodes:") {
		t.Errorf("expected node table header, got: %s", out)
	}
	if !strings.Contains(out, "Installed community nodes:") {
		t.Errorf("expected installed-nodes section, got: %s", out)
	}
	if !strings.Contains(out, "community-node") {
		t.Errorf("expected community-node in installed list, got: %s", out)
	}
}
