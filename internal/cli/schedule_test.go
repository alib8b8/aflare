// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌​‌‌​​‌​‌‌‌​​‌‌‌‌​​‌‌‌‌​​​​​​​​‌​‌​‌‌​​​‌‌‌​‌​​‌​​​​​​​​​​​​​​​​‌‌​​‌‌‌‌​‌​​​‌​​⁠
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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/scheduler"
)

// setupSchedulesStore isolates the persisted-schedules store for one test.
//
// scheduler.DefaultSchedulesPath resolves through meta.DataDir, which caches
// its result via sync.Once for the lifetime of the whole test binary. Setting
// AFLARE_DATA therefore only takes effect if this test is the first one to
// trigger that cache; once pinned, every test in the binary shares the same
// path regardless of the environment. Either way the effective store path is
// returned, and in the shared case it is reset to a known-empty state so each
// test starts from scratch.
func setupSchedulesStore(t *testing.T) string {
	t.Helper()

	dataDir := t.TempDir()
	t.Setenv("AFLARE_DATA", dataDir)

	path := scheduler.DefaultSchedulesPath()
	if path == filepath.Join(dataDir, scheduler.SchedulesFileName) {
		return path // fresh temp dir: the store is already empty
	}

	// The meta cache was pinned by an earlier test in this binary, so
	// AFLARE_DATA has no effect here. Clear whatever sits at the shared
	// path (a failure-path test may even have left a directory there) and
	// write an empty store so this test stays deterministic.
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("clear shared schedules store %s: %v", path, err)
	}
	if err := scheduler.SaveSchedules(path, []scheduler.ScheduleEntry{}); err != nil {
		t.Fatalf("reset shared schedules store %s: %v", path, err)
	}
	return path
}

// makeStoreUnreadable replaces the schedules file with a directory so that
// LoadSchedules fails, exercising the handlers' load-error paths.
func makeStoreUnreadable(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove schedules file %s: %v", path, err)
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("create directory at %s: %v", path, err)
	}
}

// captureScheduleOutput captures stdout while fn runs.
func captureScheduleOutput(fn func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		fn()
		return ""
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r) // #nosec G104 -- test-only best-effort read
	return buf.String()
}

// runScheduleCmd runs fn with stdout captured and returns the output and the
// error fn returned.
func runScheduleCmd(fn func() error) (string, error) {
	var err error
	out := captureScheduleOutput(func() { err = fn() })
	return out, err
}

func TestScheduleDispatch(t *testing.T) {
	setupSchedulesStore(t)

	cases := []struct {
		name      string
		args      []string
		wantCode  int // 0 means the handler must return nil
		wantUsage bool
	}{
		{name: "no args prints usage", args: nil, wantCode: 1, wantUsage: true},
		{name: "help", args: []string{"help"}, wantCode: 0, wantUsage: true},
		{name: "short help", args: []string{"-h"}, wantCode: 0, wantUsage: true},
		{name: "long help", args: []string{"--help"}, wantCode: 0, wantUsage: true},
		{name: "unknown subcommand", args: []string{"frobnicate"}, wantCode: 1, wantUsage: true},
		{name: "add error propagates", args: []string{"add", "--cron"}, wantCode: 1, wantUsage: false},
		{name: "remove error propagates", args: []string{"remove"}, wantCode: 1, wantUsage: true},
		{name: "list on empty store", args: []string{"list"}, wantCode: 0, wantUsage: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runScheduleCmd(func() error { return HandleSchedule(tc.args) })
			if code := ExitCode(err); code != tc.wantCode {
				t.Errorf("HandleSchedule(%q) exit code = %d, want %d (err=%v)",
					tc.args, code, tc.wantCode, err)
			}
			if got := strings.Contains(out, "Usage: aflare schedule"); got != tc.wantUsage {
				t.Errorf("HandleSchedule(%q) usage printed = %v, want %v (output: %s)",
					tc.args, got, tc.wantUsage, out)
			}
		})
	}
}

