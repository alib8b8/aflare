package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// GenerateWorkflow creates a workflow from a description using rule-based
// keyword matching. It is NOT an AI / LLM-based generator — it recognizes a
// fixed set of keywords (e.g. "summarize", "translate", "github") and maps
// them to built-in node steps. For complex or dynamic workflows, define the
// YAML directly.
func GenerateWorkflow(description string) (*Workflow, error) {
	desc := strings.ToLower(description)
	wf := &Workflow{}

	var llmNode string
	var llmModel string
	switch {
	case containsLLMKeyword(desc, "deepseek"):
		llmNode = "deepseek"
		llmModel = "deepseek-chat"
	case containsLLMKeyword(desc, "qwen"):
		llmNode = "qwen"
		llmModel = "qwen-turbo"
	case containsLLMKeyword(desc, "xverse"):
		llmNode = "xverse"
		llmModel = "XVERSE-7B-Chat"
	case containsLLMKeyword(desc, "yi"):
		llmNode = "yi"
		llmModel = "yi-lightning"
	case containsLLMKeyword(desc, "baichuan"):
		llmNode = "baichuan"
		llmModel = "Baichuan4"
	case containsLLMKeyword(desc, "internlm"):
		llmNode = "internlm"
		llmModel = "internlm3-latest"
	case containsLLMKeyword(desc, "mistral"):
		llmNode = "mistral"
		llmModel = "mistral-large-latest"
	case containsLLMKeyword(desc, "mimo"):
		llmNode = "mimo"
		llmModel = "mimo-v2.5-pro"
	case containsLLMKeyword(desc, "ima"):
		llmNode = "ima"
		llmModel = "gpt-4o"
	case containsLLMKeyword(desc, "kimi"):
		llmNode = "kimi"
		llmModel = "moonshot-v1-8k"
	case containsLLMKeyword(desc, "minimax"):
		llmNode = "minimax"
		llmModel = "abab6.5s-chat"
	case containsLLMKeyword(desc, "coze"):
		llmNode = "coze"
		llmModel = "glm-4"
	case containsLLMKeyword(desc, "glm"):
		llmNode = "glm"
		llmModel = "glm-4"
	default:
		llmNode = "ollama"
		llmModel = "llama3"
	}

	// Try to extract URL (with or without protocol)
	var urlMatch string
	urlRegex := regexp.MustCompile(`(https?://[^\s]+)`)
	if m := urlRegex.FindString(description); m != "" {
		urlMatch = m
	} else {
		// Try to match a plain domain like example.com, github.com, etc.
		domainRegex := regexp.MustCompile(`\b([a-zA-Z0-9][-a-zA-Z0-9]*\.(?:com|org|net|io|edu|gov|me|dev|ai|app|xyz|co|info)\S*)\b`)
		if m := domainRegex.FindString(description); m != "" {
			urlMatch = "https://" + m
		}
	}
	if urlMatch != "" {
		step := WorkflowStep{Node: "fetch_url", Params: map[string]string{"url": urlMatch}}
		wf.Steps = append(wf.Steps, step)
	}

	// Try to extract file path (only allow simple filenames, not paths)
	fileRegex := regexp.MustCompile(`(save|write|to)\s+([a-zA-Z0-9_-]+\.(txt|md|yaml|json|html|csv|xml))`)
	fileMatch := fileRegex.FindStringSubmatch(desc)
	if len(fileMatch) >= 3 {
		path := fileMatch[2]
		step := WorkflowStep{Node: "file_write", Params: map[string]string{"path": path}}
		wf.Steps = append(wf.Steps, step)
	}

	// Check for common patterns
	if containsActionKeyword(desc, "github") {
		if urlMatch == "" {
			step := WorkflowStep{Node: "fetch_url", Params: map[string]string{"url": "https://github.com/"}}
			wf.Steps = append(wf.Steps, step)
		}
	}

	if containsActionKeyword(desc, "summarize") {
		systemPrompt := getSystemPrompt("summarize")
		if systemPrompt == "" {
			systemPrompt = "You are a helpful assistant that summarizes text concisely."
		}
		step := WorkflowStep{
			Node:   llmNode,
			Params: map[string]string{"model": llmModel, "system": systemPrompt},
		}
		if len(wf.Steps) > 0 && wf.Steps[len(wf.Steps)-1].Node == "file_write" {
			lastStep := wf.Steps[len(wf.Steps)-1]
			steps := make([]WorkflowStep, len(wf.Steps)-1)
			copy(steps, wf.Steps[:len(wf.Steps)-1])
			steps = append(steps, step)
			steps = append(steps, lastStep)
			wf.Steps = steps
		} else {
			wf.Steps = append(wf.Steps, step)
		}
	}

	if containsActionKeyword(desc, "translate") {
		systemPrompt := getSystemPrompt("translate")
		if systemPrompt == "" {
			systemPrompt = "You are a translator. Translate the following text to English."
		}
		step := WorkflowStep{
			Node:   llmNode,
			Params: map[string]string{"model": llmModel, "system": systemPrompt},
		}
		wf.Steps = append(wf.Steps, step)
	}

	if containsActionKeyword(desc, "git") {
		step := WorkflowStep{
			Node:   "execute",
			Params: map[string]string{"command": "git log --oneline -10"},
		}
		wf.Steps = append(wf.Steps, step)
	}



	// Generate workflow name
	wf.Name = generateWorkflowName(description)
	wf.Description = description

	// If no steps were generated, add a default execute step
	if len(wf.Steps) == 0 {
		wf.Steps = append(wf.Steps, WorkflowStep{
			Node:   "combine",
			Params: map[string]string{"format": "text"},
		})
	}

	return wf, nil
}

