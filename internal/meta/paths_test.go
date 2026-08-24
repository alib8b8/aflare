// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​‌​‌‌​‌​​​​‌‌​​​‌‌‌​‌‌‌‌‌‌​‌‌​​​‌‌‌​‌‌‌​​​‌‌​​​​​​​​​​​​​​​​​​‌‌‌‌‌‌‌​‌‌‌​​​⁠
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

package meta

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetPathsForTesting() {
	once = sync.Once{}
	homeDir = ""
	dataDir = ""
}

func TestHomeDir(t *testing.T) {
	resetPathsForTesting()
	tmpHome := t.TempDir()
	t.Setenv(EnvHome, tmpHome)

	if got := HomeDir(); got != tmpHome {
		t.Errorf("HomeDir() = %q, want %q", got, tmpHome)
	}
}

func TestDataDir(t *testing.T) {
	resetPathsForTesting()
	tmpData := t.TempDir()
	t.Setenv(EnvData, tmpData)

	if got := DataDir(); got != tmpData {
		t.Errorf("DataDir() = %q, want %q", got, tmpData)
	}
}

func TestSubDirs(t *testing.T) {
	resetPathsForTesting()
	tmpHome := t.TempDir()
	tmpData := t.TempDir()
	t.Setenv(EnvHome, tmpHome)
	t.Setenv(EnvData, tmpData)

	homeBased := []struct {
		name string
		got  string
		want string
	}{
		{"TemplatesDir", TemplatesDir(), filepath.Join(tmpHome, TemplatesDirName)},
		{"SkillsDir", SkillsDir(), filepath.Join(tmpHome, SkillsDirName)},
		{"BinDir", BinDir(), filepath.Join(tmpHome, BinDirName)},
	}

	for _, tt := range homeBased {
		if tt.got != tt.want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	dataBased := []struct {
		name string
		got  string
		want string
	}{
		{"ConfigDir", ConfigDir(), filepath.Join(tmpData, ConfigDirName)},
		{"LogsDir", LogsDir(), filepath.Join(tmpData, LogsDirName)},
		{"CacheDir", CacheDir(), filepath.Join(tmpData, CacheDirName)},
		{"WorkspacesDir", WorkspacesDir(), filepath.Join(tmpData, WorkspacesDirName)},
	}

	for _, tt := range dataBased {
		if tt.got != tt.want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestFileHelpers(t *testing.T) {
	resetPathsForTesting()
	tmpHome := t.TempDir()
	tmpData := t.TempDir()
	t.Setenv(EnvHome, tmpHome)
	t.Setenv(EnvData, tmpData)

	if got := ConfigFile("config.yaml"); got != filepath.Join(tmpData, ConfigDirName, "config.yaml") {
		t.Errorf("ConfigFile() = %q, want %q", got, filepath.Join(tmpData, ConfigDirName, "config.yaml"))
	}

	if got := LogFile("app.log"); got != filepath.Join(tmpData, LogsDirName, "app.log") {
		t.Errorf("LogFile() = %q, want %q", got, filepath.Join(tmpData, LogsDirName, "app.log"))
	}

	if got := CacheFile("data.cache"); got != filepath.Join(tmpData, CacheDirName, "data.cache") {
		t.Errorf("CacheFile() = %q, want %q", got, filepath.Join(tmpData, CacheDirName, "data.cache"))
	}
}

func TestEnsureDirs(t *testing.T) {
	resetPathsForTesting()
	tmpHome := t.TempDir()
	tmpData := t.TempDir()
	t.Setenv(EnvHome, tmpHome)
	t.Setenv(EnvData, tmpData)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() returned error: %v", err)
	}

	dirs := []string{
		HomeDir(),
		TemplatesDir(),
		SkillsDir(),
		BinDir(),
		DataDir(),
		ConfigDir(),
		LogsDir(),
		CacheDir(),
		WorkspacesDir(),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("dir %q does not exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}