func TestScheduleAddValidationErrors(t *testing.T) {
	wfDir := t.TempDir()
	existing := filepath.Join(wfDir, "exists.yaml")
	if err := os.WriteFile(existing, []byte("steps: []\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{name: "no args at all", args: nil, wantIn: "--cron is required"},
		{name: "desc only, no cron", args: []string{"--desc", "x"}, wantIn: "--cron is required"},
		{name: "workflow only, no cron", args: []string{existing}, wantIn: "--cron is required"},
		{name: "cron but neither workflow nor desc", args: []string{"--cron", "0 9 * * *"},
			wantIn: "Either a workflow file or --desc"},
		{name: "workflow file missing",
			args:   []string{"--cron", "0 9 * * *", filepath.Join(wfDir, "missing.yaml")},
			wantIn: "Workflow file not found"},
		{name: "invalid cron expression", args: []string{"--cron", "not-a-cron", "--desc", "d"},
			wantIn: "Invalid cron expression"},
		{name: "cron field out of range", args: []string{"--cron", "99 * * * *", "--desc", "d"},
			wantIn: "Invalid cron expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runScheduleCmd(func() error { return HandleScheduleAdd(tc.args) })
			if code := ExitCode(err); code != 1 {
				t.Errorf("HandleScheduleAdd(%q) exit code = %d, want 1 (err=%v)",
					tc.args, code, err)
			}
			if !strings.Contains(out, tc.wantIn) {
				t.Errorf("HandleScheduleAdd(%q) output %q does not contain %q",
					tc.args, out, tc.wantIn)
			}
		})
	}
}

func TestScheduleParseAddArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
		wantIn  string
		cron    string
		id      string
		wf      string
		desc    string
		add     string
	}{
		{
			name: "space separated flags and positional workflow",
			args: []string{"--cron", "0 9 * * *", "--id", "t1", "--desc", "d1", "--add", "a1", "wf.yaml"},
			cron: "0 9 * * *", id: "t1", wf: "wf.yaml", desc: "d1", add: "a1",
		},
		{
			name: "equals separated flags",
			args: []string{"--cron=0 9 * * *", "--id=t2", "--desc=d2", "--add=a2", "wf2.yaml"},
			cron: "0 9 * * *", id: "t2", wf: "wf2.yaml", desc: "d2", add: "a2",
		},
		{name: "positional workflow only", args: []string{"wf3.yaml"}, wf: "wf3.yaml"},
		{name: "short help prints usage and stops", args: []string{"-h"}, wantIn: "Usage: aflare schedule"},
		{name: "long help prints usage and stops", args: []string{"--help"}, wantIn: "Usage: aflare schedule"},
		{name: "missing cron value", args: []string{"--cron"}, wantErr: true, wantIn: "--cron requires a value"},
		{name: "missing desc value", args: []string{"--desc"}, wantErr: true, wantIn: "--desc requires a value"},
		{name: "missing id value", args: []string{"--id"}, wantErr: true, wantIn: "--id requires a value"},
		{name: "missing add value", args: []string{"--add"}, wantErr: true, wantIn: "--add requires a value"},
		{name: "unknown long flag", args: []string{"--bogus"}, wantErr: true, wantIn: "Unknown argument"},
		{name: "unknown short flag", args: []string{"-x"}, wantErr: true, wantIn: "Unknown argument"},
		{name: "second positional argument", args: []string{"a.yaml", "b.yaml"}, wantErr: true,
			wantIn: "Unknown argument", wf: "a.yaml"}, // first positional sticks before the error
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cronExpr, taskID, wfPath, desc, autoParse string
			var maxRetries int
			var retryDelay string
			out, err := runScheduleCmd(func() error {
				return parseScheduleAddArgs(tc.args, &cronExpr, &taskID, &wfPath, &desc, &autoParse, &maxRetries, &retryDelay)
			})
			if tc.wantErr {
				if code := ExitCode(err); code != 1 {
					t.Errorf("parseScheduleAddArgs(%q) exit code = %d, want 1 (err=%v)",
						tc.args, code, err)
				}
			} else if err != nil {
				t.Errorf("parseScheduleAddArgs(%q) unexpected error: %v", tc.args, err)
			}
			if tc.wantIn != "" && !strings.Contains(out, tc.wantIn) {
				t.Errorf("parseScheduleAddArgs(%q) output %q does not contain %q",
					tc.args, out, tc.wantIn)
			}
			if cronExpr != tc.cron {
				t.Errorf("cron = %q, want %q", cronExpr, tc.cron)
			}
			if taskID != tc.id {
				t.Errorf("id = %q, want %q", taskID, tc.id)
			}
			if wfPath != tc.wf {
				t.Errorf("wfPath = %q, want %q", wfPath, tc.wf)
			}
			if desc != tc.desc {
				t.Errorf("desc = %q, want %q", desc, tc.desc)
			}
			if autoParse != tc.add {
				t.Errorf("autoParse = %q, want %q", autoParse, tc.add)
			}
		})
	}
}

