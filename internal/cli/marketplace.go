// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​​​​‌‌​‌‌‌​​‌​​‌​‌​‌‌‌‌‌‌‌​​‌‌​‌‌‌‌‌‌‌‌‌​‌‌‌​​​​​​​​​​​​​​​​​​‌​‌​‌‌‌​‌​​​​​⁠
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

package cli

import (
	"fmt"
	"os"

	"github.com/alib8b8/aflare/internal/agentplugins"
	"github.com/alib8b8/aflare/internal/marketplace"
	"github.com/alib8b8/aflare/internal/meta"
)

// HandleMarketplace handles the "marketplace" command.
func HandleMarketplace(args []string) {
	if len(args) == 0 {
		PrintMarketplaceUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "export":
		HandleMarketplaceExport(args[1:])
	case "import":
		HandleMarketplaceImport(args[1:])
	case "install":
		HandleMarketplaceInstall(args[1:])
	case "-h", "--help", "help":
		PrintMarketplaceUsage()
	default:
		fmt.Printf("Unknown marketplace subcommand: %s\n\n", subCmd)
		PrintMarketplaceUsage()
		os.Exit(1)
	}
}

// HandleMarketplaceExport handles the "marketplace export" subcommand.
func HandleMarketplaceExport(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare marketplace export <package-name> [--dir <output-dir>]")
		os.Exit(1)
	}

	packageName := args[0]
	outputDir := "."
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--dir", "-d":
			if i+1 < len(args) {
				outputDir = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: aflare marketplace export <package-name> [--dir <output-dir>]")
			fmt.Println("\nExport a workflow package as an Agent Plugins 1.0.0 compatible directory.")
			fmt.Println("\nOptions:")
			fmt.Println("  --dir, -d <dir>   Output directory (default: current directory)")
			return
		}
	}

	reg := marketplace.NewRegistry()
	pluginDir, err := reg.ExportPlugin(packageName, outputDir)
	if err != nil {
		fmt.Printf("❌ Export failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Exported %q to %s\n", packageName, pluginDir)
	fmt.Println()
	fmt.Println("   Agent Plugins 1.0.0 format:")
	fmt.Printf("   ├── plugin.json\n")
	fmt.Printf("   ├── skills/%s/SKILL.md\n", packageName)
	fmt.Printf("   └── mcp.json\n")
	fmt.Println()
	fmt.Println("   Compatible with: VS Code, Cursor, GitHub Copilot, ChatGPT, Codex, Kiro")
}

// HandleMarketplaceImport handles the "marketplace import" subcommand.
func HandleMarketplaceImport(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare marketplace import <plugin-dir>")
		os.Exit(1)
	}

	pluginDir := args[0]
	manifest, err := marketplace.ImportPlugin(pluginDir)
	if err != nil {
		fmt.Printf("❌ Import failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Imported Agent Plugin: %s\n", manifest.Name)
	if manifest.Version != "" {
		fmt.Printf("   Version:     %s\n", manifest.Version)
	}
	if manifest.Description != "" {
		fmt.Printf("   Description: %s\n", manifest.Description)
	}
	if manifest.Author != "" {
		fmt.Printf("   Author:      %s\n", manifest.Author)
	}
	if len(manifest.Keywords) > 0 {
		fmt.Printf("   Keywords:    %v\n", manifest.Keywords)
	}
}

// HandleMarketplaceInstall handles the "marketplace install" subcommand: it
// installs an Agent Plugins 1.0.0 directory into aflare — skills become
// runnable workflows, stdio MCP servers are registered into .mcp.json.
func HandleMarketplaceInstall(args []string) {
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: aflare marketplace install <plugin-dir>")
		fmt.Println("\nInstall an Agent Plugins 1.0.0 plugin into aflare.")
		fmt.Println("  - skills/*/SKILL.md become runnable skills under the 'plugin' category")
		fmt.Println("  - stdio servers from mcp.json are registered into .mcp.json")
		fmt.Println("  - nothing from the plugin is executed during installation")
		return
	}
	pluginDir := args[0]

	res, err := agentplugins.InstallPlugin(pluginDir, agentplugins.InstallOptions{
		SkillsBaseDir: meta.ResolveTemplatesPath(),
		MCPConfigPath: ".mcp.json",
	})
	if err != nil {
		fmt.Printf("❌ Install failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Installed Agent Plugin: %s\n", res.Manifest.Name)
	if res.Manifest.Version != "" {
		fmt.Printf("   Version: %s\n", res.Manifest.Version)
	}
	for _, id := range res.SkillsInstalled {
		fmt.Printf("   Skill:    %s (run: aflare run %s)\n", id, id)
	}
	for _, name := range res.MCPServers {
		fmt.Printf("   MCP:      %s (stdio)\n", name)
	}
	if len(res.SkillsInstalled) == 0 && len(res.MCPServers) == 0 {
		fmt.Println("   (plugin shipped no installable components)")
	}
}

// PrintMarketplaceUsage prints usage information for the marketplace command.
func PrintMarketplaceUsage() {
	fmt.Println("Usage: aflare marketplace <command> [options]")
	fmt.Println("\nManage workflow packages with Agent Plugins 1.0.0 compatibility.")
	fmt.Println("\nCommands:")
	fmt.Println("  export <name> [--dir <dir>]   Export a workflow as an Agent Plugin")
	fmt.Println("  import <plugin-dir>           Import an Agent Plugin and show metadata")
	fmt.Println("  install <plugin-dir>          Install an Agent Plugin (skills + MCP servers)")
	fmt.Println("  -h, --help                    Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare marketplace export btc-monitor")
	fmt.Println("  aflare marketplace export btc-monitor --dir ./plugins")
	fmt.Println("  aflare marketplace import ./my-plugin")
	fmt.Println("  aflare marketplace install ./my-plugin")
	fmt.Println()
	fmt.Println("Agent Plugins 1.0.0 is an open standard backed by OpenAI, Google,")
	fmt.Println("Amazon, Microsoft, Cursor, and Vercel. Export once, use everywhere.")
}
