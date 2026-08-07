# Plugin System

aflare supports a plugin system for extending functionality with community-contributed plugins.

## Overview

The plugin system allows you to:

- Add custom nodes to aflare
- Extend existing functionality
- Share plugins with the community
- Build ecosystem around aflare

## Plugin Types

| Type | Description | Example |
|------|-------------|---------|
| **Node Plugin** | Adds new nodes to the workflow engine | Custom LLM provider, database connector |
| **Extension Plugin** | Extends core functionality | Custom authentication, logging |

## Quick Start

### List Installed Plugins

```bash
aflare plugins list

# List by type
aflare plugins list --type node
aflare plugins list --type extension
```

### Enable a Plugin

```bash
aflare plugins enable my-plugin
```

### Disable a Plugin

```bash
aflare plugins disable my-plugin
```

### Check Plugin Status

```bash
aflare plugins info my-plugin
```

## Plugin Structure

### Directory Layout

```
~/.aflare/plugins/
├── my-plugin/
│   ├── plugin.yaml
│   ├── main.go
│   └── nodes/
│       └── custom_node.py
└── another-plugin/
    ├── plugin.yaml
    └── extension.js
```

### plugin.yaml

```yaml
name: "my-plugin"
version: "1.0.0"
description: "A custom plugin for aflare"
author: "John Doe"
type: "node"
dependencies:
  - "base-plugin"
```

## Developing Plugins

### Node Plugin Example (Go)

```go
package main

import (
    "github.com/alib8b8/aflare/internal/plugins"
)

type MyNodePlugin struct {
    info plugins.PluginInfo
}

func NewMyNodePlugin() *MyNodePlugin {
    return &MyNodePlugin{
        info: plugins.PluginInfo{
            Name:         "my-node-plugin",
            Version:      "1.0.0",
            Description:  "Custom node plugin",
            Author:       "John Doe",
            Type:         plugins.PluginTypeNode,
            Dependencies: []string{},
        },
    }
}

func (p *MyNodePlugin) GetInfo() plugins.PluginInfo {
    return p.info
}

func (p *MyNodePlugin) Init() error {
    // Initialize plugin
    return nil
}

func (p *MyNodePlugin) Shutdown() error {
    // Cleanup
    return nil
}

func (p *MyNodePlugin) GetNodes() []interface{} {
    return []interface{}{"my_custom_node"}
}
```

### Node Plugin Example (Python)

```python
#!/usr/bin/env python3
"""Custom node plugin in Python"""

import sys
import json

def execute(input_data, params):
    """Execute the custom node"""
    return f"Processed: {input_data}"

if __name__ == "__main__":
    payload = json.loads(sys.stdin.read())
    result = execute(payload["input"], payload["params"])
    print(json.dumps({"output": result}))
```

### Registering the Plugin

```go
pm := plugins.NewPluginManager()
pm.Register(NewMyNodePlugin())
pm.Enable("my-node-plugin")
```

## Plugin Lifecycle

1. **Register**: Plugin is added to the plugin manager
2. **Enable**: Plugin is initialized and its nodes/resources are made available
3. **Execute**: Plugin nodes are used in workflows
4. **Disable**: Plugin is shutdown and removed from available resources
5. **Unregister**: Plugin is completely removed  

## Dependency Management

Plugins can declare dependencies on other plugins:

```yaml
name: "advanced-plugin"
version: "1.0.0"
type: "node"
dependencies:
  - "base-plugin"
  - "utils-plugin"
```

Dependencies are automatically checked when enabling a plugin:
- Missing dependencies cause enablement to fail
- Disabled dependencies cause enablement to fail
- Dependencies are initialized before the dependent plugin

## Security Considerations

1. **Review Plugins**: Only install plugins from trusted sources
2. **Sandbox Execution**: External node plugins run in a sandboxed environment
3. **Sensitive Data**: Sensitive parameters (API keys, passwords) are filtered from external plugins
4. **Permission Control**: Plugins run with the same permissions as the aflare process

## Plugin Market

### Finding Plugins

- **GitHub**: Search for `aflare-plugin`
- **Official Registry**: (Coming soon)
- **Community**: Check Discord/Slack channels

### Publishing Plugins

1. Create a GitHub repository for your plugin
2. Add a `plugin.yaml` file
3. Implement the plugin interface
4. Add documentation
5. Tag releases with semantic versioning

## Built-in Plugins

### Echo Plugin

A simple plugin that echoes input:

```yaml
steps:
  - node: echo_node
    input: "Hello World"
```

### Reverse Plugin

Reverses string input (depends on Echo plugin):

```yaml
steps:
  - node: reverse_node
    input: "Hello World"
```

## Troubleshooting

### Plugin Not Found

1. Check plugin is registered: `aflare plugins list`
2. Verify plugin directory exists in `~/.aflare/plugins/`
3. Check `plugin.yaml` has correct format

### Dependency Error

1. Install required dependencies
2. Enable dependencies before enabling the plugin
3. Check dependency version compatibility

### Plugin Initialization Failed

1. Check plugin logs
2. Verify all required dependencies are available
3. Check for configuration errors

## Best Practices

1. **Keep It Simple**: Focus on one functionality per plugin
2. **Document Well**: Provide clear documentation for users
3. **Use Semantic Versioning**: Follow semantic versioning for releases
4. **Handle Errors Gracefully**: Provide meaningful error messages
5. **Test Thoroughly**: Test your plugin with different workflows
6. **Respect Security**: Don't expose sensitive data
