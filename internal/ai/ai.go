// Copyright (c) 2026 llm-box Contributors
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

package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/workflow"
	"gopkg.in/yaml.v3"
)

// Severity levels for optimization suggestions.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Suggestion represents a single optimization suggestion.
type Suggestion struct {
	Severity  Severity `json:"severity"`
	Message   string   `json:"message"`
	StepIndex int      `json:"step_index,omitempty"`
	Fix       string   `json:"fix,omitempty"`
}

// OptimizationReport holds the overall optimization score and suggestions.
type OptimizationReport struct {
	Score       int          `json:"score"`
	Suggestions []Suggestion `json:"suggestions"`
}

// ---------------------------------------------------------------------------
// WorkflowOptimizer
// ---------------------------------------------------------------------------

// WorkflowOptimizer analyzes a workflow YAML and provides optimization suggestions.
type WorkflowOptimizer struct{}

// NewWorkflowOptimizer creates a new WorkflowOptimizer.
func NewWorkflowOptimizer() *WorkflowOptimizer {
	return &WorkflowOptimizer{}
}

// Analyze parses the workflow YAML and runs a set of rule-based checks.
func (o *WorkflowOptimizer) Analyze(workflowYAML string) OptimizationReport {
	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(workflowYAML), &wf); err != nil {
		return OptimizationReport{
			Score: 0,
			Suggestions: []Suggestion{
				{Severity: SeverityError, Message: "Invalid workflow YAML: " + err.Error()},
			},
		}
	}

	report := OptimizationReport{Score: 100, Suggestions: []Suggestion{}}

	checks := []func(*workflow.Workflow, *OptimizationReport){
		checkUnusedOutputs,
		checkDuplicateNodes,
		checkComplexConditions,
		checkParallelization,
		checkTimeoutConfig,
		checkHardcodedSecrets,
		checkRetryPolicy,
		checkBasicStructure,
	}

	for _, chk := range checks {
		chk(&wf, &report)
	}

	// Clamp score
	if report.Score < 0 {
		report.Score = 0
	}
	if report.Score > 100 {
		report.Score = 100
	}
	return report
}

func addSuggestion(r *OptimizationReport, s Suggestion) {
	r.Suggestions = append(r.Suggestions, s)
	switch s.Severity {
	case SeverityError:
		r.Score -= 15
	case SeverityWarning:
		r.Score -= 8
	case SeverityInfo:
		r.Score -= 2
	}
}

// checkBasicStructure validates name and steps presence.
func checkBasicStructure(wf *workflow.Workflow, r *OptimizationReport) {
	if wf.Name == "" {
		addSuggestion(r, Suggestion{Severity: SeverityWarning, Message: "Workflow has no name"})
	}
	if len(wf.Steps) == 0 {
		addSuggestion(r, Suggestion{Severity: SeverityError, Message: "Workflow has no steps"})
	}
}

// nodes that typically produce output consumed by later steps.
var outputProducingNodes = map[string]bool{
	"fetch_url":       true,
	"file_read":       true,
	"execute":         true,
	"http_request":    true,
	"json_parse":      true,
	"transform":       true,
	"combine":         true,
	"template_render": true,
}

// checkUnusedOutputs looks for steps whose results are never referenced.
func checkUnusedOutputs(wf *workflow.Workflow, r *OptimizationReport) {
	if len(wf.Steps) == 0 {
		return
	}

	// Gather all references in params / conditions / output / parallel steps.
	allRefs := gatherAllReferences(wf)

	for i, step := range wf.Steps {
		if !outputProducingNodes[step.Node] {
			continue
		}
		used := false
		// Check if any later step or workflow output references this step.
		for j := i + 1; j < len(wf.Steps); j++ {
			if stepRefs(wf.Steps[j], i, step.Name) {
				used = true
				break
			}
		}
		if !used {
			if wf.Output != "" && (strings.Contains(wf.Output, fmt.Sprintf("step.%d", i)) ||
				(step.Name != "" && strings.Contains(wf.Output, step.Name))) {
				used = true
			}
		}
		// Also check global refs (e.g. vars)
		if !used && step.Name != "" {
			if allRefs[step.Name] {
				used = true
			}
		}
		if !used {
			addSuggestion(r, Suggestion{
				Severity:  SeverityInfo,
				Message:   fmt.Sprintf("Step %d (%s) output may be unused by subsequent steps", i, step.Node),
				StepIndex: i,
			})
		}
	}
}

