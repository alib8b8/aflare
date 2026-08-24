// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​​‌‌‌‌​​‌​​​​​​​​​‌​​‌‌​‌​‌​‌​‌‌​​​​​‌‌‌​​​​​​​​​​​​​​​​​​​​​‌​​‌‌​‌​​‌​‌​‌​⁠
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

package history

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSaveAndListRecords(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	record := Record{
		ID:          "test-1",
		Name:        "Test Workflow",
		StartedAt:   time.Now().Add(-time.Minute),
		EndedAt:     time.Now(),
		Success:     true,
		FinalOutput: "hello",
		Steps: []StepRecord{
			{Index: 0, Node: "fetch_url", Duration: 100 * time.Millisecond, Success: true},
			{Index: 1, Node: "ollama", Duration: 2 * time.Second, Success: true},
		},
	}

	err := SaveRecord(record)
	if err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	records, err := ListRecords()
	if err != nil {
		t.Fatalf("failed to list records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", records[0].Name)
	}

	if !records[0].Success {
		t.Error("expected success to be true")
	}

	if len(records[0].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(records[0].Steps))
	}
}

func TestGetRecord(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	record := Record{
		ID:      "test-get",
		Name:    "Get Test",
		Success: false,
		Error:   "something failed",
	}

	err := SaveRecord(record)
	if err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	retrieved, err := GetRecord("test-get")
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}

	if retrieved.Name != "Get Test" {
		t.Errorf("expected name 'Get Test', got '%s'", retrieved.Name)
	}

	if retrieved.Error != "something failed" {
		t.Errorf("expected error 'something failed', got '%s'", retrieved.Error)
	}
}

func TestGetRecord_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	_, err := GetRecord("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent record")
	}
}

func TestClearHistory(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	record := Record{ID: "to-clear", Name: "Clear Me"}
	SaveRecord(record)

	records, _ := ListRecords()
	if len(records) == 0 {
		t.Fatal("expected at least one record before clear")
	}

	err := ClearHistory()
	if err != nil {
		t.Fatalf("failed to clear history: %v", err)
	}

	records, _ = ListRecords()
	if len(records) != 0 {
		t.Errorf("expected 0 records after clear, got %d", len(records))
	}
}

func TestListRecords_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	records, err := ListRecords()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestListRecords_Sorted(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	now := time.Now()

	SaveRecord(Record{ID: "old", Name: "Old", StartedAt: now.Add(-2 * time.Hour)})
	SaveRecord(Record{ID: "new", Name: "New", StartedAt: now})
	SaveRecord(Record{ID: "mid", Name: "Mid", StartedAt: now.Add(-1 * time.Hour)})

	records, err := ListRecords()
	if err != nil {
		t.Fatalf("failed to list records: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].Name != "New" {
		t.Errorf("expected first record 'New', got '%s'", records[0].Name)
	}
	if records[1].Name != "Mid" {
		t.Errorf("expected second record 'Mid', got '%s'", records[1].Name)
	}
	if records[2].Name != "Old" {
		t.Errorf("expected third record 'Old', got '%s'", records[2].Name)
	}
}

func TestSaveRecord_GeneratesID(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	record := Record{Name: "No ID"}
	err := SaveRecord(record)
	if err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	records, _ := ListRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestRecordNewFields(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	started := time.Now().Add(-5 * time.Second)
	ended := time.Now()
	record := Record{
		ID:        "test-new-fields",
		Name:      "New Fields Test",
		Trigger:   TriggerCLI,
		User:      "alice",
		Version:   "v1.2.3",
		Duration:  5 * time.Second,
		StartedAt: started,
		EndedAt:   ended,
		Success:   true,
		Steps: []StepRecord{
			{
				Index:      0,
				Node:       "step1",
				Params:     `{"url":"http://example.com"}`,
				RetryCount: 2,
				InputSize:  1024,
				OutputSize: 2048,
				Duration:   2 * time.Second,
				Success:    true,
			},
		},
	}

	err := SaveRecord(record)
	if err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	retrieved, err := GetRecord("test-new-fields")
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}

	if retrieved.Trigger != TriggerCLI {
		t.Errorf("expected trigger 'cli', got '%s'", retrieved.Trigger)
	}
	if retrieved.User != "alice" {
		t.Errorf("expected user 'alice', got '%s'", retrieved.User)
	}
	if retrieved.Version != "v1.2.3" {
		t.Errorf("expected version 'v1.2.3', got '%s'", retrieved.Version)
	}
	if retrieved.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", retrieved.Duration)
	}
	if len(retrieved.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(retrieved.Steps))
	}
	if retrieved.Steps[0].RetryCount != 2 {
		t.Errorf("expected retry count 2, got %d", retrieved.Steps[0].RetryCount)
	}
	if retrieved.Steps[0].InputSize != 1024 {
		t.Errorf("expected input size 1024, got %d", retrieved.Steps[0].InputSize)
	}
	if retrieved.Steps[0].OutputSize != 2048 {
		t.Errorf("expected output size 2048, got %d", retrieved.Steps[0].OutputSize)
	}
}

