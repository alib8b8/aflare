package history

import (
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

	// Should be sorted newest first
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
