// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

type AITraceFinding struct {
	RuleID     string
	Severity   string
	Category   string
	Title      string
	Detail     string
	Matched    string
	Suggestion string
}

type OutputQualityScore struct {
	Naturalness  float64
	Conciseness  float64
	Personality  float64
	InfoDensity  float64
	StructureVar float64
	Overall      float64
	Grade        string
}

type OutputQualityNode struct{}

func init() {
	Register(&OutputQualityNode{})
}

func (n *OutputQualityNode) Name() string {
	return "output_quality"
}

func (n *OutputQualityNode) Description() string {
	return "Anti-AI-flavor output quality analyzer: detect AI writing traces and score naturalness (hallmark inspired)"
}

func (n *OutputQualityNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "output_quality",
		Description: "Analyze output text for AI-generated traces and compute naturalness scores. Inspired by Nutlope/hallmark (57 anti-AI-taste detection checks). Detects template phrases, robotic structure, and generic content. Provides rewrite suggestions.",
		Input:       "string - the text to analyze for AI traces and quality",
		Output:      "string - quality report with scores, detected issues, and suggestions",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: analyze|gate|suggest|checklist (default: analyze)", Required: false, Default: "analyze"},
			{Name: "min_score", Type: "string", Description: "Minimum pass score 0-100 for gate action (default: 60)", Required: false, Default: "60"},
			{Name: "detail", Type: "string", Description: "Detail level: brief|full (default: full)", Required: false, Default: "full"},
			{Name: "lang", Type: "string", Description: "Language hint: auto|zh|en (default: auto)", Required: false, Default: "auto"},
		},
	}
}

var aiTraceRules = []struct {
	ID         string
	Severity   string
	Category   string
	Pattern    *regexp.Regexp
	Title      string
	Suggestion string
}{
	{
		ID: "TPL-001", Severity: "high", Category: "template",
		Pattern:    regexp.MustCompile(`(?i)(希望对你有帮助|希望能帮到你|如有疑问|请随时|不要犹豫|请联系我)`),
		Title:      "AI 客套结束语",
		Suggestion: "删除套话，直接给出结论或行动项",
	},
	{
		ID: "TPL-002", Severity: "high", Category: "template",
		Pattern:    regexp.MustCompile(`(?i)(总的来说|总而言之|综上所述|一言以蔽之|总结一下)`),
		Title:      "模板化总结词",
		Suggestion: "用具体结论替代模板化总结，或直接省略",
	},
	{
		ID: "TPL-003", Severity: "medium", Category: "template",
		Pattern:    regexp.MustCompile(`(?i)(首先|其次|再次|然后|接着|最后|综上所述|不仅如此|更重要的是)`),
		Title:      "机械连接词堆砌",
		Suggestion: "减少显性连接词，通过内容逻辑自然过渡",
	},
	{
		ID: "TPL-004", Severity: "medium", Category: "template",
		Pattern:    regexp.MustCompile(`(?i)(非常重要|值得注意|需要强调|特别指出|关键在于|核心是|本质上)`),
		Title:      "空洞强调词",
		Suggestion: "用具体数据或例子替代空泛的强调",
	},
	{
		ID: "TPL-005", Severity: "high", Category: "template",
		Pattern:    regexp.MustCompile(`(?i)(在当今|在这个|在现代|在当下|在快速发展|在日新月异)`),
		Title:      "AI 式背景铺垫",
		Suggestion: "删除背景铺垫，直接切入主题",
	},
	{
		ID: "EMO-001", Severity: "medium", Category: "emoji",
		Pattern:    regexp.MustCompile(`[🚀✨🎯✅🔧📊💡🔥⭐🛠️⚡🎨📝📌💪🎯🚀🔍📈]{3,}`),
		Title:      "过度使用 Emoji (AI 特征)",
		Suggestion: "减少 emoji 密度，每 3-5 段不超过 1 个 emoji",
	},
	{
		ID: "EMO-002", Severity: "low", Category: "emoji",
		Pattern:    regexp.MustCompile(`^#{1,3}\s*[🚀✨🎯✅🔧📊💡🔥⭐🛠️]`),
		Title:      "标题开头用 Emoji (AI 习惯)",
		Suggestion: "标题使用纯文本，emoji 放正文或省略",
	},
	{
		ID: "GEN-001", Severity: "medium", Category: "generic",
		Pattern:    regexp.MustCompile(`(?i)(多种方法|一些建议|几个方面|不同角度|各种因素|相关问题|一定程度)`),
		Title:      "模糊量化词 (AI 回避)",
		Suggestion: "用具体数字替代模糊描述",
	},
	{
		ID: "GEN-002", Severity: "high", Category: "generic",
		Pattern:    regexp.MustCompile(`(?i)(可以帮助|能够提高|有助于|有利于|促进|增强|提升)`),
		Title:      "万能动词 (AI 模板)",
		Suggestion: "用具体的因果描述替代万能动词",
	},
	{
		ID: "ENG-001", Severity: "high", Category: "english",
		Pattern:    regexp.MustCompile(`(?i)(it is important to note that|it should be noted that|it is worth mentioning)`),
		Title:      "英文 AI 套话 (it is important)",
		Suggestion: "删除套话，直接陈述内容",
	},
	{
		ID: "ENG-002", Severity: "medium", Category: "english",
		Pattern:    regexp.MustCompile(`(?i)(in conclusion|to summarize|in summary|as mentioned above|as we have seen)`),
		Title:      "英文模板化总结",
		Suggestion: "省略模板总结，用具体结论收尾",
	},
	{
		ID: "ENG-003", Severity: "medium", Category: "english",
		Pattern:    regexp.MustCompile(`(?i)(first and foremost|last but not least|by the same token|in the same vein)`),
		Title:      "英文机械连接词",
		Suggestion: "用内容逻辑替代过渡套话",
	},
}