func TestSanitizeParams(t *testing.T) {
	params := map[string]interface{}{
		"name":      "test",
		"api_key":   "secret123",
		"token":     "abc456",
		"password":  "mypassword",
		"SecretKey": "hidden",
		"nested": map[string]interface{}{
			"inner_token": "inner_secret",
			"safe_field":  "ok",
		},
	}

	sanitized := SanitizeParams(params)

	if sanitized["name"] != "test" {
		t.Errorf("expected name to be 'test', got '%v'", sanitized["name"])
	}
	if sanitized["api_key"] != "***" {
		t.Errorf("expected api_key to be '***', got '%v'", sanitized["api_key"])
	}
	if sanitized["token"] != "***" {
		t.Errorf("expected token to be '***', got '%v'", sanitized["token"])
	}
	if sanitized["password"] != "***" {
		t.Errorf("expected password to be '***', got '%v'", sanitized["password"])
	}
	if sanitized["SecretKey"] != "***" {
		t.Errorf("expected SecretKey to be '***', got '%v'", sanitized["SecretKey"])
	}

	nested, ok := sanitized["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested to be a map")
	}
	if nested["inner_token"] != "***" {
		t.Errorf("expected inner_token to be '***', got '%v'", nested["inner_token"])
	}
	if nested["safe_field"] != "ok" {
		t.Errorf("expected safe_field to be 'ok', got '%v'", nested["safe_field"])
	}
}

