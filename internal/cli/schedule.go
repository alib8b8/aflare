// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​‌‌​‌‌​‌‌‌​‌​​‌‌​​​‌​​‌​‌​‌‌​​‌​​​‌​​​‌‌​​‌‌​​​​​​​​​​​​​​​​​​‌‌​​‌‌​‌​​​​​‌⁠
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
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/scheduler"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleSchedule handles the "schedule" command.
func HandleSchedule(args []string) error {
	if len(args) == 0 {
		PrintScheduleUsage()
		return exitErr(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "add":
		if err := HandleScheduleAdd(args[1:]); err != nil {
			return err
		}
	case "list":
		if err := HandleScheduleList(); err != nil {
			return err
		}
	case "remove":
		if err := HandleScheduleRemove(args[1:]); err != nil {
			return err
		}
	case "start":
		if err := HandleScheduleStart(); err != nil {
			return err
		}
	case "-h", "--help", "help":
		PrintScheduleUsage()
	default:
		fmt.Printf("Unknown schedule subcommand: %s\n\n", subCmd)
		PrintScheduleUsage()
		return exitErr(1)
	}
	return nil
}

// HandleScheduleAdd handles the "schedule add" subcommand.
// Supports both workflow-based and description-based tasks:
//
//	aflare schedule add --cron "0 9 * * *" my-workflow.yaml
//	aflare schedule add --cron "0 9 * * *" --desc "Check git repo status"
//	aflare schedule add --add "每天9点检查git仓库状态"  (auto-parse cron)
func HandleScheduleAdd(args []string) error {
	var cronExpr, taskID, wfPath, desc string
	var autoParse string

	if err := parseScheduleAddArgs(args, &cronExpr, &taskID, &wfPath, &desc, &autoParse); err != nil {
		return err
	}

	// ── Auto-parse natural language ─────────────────────────────────────
	if autoParse != "" {
		parsedCron, parsedDesc := parseNaturalSchedule(autoParse)
		if parsedCron == "" {
			fmt.Printf("❌ Could not parse schedule from: %s\n", autoParse)
			fmt.Println("   Please use --cron with an explicit cron expression.")
			fmt.Println("   Examples: '0 9 * * *' (daily 9:00), '0 */2 * * *' (every 2 hours)")
			return exitErr(1)
		}
		cronExpr = parsedCron
		if desc == "" {
			desc = parsedDesc
		}
	}

	if cronExpr == "" {
		fmt.Println("❌ --cron is required (or use --add for natural language)")
		PrintScheduleUsage()
		return exitErr(1)
	}

	// At least one of: workflow file or description
	if wfPath == "" && desc == "" {
		fmt.Println("❌ Either a workflow file or --desc is required")
		PrintScheduleUsage()
		return exitErr(1)
	}

	// Validate workflow file if provided
	if wfPath != "" {
		absPath, err := filepath.Abs(wfPath)
		if err != nil {
			fmt.Printf("❌ Failed to resolve workflow path: %v\n", err)
			return exitErr(1)
		}
		if _, err := os.Stat(absPath); err != nil {
			fmt.Printf("❌ Workflow file not found: %s\n", absPath)
			return exitErr(1)
		}
		wfPath = absPath
	}

	// Default task ID
	if taskID == "" {
		if wfPath != "" {
			base := filepath.Base(wfPath)
			taskID = strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			// Generate ID from description
			taskID = generateTaskID(desc)
		}
	}

	// Validate the cron expression using a throwaway scheduler.
	validateSched := scheduler.New()
	if err := validateSched.AddTask(taskID, cronExpr, func(context.Context) {}); err != nil {
		fmt.Printf("❌ Invalid cron expression: %v\n", err)
		return exitErr(1)
	}

	// Load existing schedules and check for duplicate ID.
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		return exitErr(1)
	}
	for _, e := range entries {
		if e.ID == taskID {
			fmt.Printf("❌ Task with id %q already exists (use --id to specify a different id)\n", taskID)
			return exitErr(1)
		}
	}

	entries = append(entries, scheduler.ScheduleEntry{
		ID:           taskID,
		Cron:         cronExpr,
		WorkflowPath: wfPath,
		Description:  desc,
	})
	if err := scheduler.SaveSchedules(path, entries); err != nil {
		fmt.Printf("❌ Failed to save schedule: %v\n", err)
		return exitErr(1)
	}

	fmt.Printf("✅ Scheduled task %q added\n", taskID)
	fmt.Printf("   Cron:     %s\n", cronExpr)
	if desc != "" {
		fmt.Printf("   Task:     %s\n", desc)
	}
	if wfPath != "" {
		fmt.Printf("   Workflow: %s\n", wfPath)
	}
	return nil
}

