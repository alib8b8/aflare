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

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/skills"
)

// HandleSkills handles the "skills" command.
func HandleSkills(args []string) {
	if len(args) == 0 {
		PrintSkillsUsage()
		return
	}

	templatesDir := meta.ResolveTemplatesPath()
	_ = skills.EnsureEmbeddedTemplates(templatesDir)
	registry := skills.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("❌ Failed to load skills: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "list", "ls":
		skillList := registry.List()
		fmt.Printf("📦 Total skills: %d\n\n", len(skillList))
		for _, s := range skillList {
			fmt.Printf("  %-45s v%-8s %s\n", s.ID, s.Version, s.Description)
		}
	case "scan", "index":
		count := registry.Count()
		fmt.Printf("🔍 Scanned %d skills\n", count)
		if err := registry.SaveRegistry(); err != nil {
			fmt.Printf("❌ Failed to save registry: %v\n", err)
			os.Exit(1)
		}
		generated := registry.GenerateMissingMetas()
		fmt.Printf("✅ Registry saved, %d missing skill.json generated\n", generated)
	case "generate", "gen":
		generated := registry.GenerateMissingMetas()
		fmt.Printf("✅ Generated %d skill.json files\n", generated)
	case "save", "export":
		if err := registry.SaveRegistry(); err != nil {
			fmt.Printf("❌ Failed to save registry: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Registry saved to %s/%s\n", templatesDir, skills.RegistryFileName)
	case "search":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills search <keyword>")
			os.Exit(1)
		}
		results := registry.Search(args[1])
		fmt.Printf("🔍 Found %d skills matching \"%s\":\n\n", len(results), args[1])
		for _, s := range results {
			fmt.Printf("  %-45s %s\n", s.ID, s.Description)
		}
	case "categories", "cats":
		cats := registry.Categories()
		fmt.Printf("📂 Categories (%d):\n\n", len(cats))
		for _, cat := range cats {
			catSkills := registry.ListByCategory(cat)
			fmt.Printf("  %-30s %d skills\n", cat, len(catSkills))
		}
	case "category", "cat":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills category <name>")
			os.Exit(1)
		}
		catSkills := registry.ListByCategory(args[1])
		fmt.Printf("📂 Category: %s (%d skills)\n\n", args[1], len(catSkills))
		for _, s := range catSkills {
			fmt.Printf("  %-45s %s\n", s.ID, s.Description)
		}
	case "show", "get":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills show <id>")
			os.Exit(1)
		}
		s, err := registry.Get(args[1])
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📦 Skill: %s\n", s.ID)
		fmt.Printf("   Name: %s\n", s.Name)
		fmt.Printf("   Version: %s\n", s.Version)
		fmt.Printf("   Author: %s\n", s.Author)
		fmt.Printf("   Category: %s\n", s.Category)
		fmt.Printf("   Description: %s\n", s.Description)
		fmt.Printf("   Tags: %v\n", s.Tags)
		fmt.Printf("   Keywords: %v\n", s.Keywords)
		if s.Path != "" {
			fmt.Printf("   Path: %s\n", s.Path)
		}
	case "-h", "--help", "help":
		PrintSkillsUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		PrintSkillsUsage()
		os.Exit(1)
	}
}

// PrintSkillsUsage prints usage information for the skills command.
func PrintSkillsUsage() {
	fmt.Println("Usage: aflare skills <command> [options]")
	fmt.Println("\nManage and discover aflare skills/templates.")
	fmt.Println("\nCommands:")
	fmt.Println("  list, ls               List all available skills")
	fmt.Println("  scan, index            Scan templates and save registry")
	fmt.Println("  generate, gen          Generate missing skill.json files")
	fmt.Println("  save, export           Export skills registry to JSON")
	fmt.Println("  search <keyword>       Search skills by keyword")
	fmt.Println("  categories, cats       List all skill categories")
	fmt.Println("  category <name>        List skills in a category")
	fmt.Println("  show <id>              Show skill details")
	fmt.Println("  -h, --help             Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare skills scan")
	fmt.Println("  aflare skills list")
	fmt.Println("  aflare skills search security")
	fmt.Println("  aflare skills category development")
}
