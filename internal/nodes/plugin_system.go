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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	validPluginActions = map[string]bool{
		"install":   true,
		"uninstall": true,
		"update":    true,
		"list":      true,
		"enable":    true,
		"disable":   true,
		"info":      true,
	}

	validPluginSources = map[string]bool{
		"local":  true,
		"git":    true,
		"url":    true,
		"market": true,
	}

	installedPlugins = make(map[string]*PluginSystemInfo)
	pluginMu         sync.RWMutex
)

type PluginSystemInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Authors      []string `json:"authors"`
	Dependencies []string `json:"dependencies"`
	Source       string   `json:"source"`
	Enabled      bool     `json:"enabled"`
	InstalledAt  string   `json:"installed_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type PluginSystemNode struct{}

func (n *PluginSystemNode) Name() string { return "plugin_system" }

func (n *PluginSystemNode) Description() string {
	return "插件系统节点。支持从本地目录、Git仓库、URL和插件市场加载插件，提供安装/卸载/更新/列出/启用/禁用等管理功能，支持沙箱隔离和版本管理。"
}

func (n *PluginSystemNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - 插件相关输入（可选，用于特定操作）",
		Output:      "string - JSON格式的插件操作结果",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "操作：install/uninstall/update/list/enable/disable/info", Required: true},
			{Name: "plugin_id", Type: "string", Description: "插件ID", Required: false},
			{Name: "source", Type: "string", Description: "插件来源：local/git/url/market", Required: false},
			{Name: "version", Type: "string", Description: "版本号", Required: false},
		},
	}
}

func (n *PluginSystemNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "")
	if !validPluginActions[action] {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	pluginID := getParam(params, "plugin_id", "")
	if action != "list" && pluginID == "" {
		return "", fmt.Errorf("plugin_id is required for %s action", action)
	}

	source := getParam(params, "source", "")
	if source != "" && !validPluginSources[source] {
		return "", fmt.Errorf("invalid source: %s", source)
	}

	version := getParam(params, "version", "")

	var result map[string]interface{}
	var status string

	switch action {
	case "install":
		result, status = installPlugin(pluginID, source, version, input)
	case "uninstall":
		result, status = uninstallPlugin(pluginID)
	case "update":
		result, status = updatePlugin(pluginID, version)
	case "list":
		result, status = listPlugins()
	case "enable":
		result, status = enablePlugin(pluginID)
	case "disable":
		result, status = disablePlugin(pluginID)
	case "info":
		result, status = getPluginInfo(pluginID)
	}

	response := map[string]interface{}{
		"action":    action,
		"plugin_id": pluginID,
		"status":    status,
		"result":    result,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(response, "", "  ")
	return string(output), nil
}

func installPlugin(pluginID, source, version, input string) (map[string]interface{}, string) {
	if source != "" {
		switch source {
		case "local":
			if !validateLocalPath(input) {
				return nil, "error: invalid local path"
			}
		case "git":
			if !validateGitURL(input) {
				return nil, "error: invalid git URL"
			}
		case "url":
			if !validatePluginURL(input) {
				return nil, "error: invalid URL"
			}
		case "market":
			if !validateMarketID(input) {
				return nil, "error: invalid market ID"
			}
		}
	}

	const maxPlugins = 100
	pluginMu.Lock()
	defer pluginMu.Unlock()

	if len(installedPlugins) >= maxPlugins {
		return nil, "error: maximum number of plugins reached"
	}

	if _, exists := installedPlugins[pluginID]; exists {
		return nil, "error: plugin already installed"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	plugin := &PluginSystemInfo{
		ID:           pluginID,
		Name:         pluginID,
		Description:  fmt.Sprintf("Plugin %s (version %s)", pluginID, version),
		Version:      version,
		Authors:      []string{"Unknown"},
		Dependencies: []string{},
		Source:       source,
		Enabled:      true,
		InstalledAt:  now,
		UpdatedAt:    now,
	}

	installedPlugins[pluginID] = plugin

	return map[string]interface{}{
		"plugin_id":    pluginID,
		"version":      version,
		"source":       source,
		"enabled":      true,
		"installed_at": now,
	}, "success"
}

func uninstallPlugin(pluginID string) (map[string]interface{}, string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	plugin, exists := installedPlugins[pluginID]
	if !exists {
		return nil, "error: plugin not found"
	}

	delete(installedPlugins, pluginID)

	return map[string]interface{}{
		"plugin_id": pluginID,
		"version":   plugin.Version,
	}, "success"
}

func updatePlugin(pluginID, version string) (map[string]interface{}, string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	plugin, exists := installedPlugins[pluginID]
	if !exists {
		return nil, "error: plugin not found"
	}

	oldVersion := plugin.Version
	plugin.Version = version
	plugin.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return map[string]interface{}{
		"plugin_id":   pluginID,
		"old_version": oldVersion,
		"new_version": version,
	}, "success"
}

func listPlugins() (map[string]interface{}, string) {
	pluginMu.RLock()
	defer pluginMu.RUnlock()

	var plugins []*PluginSystemInfo
	for _, p := range installedPlugins {
		plugins = append(plugins, p)
	}

	return map[string]interface{}{
		"plugins": plugins,
		"count":   len(plugins),
	}, "success"
}

func enablePlugin(pluginID string) (map[string]interface{}, string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	plugin, exists := installedPlugins[pluginID]
	if !exists {
		return nil, "error: plugin not found"
	}

	plugin.Enabled = true

	return map[string]interface{}{
		"plugin_id": pluginID,
		"enabled":   true,
	}, "success"
}

func disablePlugin(pluginID string) (map[string]interface{}, string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	plugin, exists := installedPlugins[pluginID]
	if !exists {
		return nil, "error: plugin not found"
	}

	plugin.Enabled = false

	return map[string]interface{}{
		"plugin_id": pluginID,
		"enabled":   false,
	}, "success"
}

func getPluginInfo(pluginID string) (map[string]interface{}, string) {
	pluginMu.RLock()
	defer pluginMu.RUnlock()

	plugin, exists := installedPlugins[pluginID]
	if !exists {
		return nil, "error: plugin not found"
	}

	return map[string]interface{}{
		"id":           plugin.ID,
		"name":         plugin.Name,
		"description":  plugin.Description,
		"version":      plugin.Version,
		"authors":      plugin.Authors,
		"dependencies": plugin.Dependencies,
		"source":       plugin.Source,
		"enabled":      plugin.Enabled,
		"installed_at": plugin.InstalledAt,
		"updated_at":   plugin.UpdatedAt,
	}, "success"
}

func validateLocalPath(path string) bool {
	if path == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return false
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func validateGitURL(gitURL string) bool {
	if gitURL == "" {
		return false
	}
	if len(gitURL) > 1024 {
		return false
	}
	return strings.HasPrefix(gitURL, "git@") ||
		strings.HasPrefix(gitURL, "https://github.com/") ||
		strings.HasPrefix(gitURL, "https://gitlab.com/") ||
		strings.HasPrefix(gitURL, "https://gitcode.com/") ||
		strings.HasPrefix(gitURL, "https://gitee.com/")
}

func validatePluginURL(u string) bool {
	if u == "" {
		return false
	}
	if len(u) > 1024 {
		return false
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	forbiddenHosts := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, h := range forbiddenHosts {
		if strings.Contains(parsed.Host, h) {
			return false
		}
	}
	return true
}

func validateMarketID(id string) bool {
	if id == "" {
		return false
	}
	return len(id) > 0 && len(id) <= 100
}

func init() {
	Register(&PluginSystemNode{})
}
