package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GenerateWorkflow creates a workflow from a natural language description
func GenerateWorkflow(description string) (*Workflow, error) {
	desc := strings.ToLower(description)
	wf := &Workflow{}

	useDeepSeek := strings.Contains(desc, "deepseek")
	var llmNode string
	var llmModel string
	if useDeepSeek {
		llmNode = "deepseek"
		llmModel = "deepseek-chat"
	} else {
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
		step := Step{Node: "fetch_url", Params: map[string]string{"url": urlMatch}}
		wf.Steps = append(wf.Steps, step)
	}

	// Try to extract file path
	fileRegex := regexp.MustCompile(`(save|write|to)\s+([^\s]+\.(txt|md|yaml|json|html|csv|xml))`)
	fileMatch := fileRegex.FindStringSubmatch(desc)
	if len(fileMatch) >= 3 {
		path := fileMatch[2]
		step := Step{Node: "file_write", Params: map[string]string{"path": path}}
		wf.Steps = append(wf.Steps, step)
	}

	// Check for common patterns
	if strings.Contains(desc, "github") {
		if urlMatch == "" {
			step := Step{Node: "fetch_url", Params: map[string]string{"url": "https://github.com/"}}
			wf.Steps = append(wf.Steps, step)
		}
	}

	if strings.Contains(desc, "summarize") || strings.Contains(desc, "总结") {
		step := Step{
			Node:   llmNode,
			Params: map[string]string{"model": llmModel, "system": "You are a helpful assistant that summarizes text concisely."},
		}
		// Insert before file_write if exists
		if len(wf.Steps) > 0 && wf.Steps[len(wf.Steps)-1].Node == "file_write" {
			lastStep := wf.Steps[len(wf.Steps)-1]
			steps := make([]Step, len(wf.Steps)-1)
			copy(steps, wf.Steps[:len(wf.Steps)-1])
			steps = append(steps, step)
			steps = append(steps, lastStep)
			wf.Steps = steps
		} else {
			wf.Steps = append(wf.Steps, step)
		}
	}

	if strings.Contains(desc, "translate") || strings.Contains(desc, "翻译") {
		step := Step{
			Node:   llmNode,
			Params: map[string]string{"model": llmModel, "system": "You are a translator. Translate the following text to English."},
		}
		wf.Steps = append(wf.Steps, step)
	}

	if strings.Contains(desc, "git") || strings.Contains(desc, "commit") || strings.Contains(desc, "release") {
		step := Step{
			Node:   "execute",
			Params: map[string]string{"command": "git log --oneline -10"},
		}
		wf.Steps = append(wf.Steps, step)
	}

	if strings.Contains(desc, "log") || strings.Contains(desc, "monitor") {
		step := Step{
			Node:   "execute",
			Params: map[string]string{"command": "tail -n 100 /var/log/syslog"},
		}
		wf.Steps = append(wf.Steps, step)
	}

	// Generate workflow name
	wf.Name = generateWorkflowName(description)
	wf.Description = description

	// If no steps were generated, add a default execute step
	if len(wf.Steps) == 0 {
		wf.Steps = append(wf.Steps, Step{
			Node:   "execute",
			Params: map[string]string{"command": "echo 'Custom workflow: " + description + "'"},
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
		// Clean word
		word = strings.Trim(word, ".,!?\"'")
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
		word = strings.Trim(word, ".,!?\"'")
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

// ToYAML converts the workflow to YAML string
func (wf *Workflow) ToYAML() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("name: \"%s\"\n", wf.Name))
	if wf.Description != "" {
		sb.WriteString(fmt.Sprintf("description: \"%s\"\n", wf.Description))
	}
	sb.WriteString("\nsteps:\n")

	for _, step := range wf.Steps {
		sb.WriteString(fmt.Sprintf("  - node: %s\n", step.Node))
		if len(step.Params) > 0 {
			sb.WriteString("    params:\n")
			for key, value := range step.Params {
				// Escape quotes in strings
				escaped := strings.ReplaceAll(value, "\"", "\\\"")
				sb.WriteString(fmt.Sprintf("      %s: \"%s\"\n", key, escaped))
			}
		}
	}

	return sb.String()
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
