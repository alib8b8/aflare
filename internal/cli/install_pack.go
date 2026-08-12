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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/packs"
	"github.com/alib8b8/aflare/internal/skills"
)

// HandleInstallPack handles the "install-pack" command.
// Usage: aflare install-pack <pack-name>
// Usage: aflare install-pack --list
func HandleInstallPack(args []string) {
	if len(args) == 0 {
		PrintInstallPackUsage()
		os.Exit(1)
	}

	// Handle --list flag
	if args[0] == "--list" || args[0] == "-l" {
		listPacks()
		return
	}

	if args[0] == "--help" || args[0] == "-h" {
		PrintInstallPackUsage()
		return
	}

	packName := args[0]
	pack := packs.GetPack(packName)
	if pack == nil {
		fmt.Printf("Error: pack %q not found.\n\n", packName)
		fmt.Println("Available packs:")
		listPacks()
		os.Exit(1)
	}

	// Load the skills registry.
	templatesDir := meta.ResolveTemplatesPath()
	registry := skills.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("Error: failed to load skills registry: %v\n", err)
		os.Exit(1)
	}

	// Collect matching skills.
	var matchingSkills []*skills.SkillMeta
	if packName == "all" || len(pack.Categories) == 0 {
		// "all" pack includes everything.
		matchingSkills = registry.List()
	} else {
		for _, cat := range pack.Categories {
			catSkills := registry.ListByCategory(cat)
			matchingSkills = append(matchingSkills, catSkills...)
		}
	}

	if len(matchingSkills) == 0 {
		fmt.Printf("No templates found for pack %q. Try running 'aflare skills scan' first.\n", packName)
		os.Exit(1)
	}

	// Deduplicate by ID.
	seen := make(map[string]bool)
	var unique []*skills.SkillMeta
	for _, s := range matchingSkills {
		if !seen[s.ID] {
			seen[s.ID] = true
			unique = append(unique, s)
		}
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].ID < unique[j].ID
	})

	// Display summary.
	fmt.Printf("Installing pack: %s\n", pack.Name)
	fmt.Printf("  Description:  %s\n", pack.Description)
	fmt.Printf("  Templates:    %d\n", len(unique))
	fmt.Printf("  Capabilities: %s\n", strings.Join(pack.Capabilities, ", "))
	fmt.Println()

	// Generate recommended config.
	configDir := meta.DataDir()
	configPath := filepath.Join(configDir, "pack-config.json")
	config := map[string]interface{}{
		"pack":         pack.Name,
		"installed_at": os.Getenv("TIMESTAMP"), // filled by the runner
		"templates":    len(unique),
		"capabilities": pack.Capabilities,
		"categories":   pack.Categories,
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Warning: failed to generate config: %v\n", err)
	} else {
		_ = os.MkdirAll(configDir, 0755)
		_ = os.WriteFile(configPath, configData, 0644)
	}

	// Print template list.
	fmt.Println("Included templates:")
	for _, s := range unique {
		fmt.Printf("  %-50s %s\n", s.ID, s.Description)
	}
	fmt.Println()

	// Print recommended next steps.
	fmt.Println("Recommended configuration:")
	fmt.Printf("  aflare agent -c %s\n", strings.Join(pack.Capabilities, ","))
	fmt.Println()
	fmt.Println("Quick start:")
	fmt.Printf("  # Run a template from this pack:\n")
	if len(unique) > 0 {
		fmt.Printf("  aflare run %s/workflow.yaml\n", unique[0].ID)
	}
	fmt.Printf("  # Start agent with pack capabilities:\n")
	fmt.Printf("  aflare agent -c %s\n", strings.Join(pack.Capabilities, ","))
	fmt.Println()
	fmt.Printf("Pack %q installed successfully. %d templates ready to use.\n", pack.Name, len(unique))
}

// listPacks prints all available scenario packs.
func listPacks() {
	allPacks := packs.AllPacks()
	fmt.Printf("Available scenario packs (%d):\n\n", len(allPacks))
	for _, p := range allPacks {
		fmt.Printf("  %-16s %s", p.Name, p.Description)
		if len(p.Capabilities) > 0 {
			fmt.Printf("  [caps: %s]", strings.Join(p.Capabilities, ", "))
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Install a pack: aflare install-pack <name>")
}

// PrintInstallPackUsage prints usage for the install-pack command.
func PrintInstallPackUsage() {
	fmt.Println("Usage: aflare install-pack <pack-name>")
	fmt.Println("       aflare install-pack --list")
	fmt.Println()
	fmt.Println("Installs a scenario-based pack of templates with recommended capabilities.")
	fmt.Println("Each pack bundles all templates from one or more categories, plus")
	fmt.Println("a suggested capability configuration for the best experience.")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  <pack-name>    Install the named pack")
	fmt.Println("  --list, -l     List all available packs")
	fmt.Println("  --help, -h     Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare install-pack devops")
	fmt.Println("  aflare install-pack security")
	fmt.Println("  aflare install-pack finance")
	fmt.Println("  aflare install-pack --list")
	fmt.Println("  aflare install-pack all")
	fmt.Println()
	listPacks()
}