func TestScheduleAddNaturalLanguage(t *testing.T) {
	storePath := setupSchedulesStore(t)

	out, err := runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{"--add", "每天9点检查git仓库状态"})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("natural-language add failed: %v", err)
	}
	if !strings.Contains(out, "0 9 * * *") {
		t.Errorf("expected parsed cron in output, got: %s", out)
	}

	entries, lerr := scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry after add, got %d (err=%v)", len(entries), lerr)
	}
	if entries[0].Cron != "0 9 * * *" {
		t.Errorf("cron = %q, want %q", entries[0].Cron, "0 9 * * *")
	}
	if entries[0].Description != "每天9点检查git仓库状态" {
		t.Errorf("description = %q, want the original input", entries[0].Description)
	}
	if entries[0].WorkflowPath != "" {
		t.Errorf("workflow path = %q, want empty", entries[0].WorkflowPath)
	}
}

func TestScheduleAddNaturalLanguageUnparseable(t *testing.T) {
	setupSchedulesStore(t)

	out, err := runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{"--add", "whenever the mood strikes"})
	})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit code = %d, want 1 for unparseable --add (err=%v)", code, err)
	}
	if !strings.Contains(out, "Could not parse schedule") {
		t.Errorf("expected parse-failure message, got: %s", out)
	}
}

func TestScheduleAddListRemoveLifecycle(t *testing.T) {
	storePath := setupSchedulesStore(t)

	// Dummy workflow file; add only stats it, content is irrelevant.
	wfPath := filepath.Join(t.TempDir(), "daily-report.yaml")
	if err := os.WriteFile(wfPath, []byte("steps: []\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// add: explicit id + desc + workflow file.
	out, err := runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{
			"--id", "daily-check",
			"--cron", "0 9 * * *",
			"--desc", "Daily check",
			wfPath,
		})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("schedule add failed: %v", err)
	}
	if !strings.Contains(out, `Scheduled task "daily-check" added`) {
		t.Errorf("add output missing confirmation, got: %s", out)
	}

	entries, lerr := scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry after add, got %d (err=%v)", len(entries), lerr)
	}
	if entries[0].ID != "daily-check" || entries[0].Cron != "0 9 * * *" ||
		entries[0].Description != "Daily check" || entries[0].WorkflowPath != wfPath {
		t.Errorf("stored entry mismatch: %+v", entries[0])
	}

	// list: the description is preferred over the workflow path for display.
	out, err = runScheduleCmd(HandleScheduleList)
	if ExitCode(err) != 0 {
		t.Fatalf("schedule list failed: %v", err)
	}
	if !strings.Contains(out, "Scheduled tasks (1)") {
		t.Errorf("expected task count header, got: %s", out)
	}
	if !strings.Contains(out, "daily-check") || !strings.Contains(out, "Daily check") {
		t.Errorf("expected id and description in list, got: %s", out)
	}
	if strings.Contains(out, wfPath) {
		t.Errorf("list should prefer description over workflow path, got: %s", out)
	}

	// add: workflow-only, default id derived from the file base name.
	_, err = runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{"--cron", "*/15 * * * *", wfPath})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("workflow-only add failed: %v", err)
	}
	entries, lerr = scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 2 {
		t.Fatalf("expected 2 entries after second add, got %d (err=%v)", len(entries), lerr)
	}
	found := false
	for _, e := range entries {
		if e.ID == "daily-report" {
			found = true
			if e.WorkflowPath != wfPath || e.Description != "" {
				t.Errorf("workflow-only entry mismatch: %+v", e)
			}
		}
	}
	if !found {
		t.Errorf("expected default id %q derived from file name, entries: %+v", "daily-report", entries)
	}

	// list: workflow path is displayed when there is no description.
	out, err = runScheduleCmd(HandleScheduleList)
	if ExitCode(err) != 0 {
		t.Fatalf("schedule list failed: %v", err)
	}
	if !strings.Contains(out, "daily-report") || !strings.Contains(out, wfPath) {
		t.Errorf("expected default id and workflow path in list, got: %s", out)
	}

	// remove via the top-level dispatcher.
	out, err = runScheduleCmd(func() error {
		return HandleSchedule([]string{"remove", "daily-report"})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("schedule remove failed: %v", err)
	}
	if !strings.Contains(out, `Removed task "daily-report"`) {
		t.Errorf("remove output missing confirmation, got: %s", out)
	}

	// remove the remaining entry directly.
	_, err = runScheduleCmd(func() error {
		return HandleScheduleRemove([]string{"daily-check"})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("schedule remove failed: %v", err)
	}

	entries, lerr = scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 0 {
		t.Errorf("expected empty store after removals, got %d entries (err=%v)",
			len(entries), lerr)
	}

	// list on the now-empty store.
	out, err = runScheduleCmd(HandleScheduleList)
	if ExitCode(err) != 0 {
		t.Fatalf("schedule list failed: %v", err)
	}
	if !strings.Contains(out, "No scheduled tasks") {
		t.Errorf("expected empty-store message, got: %s", out)
	}
}

func TestScheduleAddDuplicateID(t *testing.T) {
	storePath := setupSchedulesStore(t)

	add := func() error {
		return HandleScheduleAdd([]string{"--id", "dup-task", "--cron", "0 9 * * *", "--desc", "run backups"})
	}
	if _, err := runScheduleCmd(add); ExitCode(err) != 0 {
		t.Fatalf("first add failed: %v", err)
	}

	out, err := runScheduleCmd(add)
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit code = %d, want 1 for duplicate id (err=%v)", code, err)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected duplicate-id message, got: %s", out)
	}

	entries, lerr := scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 1 {
		t.Errorf("expected exactly 1 entry after duplicate rejection, got %d (err=%v)",
			len(entries), lerr)
	}
}

