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
	"strings"

	"github.com/alib8b8/aflare/internal/badge"
	"github.com/alib8b8/aflare/internal/meta"
	skillsPkg "github.com/alib8b8/aflare/internal/skills"
)

// HandleTemplateSubmit handles the "template" command.
// Usage: aflare template submit <workflow.yaml> [--category <cat>] [--author <name>]
func HandleTemplateSubmit(args []string) {
	if len(args) == 0 {
		PrintTemplateSubmitUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "submit":
		handleTemplateSubmit(args[1:])
	case "--help", "-h", "help":
		PrintTemplateSubmitUsage()
	default:
		fmt.Printf("Unknown template subcommand: %s\n\n", subCmd)
		PrintTemplateSubmitUsage()
		os.Exit(1)
	}
}

// handleTemplateSubmit handles the "template submit" subcommand.
// It validates a community-contributed template and prepares it for PR submission.
func handleTemplateSubmit(args []string) {
	var yamlPath, category, author string

	parseTemplateSubmitArgs(args, &yamlPath, &category, &author)

	if yamlPath == "" {
		fmt.Println("Error: workflow YAML file path is required")
		PrintTemplateSubmitUsage()
		os.Exit(1)
	}

	// Validate the file exists and is a YAML file.
	absPath, err := validateTemplateFile(yamlPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	ext := strings.ToLower(filepath.Ext(absPath))

	// Read the workflow file.
	data, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Printf("Error: failed to read file: %v\n", err)
		os.Exit(1)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		fmt.Println("Error: workflow file is empty")
		os.Exit(1)
	}

	// Extract workflow name from the YAML.
	baseName := strings.TrimSuffix(filepath.Base(absPath), ext)
	templateName := extractYAMLField(content, "name")
	if templateName == "" {
		templateName = baseName
	}

	// Determine category.
	if category == "" {
		category = guessCategoryFromName(baseName)
	}

	// Validate category is known.
	validCategories := validSkillCategories()
	categoryValid := false
	for _, c := range validCategories {
		if c == category {
			categoryValid = true
			break
		}
	}
	if !categoryValid {
		fmt.Printf("Warning: category %q is not in the standard list.\n", category)
		fmt.Printf("Standard categories: %s\n", strings.Join(validCategories, ", "))
		fmt.Println("Proceeding anyway — you can update this later.")
	}

	// Generate skill ID: category/template-name
	skillID := fmt.Sprintf("%s/%s", category, baseName)

	// Build the skill metadata.
	desc := extractYAMLField(content, "description")
	if desc == "" {
		desc = fmt.Sprintf("%s workflow template", templateName)
	}
	if author == "" {
		author = "community contributor"
	}

	skillMeta := skillsPkg.SkillMeta{
		ID:          skillID,
		Name:        baseName,
		Version:     "1.0.0",
		Description: desc,
		Author:      author,
		Category:    category,
		Tags:        []string{category, "workflow"},
		Keywords:    []string{baseName, category},
	}

	// Compute the target directory in the templates tree.
	templatesDir := meta.ResolveTemplatesPath()
	targetDir := filepath.Join(templatesDir, category, baseName)

	// Create the target directory.
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Printf("Error: failed to create template directory: %v\n", err)
		os.Exit(1)
	}

	// Copy the workflow YAML.
	workflowDest := filepath.Join(targetDir, "workflow.yaml")
	if err := os.WriteFile(workflowDest, data, 0644); err != nil {
		fmt.Printf("Error: failed to write workflow file: %v\n", err)
		os.Exit(1)
	}

	// Write skill.json metadata.
	metaData, err := json.MarshalIndent(skillMeta, "", "  ")
	if err != nil {
		fmt.Printf("Error: failed to marshal metadata: %v\n", err)
		os.Exit(1)
	}
	metaPath := filepath.Join(targetDir, skillsPkg.SkillMetaFile)
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		fmt.Printf("Error: failed to write metadata: %v\n", err)
		os.Exit(1)
	}

	// Write a README stub.
	readmeContent := fmt.Sprintf("# %s\n\n%s\n\n## Usage\n\n```bash\naflare run %s\n```\n",
		templateName, desc, filepath.Join(category, baseName, "workflow.yaml"))
	readmePath := filepath.Join(targetDir, "README.md")
	_ = os.WriteFile(readmePath, []byte(readmeContent), 0644)

	// Rebuild the registry.
	registry := skillsPkg.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("Warning: failed to reload registry: %v\n", err)
	}
	if err := registry.SaveRegistry(); err != nil {
		fmt.Printf("Warning: failed to save registry: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Template prepared for submission!")
	fmt.Println()
	fmt.Printf("  Template:  %s\n", skillID)
	fmt.Printf("  Category:  %s\n", category)
	fmt.Printf("  Author:    %s\n", author)
	fmt.Printf("  Location:  %s\n", targetDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the generated files:")
	fmt.Printf("     %s/workflow.yaml\n", targetDir)
	fmt.Printf("     %s/skill.json\n", targetDir)
	fmt.Println()
	fmt.Println("  2. Run the template to verify:")
	fmt.Printf("     aflare run %s/%s/workflow.yaml\n", category, baseName)
	fmt.Println()
	fmt.Println("  3. Submit a Pull Request:")
	fmt.Println("     git add templates/")
	fmt.Println("     git commit -m \"feat: add template " + skillID + "\"")
	fmt.Println("     git push origin your-branch")
	fmt.Println()
	fmt.Println("  4. Open a PR at https://github.com/alib8b8/aflare")
	fmt.Println("     Use the \"New Template\" PR template.")
	fmt.Println()
	fmt.Println("Thank you for contributing to the aflare community!")

	// Award a virtual badge for the template contribution.
	awardBadgeForTemplate(author, skillID)
}

// validateTemplateFile validates that the given path is an existing YAML file.
// Returns the absolute path on success, or an error describing the problem.
func validateTemplateFile(yamlPath string) (string, error) {
	absPath, err := filepath.Abs(yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %v", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", absPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, expected a .yaml workflow file", absPath)
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".yaml" && ext != ".yml" {
		return "", fmt.Errorf("%s is not a YAML file (got %s)", absPath, ext)
	}

	return absPath, nil
}

// parseTemplateSubmitArgs parses the command-line arguments for template submit.
func parseTemplateSubmitArgs(args []string, yamlPath, category, author *string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--category", "-c":
			if i+1 < len(args) {
				*category = args[i+1]
				i++
			}
		case "--author", "-a":
			if i+1 < len(args) {
				*author = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintTemplateSubmitUsage()
			return
		default:
			if !strings.HasPrefix(args[i], "-") && *yamlPath == "" {
				*yamlPath = args[i]
			} else {
				fmt.Printf("Unknown argument: %s\n", args[i])
				PrintTemplateSubmitUsage()
				os.Exit(1)
			}
		}
	}
}

// PrintTemplateSubmitUsage prints usage for the template submit command.
func PrintTemplateSubmitUsage() {
	fmt.Println("Usage: aflare template submit <workflow.yaml> [options]")
	fmt.Println()
	fmt.Println("Validates and prepares a community-contributed workflow template for submission.")
	fmt.Println("The command will:")
	fmt.Println("  1. Validate the YAML file exists and is well-formed")
	fmt.Println("  2. Generate skill.json metadata")
	fmt.Println("  3. Place the template in the correct templates/ directory")
	fmt.Println("  4. Rebuild the skills registry")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --category, -c <name>   Skill category (default: auto-detected)")
	fmt.Println("  --author, -a <name>     Author name (default: \"community contributor\")")
	fmt.Println("  --help, -h              Show this help")
	fmt.Println()
	fmt.Println("Standard categories:")
	fmt.Printf("  %s\n", strings.Join(validSkillCategories(), ", "))
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare template submit my-workflow.yaml --category devops-infra")
	fmt.Println("  aflare template submit stock-analyzer.yaml -c finance -a \"Your Name\"")
	fmt.Println()
	fmt.Println("After submission, follow the printed instructions to create a PR.")
}

// extractYAMLField extracts a simple top-level YAML field value.
// This is a best-effort parser; it handles the common case of:
//
//	field: value
func extractYAMLField(content, field string) string {
	prefix := field + ":"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			// Remove surrounding quotes
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// guessCategoryFromName tries to guess the category from the template name.
func guessCategoryFromName(name string) string {
	lower := strings.ToLower(name)
	keywords := map[string]string{
		"devops":    "devops-infra",
		"deploy":    "devops-infra",
		"docker":    "devops-infra",
		"k8s":       "devops-infra",
		"kubernetes": "devops-infra",
		"ci":        "devops-infra",
		"cd":        "devops-infra",
		"monitor":   "devops-infra",
		"security":  "security",
		"audit":     "security",
		"vuln":      "security",
		"finance":   "finance",
		"stock":     "finance",
		"trading":   "finance",
		"code":      "software-engineering",
		"review":    "software-engineering",
		"test":      "software-engineering",
		"data":      "data-ai",
		"etl":       "data-ai",
		"ml":        "data-ai",
		"ai":        "data-ai",
		"business":  "business",
		"marketing": "marketing",
		"seo":       "marketing",
		"content":   "content-creative",
		"health":    "healthcare",
		"medical":   "healthcare",
		"hr":        "hr",
		"recruit":   "hr",
		"legal":     "legal",
		"contract":  "legal",
		"shop":      "ecommerce",
		"ecommerce": "ecommerce",
		"product":   "ecommerce",
		"iot":       "iot",
		"sensor":    "iot",
		"supply":    "supply-chain",
		"logistics": "supply-chain",
	}
	for key, cat := range keywords {
		if strings.Contains(lower, key) {
			return cat
		}
	}
	return "uncategorized"
}

// validSkillCategories returns the list of standard skill categories.
func validSkillCategories() []string {
	return []string{
		"business",
		"content-creative",
		"data-ai",
		"devops-infra",
		"ecommerce",
		"education",
		"finance",
		"healthcare",
		"hr",
		"integrations",
		"iot",
		"legal",
		"lifestyle",
		"marketing",
		"software-engineering",
		"supply-chain",
	}
}

// awardBadgeForTemplate records a contribution and awards a badge if earned.
// The author name is used as the contributor identifier. If no email is
// available, a placeholder is used.
func awardBadgeForTemplate(author, templateID string) {
	// Generate a contributor ID from the author name alone (email unknown).
	cid := badge.ContributorID(author, author+"@contributor.aflare.dev")

	store, err := badge.LoadStore(badge.DefaultStorePath())
	if err != nil {
		// Non-fatal: badge store failure should not block template submission.
		return
	}

	reason := fmt.Sprintf("Submitted template: %s", templateID)
	b, awarded := store.RecordContribution(cid, author, reason, badge.ContributionTemplate)
	if err := store.Save(); err != nil {
		return
	}

	if awarded {
		fmt.Println()
		fmt.Printf("  %s Virtual badge earned: %s!\n", badge.TierEmoji[b.Tier], b.Tier)
		fmt.Printf("  View your badges: aflare badge show %s\n", cid[:8])
	}
}