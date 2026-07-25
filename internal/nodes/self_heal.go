package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SelfHealNode struct{}

type HealCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Issue   string `json:"issue,omitempty"`
	Fixed   bool   `json:"fixed,omitempty"`
	Message string `json:"message,omitempty"`
}

type HealReport struct {
	Checks    []HealCheck `json:"checks"`
	Issues    int         `json:"issues_found"`
	Fixed     int         `json:"issues_fixed"`
	Remaining int         `json:"issues_remaining"`
	Duration  string      `json:"duration_ms"`
	Timestamp time.Time   `json:"timestamp"`
}

func init() {
	Register(&SelfHealNode{})
}

func (n *SelfHealNode) Name() string {
	return "self_heal"
}

func (n *SelfHealNode) Description() string {
	return "Self-diagnose and auto-fix common project issues (xiaobei inspired autonomous repair)"
}

func (n *SelfHealNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "self_heal",
		Description: "Self-diagnose and attempt automatic repair of project issues: build errors, formatting, missing deps, test failures, version mismatches. Runs gofmt/go vet/go build/go test and auto-fixes where possible (xiaobei inspired autonomous repair mechanism).",
		Input:       "string - optional: specific area to check (build|format|deps|test|all)",
		Output:      "string - JSON or formatted heal report",
		Params: []ParamSchema{
			{Name: "auto_fix", Type: "string", Description: "true|false, attempt automatic fixes (default: true)", Required: false, Default: "true"},
			{Name: "output", Type: "string", Description: "json|markdown|text (default: markdown)", Required: false, Default: "markdown"},
		},
	}
}

func (n *SelfHealNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	startTime := time.Now()
	area := strings.TrimSpace(strings.ToLower(input))
	if area == "" {
		area = "all"
	}
	autoFix := getParam(params, "auto_fix", "true") == "true"
	outputFmt := getParam(params, "output", "markdown")

	var checks []HealCheck

	if area == "all" || area == "format" {
		checks = append(checks, checkGofmt(autoFix))
	}
	if area == "all" || area == "build" {
		checks = append(checks, checkGoVet(autoFix))
		checks = append(checks, checkGoBuild(autoFix))
	}
	if area == "all" || area == "deps" {
		checks = append(checks, checkGoModTidy(autoFix))
	}
	if area == "all" || area == "test" {
		checks = append(checks, checkGoTest(autoFix))
	}

	issues := 0
	fixed := 0
	remaining := 0
	for _, c := range checks {
		if c.Status == "issue" {
			issues++
			if c.Fixed {
				fixed++
			} else {
				remaining++
			}
		}
	}

	report := HealReport{
		Checks:    checks,
		Issues:    issues,
		Fixed:     fixed,
		Remaining: remaining,
		Duration:  fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
		Timestamp: time.Now(),
	}

	switch outputFmt {
	case "json":
		data, _ := json.MarshalIndent(report, "", "  ")
		return string(data), nil
	case "text":
		return formatHealText(report), nil
	default:
		return formatHealMarkdown(report), nil
	}
}

func runCmd(cmdStr string) (string, int, error) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", -1, fmt.Errorf("empty command")
	}
	return runCmdArgs(parts[0], parts[1:]...)
}

func runCmdArgs(name string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(out), exitCode, err
}

func checkGofmt(autoFix bool) HealCheck {
	c := HealCheck{Name: "gofmt", Status: "ok"}
	out, _, _ := runCmd("gofmt -l .")
	unformatted := []string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasSuffix(line, ".go") {
			unformatted = append(unformatted, line)
		}
	}
	if len(unformatted) > 0 {
		c.Status = "issue"
		c.Issue = fmt.Sprintf("%d files not formatted: %s", len(unformatted), strings.Join(unformatted, ", "))
		if autoFix {
			for _, f := range unformatted {
				runCmdArgs("gofmt", "-w", f)
			}
			c.Fixed = true
			c.Message = fmt.Sprintf("Auto-formatted %d files", len(unformatted))
		}
	}
	return c
}

