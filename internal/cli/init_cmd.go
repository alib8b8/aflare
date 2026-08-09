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

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/aflare/internal/autoupgrade"
)

// HandleInit handles the "init" command.
func HandleInit(args []string) {
	mcpTarget := ""
	agentTarget := ""
	channel := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mcp":
			if i+1 < len(args) {
				mcpTarget = args[i+1]
				i++
			} else {
				mcpTarget = "all"
			}
		case "--agent":
			if i+1 < len(args) {
				agentTarget = args[i+1]
				i++
			} else {
				agentTarget = "all"
			}
		case "--channel":
			if i+1 < len(args) {
				channel = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintInitUsage()
			return
		default:
			if strings.HasPrefix(args[i], "--mcp=") {
				mcpTarget = strings.TrimPrefix(args[i], "--mcp=")
			} else if strings.HasPrefix(args[i], "--agent=") {
				agentTarget = strings.TrimPrefix(args[i], "--agent=")
			} else if strings.HasPrefix(args[i], "--channel=") {
				channel = strings.TrimPrefix(args[i], "--channel=")
			}
		}
	}

	if mcpTarget == "" && agentTarget == "" && channel == "" {
		PrintInitUsage()
		os.Exit(1)
	}

	if mcpTarget != "" {
		result, err := SetupMCP(mcpTarget)
		if err != nil {
			fmt.Printf("❌ MCP setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ MCP configured for %s\n", result.Agent)
		fmt.Printf("   Config: %s\n", result.ConfigPath)
		fmt.Printf("   Command: %s\n", result.Command)
	}

	if agentTarget != "" {
		result, err := InstallSkills(agentTarget)
		if err != nil {
			fmt.Printf("❌ Skill installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Skills installed for %s\n", result.Agent)
		fmt.Printf("   Path: %s\n", result.SkillPath)
		if result.Installed {
			fmt.Println("   Status: skills copied successfully")
		} else {
			fmt.Println("   Status: skills directory ready (no source templates found)")
		}
	}

	if channel != "" {
		config, err := autoupgrade.LoadConfig()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SetChannel(config, channel); err != nil {
			fmt.Printf("❌ Failed to set channel: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Update channel set to: %s\n", channel)
	}
}

// PrintInitUsage prints usage information for the init command.
func PrintInitUsage() {
	fmt.Println("Usage: aflare init [options]")
	fmt.Println("\nInitialize aflare integration with AI agents and configure settings.")
	fmt.Println("\nOptions:")
	fmt.Println("  --mcp <agent>       Setup MCP server configuration (claude-code, opencode, all)")
	fmt.Println("  --agent <agent>     Install aflare skills to agent (claude-code, opencode, all)")
	fmt.Println("  --channel <channel> Set update channel (stable, beta, nightly)")
	fmt.Println("  -h, --help          Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare init --mcp all")
	fmt.Println("  aflare init --mcp claude-code --agent all")
	fmt.Println("  aflare init --channel beta")
}

// HandleChannelQuick handles the --channel flag (pre-command flag, not "init" subcommand).
func HandleChannelQuick(updateChannel string) {
	config, err := autoupgrade.LoadConfig()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if err := autoupgrade.SetChannel(config, updateChannel); err != nil {
		fmt.Printf("❌ Failed to set channel: %v\n", err)
		os.Exit(1)
	}
	if err := autoupgrade.SaveConfig(config); err != nil {
		fmt.Printf("❌ Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Update channel set to: %s\n", updateChannel)
}

// HandleInitMCPQuick handles the --mcp flag (pre-command flag).
func HandleInitMCPQuick(initMCP string) {
	result, err := SetupMCP(initMCP)
	if err != nil {
		fmt.Printf("❌ MCP setup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ MCP configured for %s\n", result.Agent)
	fmt.Printf("   Config: %s\n", result.ConfigPath)
	fmt.Printf("   Command: %s\n", result.Command)
}

// HandleInitAgentQuick handles the --agent flag (pre-command flag).
func HandleInitAgentQuick(initAgent string) {
	result, err := InstallSkills(initAgent)
	if err != nil {
		fmt.Printf("❌ Skill installation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Skills installed for %s\n", result.Agent)
	fmt.Printf("   Path: %s\n", result.SkillPath)
	if result.Installed {
		fmt.Println("   Status: skills copied successfully")
	} else {
		fmt.Println("   Status: skills directory ready (no source templates found)")
	}
}