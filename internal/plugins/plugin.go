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

package plugins

import (
	"fmt"
	"sort"
	"sync"
)

const (
	PluginTypeNode      = "node"
	PluginTypeExtension = "extension"
)

type PluginInfo struct {
	Name         string
	Version      string
	Description  string
	Author       string
	Type         string
	Dependencies []string
	Enabled      bool
}

type Plugin interface {
	GetInfo() PluginInfo
	Init() error
	Shutdown() error
}

type NodePlugin interface {
	Plugin
	GetNodes() []interface{}
}

type pluginEntry struct {
	plugin  Plugin
	enabled bool
}

type PluginManager struct {
	plugins map[string]*pluginEntry
	mu      sync.RWMutex
}

// NewPluginManager 创建并返回一个空的插件管理器。
func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]*pluginEntry),
	}
}

// Register 注册插件，校验名称与类型，重复注册返回错误。
func (pm *PluginManager) Register(plugin Plugin) error {
	info := plugin.GetInfo()

	if info.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if info.Type != PluginTypeNode && info.Type != PluginTypeExtension {
		return fmt.Errorf("invalid plugin type: %s (must be 'node' or 'extension')", info.Type)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.plugins[info.Name]; exists {
		return fmt.Errorf("plugin '%s' already registered", info.Name)
	}

	pm.plugins[info.Name] = &pluginEntry{
		plugin:  plugin,
		enabled: false,
	}
	return nil
}

// Unregister 移除插件，若插件处于启用状态则先调用其 Shutdown。
func (pm *PluginManager) Unregister(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entry, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if entry.enabled {
		if err := entry.plugin.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown plugin '%s': %w", name, err)
		}
	}

	delete(pm.plugins, name)
	return nil
}

// Get 按名称获取插件，未找到时第二个返回值为 false。
func (pm *PluginManager) Get(name string) (Plugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	entry, ok := pm.plugins[name]
	if !ok {
		return nil, false
	}
	return entry.plugin, true
}

// List 返回所有插件信息，按名称排序。
func (pm *PluginManager) List() []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(pm.plugins))
	for _, entry := range pm.plugins {
		info := entry.plugin.GetInfo()
		info.Enabled = entry.enabled
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// ListByType 返回指定类型的插件信息，按名称排序。
func (pm *PluginManager) ListByType(pluginType string) []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var infos []PluginInfo
	for _, entry := range pm.plugins {
		info := entry.plugin.GetInfo()
		if info.Type == pluginType {
			info.Enabled = entry.enabled
			infos = append(infos, info)
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// Enable 启用插件：校验依赖、调用 Init，并将其标记为已启用。
func (pm *PluginManager) Enable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entry, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if entry.enabled {
		return nil
	}

	if err := pm.checkDependenciesLocked(name); err != nil {
		return fmt.Errorf("dependency check failed for plugin '%s': %w", name, err)
	}

	if err := entry.plugin.Init(); err != nil {
		return fmt.Errorf("failed to init plugin '%s': %w", name, err)
	}

	entry.enabled = true
	return nil
}

func (pm *PluginManager) checkDependenciesLocked(name string) error {
	entry, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	info := entry.plugin.GetInfo()
	for _, dep := range info.Dependencies {
		depEntry, depExists := pm.plugins[dep]
		if !depExists {
			return fmt.Errorf("missing dependency '%s'", dep)
		}
		if !depEntry.enabled {
			return fmt.Errorf("dependency '%s' is not enabled", dep)
		}
	}

	return nil
}

// Disable 禁用插件，调用其 Shutdown 并清除启用标记。
func (pm *PluginManager) Disable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entry, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if !entry.enabled {
		return nil
	}

	if err := entry.plugin.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown plugin '%s': %w", name, err)
	}

	entry.enabled = false
	return nil
}

