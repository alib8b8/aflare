// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌‌​‌​​​​‌‌​​‌​‌‌​‌​‌​​​​​‌​​​‌​‌‌‌​​​‌​‌‌‌‌​‌​​​​​​​​​​​​​​​​‌​​‌​​‌​‌​‌​​​​‌⁠
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
	"sync"
	"testing"
)

func TestNewPluginManager(t *testing.T) {
	pm := NewPluginManager()
	if pm == nil {
		t.Fatal("NewPluginManager returned nil")
	}
	if len(pm.List()) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(pm.List()))
	}
}

func TestRegister(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	err := pm.Register(echo)
	if err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	plugins := pm.List()
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "echo" {
		t.Errorf("expected plugin name 'echo', got '%s'", plugins[0].Name)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	pm := NewPluginManager()
	echo1 := NewEchoPlugin()
	echo2 := NewEchoPlugin()

	err := pm.Register(echo1)
	if err != nil {
		t.Fatalf("failed to register first plugin: %v", err)
	}

	err = pm.Register(echo2)
	if err == nil {
		t.Error("expected error for duplicate registration, got nil")
	}
}

func TestRegisterEmptyName(t *testing.T) {
	pm := NewPluginManager()
	plugin := &EchoPlugin{
		info: PluginInfo{
			Name: "",
			Type: PluginTypeNode,
		},
	}

	err := pm.Register(plugin)
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestRegisterInvalidType(t *testing.T) {
	pm := NewPluginManager()
	plugin := &EchoPlugin{
		info: PluginInfo{
			Name: "test",
			Type: "invalid",
		},
	}

	err := pm.Register(plugin)
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestUnregister(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	err := pm.Unregister("echo")
	if err != nil {
		t.Fatalf("failed to unregister plugin: %v", err)
	}

	if len(pm.List()) != 0 {
		t.Errorf("expected 0 plugins after unregister, got %d", len(pm.List()))
	}
}

func TestUnregisterNotFound(t *testing.T) {
	pm := NewPluginManager()
	err := pm.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for unregistering nonexistent plugin, got nil")
	}
}

func TestUnregisterEnabledPlugin(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	pm.Enable("echo")

	err := pm.Unregister("echo")
	if err != nil {
		t.Fatalf("failed to unregister enabled plugin: %v", err)
	}

	if len(pm.List()) != 0 {
		t.Errorf("expected 0 plugins after unregister, got %d", len(pm.List()))
	}
}

func TestGet(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)

	plugin, ok := pm.Get("echo")
	if !ok {
		t.Fatal("expected to find plugin 'echo'")
	}
	if plugin.GetInfo().Name != "echo" {
		t.Errorf("expected plugin name 'echo', got '%s'", plugin.GetInfo().Name)
	}
}

func TestGetNotFound(t *testing.T) {
	pm := NewPluginManager()
	_, ok := pm.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent plugin")
	}
}

func TestList(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()

	pm.Register(echo)
	pm.Register(reverse)

	plugins := pm.List()
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name != "echo" {
		t.Errorf("expected first plugin 'echo', got '%s'", plugins[0].Name)
	}
	if plugins[1].Name != "reverse" {
		t.Errorf("expected second plugin 'reverse', got '%s'", plugins[1].Name)
	}
}

func TestListByType(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	ext := &testExtensionPlugin{
		info: PluginInfo{
			Name: "test-ext",
			Type: PluginTypeExtension,
		},
	}

	pm.Register(echo)
	pm.Register(ext)

	nodePlugins := pm.ListByType(PluginTypeNode)
	if len(nodePlugins) != 1 {
		t.Errorf("expected 1 node plugin, got %d", len(nodePlugins))
	}
	if nodePlugins[0].Name != "echo" {
		t.Errorf("expected node plugin 'echo', got '%s'", nodePlugins[0].Name)
	}

	extPlugins := pm.ListByType(PluginTypeExtension)
	if len(extPlugins) != 1 {
		t.Errorf("expected 1 extension plugin, got %d", len(extPlugins))
	}
	if extPlugins[0].Name != "test-ext" {
		t.Errorf("expected extension plugin 'test-ext', got '%s'", extPlugins[0].Name)
	}
}

func TestEnable(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	err := pm.Enable("echo")
	if err != nil {
		t.Fatalf("failed to enable plugin: %v", err)
	}

	plugins := pm.List()
	if !plugins[0].Enabled {
		t.Error("expected plugin to be enabled")
	}
}

func TestEnableAlreadyEnabled(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	pm.Enable("echo")
	err := pm.Enable("echo")
	if err != nil {
		t.Errorf("expected no error for enabling already enabled plugin, got: %v", err)
	}
}

func TestEnableNotFound(t *testing.T) {
	pm := NewPluginManager()
	err := pm.Enable("nonexistent")
	if err == nil {
		t.Error("expected error for enabling nonexistent plugin, got nil")
	}
}

