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
	"time"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/packs"
	"github.com/alib8b8/aflare/internal/skills"
)

// installedPackManifest records the result of a pack installation.
type installedPackManifest struct {
	Pack         string   `json:"pack"`
	Description  string   `json:"description"`
	InstalledAt  string   `json:"installed_at"`
	Templates    int      `json:"templates"`
	Capabilities []string `json:"capabilities"`
	Categories   []string `json:"categories"`
}

// installedPacksDir returns the directory where pack installation manifests
// are stored (~/.aflare/installed-packs/).
func installedPacksDir() string {
	return filepath.Join(meta.DataDir(), "installed-packs")
}

// isPackInstalled checks whether a pack manifest exists on disk.
func isPackInstalled(name string) bool {
	_, err := os.Stat(filepath.Join(installedPacksDir(), name+".json"))
	return err == nil
}

// loadInstalledPack reads a previously installed pack manifest.
func loadInstalledPack(name string) (*installedPackManifest, error) {
	data, err := os.ReadFile(filepath.Join(installedPacksDir(), name+".json"))
	if err != nil {
		return nil, err
	}
	var m installedPackManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// saveInstalledPack writes a pack installation manifest to disk.
func saveInstalledPack(m *installedPackManifest) error {
	dir := installedPacksDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create installed-packs directory: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, m.Pack+".json"), data, 0644)
}

// HandleInstallPack handles the "install-pack" command.
// Usage: aflare install-pack <pack-name>
// Usage: aflare install-pack --list
// Usage: aflare install-pack <pack-name> --force
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
	force := false
	for _, a := range args[1:] {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	pack := packs.GetPack(packName)
	if pack == nil {
		fmt.Printf("Error: pack %q not found.\n\n", packName)
		fmt.Println("Available packs:")
		listPacks()
		os.Exit(1)
	}

	// Idempotent: check if already installed.
	if isPackInstalled(packName) && !force {
		existing, err := loadInstalledPack(packName)
		if err == nil {
			fmt.Printf("Pack %q is already installed (since %s).\n", packName, existing.InstalledAt)
			fmt.Printf("  Templates:    %d\n", existing.Templates)
			fmt.Printf("  Capabilities: %s\n", strings.Join(existing.Capabilities, ", "))
			fmt.Println()
			fmt.Println("Use --force to reinstall:")
			fmt.Printf("  aflare install-pack %s --force\n", packName)
			return
		}
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
	if force {
		fmt.Printf("Reinstalling pack: %s\n", pack.Name)
	} else {
		fmt.Printf("Installing pack: %s\n", pack.Name)
	}
	fmt.Printf("  Description:  %s\n", pack.Description)
	fmt.Printf("  Templates:    %d\n", len(unique))
	fmt.Printf("  Capabilities: %s\n", strings.Join(pack.Capabilities, ", "))
	fmt.Println()

	// Save installation manifest.
	manifest := &installedPackManifest{
		Pack:         pack.Name,
		Description:  pack.Description,
		InstalledAt:  time.Now().Format(time.RFC3339),
		Templates:    len(unique),
		Capabilities: pack.Capabilities,
		Categories:   pack.Categories,
	}

	if err := saveInstalledPack(manifest); err != nil {
		fmt.Printf("Warning: failed to save installation manifest: %v\n", err)
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

// listPacks prints all available scenario packs and marks installed ones.
func listPacks() {
	allPacks := packs.AllPacks()
	fmt.Printf("Available scenario packs (%d):\n\n", len(allPacks))
	for _, p := range allPacks {
		marker := " "
		if isPackInstalled(p.Name) {
			marker = "*"
		}
		fmt.Printf(" %s %-16s %s", marker, p.Name, p.Description)
		if len(p.Capabilities) > 0 {
			fmt.Printf("  [caps: %s]", strings.Join(p.Capabilities, ", "))
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("  * = installed")
	fmt.Println()
	fmt.Println("Install a pack: aflare install-pack <name>")
}

// PrintInstallPackUsage prints usage for the install-pack command.
func PrintInstallPackUsage() {
	fmt.Println("Usage: aflare install-pack <pack-name>")
	fmt.Println("       aflare install-pack --list")
	fmt.Println("       aflare install-pack <pack-name> --force")
	fmt.Println()
	fmt.Println("Installs a scenario-based pack of templates with recommended capabilities.")
	fmt.Println("Each pack bundles all templates from one or more categories, plus")
	fmt.Println("a suggested capability configuration for the best experience.")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  <pack-name>    Install the named pack")
	fmt.Println("  --list, -l     List all available packs (* = installed)")
	fmt.Println("  --force, -f    Reinstall even if already installed")
	fmt.Println("  --help, -h     Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare install-pack devops")
	fmt.Println("  aflare install-pack security")
	fmt.Println("  aflare install-pack finance")
	fmt.Println("  aflare install-pack --list")
	fmt.Println("  aflare install-pack all")
	fmt.Println("  aflare install-pack devops --force")
	fmt.Println()
	listPacks()
}