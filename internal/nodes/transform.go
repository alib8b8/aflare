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
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

type TransformNode struct{}

func init() {
	Register(&TransformNode{})
}

func (n *TransformNode) Name() string {
	return "transform"
}

func (n *TransformNode) Description() string {
	return "Transform text using regex or string operations"
}

func (n *TransformNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "transform",
		Description: "Transform text using string operations",
		Input:       "string - text to transform",
		Output:      "string - transformed text",
		Params: []ParamSchema{
			{Name: "operation", Type: "string", Description: "Transformation operation", Required: false},
		},
	}
}

func (n *TransformNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation, ok := params["operation"]
	if !ok || operation == "" {
		return input, nil
	}

	switch strings.ToLower(operation) {
	case "upper":
		return strings.ToUpper(input), nil
	case "lower":
		return strings.ToLower(input), nil
	case "trim":
		return strings.TrimSpace(input), nil
	case "lines":
		lines := strings.Split(input, "\n")
		return fmt.Sprintf("%d lines", len(lines)), nil
	case "words":
		words := strings.Fields(input)
		return fmt.Sprintf("%d words", len(words)), nil
	case "chars":
		return fmt.Sprintf("%d characters", len(input)), nil
	case "first_line":
		lines := strings.SplitN(input, "\n", 2)
		if len(lines) > 0 {
			return lines[0], nil
		}
		return "", nil
	case "first_500":
		runes := []rune(input)
		if len(runes) > 500 {
			return string(runes[:500]) + "...", nil
		}
		return input, nil
	case "first_1000":
		runes := []rune(input)
		if len(runes) > 1000 {
			return string(runes[:1000]) + "...", nil
		}
		return input, nil
	case "summary":
		runes := []rune(input)
		if len(runes) > 200 {
			return string(runes[:200]) + "...", nil
		}
		return input, nil
	case "reverse":
		runes := []rune(input)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil
	case "unique_lines":
		lines := strings.Split(input, "\n")
		seen := make(map[string]bool)
		var result []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !seen[trimmed] {
				seen[trimmed] = true
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil
	case "sort_lines":
		lines := strings.Split(input, "\n")
		sort.Strings(lines)
		return strings.Join(lines, "\n"), nil
	case "remove_blank_lines":
		lines := strings.Split(input, "\n")
		var result []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil
	case "filter_errors":
		lines := strings.Split(input, "\n")
		var result []string
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "exception") || strings.Contains(lower, "fatal") {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil
	case "extract_urls":
		re := regexp.MustCompile(`https?://[^\s<>"']+`)
		urls := re.FindAllString(input, -1)
		return strings.Join(urls, "\n"), nil
	case "extract_emails":
		re := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
		emails := re.FindAllString(input, -1)
		return strings.Join(emails, "\n"), nil
	case "markdown_to_html":
		return markdownToHTML(input), nil
	case "html_to_markdown":
		return htmlToMarkdownTransform(input), nil
	case "extract_repos_and_activity":
		return extractReposAndActivity(input), nil
	case "combine_and_summarize":
		return combineAndSummarize(input), nil
	case "extract_functions_and_types":
		return extractFunctionsAndTypes(input), nil
	case "group_by_commit_type":
		return groupByCommitType(input), nil
	case "group_by_extension":
		return groupByExtension(input), nil
	case "count_by_label":
		return countByLabel(input), nil
	default:
		return input, nil
	}
}

func markdownToHTML(md string) string {
	html := md

	html = regexp.MustCompile(`(?m)^###### (.*)$`).ReplaceAllString(html, "<h6>$1</h6>")
	html = regexp.MustCompile(`(?m)^##### (.*)$`).ReplaceAllString(html, "<h5>$1</h5>")
	html = regexp.MustCompile(`(?m)^#### (.*)$`).ReplaceAllString(html, "<h4>$1</h4>")
	html = regexp.MustCompile(`(?m)^### (.*)$`).ReplaceAllString(html, "<h3>$1</h3>")
	html = regexp.MustCompile(`(?m)^## (.*)$`).ReplaceAllString(html, "<h2>$1</h2>")
	html = regexp.MustCompile(`(?m)^# (.*)$`).ReplaceAllString(html, "<h1>$1</h1>")

	html = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(html, "<strong>$1</strong>")
	html = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(html, "<em>$1</em>")
	html = regexp.MustCompile("`(.+?)`").ReplaceAllString(html, "<code>$1</code>")

	html = regexp.MustCompile(`\[(.+?)\]\((.+?)\)`).ReplaceAllString(html, "<a href=\"$2\">$1</a>")

	html = regexp.MustCompile(`(?m)^- (.*)$`).ReplaceAllString(html, "<li>$1</li>")
	html = regexp.MustCompile(`(?m)^\d+\. (.*)$`).ReplaceAllString(html, "<li>$1</li>")

	lines := strings.Split(html, "\n")
	var result []string
	inPara := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inPara {
				result = append(result, "</p>")
				inPara = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<h") || strings.HasPrefix(trimmed, "<li") || strings.HasPrefix(trimmed, "<ul") || strings.HasPrefix(trimmed, "<pre") || strings.HasPrefix(trimmed, "<blockquote") {
			if inPara {
				result = append(result, "</p>")
				inPara = false
			}
			result = append(result, line)
		} else {
			if !inPara {
				result = append(result, "<p>")
				inPara = true
			}
			result = append(result, line)
		}
	}
	if inPara {
		result = append(result, "</p>")
	}

	return strings.Join(result, "\n")
}

