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

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]*pluginEntry),
	}
}

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

func (pm *PluginManager) Get(name string) (Plugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	entry, ok := pm.plugins[name]
	if !ok {
		return nil, false
	}
	return entry.plugin, true
}

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

func (e *EchoPlugin) GetInfo() PluginInfo {
	return e.info
}

func (e *EchoPlugin) Init() error {
	return nil
}

func (e *EchoPlugin) Shutdown() error {
	return nil
}

func (e *EchoPlugin) GetNodes() []interface{} {
	return []interface{}{"echo_node"}
}

type ReversePlugin struct {
	info PluginInfo
}

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

func (r *ReversePlugin) GetInfo() PluginInfo {
	return r.info
}

func (r *ReversePlugin) Init() error {
	return nil
}

func (r *ReversePlugin) Shutdown() error {
	return nil
}

func (r *ReversePlugin) GetNodes() []interface{} {
	return []interface{}{"reverse_node"}
}