// parseScheduleAddArgs parses the command-line arguments for schedule add.
func parseScheduleAddArgs(args []string, cronExpr, taskID, wfPath, desc, autoParse *string) error {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cron":
			if i+1 >= len(args) {
				fmt.Println("❌ --cron requires a value")
				return exitErr(1)
			}
			*cronExpr = args[i+1]
			i++
		case "--desc":
			if i+1 >= len(args) {
				fmt.Println("❌ --desc requires a value")
				return exitErr(1)
			}
			*desc = args[i+1]
			i++
		case "--id":
			if i+1 >= len(args) {
				fmt.Println("❌ --id requires a value")
				return exitErr(1)
			}
			*taskID = args[i+1]
			i++
		case "--add":
			if i+1 >= len(args) {
				fmt.Println("❌ --add requires a value")
				return exitErr(1)
			}
			*autoParse = args[i+1]
			i++
		case "--help", "-h":
			PrintScheduleUsage()
			return nil
		default:
			switch {
			case strings.HasPrefix(args[i], "--cron="):
				*cronExpr = strings.TrimPrefix(args[i], "--cron=")
			case strings.HasPrefix(args[i], "--id="):
				*taskID = strings.TrimPrefix(args[i], "--id=")
			case strings.HasPrefix(args[i], "--desc="):
				*desc = strings.TrimPrefix(args[i], "--desc=")
			case strings.HasPrefix(args[i], "--add="):
				*autoParse = strings.TrimPrefix(args[i], "--add=")
			case !strings.HasPrefix(args[i], "-") && *wfPath == "":
				*wfPath = args[i]
			default:
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				return exitErr(1)
			}
		}
	}
	return nil
}

// HandleScheduleList handles the "schedule list" subcommand.
func HandleScheduleList() error {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		return exitErr(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		return nil
	}

	fmt.Printf("Scheduled tasks (%d):\n", len(entries))
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %-20s %s\n", "ID", "CRON", "TASK")
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, e := range entries {
		display := e.WorkflowPath
		if e.Description != "" {
			display = e.Description
		}
		fmt.Printf("  %-20s %-20s %s\n", e.ID, e.Cron, display)
	}
	return nil
}

// HandleScheduleRemove handles the "schedule remove" subcommand.
func HandleScheduleRemove(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: aflare schedule remove <id>")
		return exitErr(1)
	}
	id := args[0]

	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		return exitErr(1)
	}

	found := false
	updated := make([]scheduler.ScheduleEntry, 0, len(entries))
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		updated = append(updated, e)
	}
	if !found {
		fmt.Printf("❌ Task with id %q not found\n", id)
		return exitErr(1)
	}

	if err := scheduler.SaveSchedules(path, updated); err != nil {
		fmt.Printf("❌ Failed to save schedules: %v\n", err)
		return exitErr(1)
	}
	fmt.Printf("✅ Removed task %q\n", id)
	return nil
}

// HandleScheduleStart handles the "schedule start" subcommand.
func HandleScheduleStart() error {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		return exitErr(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		return exitErr(1)
	}

	sched := scheduler.New()
	for _, e := range entries {
		entry := e // capture for closure
		if entry.WorkflowPath != "" {
			// Workflow-based task
			wf, reg, err := PrepareWorkflow(entry.WorkflowPath)
			if err != nil {
				fmt.Printf("❌ Failed to prepare workflow %q: %v\n", entry.WorkflowPath, err)
				return exitErr(1)
			}
			taskFunc := func(ctx context.Context) {
				if _, _, err := workflow.ExecuteWorkflow(ctx, wf, reg); err != nil {
					log.Printf("scheduled workflow %q execution failed: %v", entry.ID, err)
				}
			}
			if err := sched.AddTask(entry.ID, entry.Cron, taskFunc); err != nil {
				fmt.Printf("❌ Failed to add task %q: %v\n", entry.ID, err)
				return exitErr(1)
			}
			fmt.Printf("📋 Loaded workflow task %q (%s -> %s)\n", entry.ID, entry.Cron, entry.WorkflowPath)
		} else {
			// Description-based task (placeholder — executed by agent daemon, not here)
			// The standalone scheduler can't execute description-based tasks;
			// these are meant for `aflare agent` which has the LLM-powered agent loop.
			fmt.Printf("📋 Skipped description task %q (%s) — use 'aflare agent' to run this\n", entry.ID, entry.Cron)
		}
	}

	sched.Start()
	activeCount := 0
	for _, e := range entries {
		if e.WorkflowPath != "" {
			activeCount++
		}
	}
	fmt.Printf("\n🚀 Scheduler started with %d task(s). Press Ctrl+C to stop.\n", activeCount)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n⏹  Stopping scheduler...")
	sched.Stop()
	fmt.Println("✅ Scheduler stopped.")
	return nil
}