func (n *OutputQualityNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "analyze")
	minScoreStr := getParam(params, "min_score", "60")
	detail := getParam(params, "detail", "full")
	lang := getParam(params, "lang", "auto")

	minScore := 60.0
	fmt.Sscanf(minScoreStr, "%f", &minScore)

	text := strings.TrimSpace(input)
	if text == "" {
		return "", fmt.Errorf("input text cannot be empty")
	}

	if lang == "auto" {
		lang = detectLang(text)
	}

	findings := n.detectAITraces(text, lang)
	scores := n.computeScores(text, lang, findings)

	switch action {
	case "gate":
		return n.actionGate(text, scores, minScore, findings), nil
	case "suggest":
		return n.actionSuggest(text, findings, detail), nil
	case "checklist":
		return n.actionChecklist(), nil
	case "analyze":
		fallthrough
	default:
		return n.actionAnalyze(text, scores, findings, detail), nil
	}
}

func (n *OutputQualityNode) detectAITraces(text, lang string) []AITraceFinding {
	var findings []AITraceFinding

	for _, rule := range aiTraceRules {
		if rule.Category == "english" && lang != "en" {
			continue
		}
		if rule.Category != "english" && rule.Category != "emoji" && lang == "en" &&
			(rule.ID[:3] == "TPL" || rule.ID[:3] == "GEN") {
			continue
		}

		matches := rule.Pattern.FindAllString(text, -1)
		for _, m := range matches {
			findings = append(findings, AITraceFinding{
				RuleID:     rule.ID,
				Severity:   rule.Severity,
				Category:   rule.Category,
				Title:      rule.Title,
				Detail:     m,
				Matched:    m,
				Suggestion: rule.Suggestion,
			})
		}
	}

	structureIssues := n.detectStructureIssues(text, lang)
	findings = append(findings, structureIssues...)

	return findings
}

func (n *OutputQualityNode) detectStructureIssues(text, lang string) []AITraceFinding {
	var findings []AITraceFinding

	lines := strings.Split(text, "\n")
	var nonEmptyLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			nonEmptyLines = append(nonEmptyLines, trimmed)
		}
	}

	if len(nonEmptyLines) >= 4 {
		uniformCount := 0
		for i := 0; i < len(nonEmptyLines)-1; i++ {
			len1 := utf8.RuneCountInString(nonEmptyLines[i])
			len2 := utf8.RuneCountInString(nonEmptyLines[i+1])
			if math.Abs(float64(len1-len2)) < float64(len1)*0.15 {
				uniformCount++
			}
		}
		if uniformCount >= int(float64(len(nonEmptyLines))*0.6) {
			findings = append(findings, AITraceFinding{
				RuleID: "STR-001", Severity: "medium", Category: "structure",
				Title:      "段落长度过度均匀 (AI 特征)",
				Suggestion: "有意制造长短段落交错，模拟人类写作节奏",
			})
		}
	}

	sentences := regexp.MustCompile(`[。！？.!?]+`).Split(text, -1)
	if len(sentences) >= 5 {
		lengths := make([]float64, 0, len(sentences))
		for _, s := range sentences {
			if strings.TrimSpace(s) != "" {
				lengths = append(lengths, float64(utf8.RuneCountInString(strings.TrimSpace(s))))
			}
		}
		if len(lengths) >= 5 {
			mean := 0.0
			for _, l := range lengths {
				mean += l
			}
			mean /= float64(len(lengths))
			variance := 0.0
			for _, l := range lengths {
				variance += (l - mean) * (l - mean)
			}
			variance /= float64(len(lengths))
			stdDev := math.Sqrt(variance)
			if stdDev < mean*0.25 {
				findings = append(findings, AITraceFinding{
					RuleID: "STR-002", Severity: "high", Category: "structure",
					Title:      "句子长度方差过小 (典型 AI 特征)",
					Suggestion: "混合长句与短句，加入1-2个非常短的断句",
				})
			}
		}
	}

	return findings
}

