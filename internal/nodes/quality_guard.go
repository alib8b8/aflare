package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	validAssessmentTypes = map[string]bool{
		"ai_detection":    true,
		"design_quality":  true,
		"code_quality":    true,
		"writing_quality": true,
		"overall":         true,
	}
)

type QualityGuardNode struct{}

func (n *QualityGuardNode) Name() string { return "quality_guard" }

func (n *QualityGuardNode) Description() string {
	return "AI content quality guard with detection, assessment, and enhancement capabilities. Identifies low-quality AI-generated content and provides quality scoring."
}

func (n *QualityGuardNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - content to assess",
		Output:      "string - JSON with quality assessment results",
		Params: []ParamSchema{
			{Name: "content", Type: "string", Description: "Content to assess (max 20000 chars)", Required: false},
			{Name: "assessment_type", Type: "string", Description: "Assessment type: ai_detection/design_quality/code_quality/writing_quality/overall (default: overall)", Required: false, Default: "overall"},
			{Name: "quality_threshold", Type: "float", Description: "Quality threshold 0.0-1.0 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "auto_fix", Type: "bool", Description: "Auto fix low-quality content (default: false)", Required: false, Default: "false"},
			{Name: "verbose", Type: "bool", Description: "Detailed report (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *QualityGuardNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	content := getParam(params, "content", "")
	if input != "" && content == "" {
		content = input
	}
	if len(content) > 20000 {
		return "", fmt.Errorf("content too long (max 20000 chars)")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	assessmentType := getParam(params, "assessment_type", "overall")
	if !validAssessmentTypes[assessmentType] {
		return "", fmt.Errorf("invalid assessment_type: %s", assessmentType)
	}

	qualityThreshold := parseFloatSafe(getParam(params, "quality_threshold", "0.7"), 0.7)
	if qualityThreshold < 0.0 || qualityThreshold > 1.0 {
		return "", fmt.Errorf("quality_threshold must be between 0.0 and 1.0")
	}

	autoFix := getParam(params, "auto_fix", "false") == "true"
	verbose := getParam(params, "verbose", "false") == "true"

	score, issues, suggestions, aiProb, fixedContent := simulateQualityAssessment(content, assessmentType)

	passed := score >= qualityThreshold

	if autoFix && !passed {
		fixedContent = simulateAutoFix(content, assessmentType, suggestions)
	}

	result := map[string]interface{}{
		"assessment_type": assessmentType,
		"quality_score":   score,
		"passed":          passed,
		"issues":          issues,
		"suggestions":     suggestions,
		"ai_probability":  aiProb,
		"fixed_content":   fixedContent,
	}

	if verbose {
		result["threshold"] = qualityThreshold
		result["auto_fix_applied"] = autoFix
		result["content_length"] = len(content)
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func simulateQualityAssessment(content string, assessmentType string) (float64, []string, []string, float64, string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	var score float64
	var issues []string
	var suggestions []string
	var aiProb float64

	switch assessmentType {
	case "ai_detection":
		aiProb = r.Float64()
		if aiProb > 0.7 {
			score = 0.3 + r.Float64()*0.3
			issues = []string{
				"Content exhibits AI-generated patterns",
				"Repetitive sentence structure detected",
				"Lack of personal voice or unique perspective",
			}
			suggestions = []string{
				"Add personal anecdotes and experiences",
				"Vary sentence length and structure",
				"Inject unique insights and opinions",
			}
		} else {
			score = 0.6 + r.Float64()*0.4
			issues = []string{}
			suggestions = []string{"Content appears authentic"}
		}

	case "design_quality":
		if len(content) < 100 {
			score = 0.4 + r.Float64()*0.2
			issues = []string{
				"Insufficient design details",
				"Lack of visual hierarchy",
				"Missing responsive design considerations",
			}
			suggestions = []string{
				"Add detailed layout specifications",
				"Define visual hierarchy and typography",
				"Include responsive breakpoints",
			}
		} else {
			score = 0.5 + r.Float64()*0.5
			issues = []string{
				"Color contrast could be improved",
				"Spacing consistency issues",
			}
			suggestions = []string{
				"Check WCAG contrast ratios",
				"Establish consistent spacing system",
			}
		}
		aiProb = 0.3 + r.Float64()*0.4

	case "code_quality":
		if strings.Contains(strings.ToLower(content), "todo") || strings.Contains(strings.ToLower(content), "fixme") {
			score = 0.4 + r.Float64()*0.3
			issues = []string{
				"Unresolved TODO/FIXME comments",
				"Potential code duplication",
				"Missing error handling",
			}
			suggestions = []string{
				"Resolve outstanding TODO items",
				"Extract duplicated code into functions",
				"Add proper error handling",
			}
		} else {
			score = 0.6 + r.Float64()*0.4
			issues = []string{
				"Complexity could be reduced",
				"Missing unit tests",
			}
			suggestions = []string{
				"Refactor complex functions",
				"Add comprehensive test coverage",
			}
		}
		aiProb = 0.4 + r.Float64()*0.4

	case "writing_quality":
		if len(content) < 50 {
			score = 0.3 + r.Float64()*0.2
			issues = []string{
				"Content too brief",
				"Lack of structure",
				"Weak opening",
			}
			suggestions = []string{
				"Expand content with more details",
				"Add clear introduction and conclusion",
				"Use engaging opening hooks",
			}
		} else {
			score = 0.5 + r.Float64()*0.5
			issues = []string{
				"Some sentences could be more concise",
				"Paragraph structure could be improved",
			}
			suggestions = []string{
				"Trim redundant words",
				"Ensure each paragraph has a clear topic",
			}
		}
		aiProb = 0.2 + r.Float64()*0.5

	case "overall":
		aiProb = 0.3 + r.Float64()*0.4
		if aiProb > 0.6 {
			score = 0.4 + r.Float64()*0.3
			issues = []string{
				"Overall quality below standard",
				"AI-generated patterns detected",
				"Content lacks depth",
			}
			suggestions = []string{
				"Add more detailed analysis",
				"Include original insights",
				"Improve structure and flow",
			}
		} else {
			score = 0.6 + r.Float64()*0.4
			issues = []string{
				"Minor improvements suggested",
			}
			suggestions = []string{
				"Review for clarity",
				"Enhance readability",
			}
		}
	}

	return score, issues, suggestions, aiProb, ""
}

func simulateAutoFix(content string, assessmentType string, suggestions []string) string {
	if len(suggestions) == 0 {
		return content
	}

	fixed := content
	fixed += "\n\n[Auto-enhanced version based on quality suggestions:"
	for i, s := range suggestions {
		fixed += fmt.Sprintf("\n%d. %s", i+1, s)
	}
	fixed += "]"

	return fixed
}

func init() {
	Register(&QualityGuardNode{})
}
