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
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/alib8b8/aflare/internal/scheduler"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleSchedule handles the "schedule" command.
func HandleSchedule(args []string) {
	if len(args) == 0 {
		PrintScheduleUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "add":
		HandleScheduleAdd(args[1:])
	case "list":
		HandleScheduleList()
	case "remove":
		HandleScheduleRemove(args[1:])
	case "start":
		HandleScheduleStart()
	case "-h", "--help", "help":
		PrintScheduleUsage()
	default:
		fmt.Printf("Unknown schedule subcommand: %s\n\n", subCmd)
		PrintScheduleUsage()
		os.Exit(1)
	}
}

// HandleScheduleAdd handles the "schedule add" subcommand.
func HandleScheduleAdd(args []string) {
	var cronExpr, taskID, wfPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cron":
			if i+1 >= len(args) {
				fmt.Println("❌ --cron requires a value")
				os.Exit(1)
			}
			cronExpr = args[i+1]
			i++
		case "--id":
			if i+1 >= len(args) {
				fmt.Println("❌ --id requires a value")
				os.Exit(1)
			}
			taskID = args[i+1]
			i++
		case "--help", "-h":
			PrintScheduleUsage()
			return
		default:
			if strings.HasPrefix(args[i], "--cron=") {
				cronExpr = strings.TrimPrefix(args[i], "--cron=")
			} else if strings.HasPrefix(args[i], "--id=") {
				taskID = strings.TrimPrefix(args[i], "--id=")
			} else if !strings.HasPrefix(args[i], "-") && wfPath == "" {
				wfPath = args[i]
			} else {
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	if cronExpr == "" {
		fmt.Println("❌ --cron is required")
		PrintScheduleUsage()
		os.Exit(1)
	}
	if wfPath == "" {
		fmt.Println("❌ workflow file path is required")
		PrintScheduleUsage()
		os.Exit(1)
	}

	// Resolve to an absolute path so the schedule survives directory changes.
	absPath, err := filepath.Abs(wfPath)
	if err != nil {
		fmt.Printf("❌ Failed to resolve workflow path: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(absPath); err != nil {
		fmt.Printf("❌ Workflow file not found: %s\n", absPath)
		os.Exit(1)
	}
	wfPath = absPath

	// Default task ID: workflow filename stem.
	if taskID == "" {
		base := filepath.Base(wfPath)
		taskID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Validate the cron expression using a throwaway scheduler.
	validateSched := scheduler.New()
	if err := validateSched.AddTask(taskID, cronExpr, func(context.Context) {}); err != nil {
		fmt.Printf("❌ Invalid cron expression: %v\n", err)
		os.Exit(1)
	}

	// Load existing schedules and check for duplicate ID.
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	for _, e := range entries {
		if e.ID == taskID {
			fmt.Printf("❌ Task with id %q already exists (use --id to specify a different id)\n", taskID)
			os.Exit(1)
		}
	}

	entries = append(entries, scheduler.ScheduleEntry{
		ID:           taskID,
		Cron:         cronExpr,
		WorkflowPath: wfPath,
	})
	if err := scheduler.SaveSchedules(path, entries); err != nil {
		fmt.Printf("❌ Failed to save schedule: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Scheduled task %q added\n", taskID)
	fmt.Printf("   Cron:     %s\n", cronExpr)
	fmt.Printf("   Workflow: %s\n", wfPath)
	fmt.Printf("   Run 'aflare schedule start' to begin executing on schedule.\n")
}

// HandleScheduleList handles the "schedule list" subcommand.
func HandleScheduleList() {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		return
	}

	fmt.Printf("Scheduled tasks (%d):\n", len(entries))
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %-20s %s\n", "ID", "CRON", "WORKFLOW")
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, e := range entries {
		fmt.Printf("  %-20s %-20s %s\n", e.ID, e.Cron, e.WorkflowPath)
	}
}

// HandleScheduleRemove handles the "schedule remove" subcommand.
func HandleScheduleRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare schedule remove <id>")
		os.Exit(1)
	}
	id := args[0]

	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
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
		os.Exit(1)
	}

	if err := scheduler.SaveSchedules(path, updated); err != nil {
		fmt.Printf("❌ Failed to save schedules: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Removed task %q\n", id)
}

// HandleScheduleStart handles the "schedule start" subcommand.
func HandleScheduleStart() {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		os.Exit(1)
	}

	sched := scheduler.New()
	for _, e := range entries {
		entry := e // capture for closure
		wf, reg, err := PrepareWorkflow(entry.WorkflowPath)
		if err != nil {
			fmt.Printf("❌ Failed to prepare workflow %q: %v\n", entry.WorkflowPath, err)
			os.Exit(1)
		}
		taskFunc := func(ctx context.Context) {
			if _, _, err := workflow.ExecuteWorkflow(ctx, wf, reg); err != nil {
				log.Printf("scheduled workflow %q execution failed: %v", entry.ID, err)
			}
		}
		if err := sched.AddTask(entry.ID, entry.Cron, taskFunc); err != nil {
			fmt.Printf("❌ Failed to add task %q: %v\n", entry.ID, err)
			os.Exit(1)
		}
		fmt.Printf("📋 Loaded task %q (%s -> %s)\n", entry.ID, entry.Cron, entry.WorkflowPath)
	}

	sched.Start()
	fmt.Printf("\n🚀 Scheduler started with %d task(s). Press Ctrl+C to stop.\n", len(entries))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n⏹  Stopping scheduler...")
	sched.Stop()
	fmt.Println("✅ Scheduler stopped.")
}

// PrintScheduleUsage prints usage information for the schedule command.
func PrintScheduleUsage() {
	fmt.Println("Usage: aflare schedule <command> [options]")
	fmt.Println("\nSchedule workflows to run at specified times using cron expressions.")
	fmt.Println("\nCommands:")
	fmt.Println("  add --cron \"<expr>\" [--id <id>] <workflow.yaml>  Add a scheduled task")
	fmt.Println("  list                                            List all scheduled tasks")
	fmt.Println("  remove <id>                                     Remove a scheduled task")
	fmt.Println("  start                                           Start the scheduler (foreground)")
	fmt.Println("  -h, --help                                      Show this help message")
	fmt.Println("\nCron expression (5 fields): minute hour day-of-month month day-of-week")
	fmt.Println("  e.g. \"0 9 * * *\"      - daily at 09:00")
	fmt.Println("       \"*/15 * * * *\"   - every 15 minutes")
	fmt.Println("       \"0 9 * * 1-5\"    - weekdays at 09:00")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare schedule add --cron \"0 9 * * *\" my-workflow.yaml")
	fmt.Println("  aflare schedule add --id daily-report --cron \"0 9 * * *\" report.yaml")
	fmt.Println("  aflare schedule list")
	fmt.Println("  aflare schedule remove daily-report")
	fmt.Println("  aflare schedule start")
}