func (n *OutputQualityNode) computeScores(text, lang string, findings []AITraceFinding) OutputQualityScore {
	totalChars := float64(utf8.RuneCountInString(text))
	if totalChars < 1 {
		totalChars = 1
	}

	highCount, mediumCount, lowCount := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	penaltyPer100Chars := (float64(highCount)*3.0 + float64(mediumCount)*1.5 + float64(lowCount)*0.5) / (totalChars / 100.0)
	naturalness := math.Max(0, 100.0-penaltyPer100Chars*8.0)

	nonSpaceChars := float64(len(strings.ReplaceAll(text, " ", "")))
	spaceRatio := nonSpaceChars / totalChars
	conciseness := 60.0 + spaceRatio*40.0
	if conciseness > 100 {
		conciseness = 100
	}

	uniqueWords := map[string]bool{}
	words := regexp.MustCompile(`\p{Han}+|[a-zA-Z]+`).FindAllString(text, -1)
	for _, w := range words {
		uniqueWords[strings.ToLower(w)] = true
	}
	vocabDiversity := 0.0
	if len(words) > 0 {
		vocabDiversity = float64(len(uniqueWords)) / float64(len(words)) * 100.0
	}
	personality := math.Min(100, vocabDiversity*1.2)

	numbers := regexp.MustCompile(`\d+`).FindAllString(text, -1)
	dataDensity := float64(len(numbers)) / (totalChars / 200.0) * 20.0
	infoDensity := math.Min(100, 40.0+dataDensity)

	structureVar := 80.0
	for _, f := range findings {
		if f.Category == "structure" {
			if f.Severity == "high" {
				structureVar -= 25
			} else {
				structureVar -= 15
			}
		}
	}
	structureVar = math.Max(0, structureVar)

	overall := naturalness*0.30 + conciseness*0.15 + personality*0.20 + infoDensity*0.15 + structureVar*0.20

	var grade string
	switch {
	case overall >= 90:
		grade = "S"
	case overall >= 80:
		grade = "A"
	case overall >= 70:
		grade = "B"
	case overall >= 60:
		grade = "C"
	case overall >= 50:
		grade = "D"
	default:
		grade = "F"
	}

	return OutputQualityScore{
		Naturalness:  round1(naturalness),
		Conciseness:  round1(conciseness),
		Personality:  round1(personality),
		InfoDensity:  round1(infoDensity),
		StructureVar: round1(structureVar),
		Overall:      round1(overall),
		Grade:        grade,
	}
}

func (n *OutputQualityNode) actionAnalyze(text string, scores OutputQualityScore, findings []AITraceFinding, detail string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 📊 Output Quality Report\n\n"))
	sb.WriteString(fmt.Sprintf("### Grade: **%s** | Overall: **%.1f/100**\n\n", scores.Grade, scores.Overall))

	sb.WriteString("| Dimension | Score | Bar |\n")
	sb.WriteString("|-----------|-------|-----|\n")
	sb.WriteString(fmt.Sprintf("| Naturalness | %.1f | %s |\n", scores.Naturalness, renderBar(scores.Naturalness)))
	sb.WriteString(fmt.Sprintf("| Conciseness | %.1f | %s |\n", scores.Conciseness, renderBar(scores.Conciseness)))
	sb.WriteString(fmt.Sprintf("| Personality | %.1f | %s |\n", scores.Personality, renderBar(scores.Personality)))
	sb.WriteString(fmt.Sprintf("| Info Density | %.1f | %s |\n", scores.InfoDensity, renderBar(scores.InfoDensity)))
	sb.WriteString(fmt.Sprintf("| Structure Var | %.1f | %s |\n", scores.StructureVar, renderBar(scores.StructureVar)))
	sb.WriteString("\n")

	if detail == "brief" && len(findings) > 5 {
		findings = findings[:5]
	}

	if len(findings) > 0 {
		sb.WriteString(fmt.Sprintf("### 🔍 Detected AI Traces (%d issues)\n\n", len(findings)))
		for i, f := range findings {
			sevEmoji := "🟡"
			if f.Severity == "high" {
				sevEmoji = "🔴"
			} else if f.Severity == "low" {
				sevEmoji = "🟢"
			}
			sb.WriteString(fmt.Sprintf("%d. %s **%s** `[%s]`\n", i+1, sevEmoji, f.Title, f.RuleID))
			sb.WriteString(fmt.Sprintf("   Matched: `%s`\n", truncateOutput(f.Detail, 60)))
			sb.WriteString(fmt.Sprintf("   💡 %s\n\n", f.Suggestion))
		}
	} else {
		sb.WriteString("### ✨ No significant AI traces detected\n\n")
		sb.WriteString("The text reads naturally with good human-like characteristics.\n")
	}

	return sb.String()
}