func htmlToMarkdownTransform(html string) string {
	text := html

	text = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`).ReplaceAllString(text, "# $1\n\n")
	text = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`).ReplaceAllString(text, "## $1\n\n")
	text = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`).ReplaceAllString(text, "### $1\n\n")
	text = regexp.MustCompile(`(?is)<h4[^>]*>(.*?)</h4>`).ReplaceAllString(text, "#### $1\n\n")
	text = regexp.MustCompile(`(?is)<h5[^>]*>(.*?)</h5>`).ReplaceAllString(text, "##### $1\n\n")
	text = regexp.MustCompile(`(?is)<h6[^>]*>(.*?)</h6>`).ReplaceAllString(text, "###### $1\n\n")

	text = regexp.MustCompile(`(?is)<strong[^>]*>(.*?)</strong>`).ReplaceAllString(text, "**$1**")
	text = regexp.MustCompile(`(?is)<b[^>]*>(.*?)</b>`).ReplaceAllString(text, "**$1**")
	text = regexp.MustCompile(`(?is)<em[^>]*>(.*?)</em>`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`(?is)<i[^>]*>(.*?)</i>`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`).ReplaceAllString(text, "`$1`")

	text = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`).ReplaceAllString(text, "[$2]($1)")

	text = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`).ReplaceAllString(text, "- $1\n")
	text = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`).ReplaceAllString(text, "$1\n\n")
	text = regexp.MustCompile(`(?is)<br\s*/?>`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`(?is)<hr\s*/?>`).ReplaceAllString(text, "\n---\n\n")

	text = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.*?)</code></pre>`).ReplaceAllString(text, "```\n$1\n```\n\n")
	text = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`).ReplaceAllString(text, "```\n$1\n```\n\n")

	re := regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, "")

	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	return strings.TrimSpace(text)
}

func extractReposAndActivity(input string) string {
	re := regexp.MustCompile(`href=["']/([^/"]+)/([^/"]+)["']`)
	matches := re.FindAllStringSubmatch(input, -1)

	seen := make(map[string]bool)
	var repos []string
	for _, m := range matches {
		if len(m) >= 3 {
			repo := m[1] + "/" + m[2]
			if !seen[repo] && !strings.Contains(repo, "trending") && !strings.Contains(repo, "topics") && !strings.Contains(repo, "sponsors") && !strings.Contains(repo, "explore") {
				seen[repo] = true
				repos = append(repos, repo)
			}
		}
	}

	if len(repos) == 0 {
		return "No repositories found."
	}

	result := fmt.Sprintf("Found %d repositories:\n\n", len(repos))
	for i, repo := range repos {
		if i >= 20 {
			result += fmt.Sprintf("\n... and %d more", len(repos)-20)
			break
		}
		result += fmt.Sprintf("%d. %s\n", i+1, repo)
	}
	return result
}

func combineAndSummarize(input string) string {
	sections := strings.Split(input, "\n---\n")
	if len(sections) <= 1 {
		return input
	}

	result := fmt.Sprintf("Combined summary of %d sources:\n\n", len(sections))
	for i, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		runes := []rune(section)
		summary := string(runes)
		if len(runes) > 300 {
			summary = string(runes[:300]) + "..."
		}
		result += fmt.Sprintf("## Source %d\n\n%s\n\n", i+1, summary)
	}
	return result
}