func TestEnableWithMissingDependency(t *testing.T) {
	pm := NewPluginManager()
	reverse := NewReversePlugin()

	pm.Register(reverse)
	err := pm.Enable("reverse")
	if err == nil {
		t.Error("expected error for enabling plugin with missing dependency, got nil")
	}
}

func TestEnableWithDisabledDependency(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()

	pm.Register(echo)
	pm.Register(reverse)

	err := pm.Enable("reverse")
	if err == nil {
		t.Error("expected error for enabling plugin with disabled dependency, got nil")
	}
}

func TestEnableWithDependencies(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()

	pm.Register(echo)
	pm.Register(reverse)

	err := pm.Enable("echo")
	if err != nil {
		t.Fatalf("failed to enable echo: %v", err)
	}

	err = pm.Enable("reverse")
	if err != nil {
		t.Fatalf("failed to enable reverse: %v", err)
	}

	plugins := pm.List()
	for _, p := range plugins {
		if !p.Enabled {
			t.Errorf("expected plugin '%s' to be enabled", p.Name)
		}
	}
}

func TestDisable(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	pm.Enable("echo")
	err := pm.Disable("echo")
	if err != nil {
		t.Fatalf("failed to disable plugin: %v", err)
	}

	plugins := pm.List()
	if plugins[0].Enabled {
		t.Error("expected plugin to be disabled")
	}
}

func TestDisableAlreadyDisabled(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()

	pm.Register(echo)
	err := pm.Disable("echo")
	if err != nil {
		t.Errorf("expected no error for disabling already disabled plugin, got: %v", err)
	}
}

func TestDisableNotFound(t *testing.T) {
	pm := NewPluginManager()
	err := pm.Disable("nonexistent")
	if err == nil {
		t.Error("expected error for disabling nonexistent plugin, got nil")
	}
}

func TestInitAll(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()

	pm.Register(echo)
	pm.Register(reverse)
	pm.Enable("echo")
	pm.Enable("reverse")

	err := pm.InitAll()
	if err != nil {
		t.Fatalf("InitAll failed: %v", err)
	}
}

func TestShutdownAll(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()

	pm.Register(echo)
	pm.Register(reverse)
	pm.Enable("echo")
	pm.Enable("reverse")

	err := pm.ShutdownAll()
	if err != nil {
		t.Fatalf("ShutdownAll failed: %v", err)
	}
}

func TestGetNodePlugins(t *testing.T) {
	pm := NewPluginManager()
	echo := NewEchoPlugin()
	reverse := NewReversePlugin()
	ext := &testExtensionPlugin{
		info: PluginInfo{
			Name: "test-ext",
			Type: PluginTypeExtension,
		},
	}

	pm.Register(echo)
	pm.Register(reverse)
	pm.Register(ext)

	pm.Enable("echo")

	nodePlugins := pm.GetNodePlugins()
	if len(nodePlugins) != 1 {
		t.Errorf("expected 1 enabled node plugin, got %d", len(nodePlugins))
	}
	if nodePlugins[0].GetInfo().Name != "echo" {
		t.Errorf("expected node plugin 'echo', got '%s'", nodePlugins[0].GetInfo().Name)
	}

	pm.Enable("reverse")
	nodePlugins = pm.GetNodePlugins()
	if len(nodePlugins) != 2 {
		t.Errorf("expected 2 enabled node plugins, got %d", len(nodePlugins))
	}
}

func TestEchoPlugin(t *testing.T) {
	echo := NewEchoPlugin()
	info := echo.GetInfo()

	if info.Name != "echo" {
		t.Errorf("expected name 'echo', got '%s'", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", info.Version)
	}
	if info.Type != PluginTypeNode {
		t.Errorf("expected type '%s', got '%s'", PluginTypeNode, info.Type)
	}

	nodes := echo.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestReversePlugin(t *testing.T) {
	reverse := NewReversePlugin()
	info := reverse.GetInfo()

	if info.Name != "reverse" {
		t.Errorf("expected name 'reverse', got '%s'", info.Name)
	}
	if len(info.Dependencies) != 1 || info.Dependencies[0] != "echo" {
		t.Errorf("expected dependency on 'echo', got %v", info.Dependencies)
	}

	nodes := reverse.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestConcurrentAccess(t *testing.T) {
	pm := NewPluginManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", i)
			plugin := &testExtensionPlugin{
				info: PluginInfo{
					Name: name,
					Type: PluginTypeExtension,
				},
			}
			pm.Register(plugin)
			pm.Enable(name)
			pm.Get(name)
			pm.List()
			pm.ListByType(PluginTypeExtension)
			pm.Disable(name)
			pm.Unregister(name)
		}(i)
	}

	wg.Wait()

	if len(pm.List()) != 0 {
		t.Errorf("expected 0 plugins after concurrent test, got %d", len(pm.List()))
	}
}

type testExtensionPlugin struct {
	info PluginInfo
}

func (t *testExtensionPlugin) GetInfo() PluginInfo {
	return t.info
}

func (t *testExtensionPlugin) Init() error {
	return nil
}

func (t *testExtensionPlugin) Shutdown() error {
	return nil
}