func TestSanitizeParams_Nil(t *testing.T) {
	var result = SanitizeParams(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestSummarizeParams(t *testing.T) {
	params := map[string]interface{}{
		"name":    "test",
		"api_key": "secret123",
		"value":   123,
	}

	summary := SummarizeParams(params, 0)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(summary) > 203 {
		t.Errorf("expected summary <= 203 chars, got %d", len(summary))
	}
	if containsSensitiveValue(summary, "secret123") {
		t.Error("summary should not contain sensitive value 'secret123'")
	}
}

func TestSummarizeParams_Truncation(t *testing.T) {
	params := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		params[fmt.Sprintf("field_%d", i)] = "value"
	}

	summary := SummarizeParams(params, 50)
	if len(summary) > 53 {
		t.Errorf("expected summary <= 53 chars, got %d", len(summary))
	}
}

func containsSensitiveValue(s, val string) bool {
	return len(s) > 0 && len(val) > 0 && searchString(s, val)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAppendAuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	log := AuditLog{
		Action:   AuditActionLogin,
		User:     "alice",
		Resource: "system",
		Success:  true,
		IP:       "192.168.1.1",
	}

	err := AppendAuditLog(log)
	if err != nil {
		t.Fatalf("failed to append audit log: %v", err)
	}

	logs, err := ReadAuditLogs()
	if err != nil {
		t.Fatalf("failed to read audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}

	if logs[0].Action != AuditActionLogin {
		t.Errorf("expected action 'login', got '%s'", logs[0].Action)
	}
	if logs[0].User != "alice" {
		t.Errorf("expected user 'alice', got '%s'", logs[0].User)
	}
	if logs[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
	if logs[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestAppendAuditLog_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	for i := 0; i < 3; i++ {
		log := AuditLog{
			Action:  AuditActionConfigChange,
			User:    "bob",
			Success: i%2 == 0,
		}
		err := AppendAuditLog(log)
		if err != nil {
			t.Fatalf("failed to append audit log %d: %v", i, err)
		}
	}

	logs, err := ReadAuditLogs()
	if err != nil {
		t.Fatalf("failed to read audit logs: %v", err)
	}

	if len(logs) != 3 {
		t.Fatalf("expected 3 audit logs, got %d", len(logs))
	}
}

func TestReadAuditLogs_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	logs, err := ReadAuditLogs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(logs) != 0 {
		t.Errorf("expected 0 audit logs, got %d", len(logs))
	}
}

func TestListRecordsWithFilter_BySuccess(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	now := time.Now()
	SaveRecord(Record{ID: "s1", Name: "Success 1", Success: true, StartedAt: now})
	SaveRecord(Record{ID: "f1", Name: "Fail 1", Success: false, StartedAt: now.Add(-time.Hour)})
	SaveRecord(Record{ID: "s2", Name: "Success 2", Success: true, StartedAt: now.Add(-2 * time.Hour)})

	success := true
	filtered, err := ListRecordsWithFilter(RecordFilter{Success: &success})
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 success records, got %d", len(filtered))
	}

	failure := false
	filtered, err = ListRecordsWithFilter(RecordFilter{Success: &failure})
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 failure record, got %d", len(filtered))
	}
}

func TestListRecordsWithFilter_ByTime(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	now := time.Now()
	SaveRecord(Record{ID: "r1", Name: "Recent", StartedAt: now.Add(-time.Hour)})
	SaveRecord(Record{ID: "r2", Name: "Old", StartedAt: now.Add(-48 * time.Hour)})
	SaveRecord(Record{ID: "r3", Name: "Medium", StartedAt: now.Add(-12 * time.Hour)})

	startTime := now.Add(-24 * time.Hour)
	filtered, err := ListRecordsWithFilter(RecordFilter{StartTime: &startTime})
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 records in last 24h, got %d", len(filtered))
	}

	endTime := now.Add(-6 * time.Hour)
	filtered, err = ListRecordsWithFilter(RecordFilter{EndTime: &endTime})
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 records older than 6h, got %d", len(filtered))
	}
}

func TestListRecordsWithFilter_ByWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	now := time.Now()
	SaveRecord(Record{ID: "w1", Name: "Data Pipeline", StartedAt: now})
	SaveRecord(Record{ID: "w2", Name: "Email Sender", StartedAt: now.Add(-time.Hour)})
	SaveRecord(Record{ID: "w3", Name: "data backup", StartedAt: now.Add(-2 * time.Hour)})

	filtered, err := ListRecordsWithFilter(RecordFilter{Workflow: "data"})
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 records matching 'data', got %d", len(filtered))
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	now := time.Now()
	SaveRecord(Record{
		ID:        "stat1",
		Name:      "Success 1",
		Success:   true,
		Duration:  10 * time.Second,
		StartedAt: now.Add(-time.Hour),
		EndedAt:   now.Add(-time.Hour).Add(10 * time.Second),
	})
	SaveRecord(Record{
		ID:        "stat2",
		Name:      "Fail 1",
		Success:   false,
		Duration:  5 * time.Second,
		StartedAt: now.Add(-2 * time.Hour),
		EndedAt:   now.Add(-2 * time.Hour).Add(5 * time.Second),
	})
	SaveRecord(Record{
		ID:        "stat3",
		Name:      "Success 2",
		Success:   true,
		Duration:  20 * time.Second,
		StartedAt: now.Add(-3 * time.Hour),
		EndedAt:   now.Add(-3 * time.Hour).Add(20 * time.Second),
	})
	SaveRecord(Record{
		ID:        "stat-old",
		Name:      "Old",
		Success:   true,
		Duration:  15 * time.Second,
		StartedAt: now.Add(-48 * time.Hour),
		EndedAt:   now.Add(-48 * time.Hour).Add(15 * time.Second),
	})

	stats, err := GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalCount != 4 {
		t.Errorf("expected total count 4, got %d", stats.TotalCount)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("expected success count 3, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Errorf("expected failure count 1, got %d", stats.FailureCount)
	}
	expectedRate := 3.0 / 4.0
	if stats.SuccessRate != expectedRate {
		t.Errorf("expected success rate %f, got %f", expectedRate, stats.SuccessRate)
	}
	expectedAvg := (10*time.Second + 5*time.Second + 20*time.Second + 15*time.Second) / 4
	if stats.AverageDuration != expectedAvg {
		t.Errorf("expected average duration %v, got %v", expectedAvg, stats.AverageDuration)
	}
	if stats.Last24hCount != 3 {
		t.Errorf("expected last 24h count 3, got %d", stats.Last24hCount)
	}
}

