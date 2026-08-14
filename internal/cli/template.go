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
// Usage: aflare template <list|new|clone|run|submit> [args]
//
// dryRun/safeMode are forwarded from the top-level flag parser so that
// `aflare --dry-run template run <id>` and `aflare --safe-mode template run <id>`
// behave like the equivalent `aflare run` invocation.
func HandleTemplateSubmit(args []string, dryRun, safeMode bool) {
	if len(args) == 0 {
		PrintTemplateSubmitUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "submit":
		handleTemplateSubmit(args[1:])
	case "list":
		handleTemplateList(args[1:])
	case "new":
		handleTemplateNew(args[1:])
	case "clone":
		handleTemplateClone(args[1:])
	case "run":
		handleTemplateRun(args[1:], dryRun, safeMode)
	case "--help", "-h", "help":
		PrintTemplateSubmitUsage()
	default:
		fmt.Printf("Unknown template subcommand: %s\n\n", subCmd)
		PrintTemplateSubmitUsage()
		os.Exit(1)
	}
}

// handleTemplateRun implements `aflare template run <template-id> [run flags]`.
// It resolves a template by ID (with the same fuzzy fallback as clone) and
// immediately executes its workflow.yaml via HandleRun, forwarding --set /
// --params-file / --resume etc. This is the one-command "try a template" path:
// no need to clone first or remember the workflow.yaml path.
func handleTemplateRun(args []string, dryRun, safeMode bool) {
	if len(args) == 0 {
		fmt.Println("Error: template run requires <template-id>")
		fmt.Println("Usage: aflare template run <template-id> [--set k=v ...] [--params-file f] [--resume]")
		fmt.Println("提示：使用 aflare template list --all 查看所有可用模板")
		os.Exit(1)
	}

	// 第一个非 flag 参数是 template-id，其余透传给 run 命令的参数解析器。
	var templateID string
	var runArgs []string
	for _, a := range args {
		if templateID == "" && !strings.HasPrefix(a, "-") {
			templateID = a
		} else {
			runArgs = append(runArgs, a)
		}
	}
	if templateID == "" {
		fmt.Println("Error: template run requires <template-id>")
		os.Exit(1)
	}

	templatesDir := meta.ResolveTemplatesPath()
	_ = skillsPkg.EnsureEmbeddedTemplates(templatesDir)
	registry := skillsPkg.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("❌ 加载模板失败：%v\n", err)
		os.Exit(1)
	}

	src, err := registry.Get(templateID)
	if err != nil {
		// Fuzzy match by name suffix (与 clone 一致的回退逻辑).
		for _, s := range registry.List() {
			if s.Name == templateID || strings.HasSuffix(s.ID, "/"+templateID) {
				src = s
				break
			}
		}
		if src == nil {
			fmt.Printf("❌ 未找到模板：%s\n", templateID)
			fmt.Println("提示：使用 aflare template list --all 查看所有可用模板")
			os.Exit(1)
		}
	}

	wfPath := filepath.Join(src.Path, "workflow.yaml")
	if _, err := os.Stat(wfPath); err != nil {
		fmt.Printf("❌ 模板缺少 workflow.yaml：%s\n", wfPath)
		os.Exit(1)
	}

	fmt.Printf("▶ 运行模板 %s\n", src.ID)
	// 复用 run 命令的参数解析（--set / --params-file / --resume 等）：
	// 把 workflow 路径放在最前，其余 flag 跟在后面，HandleRun 会正确分流。
	runArgs = append([]string{wfPath}, runArgs...)
	HandleRun(runArgs, dryRun, safeMode)
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

	// baseName is derived from the user-supplied file path and is later
	// joined into the templates tree (templatesDir/category/baseName).
	// Validate it is a single safe path component to prevent traversal
	// (e.g. a file literally named "..yaml" would otherwise let the
	// writer escape templatesDir).
	if err := validateTemplateNameComponent(baseName, "template name"); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Determine category.
	if category == "" {
		category = guessCategoryFromName(baseName)
	}
	// category (when user-supplied via --category) is joined into the
	// templates path too, so it must be a single safe component.
	if err := validateTemplateNameComponent(category, "category"); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
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
	_ = os.WriteFile(readmePath, []byte(readmeContent), 0644) // best-effort: README stub is non-critical

	// Rebuild the registry.
	_ = skillsPkg.EnsureEmbeddedTemplates(templatesDir)
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