func gatherAllReferences(wf *workflow.Workflow) map[string]bool {
	refs := make(map[string]bool)
	for _, step := range wf.Steps {
		gatherStepRefs(&step, refs)
	}
	return refs
}

func gatherStepRefs(step *workflow.WorkflowStep, refs map[string]bool) {
	for _, v := range step.Params {
		for _, m := range templateVarRegex.FindAllString(v, -1) {
			refs[strings.Trim(m, "{}")] = true
		}
	}
	if step.Condition != "" {
		for _, m := range templateVarRegex.FindAllString(step.Condition, -1) {
			refs[strings.Trim(m, "{}")] = true
		}
	}
	for _, p := range step.Parallel {
		for _, v := range p.Params {
			for _, m := range templateVarRegex.FindAllString(v, -1) {
				refs[strings.Trim(m, "{}")] = true
			}
		}
		if p.Condition != "" {
			for _, m := range templateVarRegex.FindAllString(p.Condition, -1) {
				refs[strings.Trim(m, "{}")] = true
			}
		}
	}
	if step.If != nil {
		for _, s := range step.If.Then {
			gatherStepRefs(&s, refs)
		}
		for _, s := range step.If.Else {
			gatherStepRefs(&s, refs)
		}
	}
}

var templateVarRegex = regexp.MustCompile(`\{\{[^}]+\}\}`)

func stepRefs(step workflow.WorkflowStep, idx int, name string) bool {
	check := func(text string) bool {
		if strings.Contains(text, fmt.Sprintf("step.%d", idx)) {
			return true
		}
		if name != "" && strings.Contains(text, name) {
			return true
		}
		return false
	}
	for _, v := range step.Params {
		if check(v) {
			return true
		}
	}
	if check(step.Condition) {
		return true
	}
	for _, p := range step.Parallel {
		for _, v := range p.Params {
			if check(v) {
				return true
			}
		}
		if check(p.Condition) {
			return true
		}
	}
	if step.If != nil {
		for _, s := range step.If.Then {
			if stepRefs(s, idx, name) {
				return true
			}
		}
		for _, s := range step.If.Else {
			if stepRefs(s, idx, name) {
				return true
			}
		}
	}
	return false
}

// checkDuplicateNodes detects repeated identical node invocations.
func checkDuplicateNodes(wf *workflow.Workflow, r *OptimizationReport) {
	seen := make(map[string]int)
	for i, step := range wf.Steps {
		if step.Node == "" {
			continue
		}
		key := step.Node + "|" + mapToSortedString(step.Params)
		if prev, ok := seen[key]; ok {
			addSuggestion(r, Suggestion{
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("Step %d duplicates step %d (%s with identical parameters)", i, prev, step.Node),
				StepIndex: i,
				Fix:       fmt.Sprintf("Remove step %d or merge with step %d", i, prev),
			})
		} else {
			seen[key] = i
		}
	}
}

func mapToSortedString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, k+"="+v)
	}
	// Not strictly sorted, but stable enough for dup detection within one run.
	return strings.Join(pairs, ",")
}

// checkComplexConditions flags overly complex condition expressions.
func checkComplexConditions(wf *workflow.Workflow, r *OptimizationReport) {
	for i, step := range wf.Steps {
		cond := step.Condition
		if cond == "" && step.If != nil {
			cond = step.If.Condition
		}
		if cond == "" {
			continue
		}
		logicOps := strings.Count(cond, "&&") + strings.Count(cond, "||") + strings.Count(cond, "!")
		if logicOps > 3 || len(cond) > 80 {
			addSuggestion(r, Suggestion{
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("Step %d has a complex condition expression; consider simplifying or splitting", i),
				StepIndex: i,
				Fix:       "Break the condition into smaller sub-conditions or use named intermediate variables.",
			})
		}
	}
}

// checkParallelization suggests adjacent steps that could run in parallel.
func checkParallelization(wf *workflow.Workflow, r *OptimizationReport) {
	if len(wf.Steps) < 2 {
		return
	}
	for i := 0; i < len(wf.Steps)-1; i++ {
		curr := wf.Steps[i]
		next := wf.Steps[i+1]
		// Only flag pairs that are both independent data-fetching steps.
		if !isIndependentFetchNode(curr.Node) || !isIndependentFetchNode(next.Node) {
			continue
		}
		// If next step references current step, they are dependent.
		if stepRefs(next, i, curr.Name) {
			continue
		}
		addSuggestion(r, Suggestion{
			Severity:  SeverityInfo,
			Message:   fmt.Sprintf("Steps %d and %d appear independent and could be parallelized", i, i+1),
			StepIndex: i,
			Fix:       fmt.Sprintf("Wrap steps %d and %d under a 'parallel' block.", i, i+1),
		})
	}
}