// SaveWorkflow saves a workflow to a YAML file
func SaveWorkflow(wf *Workflow, filename string) error {
	// Ensure .yaml extension
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}

	// Generate YAML content
	content := wf.ToYAML()

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetSuggestedFilename returns a suggested filename based on the description
func GetSuggestedFilename(description string) string {
	desc := strings.ToLower(description)

	// Extract key words
	words := strings.Fields(desc)
	var keywords []string
	for _, word := range words {
		// Skip common words
		if strings.Contains("the a an to and or fetch save write run from", word) {
			continue
		}
		// Clean word - keep alphanumeric, dots, hyphens, underscores
		word = regexp.MustCompile(`[^a-z0-9._-]`).ReplaceAllString(word, "")
		if len(word) > 2 {
			keywords = append(keywords, word)
		}
	}

	// Take first 3 keywords
	if len(keywords) > 3 {
		keywords = keywords[:3]
	}

	filename := strings.Join(keywords, "_")
	if filename == "" {
		filename = "workflow"
	}

	return filename + ".yaml"
}

func generateWorkflowName(description string) string {
	desc := strings.ToLower(description)

	words := strings.Fields(desc)
	var nameParts []string
	for _, word := range words {
		// Remove all non-alphanumeric characters except spaces and dots
		word = regexp.MustCompile(`[^a-z0-9 .]`).ReplaceAllString(word, "")
		if len(word) > 3 && !strings.Contains("the a an to and or fetch save write run from with", word) {
			// Simple title case: capitalize first letter
			if len(word) > 0 {
				word = strings.ToUpper(word[:1]) + word[1:]
			}
			nameParts = append(nameParts, word)
		}
		if len(nameParts) >= 3 {
			break
		}
	}

	if len(nameParts) == 0 {
		return "Custom Workflow"
	}

	return strings.Join(nameParts, " ")
}

// ToYAML converts the workflow to YAML string using the standard yaml library
// which properly handles all special characters including newlines, tabs, and quotes
func (wf *Workflow) ToYAML() string {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Sprintf("# Error: failed to marshal workflow: %v\n", err)
	}
	return string(data)
}

// GetWorkflowFilename returns the filename for a workflow
func GetWorkflowFilename(wf *Workflow) string {
	name := strings.ToLower(wf.Name)
	name = strings.ReplaceAll(name, " ", "_")
	name = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(name, "")
	if name == "" {
		name = "workflow"
	}
	return name + ".yaml"
}

// ValidateWorkflow validates a workflow and returns suggestions
func ValidateWorkflow(wf *Workflow) []string {
	var suggestions []string

	if wf.Name == "" {
		suggestions = append(suggestions, "Consider adding a workflow name")
	}

	if len(wf.Steps) == 0 {
		suggestions = append(suggestions, "Workflow has no steps")
	}

	// Check for common patterns
	hasOutput := false
	for _, step := range wf.Steps {
		if step.Node == "file_write" {
			hasOutput = true
		}
	}

	if !hasOutput && len(wf.Steps) > 0 {
		suggestions = append(suggestions, "Consider adding a file_write step to save output")
	}

	return suggestions
}

// CreateWorkflowFromDescription creates and saves a workflow from description
func CreateWorkflowFromDescription(description string) (string, error) {
	wf, err := GenerateWorkflow(description)
	if err != nil {
		return "", err
	}

	filename := GetSuggestedFilename(description)
	if err := SaveWorkflow(wf, filename); err != nil {
		return "", err
	}

	return filepath.Join(".", filename), nil
}