func TestScheduleRemoveErrors(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		out, err := runScheduleCmd(func() error { return HandleScheduleRemove(nil) })
		if code := ExitCode(err); code != 1 {
			t.Errorf("exit code = %d, want 1 (err=%v)", code, err)
		}
		if !strings.Contains(out, "Usage: aflare schedule remove") {
			t.Errorf("expected remove usage, got: %s", out)
		}
	})

	t.Run("id not found", func(t *testing.T) {
		setupSchedulesStore(t)
		out, err := runScheduleCmd(func() error { return HandleScheduleRemove([]string{"ghost"}) })
		if code := ExitCode(err); code != 1 {
			t.Errorf("exit code = %d, want 1 (err=%v)", code, err)
		}
		if !strings.Contains(out, `not found`) {
			t.Errorf("expected not-found message, got: %s", out)
		}
	})
}

func TestScheduleStoreLoadFailures(t *testing.T) {
	cases := []struct {
		name string
		call func() error
	}{
		{"add", func() error {
			return HandleScheduleAdd([]string{"--cron", "0 9 * * *", "--desc", "d"})
		}},
		{"list", HandleScheduleList},
		{"remove", func() error { return HandleScheduleRemove([]string{"x"}) }},
		// Safe: with an unreadable store, HandleScheduleStart returns
		// exitErr(1) from the LoadSchedules failure before any scheduler
		// start or signal wait.
		{"start", HandleScheduleStart},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storePath := setupSchedulesStore(t)
			makeStoreUnreadable(t, storePath)

			out, err := runScheduleCmd(tc.call)
			if code := ExitCode(err); code != 1 {
				t.Errorf("exit code = %d, want 1 (err=%v)", code, err)
			}
			if !strings.Contains(out, "Failed to load schedules") {
				t.Errorf("expected load-failure message, got: %s", out)
			}
		})
	}
}

func TestScheduleStartEmpty(t *testing.T) {
	// Safe: with an empty store HandleScheduleStart prints a message and
	// returns exitErr(1) before creating the scheduler, so it never blocks
	// on the signal wait.
	setupSchedulesStore(t)

	out, err := runScheduleCmd(HandleScheduleStart)
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit code = %d, want 1 for empty store (err=%v)", code, err)
	}
	if !strings.Contains(out, "No scheduled tasks") {
		t.Errorf("expected empty-store message, got: %s", out)
	}
}