// validateTemplateNameComponent validates that name is a single safe path
// component (no directory traversal, no path separators, no null bytes, no
// leading dot, no Windows drive letter). It is used for user-supplied
// template names, category names, and destination names that get joined
// into filesystem paths — without it, a value like "../evil" or "a/b"
// could escape the intended template directory (path traversal).
func validateTemplateNameComponent(name, field string) error {
	if name == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if len(name) > 128 {
		return fmt.Errorf("%s is too long (max 128 characters)", field)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("%s must not contain null bytes", field)
	}
	// Reject path separators (both Unix and Windows) — a name component
	// must never contain them.
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%s must not contain path separators", field)
	}
	// Reject parent-directory references and the literal "." / "..".
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("%s must not contain parent-directory references", field)
	}
	// Reject leading dots (hidden files / traversal variants like ".bashrc").
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("%s must not start with a dot", field)
	}
	// Reject Windows drive letters (e.g. "C:foo") which filepath.Join
	// treats as an absolute path on Windows.
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("%s must not look like a Windows drive path", field)
	}
	return nil
}

// validateTemplateFile validates that the given path is an existing YAML file.
// Returns the absolute path on success, or an error describing the problem.
func validateTemplateFile(yamlPath string) (string, error) {
	absPath, err := filepath.Abs(yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
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

// PrintTemplateSubmitUsage prints usage for the template command.
func PrintTemplateSubmitUsage() {
	fmt.Println("Usage: aflare template <list|new|clone|run|submit> [args]")
	fmt.Println()
	fmt.Println("子命令：")
	fmt.Println("  list                        列出可用模板（默认只显示 easy，--all 显示全部）")
	fmt.Println("  new <name>                  创建工作流骨架到 ./<name>/workflow.yaml")
	fmt.Println("  clone <source> <dest>       复制已有模板到本地进行改造")
	fmt.Println("  run <template-id> [flags]   直接按 ID 运行模板（无需 clone 或记路径）")
	fmt.Println("  submit <file.yaml>          校验并准备社区模板提交")
	fmt.Println()
	fmt.Println("list 选项：")
	fmt.Println("  --all                       显示全部模板（含需要 LLM/沙箱的）")
	fmt.Println("  --category <name>           按分类筛选")
	fmt.Println()
	fmt.Println("run 选项（透传给 aflare run）：")
	fmt.Println("  --set k=v                   注入参数（可重复）")
	fmt.Println("  --params-file <file>        从文件读取参数")
	fmt.Println("  --resume [path]             从断点恢复执行")
	fmt.Println()
	fmt.Println("submit 选项：")
	fmt.Println("  --category, -c <name>       技能分类（默认自动检测）")
	fmt.Println("  --author, -a <name>         作者名（默认 \"community contributor\"）")
	fmt.Println()
	fmt.Println("示例：")
	fmt.Println("  aflare template list")
	fmt.Println("  aflare template list --all")
	fmt.Println("  aflare template list --category devops-infra")
	fmt.Println("  aflare template new my-workflow")
	fmt.Println("  aflare template clone ssl-cert-checker my-cert-checker")
	fmt.Println("  aflare template run devops-infra/ssl-cert-checker")
	fmt.Println("  aflare template run ssl-cert-checker --set domain=example.com")
	fmt.Println("  aflare template submit my-workflow.yaml --category devops-infra")
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
		"devops":     "devops-infra",
		"deploy":     "devops-infra",
		"docker":     "devops-infra",
		"k8s":        "devops-infra",
		"kubernetes": "devops-infra",
		"ci":         "devops-infra",
		"cd":         "devops-infra",
		"monitor":    "devops-infra",
		"security":   "security",
		"audit":      "security",
		"vuln":       "security",
		"finance":    "finance",
		"stock":      "finance",
		"trading":    "finance",
		"code":       "software-engineering",
		"review":     "software-engineering",
		"test":       "software-engineering",
		"data":       "data-ai",
		"etl":        "data-ai",
		"ml":         "data-ai",
		"ai":         "data-ai",
		"business":   "business",
		"marketing":  "marketing",
		"seo":        "marketing",
		"content":    "content-creative",
		"health":     "healthcare",
		"medical":    "healthcare",
		"hr":         "hr",
		"recruit":    "hr",
		"legal":      "legal",
		"contract":   "legal",
		"shop":       "ecommerce",
		"ecommerce":  "ecommerce",
		"product":    "ecommerce",
		"iot":        "iot",
		"sensor":     "iot",
		"supply":     "supply-chain",
		"logistics":  "supply-chain",
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