// PrintScheduleUsage prints usage information for the schedule command.
func PrintScheduleUsage() {
	fmt.Println("Usage: aflare schedule <command> [options]")
	fmt.Println("\nSchedule tasks to run at specified times using cron expressions.")
	fmt.Println("Supports both workflow-based and natural language description-based tasks.")
	fmt.Println("\nCommands:")
	fmt.Println("  add --cron \"<expr>\" [--id <id>] [--desc \"<task>\"] [<workflow.yaml>]")
	fmt.Println("  add --add \"<natural language schedule>\"                     Auto-parse schedule")
	fmt.Println("  list                                                         List all scheduled tasks")
	fmt.Println("  remove <id>                                                  Remove a scheduled task")
	fmt.Println("  start                                                        Start the scheduler (foreground)")
	fmt.Println("  -h, --help                                                   Show this help message")
	fmt.Println("\nCron expression (5 fields): minute hour day-of-month month day-of-week")
	fmt.Println("  e.g. \"0 9 * * *\"      - daily at 09:00")
	fmt.Println("       \"*/15 * * * *\"   - every 15 minutes")
	fmt.Println("       \"0 9 * * 1-5\"    - weekdays at 09:00")
	fmt.Println("\nNatural language (--add):")
	fmt.Println("  \"每天9点检查git仓库状态\"          → \"0 9 * * *\"")
	fmt.Println("  \"每小时执行一次\"                  → \"0 * * * *\"")
	fmt.Println("  \"每周一早上8点\"                    → \"0 8 * * 1\"")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare schedule add --cron \"0 9 * * *\" my-workflow.yaml")
	fmt.Println("  aflare schedule add --cron \"0 9 * * *\" --desc \"Check git repo status\"")
	fmt.Println("  aflare schedule add --add \"每天9点检查git仓库状态\"")
	fmt.Println("  aflare schedule add --id daily-report --cron \"0 9 * * *\" report.yaml")
	fmt.Println("  aflare schedule list")
	fmt.Println("  aflare schedule remove daily-report")
	fmt.Println("  aflare schedule start")
}