func TestScheduleParseNaturalSchedule(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantCron string
		wantDesc string
	}{
		{"daily at 9", "每天9点检查git仓库状态", "0 9 * * *", "每天9点检查git仓库状态"},
		{"daily at 9:30", "每天9点30分执行备份", "30 9 * * *", "每天9点30分执行备份"},
		{"daily morning", "每天早上9点", "0 9 * * *", "每天早上9点"},
		{"every hour", "每小时执行一次", "0 * * * *", "每小时执行一次"},
		{"every 2 hours", "每2小时同步一次", "0 */2 * * *", "每2小时同步一次"},
		{"every 3 hours with ge", "每隔3小时", "0 */3 * * *", "每隔3小时"},
		{"every 15 minutes", "每15分钟清理缓存", "*/15 * * * *", "每15分钟清理缓存"},
		{"weekly monday 8am", "每周一早上8点开周会", "0 8 * * 1", "每周一早上8点开周会"},
		{"weekly sunday 9am", "每周日晚上9点", "0 9 * * 0", "每周日晚上9点"},
		{"morning only", "早上8点起床", "0 8 * * *", "早上8点起床"},
		{"am with hour", "上午10点站会", "0 10 * * *", "上午10点站会"},
		{"evening", "晚上11点日报", "0 11 * * *", "晚上11点日报"},
		{"afternoon to 24h", "下午3点复盘", "0 15 * * *", "下午3点复盘"},
		{"afternoon with minute", "下午3点30分", "30 15 * * *", "下午3点30分"},
		{"hour out of range", "每天25点", "", "每天25点"},
		{"english unsupported", "every day at 9:30", "", "every day at 9:30"},
		{"empty input", "", "", ""},
		{"whitespace trimmed", "  每小时  ", "0 * * * *", "每小时"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cron, desc := parseNaturalSchedule(tc.input)
			if cron != tc.wantCron {
				t.Errorf("parseNaturalSchedule(%q) cron = %q, want %q", tc.input, cron, tc.wantCron)
			}
			if desc != tc.wantDesc {
				t.Errorf("parseNaturalSchedule(%q) desc = %q, want %q", tc.input, desc, tc.wantDesc)
			}
		})
	}
}

