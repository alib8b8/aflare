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

// HomeDir 返回程序主目录路径，受 LLM_BOX_HOME 环境变量影响。
func HomeDir() string {
	once.Do(initDirs)
	return homeDir
}

// DataDir 返回数据目录路径，受 LLM_BOX_DATA 环境变量影响。
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

// TemplatesDir 返回模板目录的完整路径。
func TemplatesDir() string {
	return filepath.Join(HomeDir(), TemplatesDirName)
}

// SkillsDir 返回技能目录的完整路径。
func SkillsDir() string {
	return filepath.Join(HomeDir(), SkillsDirName)
}

// BinDir 返回二进制目录的完整路径。
func BinDir() string {
	return filepath.Join(HomeDir(), BinDirName)
}

// ConfigDir 返回配置目录的完整路径。
func ConfigDir() string {
	return filepath.Join(DataDir(), ConfigDirName)
}

// LogsDir 返回日志目录的完整路径。
func LogsDir() string {
	return filepath.Join(DataDir(), LogsDirName)
}

// CacheDir 返回缓存目录的完整路径。
func CacheDir() string {
	return filepath.Join(DataDir(), CacheDirName)
}

// WorkspacesDir 返回工作区目录的完整路径。
func WorkspacesDir() string {
	return filepath.Join(DataDir(), WorkspacesDirName)
}

// ConfigFile 返回配置目录下指定文件名的完整路径。
func ConfigFile(name string) string {
	return filepath.Join(ConfigDir(), name)
}

// LogFile 返回日志目录下指定文件名的完整路径。
func LogFile(name string) string {
	return filepath.Join(LogsDir(), name)
}

// CacheFile 返回缓存目录下指定文件名的完整路径。
func CacheFile(name string) string {
	return filepath.Join(CacheDir(), name)
}

// EnsureDirs 创建所有运行所需目录，任一创建失败即返回错误。
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

// ResolveTemplatesPath 解析模板目录路径：优先使用主目录下模板，其次当前工作目录。
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