func checkGoVet(autoFix bool) HealCheck {
	c := HealCheck{Name: "go vet", Status: "ok"}
	out, code, _ := runCmd("go vet ./...")
	if code != 0 || strings.Contains(out, "warning") || strings.Contains(out, "error") {
		c.Status = "issue"
		c.Issue = truncate(out, 300)
		if autoFix {
			c.Message = "go vet issues require manual review"
		}
	}
	return c
}

func checkGoBuild(autoFix bool) HealCheck {
	c := HealCheck{Name: "go build", Status: "ok"}
	out, code, _ := runCmd("go build ./...")
	if code != 0 {
		c.Status = "issue"
		c.Issue = truncate(out, 300)
		if autoFix {
			runCmd("go mod tidy")
			out2, code2, _ := runCmd("go build ./...")
			if code2 == 0 {
				c.Fixed = true
				c.Message = "Fixed by running go mod tidy"
			} else {
				c.Message = "Build errors require manual fix: " + truncate(out2, 200)
			}
		}
	}
	return c
}

func checkGoModTidy(autoFix bool) HealCheck {
	c := HealCheck{Name: "go mod tidy", Status: "ok"}
	_, code, _ := runCmd("go mod tidy")
	if code != 0 {
		c.Status = "issue"
		c.Issue = "go mod tidy failed"
		if autoFix {
			c.Message = "Dependency issues require manual review"
		}
	}
	return c
}

func checkGoTest(autoFix bool) HealCheck {
	c := HealCheck{Name: "go test", Status: "ok"}
	out, code, _ := runCmd("go test ./... -short -count=1")
	if code != 0 {
		c.Status = "issue"
		c.Issue = truncate(out, 300)
		if autoFix {
			c.Message = "Test failures require manual review"
		}
	}
	return c
}

func formatHealMarkdown(r HealReport) string {
	var sb strings.Builder
	statusIcon := "✅"
	if r.Remaining > 0 {
		statusIcon = "⚠️"
	}
	sb.WriteString(fmt.Sprintf("%s **Self-Heal Report** (%s)\n\n", statusIcon, r.Duration))
	sb.WriteString(fmt.Sprintf("- Issues found: **%d**\n", r.Issues))
	sb.WriteString(fmt.Sprintf("- Auto-fixed: **%d**\n", r.Fixed))
	sb.WriteString(fmt.Sprintf("- Remaining: **%d**\n\n", r.Remaining))
	sb.WriteString("| Check | Status | Issue | Fixed |\n")
	sb.WriteString("|-------|--------|-------|-------|\n")
	for _, c := range r.Checks {
		icon := "✅"
		if c.Status == "issue" {
			if c.Fixed {
				icon = "🔧"
			} else {
				icon = "❌"
			}
		}
		issue := c.Issue
		if c.Message != "" {
			issue = c.Message
		}
		sb.WriteString(fmt.Sprintf("| %s %s | %s | %s | %v |\n",
			icon, c.Name, c.Status, truncate(issue, 60), c.Fixed))
	}
	return sb.String()
}

func formatHealText(r HealReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Self-Heal Report: %d issues, %d fixed, %d remaining (%s)\n\n",
		r.Issues, r.Fixed, r.Remaining, r.Duration))
	for _, c := range r.Checks {
		icon := "OK"
		if c.Status == "issue" {
			if c.Fixed {
				icon = "FIXED"
			} else {
				icon = "FAIL"
			}
		}
		sb.WriteString(fmt.Sprintf("[%s] %s", icon, c.Name))
		if c.Issue != "" {
			sb.WriteString(fmt.Sprintf(": %s", truncate(c.Issue, 100)))
		}
		if c.Message != "" {
			sb.WriteString(fmt.Sprintf(" -> %s", c.Message))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
