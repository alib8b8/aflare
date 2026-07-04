package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record represents a single workflow execution record
type Record struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Path        string       `json:"path,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	EndedAt     time.Time    `json:"ended_at,omitempty"`
	Success     bool         `json:"success"`
	Steps       []StepRecord `json:"steps,omitempty"`
	FinalOutput string       `json:"final_output,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// StepRecord represents a single step execution record
type StepRecord struct {
	Index    int           `json:"index"`
	Node     string        `json:"node"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
}

var historyDir string

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		historyDir = filepath.Join(home, ".config", "llm-box", "history")
	}
}

// SetHistoryDir sets a custom history directory (useful for tests)
func SetHistoryDir(dir string) {
	historyDir = dir
}

// SaveRecord saves a workflow execution record to the history directory
func SaveRecord(record Record) error {
	if historyDir == "" {
		return fmt.Errorf("history directory not available")
	}

	if err := os.MkdirAll(historyDir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if record.ID == "" {
		record.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	filename := filepath.Join(historyDir, record.ID+".json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// ListRecords returns all history records, sorted by time (newest first)
func ListRecords() ([]Record, error) {
	if historyDir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(historyDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	// Sort by started_at descending (newest first)
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].StartedAt.After(records[i].StartedAt) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	return records, nil
}

// GetRecord retrieves a single history record by ID
func GetRecord(id string) (*Record, error) {
	if historyDir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	path := filepath.Join(historyDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read record: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to parse record: %w", err)
	}

	return &record, nil
}

// ClearHistory removes all history records
func ClearHistory() error {
	if historyDir == "" {
		return fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		os.Remove(filepath.Join(historyDir, entry.Name()))
	}

	return nil
}