func extractFunctionsAndTypes(input string) string {
	var result []string

	funcRegex := regexp.MustCompile(`(?m)^func\s+(\([^)]+\)\s+)?(\w+)\s*\(`)
	typeRegex := regexp.MustCompile(`(?m)^type\s+(\w+)\s+`)
	methodRegex := regexp.MustCompile(`(?m)^func\s+\([^)]+\s+\*?(\w+)\)\s+(\w+)\s*\(`)

	funcMatches := funcRegex.FindAllStringSubmatch(input, -1)
	for _, m := range funcMatches {
		if len(m) >= 3 && m[1] == "" {
			result = append(result, fmt.Sprintf("func %s()", m[2]))
		}
	}

	typeMatches := typeRegex.FindAllStringSubmatch(input, -1)
	for _, m := range typeMatches {
		result = append(result, fmt.Sprintf("type %s", m[1]))
	}

	methodMatches := methodRegex.FindAllStringSubmatch(input, -1)
	for _, m := range methodMatches {
		if len(m) >= 3 {
			result = append(result, fmt.Sprintf("method: (%s).%s()", m[1], m[2]))
		}
	}

	if len(result) == 0 {
		return "No functions or types found."
	}

	return fmt.Sprintf("Found %d definitions:\n\n- %s", len(result), strings.Join(result, "\n- "))
}

func groupByCommitType(input string) string {
	groups := make(map[string][]string)

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		commitType := "other"
		lower := strings.ToLower(line)
		if strings.Contains(lower, "feat") || strings.Contains(lower, "feature") {
			commitType = "features"
		} else if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") {
			commitType = "bugfixes"
		} else if strings.Contains(lower, "docs") || strings.Contains(lower, "documentation") || strings.Contains(lower, "readme") {
			commitType = "documentation"
		} else if strings.Contains(lower, "refactor") {
			commitType = "refactoring"
		} else if strings.Contains(lower, "test") {
			commitType = "tests"
		} else if strings.Contains(lower, "chore") || strings.Contains(lower, "build") || strings.Contains(lower, "ci") {
			commitType = "chores"
		} else if strings.Contains(lower, "perf") || strings.Contains(lower, "performance") {
			commitType = "performance"
		} else if strings.Contains(lower, "style") {
			commitType = "style"
		}

		groups[commitType] = append(groups[commitType], line)
	}

	var result string
	categories := []string{"features", "bugfixes", "documentation", "refactoring", "performance", "tests", "chores", "style", "other"}
	for _, cat := range categories {
		if items, ok := groups[cat]; ok && len(items) > 0 {
			result += fmt.Sprintf("## %s (%d)\n\n", core.TitleCase(cat), len(items))
			for _, item := range items {
				result += fmt.Sprintf("- %s\n", item)
			}
			result += "\n"
		}
	}

	return result
}

func groupByExtension(input string) string {
	groups := make(map[string][]string)

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		filename := ""
		for _, f := range fields {
			if strings.Contains(f, ".") && !strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "d") && len(f) < 50 {
				filename = f
				break
			}
		}

		if filename == "" {
			groups["other"] = append(groups["other"], line)
			continue
		}

		ext := ""
		if idx := strings.LastIndex(filename, "."); idx != -1 {
			ext = strings.ToLower(filename[idx:])
		}
		if ext == "" || ext == filename {
			ext = "no extension"
		}

		groups[ext] = append(groups[ext], line)
	}

	var result string
	var exts []string
	for ext := range groups {
		exts = append(exts, ext)
	}
	sort.Strings(exts)

	for _, ext := range exts {
		items := groups[ext]
		result += fmt.Sprintf("## %s (%d files)\n\n", ext, len(items))
		for _, item := range items {
			result += fmt.Sprintf("- %s\n", item)
		}
		result += "\n"
	}

	return result
}

func countByLabel(input string) string {
	counts := make(map[string]int)

	labelRegex := regexp.MustCompile(`\b(\w[\w-]*)\s*:\s*(\d+)`)
	matches := labelRegex.FindAllStringSubmatch(input, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			counts[m[1]] += 1
		}
	}

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		for _, f := range fields {
			f = strings.Trim(f, "[](){},.:;")
			if len(f) > 1 && len(f) < 30 && strings.ContainsAny(f, "abcdefghijklmnopqrstuvwxyz-") {
				lower := strings.ToLower(f)
				if lower != "the" && lower != "and" && lower != "for" && lower != "with" {
					counts[f]++
				}
			}
		}
	}

	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range counts {
		if v > 1 {
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	var result string
	result += fmt.Sprintf("Label count summary (top %d):\n\n", len(sorted))
	for i, item := range sorted {
		if i >= 20 {
			break
		}
		result += fmt.Sprintf("%d. %s: %d\n", i+1, item.Key, item.Value)
	}

	return result
}