func TestGetStats_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	stats, err := GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalCount != 0 {
		t.Errorf("expected total count 0, got %d", stats.TotalCount)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("expected success rate 0, got %f", stats.SuccessRate)
	}
}

func TestGetStats_FallbackDuration(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	started := time.Now().Add(-10 * time.Second)
	ended := time.Now()
	SaveRecord(Record{
		ID:        "fallback",
		Name:      "Fallback Test",
		Success:   true,
		StartedAt: started,
		EndedAt:   ended,
	})

	stats, err := GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.AverageDuration <= 0 {
		t.Errorf("expected positive average duration from fallback, got %v", stats.AverageDuration)
	}
}

func TestTriggerTypes(t *testing.T) {
	triggers := []TriggerType{TriggerManual, TriggerCLI, TriggerAPI, TriggerSchedule}
	expected := []string{"manual", "cli", "api", "schedule"}

	for i, tr := range triggers {
		if string(tr) != expected[i] {
			t.Errorf("trigger %d: expected '%s', got '%s'", i, expected[i], tr)
		}
	}
}

func TestAuditActions(t *testing.T) {
	actions := []AuditAction{
		AuditActionLogin,
		AuditActionLogout,
		AuditActionConfigChange,
		AuditActionSensitiveOp,
		AuditActionWorkflowStart,
		AuditActionWorkflowEnd,
	}
	expected := []string{
		"login",
		"logout",
		"config_change",
		"sensitive_operation",
		"workflow_start",
		"workflow_end",
	}

	for i, a := range actions {
		if string(a) != expected[i] {
			t.Errorf("action %d: expected '%s', got '%s'", i, expected[i], a)
		}
	}
}

// appendAuditLogsForChain appends count audit log entries with distinct detail
// values and returns the audit log file path. It is a helper for hash-chain tests.
func appendAuditLogsForChain(t *testing.T, count int) string {
	t.Helper()
	for i := 0; i < count; i++ {
		log := AuditLog{
			Action: AuditActionConfigChange,
			User:   "alice",
			Detail: fmt.Sprintf("change-%d", i),
		}
		if err := AppendAuditLog(log); err != nil {
			t.Fatalf("failed to append audit log %d: %v", i, err)
		}
	}
	path := GetAuditLogPath()
	if path == "" {
		t.Fatal("audit log path is empty")
	}
	return path
}

// readAuditLines reads the audit log file and returns its non-empty lines.
func readAuditLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func writeAuditLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("failed to write audit log: %v", err)
	}
}