func TestScheduleExtractHour(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"9点", 9},
		{"09点", 9},
		{"23时", 23},
		{"第7点", 7},
		{"每天9点30分", 9},
		{"24点", -1},  // out of range
		{"123点", -1}, // out of range
		{"9:30", -1}, // no 点/时 marker
		{"no digits点", -1},
		{"", -1},
		{"点", -1},
	}
	for _, tc := range cases {
		if got := extractHour(tc.input); got != tc.want {
			t.Errorf("extractHour(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestScheduleExtractMinute(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"30分", 30},
		{"9点30分", 30},
		{"7分", 7},
		{"9:30", 30}, // HH:MM fallback
		{"23:45", 45},
		{"12:05", 5},
		{"60分", -1},   // out of range
		{"999分", -1},  // out of range
		{"12:99", -1}, // HH:MM out of range
		{"no time here", -1},
		{"", -1},
	}
	for _, tc := range cases {
		if got := extractMinute(tc.input); got != tc.want {
			t.Errorf("extractMinute(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestScheduleExtractNumber(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"abc456def", 456},
		{"隔3小时", 3},
		{"第12条", 12},
		{"7栋3单元", 7}, // first number wins
		{"no digits", -1},
		{"", -1},
	}
	for _, tc := range cases {
		if got := extractNumber(tc.input); got != tc.want {
			t.Errorf("extractNumber(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestScheduleGenerateTaskID(t *testing.T) {
	cases := []struct {
		desc string
		want string
	}{
		{"Check Git Repo Status", "checkgitrepostatus"},
		{"Report_2024-final", "report_2024-final"},
		{"UPPER-CASE_42", "upper-case_42"},
	}
	for _, tc := range cases {
		if got := generateTaskID(tc.desc); got != tc.want {
			t.Errorf("generateTaskID(%q) = %q, want %q", tc.desc, got, tc.want)
		}
	}

	// Long ids are truncated to 30 characters.
	if got := generateTaskID(strings.Repeat("a", 50)); len(got) != 30 {
		t.Errorf("generateTaskID(len 50) length = %d, want 30", len(got))
	}

	// Descriptions with no ASCII alphanumerics fall back to a time-based id.
	// (Note "每天9点检查" would keep the digit 9; use a Chinese numeral.)
	for _, desc := range []string{"", "每天九点检查"} {
		if got := generateTaskID(desc); !strings.HasPrefix(got, "task-") || len(got) <= len("task-") {
			t.Errorf("generateTaskID(%q) = %q, want a non-empty task-<timestamp> fallback", desc, got)
		}
	}

	// Distinct descriptions yield distinct ids.
	if generateTaskID("alpha task") == generateTaskID("beta task") {
		t.Error("expected distinct ids for distinct descriptions")
	}
}

// ─── Retry options ──────────────────────────────────────────────────────────

func TestScheduleAddRetryOptions(t *testing.T) {
	storePath := setupSchedulesStore(t)

	wfPath := filepath.Join(t.TempDir(), "retry-job.yaml")
	if err := os.WriteFile(wfPath, []byte("steps: []\n"), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{
			"--id", "retry-job",
			"--cron", "0 9 * * *",
			"--retry", "3",
			"--retry-delay", "45s",
			wfPath,
		})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("schedule add with retry failed: %v", err)
	}
	if !strings.Contains(out, "Retry:") || !strings.Contains(out, "45s") {
		t.Errorf("add output missing retry summary, got: %s", out)
	}

	entries, lerr := scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (err=%v)", len(entries), lerr)
	}
	if entries[0].MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", entries[0].MaxRetries)
	}
	if entries[0].RetryDelay != "45s" {
		t.Errorf("RetryDelay = %q, want %q", entries[0].RetryDelay, "45s")
	}
}

func TestScheduleAddRetryDefaultsOmitted(t *testing.T) {
	storePath := setupSchedulesStore(t)

	_, err := runScheduleCmd(func() error {
		return HandleScheduleAdd([]string{"--cron", "0 9 * * *", "--desc", "no retry"})
	})
	if ExitCode(err) != 0 {
		t.Fatalf("schedule add without retry failed: %v", err)
	}

	entries, lerr := scheduler.LoadSchedules(storePath)
	if lerr != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (err=%v)", len(entries), lerr)
	}
	if entries[0].MaxRetries != 0 || entries[0].RetryDelay != "" {
		// omitempty keeps the store byte-compatible with older versions.
		t.Errorf("default entry must keep retry fields zero, got %+v", entries[0])
	}
}

func TestScheduleAddRetryValidation(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{"negative retry", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry", "-1"}, "--retry must be an integer"},
		{"retry over cap", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry", "99"}, "--retry must be an integer"},
		{"retry not a number", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry", "abc"}, "--retry must be an integer"},
		{"missing retry value", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry"}, "--retry requires a value"},
		{"bad delay", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry", "1", "--retry-delay", "nope"}, "--retry-delay must be a positive duration"},
		{"delay over cap", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry", "1", "--retry-delay", "10m"}, "--retry-delay must be a positive duration"},
		{"missing delay value", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry-delay"}, "--retry-delay requires a value"},
		{"equals form bad retry", []string{"--cron", "0 9 * * *", "--desc", "d", "--retry=xyz"}, "--retry must be an integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runScheduleCmd(func() error { return HandleScheduleAdd(tc.args) })
			if code := ExitCode(err); code != 1 {
				t.Errorf("exit code = %d, want 1 (err=%v)", code, err)
			}
			if !strings.Contains(out, tc.wantIn) {
				t.Errorf("output %q does not contain %q", out, tc.wantIn)
			}
		})
	}
}

func TestScheduleEntryRetryDelayDuration(t *testing.T) {
	cases := []struct {
		entry scheduler.ScheduleEntry
		want  time.Duration
	}{
		{scheduler.ScheduleEntry{}, scheduler.DefaultTaskRetryDelay},
		{scheduler.ScheduleEntry{RetryDelay: ""}, scheduler.DefaultTaskRetryDelay},
		{scheduler.ScheduleEntry{RetryDelay: "45s"}, 45 * time.Second},
		{scheduler.ScheduleEntry{RetryDelay: "1m"}, time.Minute},
		{scheduler.ScheduleEntry{RetryDelay: "garbage"}, scheduler.DefaultTaskRetryDelay},
		{scheduler.ScheduleEntry{RetryDelay: "-5s"}, scheduler.DefaultTaskRetryDelay},
	}
	for _, tc := range cases {
		if got := tc.entry.RetryDelayDuration(); got != tc.want {
			t.Errorf("RetryDelayDuration(%q) = %v, want %v", tc.entry.RetryDelay, got, tc.want)
		}
	}
}
