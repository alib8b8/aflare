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

package paths

import (
	"os"
	"path/filepath"
	"sync"
)

const (
	EnvHome = "LLM_BOX_HOME"
	EnvData = "LLM_BOX_DATA"

	DefaultHomeDirName = "llm-box"
	DefaultDataDirName = ".llm-box"

	TemplatesDirName  = "templates"
	SkillsDirName     = "skills"
	BinDirName        = "bin"
	ConfigDirName     = "config"
	LogsDirName       = "logs"
	CacheDirName      = "cache"
	WorkspacesDirName = "workspaces"
)

var (
	once    sync.Once
	homeDir string
	dataDir string
)

func HomeDir() string {
	once.Do(initDirs)
	return homeDir
}

func DataDir() string {
	once.Do(initDirs)
	return dataDir
}

func initDirs() {
	if envHome := os.Getenv(EnvHome); envHome != "" {
		homeDir = envHome
	} else {
		if home, err := os.UserHomeDir(); err == nil {
			homeDir = filepath.Join(home, DefaultHomeDirName)
		} else {
			homeDir = DefaultHomeDirName
		}
	}

	if envData := os.Getenv(EnvData); envData != "" {
		dataDir = envData
	} else {
		if home, err := os.UserHomeDir(); err == nil {
			dataDir = filepath.Join(home, DefaultDataDirName)
		} else {
			dataDir = DefaultDataDirName
		}
	}
}

func TemplatesDir() string {
	return filepath.Join(HomeDir(), TemplatesDirName)
}

func SkillsDir() string {
	return filepath.Join(HomeDir(), SkillsDirName)
}

func BinDir() string {
	return filepath.Join(HomeDir(), BinDirName)
}

func ConfigDir() string {
	return filepath.Join(DataDir(), ConfigDirName)
}

func LogsDir() string {
	return filepath.Join(DataDir(), LogsDirName)
}

func CacheDir() string {
	return filepath.Join(DataDir(), CacheDirName)
}

func WorkspacesDir() string {
	return filepath.Join(DataDir(), WorkspacesDirName)
}

func ConfigFile(name string) string {
	return filepath.Join(ConfigDir(), name)
}

func LogFile(name string) string {
	return filepath.Join(LogsDir(), name)
}

func CacheFile(name string) string {
	return filepath.Join(CacheDir(), name)
}

func EnsureDirs() error {
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func ResolveTemplatesPath() string {
	if tplDir := TemplatesDir(); dirExists(tplDir) {
		return tplDir
	}
	if cwd, err := os.Getwd(); err == nil {
		localTpl := filepath.Join(cwd, TemplatesDirName)
		if dirExists(localTpl) {
			return localTpl
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, TemplatesDirName)
	}
	return TemplatesDirName
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