// InitAll 按依赖数量升序对所有已启用插件调用 Init，遇到错误立即返回。
func (pm *PluginManager) InitAll() error {
	pm.mu.RLock()
	entries := make([]*pluginEntry, 0, len(pm.plugins))
	for _, entry := range pm.plugins {
		entries = append(entries, entry)
	}
	pm.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].plugin.GetInfo().Dependencies) < len(entries[j].plugin.GetInfo().Dependencies)
	})

	for _, entry := range entries {
		if !entry.enabled {
			continue
		}
		info := entry.plugin.GetInfo()
		if err := entry.plugin.Init(); err != nil {
			return fmt.Errorf("failed to init plugin '%s': %w", info.Name, err)
		}
	}

	return nil
}

// ShutdownAll 按依赖数量降序对所有已启用插件调用 Shutdown，仅返回首个错误。
func (pm *PluginManager) ShutdownAll() error {
	pm.mu.RLock()
	entries := make([]*pluginEntry, 0, len(pm.plugins))
	for _, entry := range pm.plugins {
		entries = append(entries, entry)
	}
	pm.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].plugin.GetInfo().Dependencies) > len(entries[j].plugin.GetInfo().Dependencies)
	})

	var firstErr error
	for _, entry := range entries {
		if !entry.enabled {
			continue
		}
		info := entry.plugin.GetInfo()
		if err := entry.plugin.Shutdown(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to shutdown plugin '%s': %w", info.Name, err)
			}
		}
	}

	return firstErr
}

// GetNodePlugins 返回所有已启用的 NodePlugin 实例，按名称排序。
func (pm *PluginManager) GetNodePlugins() []NodePlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var nodePlugins []NodePlugin
	for _, entry := range pm.plugins {
		if !entry.enabled {
			continue
		}
		info := entry.plugin.GetInfo()
		if info.Type == PluginTypeNode {
			if np, ok := entry.plugin.(NodePlugin); ok {
				nodePlugins = append(nodePlugins, np)
			}
		}
	}

	sort.Slice(nodePlugins, func(i, j int) bool {
		return nodePlugins[i].GetInfo().Name < nodePlugins[j].GetInfo().Name
	})

	return nodePlugins
}

type EchoPlugin struct {
	info PluginInfo
}

// NewEchoPlugin 创建一个简单的回显节点插件实例。
func NewEchoPlugin() *EchoPlugin {
	return &EchoPlugin{
		info: PluginInfo{
			Name:         "echo",
			Version:      "1.0.0",
			Description:  "A simple echo node plugin",
			Author:       "test",
			Type:         PluginTypeNode,
			Dependencies: []string{},
			Enabled:      false,
		},
	}
}

// GetInfo 返回 EchoPlugin 的元信息。
func (e *EchoPlugin) GetInfo() PluginInfo {
	return e.info
}

// Init 初始化 EchoPlugin，当前为空实现。
func (e *EchoPlugin) Init() error {
	return nil
}

// Shutdown 关闭 EchoPlugin，当前为空实现。
func (e *EchoPlugin) Shutdown() error {
	return nil
}

// GetNodes 返回 EchoPlugin 提供的节点列表。
func (e *EchoPlugin) GetNodes() []interface{} {
	return []interface{}{"echo_node"}
}

type ReversePlugin struct {
	info PluginInfo
}

// NewReversePlugin 创建一个字符串反转节点插件实例，依赖 echo 插件。
func NewReversePlugin() *ReversePlugin {
	return &ReversePlugin{
		info: PluginInfo{
			Name:         "reverse",
			Version:      "1.0.0",
			Description:  "A string reverse node plugin",
			Author:       "test",
			Type:         PluginTypeNode,
			Dependencies: []string{"echo"},
			Enabled:      false,
		},
	}
}

// GetInfo 返回 ReversePlugin 的元信息。
func (r *ReversePlugin) GetInfo() PluginInfo {
	return r.info
}

// Init 初始化 ReversePlugin，当前为空实现。
func (r *ReversePlugin) Init() error {
	return nil
}

// Shutdown 关闭 ReversePlugin，当前为空实现。
func (r *ReversePlugin) Shutdown() error {
	return nil
}

// GetNodes 返回 ReversePlugin 提供的节点列表。
func (r *ReversePlugin) GetNodes() []interface{} {
	return []interface{}{"reverse_node"}
}