func isIndependentFetchNode(node string) bool {
	switch node {
	case "fetch_url", "http_request", "file_read", "execute":
		return true
	}
	return false
}

// checkTimeoutConfig validates per-step timeout settings.
func checkTimeoutConfig(wf *workflow.Workflow, r *OptimizationReport) {
	for i, step := range wf.Steps {
		timeoutStr, ok := step.Params["_timeout"]
		if !ok {
			// Network/IO-heavy nodes benefit from a timeout.
			if isNetworkNode(step.Node) {
				addSuggestion(r, Suggestion{
					Severity:  SeverityInfo,
					Message:   fmt.Sprintf("Step %d (%s) does not have a timeout configured", i, step.Node),
					StepIndex: i,
					Fix:       "Add _timeout parameter (e.g. 30s).",
				})
			}
			continue
		}
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			addSuggestion(r, Suggestion{
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("Step %d has an invalid timeout value: %s", i, timeoutStr),
				StepIndex: i,
			})
			continue
		}
		if d < time.Second {
			addSuggestion(r, Suggestion{
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("Step %d timeout (%s) is very short; may cause premature failures", i, timeoutStr),
				StepIndex: i,
			})
		}
		if d > 30*time.Minute {
			addSuggestion(r, Suggestion{
				Severity:  SeverityInfo,
				Message:   fmt.Sprintf("Step %d timeout (%s) is very long; consider reducing it", i, timeoutStr),
				StepIndex: i,
			})
		}
	}
}

func isNetworkNode(node string) bool {
	switch node {
	case "fetch_url", "http_request", "openai", "deepseek", "qwen", "kimi", "glm",
		"mistral", "baichuan", "internlm", "yi", "xverse", "minimax", "mimo", "ima", "coze", "ollama":
		return true
	}
	return false
}

var secretKeyPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret|credential|auth)`)
var secretValuePattern = regexp.MustCompile(`(?i)^(sk-[a-zA-Z0-9]{20,}|Bearer\s+[a-zA-Z0-9_-]{10,}|ghp_[a-zA-Z0-9]{30,}|AK[0-9A-Z]{16,})$`)

// checkHardcodedSecrets detects sensitive values in step parameters.
func checkHardcodedSecrets(wf *workflow.Workflow, r *OptimizationReport) {
	for i, step := range wf.Steps {
		for k, v := range step.Params {
			if secretKeyPattern.MatchString(k) {
				addSuggestion(r, Suggestion{
					Severity:  SeverityError,
					Message:   fmt.Sprintf("Step %d appears to hardcode a secret in parameter '%s'", i, k),
					StepIndex: i,
					Fix:       "Use environment variables or the secrets manager instead of hardcoding secrets.",
				})
			}
			if secretValuePattern.MatchString(v) {
				addSuggestion(r, Suggestion{
					Severity:  SeverityError,
					Message:   fmt.Sprintf("Step %d parameter '%s' looks like a hardcoded secret/token", i, k),
					StepIndex: i,
					Fix:       "Move the secret to environment variables or a secrets vault.",
				})
			}
		}
	}
}

// checkRetryPolicy ensures retry is configured for external service calls.
func checkRetryPolicy(wf *workflow.Workflow, r *OptimizationReport) {
	for i, step := range wf.Steps {
		if !isNetworkNode(step.Node) {
			continue
		}
		if step.Retry <= 0 {
			addSuggestion(r, Suggestion{
				Severity:  SeverityInfo,
				Message:   fmt.Sprintf("Step %d (%s) does not have a retry policy configured", i, step.Node),
				StepIndex: i,
				Fix:       "Add 'retry: 3' and optionally a 'delay' to handle transient failures.",
			})
		}
	}
}

// ---------------------------------------------------------------------------
// WorkflowExplainer
// ---------------------------------------------------------------------------

// WorkflowExplainer generates human-readable descriptions of workflows.
type WorkflowExplainer struct{}

// NewWorkflowExplainer creates a new WorkflowExplainer.
func NewWorkflowExplainer() *WorkflowExplainer {
	return &WorkflowExplainer{}
}

// Explain returns a natural-language summary of the workflow.
func (e *WorkflowExplainer) Explain(workflowYAML string) string {
	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(workflowYAML), &wf); err != nil {
		return "Unable to explain workflow: invalid YAML."
	}

	if len(wf.Steps) == 0 {
		return "This workflow has no steps."
	}

	var parts []string
	if wf.Name != "" {
		parts = append(parts, fmt.Sprintf("The workflow '%s'", wf.Name))
	} else {
		parts = append(parts, "This workflow")
	}
	parts = append(parts, "performs the following actions:")

	for i, step := range wf.Steps {
		desc := describeStep(i, step)
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, desc))
	}

	if wf.Output != "" {
		parts = append(parts, fmt.Sprintf("Finally, it produces output using expression: %s.", wf.Output))
	}

	return strings.Join(parts, "\n")
}

func describeStep(idx int, step workflow.WorkflowStep) string {
	node := step.Node

	switch node {
	case "fetch_url":
		url := step.Params["url"]
		if url != "" {
			return fmt.Sprintf("Fetch content from %s", url)
		}
		return "Fetch content from a URL"
	case "file_read":
		path := step.Params["path"]
		if path != "" {
			return fmt.Sprintf("Read the file '%s'", path)
		}
		return "Read a file"
	case "file_write":
		path := step.Params["path"]
		if path != "" {
			return fmt.Sprintf("Write the result to '%s'", path)
		}
		return "Write output to a file"
	case "json_parse":
		return "Parse JSON data"
	case "execute":
		cmd := step.Params["command"]
		if cmd != "" {
			return fmt.Sprintf("Execute command: %s", cmd)
		}
		return "Execute a shell command"
	case "http_request":
		method := step.Params["method"]
		if method == "" {
			method = "GET"
		}
		url := step.Params["url"]
		if url != "" {
			return fmt.Sprintf("Send an HTTP %s request to %s", method, url)
		}
		return fmt.Sprintf("Send an HTTP %s request", method)
	case "condition":
		return "Evaluate a conditional expression"
	case "transform":
		return "Transform data format"
	case "combine":
		return "Combine multiple inputs"
	case "template_render":
		return "Render a template"
	case "notify":
		return "Send a notification"
	default:
		if isLLMNode(node) {
			action := inferLLMAction(step.Params)
			return fmt.Sprintf("Call %s to %s", node, action)
		}
		return fmt.Sprintf("Run '%s'", node)
	}
}

func isLLMNode(node string) bool {
	switch node {
	case "openai", "deepseek", "qwen", "kimi", "glm", "mistral", "baichuan",
		"internlm", "yi", "xverse", "minimax", "mimo", "ima", "coze", "ollama":
		return true
	}
	return false
}

func inferLLMAction(params map[string]string) string {
	system := strings.ToLower(params["system"])
	prompt := strings.ToLower(params["prompt"])
	combined := system + " " + prompt

	if strings.Contains(combined, "summar") {
		return "summarize content"
	}
	if strings.Contains(combined, "translat") {
		return "translate text"
	}
	if strings.Contains(combined, "explain") || strings.Contains(combined, "analy") {
		return "explain or analyze content"
	}
	if strings.Contains(combined, "code") || strings.Contains(combined, "program") {
		return "generate code"
	}
	if strings.Contains(combined, "email") || strings.Contains(combined, "draft") {
		return "draft an email"
	}
	if strings.Contains(combined, "report") {
		return "generate a report"
	}
	if strings.Contains(combined, "doc") {
		return "create documentation"
	}
	if strings.Contains(combined, "test") {
		return "write test cases"
	}
	if strings.Contains(combined, "rewrite") || strings.Contains(combined, "refactor") {
		return "rewrite or refactor text"
	}
	return "process content"
}

// ---------------------------------------------------------------------------
// WorkflowCompleter
// ---------------------------------------------------------------------------

// WorkflowCompleter suggests the next step(s) for a partial workflow.
type WorkflowCompleter struct {
	patterns []workflowPattern
}

type workflowPattern struct {
	name   string
	steps  []string // node names in order
	params []map[string]string
}

// NewWorkflowCompleter creates a new WorkflowCompleter with built-in patterns.
func NewWorkflowCompleter() *WorkflowCompleter {
	return &WorkflowCompleter{
		patterns: []workflowPattern{
			{
				name:  "fetch-process-save",
				steps: []string{"fetch_url", "openai", "file_write"},
				params: []map[string]string{
					{"url": "https://example.com"},
					{"model": "gpt-4", "system": "You are a helpful assistant."},
					{"path": "output.txt"},
				},
			},
			{
				name:  "read-process-save",
				steps: []string{"file_read", "openai", "file_write"},
				params: []map[string]string{
					{"path": "input.txt"},
					{"model": "gpt-4", "system": "You are a helpful assistant."},
					{"path": "output.txt"},
				},
			},
			{
				name:  "fetch-json-process-save",
				steps: []string{"fetch_url", "json_parse", "openai", "file_write"},
				params: []map[string]string{
					{"url": "https://api.example.com/data"},
					{},
					{"model": "gpt-4", "system": "You are a helpful assistant."},
					{"path": "output.txt"},
				},
			},
			{
				name:  "git-process-save",
				steps: []string{"execute", "openai", "file_write"},
				params: []map[string]string{
					{"command": "git log --oneline -10"},
					{"model": "gpt-4", "system": "Summarize the following git log."},
					{"path": "summary.txt"},
				},
			},
			{
				name:  "http-json-save",
				steps: []string{"http_request", "json_parse", "file_write"},
				params: []map[string]string{
					{"method": "GET", "url": "https://api.example.com/data"},
					{},
					{"path": "data.json"},
				},
			},
			{
				name:  "template-execute-notify",
				steps: []string{"template_render", "execute", "notify"},
				params: []map[string]string{
					{"template": "Hello {{name}}"},
					{"command": "echo 'done'"},
					{"message": "Workflow completed."},
				},
			},
			{
				name:  "fetch-summarize-notify",
				steps: []string{"fetch_url", "openai", "notify"},
				params: []map[string]string{
					{"url": "https://example.com"},
					{"model": "gpt-4", "system": "Summarize the content."},
					{"message": "Summary ready."},
				},
			},
		},
	}
}

// Complete returns a completed workflow YAML based on the partial input.
func (c *WorkflowCompleter) Complete(partialYAML string) string {
	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(partialYAML), &wf); err != nil {
		return partialYAML
	}

	if len(wf.Steps) == 0 {
		return partialYAML
	}

	// Try to match the existing step sequence against known patterns.
	bestPattern := c.findBestPattern(wf.Steps)
	if bestPattern == nil {
		// No pattern matched; add a generic completion.
		wf.Steps = append(wf.Steps, workflow.WorkflowStep{
			Node:   "combine",
			Params: map[string]string{"format": "text"},
		})
		out, _ := yaml.Marshal(&wf)
		return string(out)
	}

	// Append remaining steps from the pattern.
	for i := len(wf.Steps); i < len(bestPattern.steps); i++ {
		wf.Steps = append(wf.Steps, workflow.WorkflowStep{
			Node:   bestPattern.steps[i],
			Params: copyMap(bestPattern.params[i]),
		})
	}

	out, _ := yaml.Marshal(&wf)
	return string(out)
}

func (c *WorkflowCompleter) findBestPattern(existing []workflow.WorkflowStep) *workflowPattern {
	best := -1
	var bestPat *workflowPattern
	for i := range c.patterns {
		pat := &c.patterns[i]
		matchLen := 0
		for j := 0; j < len(existing) && j < len(pat.steps); j++ {
			if existing[j].Node == pat.steps[j] {
				matchLen++
			} else {
				break
			}
		}
		// Require at least one matched step and that there are remaining steps.
		if matchLen > 0 && matchLen > best && matchLen < len(pat.steps) {
			best = matchLen
			bestPat = pat
		}
	}
	return bestPat
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// ---------------------------------------------------------------------------
// NaturalLanguageQuery
// ---------------------------------------------------------------------------

// NaturalLanguageQuery answers questions about a workflow in natural language.
type NaturalLanguageQuery struct{}

// NewNaturalLanguageQuery creates a new NaturalLanguageQuery.
func NewNaturalLanguageQuery() *NaturalLanguageQuery {
	return &NaturalLanguageQuery{}
}

// Query answers a question about the provided workflow YAML.
func (q *NaturalLanguageQuery) Query(workflowYAML string, question string) string {
	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(workflowYAML), &wf); err != nil {
		return "I cannot answer because the workflow YAML is invalid."
	}

	lq := strings.ToLower(question)

	// Node usage (must be checked before generic step count)
	if containsAnyKeyword(lq, []string{"how many steps use", "有多少步使用", "use openai", "使用 openai"}) {
		node := extractNodeName(lq)
		if node == "" {
			node = "openai"
		}
		count := countNodeUsage(&wf, node)
		return fmt.Sprintf("%d step(s) use the '%s' node.", count, node)
	}

	// Count steps
	if containsAnyKeyword(lq, []string{"how many steps", "有几个步骤", "步骤数量", "step count"}) {
		return fmt.Sprintf("This workflow has %d step(s).", len(wf.Steps))
	}

	// What nodes
	if containsAnyKeyword(lq, []string{"what nodes", "哪些节点", "什么节点", "node types"}) {
		return describeNodeTypes(&wf)
	}

	// Timeout
	if containsAnyKeyword(lq, []string{"timeout", "超时"}) {
		return describeTimeouts(&wf)
	}

	// Retry
	if containsAnyKeyword(lq, []string{"retry", "重试"}) {
		return describeRetries(&wf)
	}

	// Parallel
	if containsAnyKeyword(lq, []string{"parallel", "并行"}) {
		return describeParallel(&wf)
	}

	// Workflow name
	if containsAnyKeyword(lq, []string{"name", "名称"}) {
		if wf.Name != "" {
			return fmt.Sprintf("The workflow name is '%s'.", wf.Name)
		}
		return "The workflow does not have a name."
	}

	// Description
	if containsAnyKeyword(lq, []string{"description", "描述"}) {
		if wf.Description != "" {
			return wf.Description
		}
		return "The workflow does not have a description."
	}

	// Fallback
	return "I'm not sure how to answer that. Try asking about steps, nodes, timeouts, retries, or parallel execution."
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func extractNodeName(q string) string {
	for _, node := range []string{
		"openai", "deepseek", "qwen", "kimi", "glm", "mistral", "baichuan",
		"internlm", "yi", "xverse", "minimax", "mimo", "ima", "coze", "ollama",
		"fetch_url", "file_read", "file_write", "json_parse", "execute",
		"http_request", "condition", "transform", "combine", "template_render", "notify",
	} {
		if strings.Contains(q, node) {
			return node
		}
	}
	return ""
}

func countNodeUsage(wf *workflow.Workflow, node string) int {
	count := 0
	for _, step := range wf.Steps {
		if step.Node == node {
			count++
		}
	}
	return count
}

func describeNodeTypes(wf *workflow.Workflow) string {
	if len(wf.Steps) == 0 {
		return "This workflow has no steps."
	}
	var nodes []string
	seen := make(map[string]bool)
	for _, step := range wf.Steps {
		if !seen[step.Node] {
			seen[step.Node] = true
			nodes = append(nodes, step.Node)
		}
	}
	return fmt.Sprintf("The workflow uses the following node(s): %s.", strings.Join(nodes, ", "))
}

func describeTimeouts(wf *workflow.Workflow) string {
	var with []string
	var without []string
	for i, step := range wf.Steps {
		label := fmt.Sprintf("step %d (%s)", i, step.Node)
		if _, ok := step.Params["_timeout"]; ok {
			with = append(with, label)
		} else {
			without = append(without, label)
		}
	}
	var parts []string
	if len(with) > 0 {
		parts = append(parts, fmt.Sprintf("Steps with timeout: %s.", strings.Join(with, ", ")))
	}
	if len(without) > 0 {
		parts = append(parts, fmt.Sprintf("Steps without timeout: %s.", strings.Join(without, ", ")))
	}
	if len(parts) == 0 {
		return "No timeout information available."
	}
	return strings.Join(parts, " ")
}

func describeRetries(wf *workflow.Workflow) string {
	var with []string
	var without []string
	for i, step := range wf.Steps {
		label := fmt.Sprintf("step %d (%s)", i, step.Node)
		if step.Retry > 0 {
			with = append(with, fmt.Sprintf("%s [retry=%d]", label, step.Retry))
		} else {
			without = append(without, label)
		}
	}
	var parts []string
	if len(with) > 0 {
		parts = append(parts, fmt.Sprintf("Steps with retry: %s.", strings.Join(with, ", ")))
	}
	if len(without) > 0 {
		parts = append(parts, fmt.Sprintf("Steps without retry: %s.", strings.Join(without, ", ")))
	}
	if len(parts) == 0 {
		return "No retry information available."
	}
	return strings.Join(parts, " ")
}

func describeParallel(wf *workflow.Workflow) string {
	count := 0
	for _, step := range wf.Steps {
		if step.IsParallel() {
			count++
		}
	}
	if count == 0 {
		return "This workflow does not use parallel steps."
	}
	return fmt.Sprintf("This workflow has %d step(s) that contain parallel branches.", count)
}
