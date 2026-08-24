// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌​​‌​​​​‌‌​​‌​​​​​‌​​​‌​‌‌​​​‌‌‌‌‌​‌‌‌​​​‌‌​​​​​​​​​​​​​​​​​​‌​‌‌‌‌​‌​‌​​​‌​⁠
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

package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/watermark"
)

// SaveWorkflow saves a workflow to a YAML file with a provenance watermark.
func SaveWorkflow(wf *Workflow, filename string) error {
	// Ensure .yaml extension
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}

	// Sanitize filename to prevent path traversal
	cleanPath := filepath.Base(filename)
	if cleanPath == "." || cleanPath == "/" || cleanPath == string(filepath.Separator) {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	filename = cleanPath

	// Generate YAML content
	content := wf.ToYAML()

	// Prepend provenance watermark comment
	wm := watermark.EncodeYAML(content)
	if wm != "" {
		content = wm + "\n" + content
	}

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
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
		if slices.Contains([]string{"the", "a", "an", "to", "and", "or", "fetch", "save", "write", "run", "from"}, word) {
			continue
		}
		// Clean word - keep alphanumeric, dots, hyphens, underscores
		word = cleanCharRegex.ReplaceAllString(word, "")
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
		word = cleanNameRegex.ReplaceAllString(word, "")
		if len(word) > 3 && !slices.Contains([]string{"the", "a", "an", "to", "and", "or", "fetch", "save", "write", "run", "from", "with"}, word) {
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
	name = cleanFileRegex.ReplaceAllString(name, "")
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
	hasOutput := hasFileWriteStep(wf.Steps)

	if !hasOutput && len(wf.Steps) > 0 {
		suggestions = append(suggestions, "Consider adding a file_write step to save output")
	}

	return suggestions
}

// hasFileWriteStep reports whether any step uses the file_write node, recursing
// into compound steps (if/then/else, parallel, map, reduce, saga,
// capture_error, on_error) so that file_write steps nested inside branches are
// detected.
func hasFileWriteStep(steps []WorkflowStep) bool {
	for _, s := range steps {
		if s.Node == "file_write" {
			return true
		}
		if s.IsIf() && (hasFileWriteStep(s.If.Then) || hasFileWriteStep(s.If.Else)) {
			return true
		}
		if s.IsMap() && hasFileWriteStep(s.Map.Steps) {
			return true
		}
		if s.IsReduce() && hasFileWriteStep(s.Reduce.Steps) {
			return true
		}
		if s.IsSaga() {
			for _, sg := range s.Saga.Steps {
				if hasFileWriteStep([]WorkflowStep{sg.Forward}) {
					return true
				}
				if sg.Compensate != nil && hasFileWriteStep([]WorkflowStep{*sg.Compensate}) {
					return true
				}
			}
		}
		if s.HasCaptureError() && hasFileWriteStep(s.CaptureError) {
			return true
		}
		if s.IsParallel() {
			for _, p := range s.Parallel {
				if p.Node == "file_write" {
					return true
				}
			}
		}
		if s.OnError != nil && s.OnError.Node == "file_write" {
			return true
		}
	}
	return false
}