func (n *OutputQualityNode) actionGate(text string, scores OutputQualityScore, minScore float64, findings []AITraceFinding) string {
	var sb strings.Builder
	passed := scores.Overall >= minScore

	sb.WriteString("## 🚦 Quality Gate\n\n")
	if passed {
		sb.WriteString(fmt.Sprintf("### ✅ PASSED (%.1f ≥ %.1f)\n\n", scores.Overall, minScore))
	} else {
		sb.WriteString(fmt.Sprintf("### ❌ FAILED (%.1f < %.1f)\n\n", scores.Overall, minScore))
	}
	sb.WriteString(fmt.Sprintf("Grade: **%s** | Naturalness: %.1f | Structure: %.1f\n\n", scores.Grade, scores.Naturalness, scores.StructureVar))

	if !passed && len(findings) > 0 {
		sb.WriteString("### Top Issues to Fix:\n\n")
		for i, f := range findings {
			if i >= 3 {
				break
			}
			sb.WriteString(fmt.Sprintf("- 🔴 **%s**: %s\n  💡 %s\n", f.Title, truncateOutput(f.Detail, 50), f.Suggestion))
		}
	}

	return sb.String()
}

func (n *OutputQualityNode) actionSuggest(text string, findings []AITraceFinding, detail string) string {
	var sb strings.Builder

	sb.WriteString("## ✍️ Rewrite Suggestions\n\n")

	if len(findings) == 0 {
		sb.WriteString("✅ No rewrite suggestions needed. The text looks natural.\n")
		return sb.String()
	}

	seen := map[string]bool{}
	suggestionNum := 1
	for _, f := range findings {
		key := f.Title + "|" + f.Suggestion
		if seen[key] {
			continue
		}
		seen[key] = true

		sb.WriteString(fmt.Sprintf("%d. **%s**\n", suggestionNum, f.Title))
		sb.WriteString(fmt.Sprintf("   Problem: `%s`\n", truncateOutput(f.Detail, 50)))
		sb.WriteString(fmt.Sprintf("   Fix: %s\n\n", f.Suggestion))
		suggestionNum++
	}

	sb.WriteString(fmt.Sprintf("---\nTotal: %d unique improvement suggestions\n", suggestionNum-1))
	return sb.String()
}

func (n *OutputQualityNode) actionChecklist() string {
	var sb strings.Builder
	sb.WriteString("## ✅ Anti-AI-Flavor Writing Checklist\n\n")

	checklist := []struct {
		Category string
		Items    []string
	}{
		{"Template Phrases", []string{
			"删除 '希望对你有帮助/如有疑问请随时' 等客套结束语",
			"替换 '首先/其次/最后/综上所述' 等机械连接词",
			"移除 '在当今/在这个快速发展' 等AI式背景铺垫",
			"替换 '非常重要/值得注意/需要强调' 等空洞强调",
		}},
		{"Emoji Usage", []string{
			"每3-5段不超过1个emoji",
			"标题不使用emoji开头",
			"避免连续3个以上emoji",
		}},
		{"Structure Variety", []string{
			"故意制造长短段落交错",
			"混合使用长句与短句",
			"加入1-2个非常短的断句",
			"避免段落长度过度均匀",
		}},
		{"Concreteness", []string{
			"用具体数字替代模糊描述",
			"用因果描述替代 '可以帮助/能够提高'",
			"用实例替代空泛的强调",
			"删除万能动词和套话",
		}},
	}

	for _, c := range checklist {
		sb.WriteString(fmt.Sprintf("### 📌 %s\n\n", c.Category))
		for _, item := range c.Items {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("---\nTotal: %d checklist items across %d categories\n",
		len(checklist[0].Items)+len(checklist[1].Items)+len(checklist[2].Items)+len(checklist[3].Items),
		len(checklist)))

	return sb.String()
}

func detectLang(text string) string {
	zhCount := 0
	enCount := 0
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			zhCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			enCount++
		}
	}
	if zhCount > enCount {
		return "zh"
	}
	return "en"
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}

func truncateOutput(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}