func TestAppendAuditLog_HashChainLinkage(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	path := appendAuditLogsForChain(t, 3)
	lines := readAuditLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	prevCurr := auditZeroHash
	for i, line := range lines {
		var entry AuditLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse line %d: %v", i+1, err)
		}
		if entry.PrevHash != prevCurr {
			t.Errorf("line %d: prev_hash mismatch (expected %s, got %s)", i+1, prevCurr, entry.PrevHash)
		}
		if entry.CurrHash == "" {
			t.Errorf("line %d: expected non-empty curr_hash", i+1)
		}
		if entry.PrevHash == entry.CurrHash {
			t.Errorf("line %d: prev_hash should differ from curr_hash", i+1)
		}
		prevCurr = entry.CurrHash
	}
}

func TestVerifyAuditChain_Valid(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	path := appendAuditLogsForChain(t, 5)

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid chain, broken at line %d", brokenAt)
	}
	if brokenAt != 0 {
		t.Errorf("expected brokenAtLine 0, got %d", brokenAt)
	}
}

func TestVerifyAuditChain_Tampered(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	path := appendAuditLogsForChain(t, 4)
	lines := readAuditLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Tamper with line 2 (index 1): change the detail value but leave curr_hash
	// unchanged so the recomputed HMAC no longer matches.
	tampered := strings.Replace(lines[1], "change-1", "change-EVIL", 1)
	if tampered == lines[1] {
		t.Fatal("tampering did not modify the line")
	}
	lines[1] = tampered
	writeAuditLines(t, path, lines)

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected chain to be invalid after tampering")
	}
	if brokenAt != 2 {
		t.Errorf("expected broken at line 2, got %d", brokenAt)
	}
}

func TestVerifyAuditChain_Deleted(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	path := appendAuditLogsForChain(t, 4)
	lines := readAuditLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Delete line 2 (index 1). The following record's prev_hash will no longer
	// link to line 1's curr_hash.
	remaining := lines[:1:1]
	remaining = append(remaining, lines[2:]...)
	writeAuditLines(t, path, remaining)

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected chain to be invalid after deletion")
	}
	// After deletion, line 2 (former line 3) has a prev_hash that doesn't match
	// line 1's curr_hash.
	if brokenAt != 2 {
		t.Errorf("expected broken at line 2, got %d", brokenAt)
	}
}

func TestVerifyAuditChain_Empty(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	// The audit log file does not exist yet in a fresh temp directory.
	path := GetAuditLogPath()
	if path == "" {
		t.Fatal("audit log path is empty")
	}

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid chain for non-existent file, broken at %d", brokenAt)
	}
	if brokenAt != 0 {
		t.Errorf("expected brokenAtLine 0, got %d", brokenAt)
	}
}

func TestVerifyAuditChain_LegacyFormat(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	path := GetAuditLogPath()
	// Write a legacy record that lacks prev_hash/curr_hash fields.
	legacy := map[string]interface{}{
		"id":        "legacy-1",
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"action":    "login",
		"success":   true,
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("failed to marshal legacy record: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatalf("failed to write legacy audit log: %v", err)
	}

	valid, brokenAt, err := VerifyAuditChain(path)
	if valid {
		t.Error("expected invalid chain for legacy format")
	}
	if brokenAt != 1 {
		t.Errorf("expected broken at line 1, got %d", brokenAt)
	}
	if err == nil {
		t.Fatal("expected error for legacy format, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible format") {
		t.Errorf("expected 'incompatible format' in error, got: %v", err)
	}
}

func TestAppendAuditLog_ExtendsExistingChain(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)

	// Build an initial chain, then append one more and verify the new tail links.
	path := appendAuditLogsForChain(t, 2)
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "bob", Detail: "extra"}); err != nil {
		t.Fatalf("failed to append extra log: %v", err)
	}

	lines := readAuditLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var last, secondLast AuditLog
	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatalf("failed to parse last line: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &secondLast); err != nil {
		t.Fatalf("failed to parse second-to-last line: %v", err)
	}
	if last.PrevHash != secondLast.CurrHash {
		t.Errorf("new record prev_hash (%s) does not match previous curr_hash (%s)", last.PrevHash, secondLast.CurrHash)
	}

	valid, _, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Error("expected chain to remain valid after extending")
	}
}