// parseNaturalSchedule parses Chinese natural language schedule descriptions
// into cron expressions. Supported patterns:
//   - "每天N点" → "0 N * * *"
//   - "每小时" → "0 * * * *"
//   - "每周一/二/...早上N点" → "0 N * * 1/2/..."
//   - "每N小时" → "0 */N * * *"
//   - "每N分钟" → "*/N * * * *"
func parseNaturalSchedule(input string) (cron, desc string) {
	input = strings.TrimSpace(input)

	// Try to extract time and schedule pattern
	// Pattern: "每天X点" or "每天早上X点" etc.
	if strings.Contains(input, "每天") {
		hour := extractHour(input)
		minute := extractMinute(input)
		if hour >= 0 {
			if minute < 0 {
				minute = 0
			}
			cron = fmt.Sprintf("%d %d * * *", minute, hour)
			desc = input
			return
		}
	}

	// Pattern: "每小时" or "每小时执行"
	if strings.Contains(input, "每小时") {
		cron = "0 * * * *"
		desc = input
		return
	}

	// Pattern: "每N小时"
	if strings.Contains(input, "小时") {
		for _, r := range []string{"每", "每隔"} {
			if idx := strings.Index(input, r); idx >= 0 {
				rest := input[idx+len(r):]
				if n := extractNumber(rest); n > 0 && n <= 23 {
					cron = fmt.Sprintf("0 */%d * * *", n)
					desc = input
					return
				}
			}
		}
	}

	// Pattern: "每N分钟"
	if strings.Contains(input, "分钟") {
		for _, r := range []string{"每", "每隔"} {
			if idx := strings.Index(input, r); idx >= 0 {
				rest := input[idx+len(r):]
				if n := extractNumber(rest); n > 0 && n <= 59 {
					cron = fmt.Sprintf("*/%d * * * *", n)
					desc = input
					return
				}
			}
		}
	}

	// Pattern: "每周X" where X is a weekday
	weekdayMap := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "日": 0, "天": 0,
		"1": 1, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "0": 0, "7": 0,
	}
	if strings.Contains(input, "每周") {
		hour := extractHour(input)
		minute := extractMinute(input)
		if hour < 0 {
			hour = 9
		}
		if minute < 0 {
			minute = 0
		}
		for key, dow := range weekdayMap {
			if strings.Contains(input, key) && key != "1" && key != "2" && key != "3" && key != "4" && key != "5" && key != "6" && key != "0" && key != "7" {
				cron = fmt.Sprintf("%d %d * * %d", minute, hour, dow)
				desc = input
				return
			}
		}
	}

	// Pattern: "每天早上" (default 9:00)
	if strings.Contains(input, "早上") || strings.Contains(input, "上午") {
		hour := extractHour(input)
		if hour < 0 {
			hour = 9
		}
		minute := extractMinute(input)
		if minute < 0 {
			minute = 0
		}
		cron = fmt.Sprintf("%d %d * * *", minute, hour)
		desc = input
		return
	}

	// Pattern: "每天晚上" or "下午"
	if strings.Contains(input, "晚上") || strings.Contains(input, "下午") {
		hour := extractHour(input)
		if hour < 0 {
			hour = 20
		}
		// Convert 12-hour to 24-hour if needed
		if strings.Contains(input, "下午") && hour >= 1 && hour <= 11 {
			hour += 12
		}
		minute := extractMinute(input)
		if minute < 0 {
			minute = 0
		}
		cron = fmt.Sprintf("%d %d * * *", minute, hour)
		desc = input
		return
	}

	return "", input
}

// extractHour extracts the hour from a Chinese natural language string.
// Returns -1 if not found.
func extractHour(s string) int {
	runes := []rune(s)
	for i := 0; i < len(runes)-1; i++ {
		if runes[i+1] == '点' || runes[i+1] == '时' {
			if runes[i] >= '0' && runes[i] <= '9' {
				j := i
				for j >= 0 && runes[j] >= '0' && runes[j] <= '9' {
					j--
				}
				j++
				num := 0
				for k := j; k <= i; k++ {
					num = num*10 + int(runes[k]-'0')
				}
				if num >= 0 && num <= 23 {
					return num
				}
			}
		}
	}
	return -1
}

// extractMinute extracts the minute from a Chinese natural language string.
// Returns -1 if not found.
func extractMinute(s string) int {
	runes := []rune(s)
	for i := 0; i < len(runes)-1; i++ {
		if runes[i+1] == '分' {
			if runes[i] >= '0' && runes[i] <= '9' {
				j := i
				for j >= 0 && runes[j] >= '0' && runes[j] <= '9' {
					j--
				}
				j++
				num := 0
				for k := j; k <= i; k++ {
					num = num*10 + int(runes[k]-'0')
				}
				if num >= 0 && num <= 59 {
					return num
				}
			}
		}
	}
	// Look for "HH:MM" pattern in the original string
	for i := 0; i < len(s)-2; i++ {
		if s[i] >= '0' && s[i] <= '9' && s[i+1] == ':' && s[i+2] >= '0' && s[i+2] <= '9' {
			j := i + 3
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			num := 0
			for k := i + 2; k < j; k++ {
				num = num*10 + int(s[k]-'0')
			}
			if num >= 0 && num <= 59 {
				return num
			}
		}
	}
	return -1
}

// extractNumber extracts the first number found in a string.
func extractNumber(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			num := 0
			for k := i; k < j; k++ {
				num = num*10 + int(s[k]-'0')
			}
			return num
		}
	}
	return -1
}

// generateTaskID generates a simple task ID from a description string.
func generateTaskID(desc string) string {
	// Extract meaningful characters from the description
	id := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, desc)
	if len(id) == 0 {
		id = fmt.Sprintf("task-%d", time.Now().Unix())
	}
	if len(id) > 30 {
		id = id[:30]
	}
	return strings.ToLower(id)
}
