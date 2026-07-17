// Package protocol defines the cross-platform AI task protocol for llm-box.
// This protocol enables AI systems from different vendors to understand and execute
// the same task descriptions, similar to how HTTP enables different browsers to
// communicate with different servers.
package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Version is the protocol version
const Version = "1.0.0"

// =============================================================================
// Intent URI - Standard format for AI task description
// =============================================================================

// IntentURI represents a parsed AI task intent URI
// Format: intent://workflow/<type>?param1=value1&param2=value2
//
// Examples:
//   - intent://workflow/book_flight?from=上海&to=北京&date=2024-01-15
//   - intent://workflow/send_message?to=张三&body=你好
//   - intent://workflow/control_device?device=灯光&action=打开
type IntentURI struct {
	// Type is the workflow type (e.g., "book_flight", "send_message")
	Type string `json:"type"`

	// Params are the workflow parameters
	Params map[string]string `json:"params"`

	// Source is the originating system (optional)
	Source string `json:"source,omitempty"`

	// Target is the target system (optional, for cross-device)
	Target string `json:"target,omitempty"`

	// Priority is the task priority (low, normal, high, urgent)
	Priority string `json:"priority,omitempty"`

	// Timeout is the maximum execution time
	Timeout string `json:"timeout,omitempty"`

	// Callback is the callback URL or node when complete
	Callback string `json:"callback,omitempty"`

	// Meta contains additional metadata
	Meta map[string]string `json:"meta,omitempty"`
}

// ParseIntentURI parses an intent URI string
func ParseIntentURI(uri string) (*IntentURI, error) {
	// Check scheme
	if !strings.HasPrefix(uri, "intent://") {
		return nil, fmt.Errorf("invalid intent URI: must start with 'intent://'")
	}

	// Remove scheme
	uri = strings.TrimPrefix(uri, "intent://")

	// Extract workflow type
	parts := strings.SplitN(uri, "?", 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid intent URI: missing workflow type")
	}

	intent := &IntentURI{
		Params: make(map[string]string),
		Meta:   make(map[string]string),
	}

	// Parse type
	typePath := parts[0]
	if strings.HasPrefix(typePath, "workflow/") {
		intent.Type = strings.TrimPrefix(typePath, "workflow/")
	} else {
		intent.Type = typePath
	}

	// Parse query parameters
	if len(parts) > 1 {
		queryStr := parts[1]
		query, err := url.ParseQuery(queryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid query string: %w", err)
		}

		for key, values := range query {
			if len(values) > 0 {
				intent.Params[key] = values[0]
			}
		}

		// Extract special params
		if v := intent.Params["source"]; v != "" {
			intent.Source = v
			delete(intent.Params, "source")
		}
		if v := intent.Params["target"]; v != "" {
			intent.Target = v
			delete(intent.Params, "target")
		}
		if v := intent.Params["priority"]; v != "" {
			intent.Priority = v
			delete(intent.Params, "priority")
		}
		if v := intent.Params["timeout"]; v != "" {
			intent.Timeout = v
			delete(intent.Params, "timeout")
		}
		if v := intent.Params["callback"]; v != "" {
			intent.Callback = v
			delete(intent.Params, "callback")
		}
	}

	return intent, nil
}

// String returns the intent URI string
func (i *IntentURI) String() string {
	var sb strings.Builder
	sb.WriteString("intent://workflow/")
	sb.WriteString(i.Type)

	if len(i.Params) > 0 || i.Source != "" || i.Target != "" || i.Priority != "" {
		sb.WriteString("?")

		params := url.Values{}
		for k, v := range i.Params {
			params.Set(k, v)
		}
		if i.Source != "" {
			params.Set("source", i.Source)
		}
		if i.Target != "" {
			params.Set("target", i.Target)
		}
		if i.Priority != "" {
			params.Set("priority", i.Priority)
		}
		if i.Timeout != "" {
			params.Set("timeout", i.Timeout)
		}
		if i.Callback != "" {
			params.Set("callback", i.Callback)
		}

		sb.WriteString(params.Encode())
	}

	return sb.String()
}

// ToJSON returns the intent as JSON
func (i *IntentURI) ToJSON() (string, error) {
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// =============================================================================
// Task Message - Standard message format for AI task exchange
// =============================================================================

// TaskMessage represents a complete AI task message
type TaskMessage struct {
	// ID is the unique message ID
	ID string `json:"id"`

	// Version is the protocol version
	Version string `json:"version"`

	// Timestamp is the message creation time
	Timestamp time.Time `json:"timestamp"`

	// Intent is the task intent
	Intent *IntentURI `json:"intent"`

	// Workflow is the generated workflow (optional, filled by receiver)
	Workflow string `json:"workflow,omitempty"`

	// Context is additional context for the task
	Context map[string]interface{} `json:"context,omitempty"`

	// Status is the task status
	Status TaskStatus `json:"status"`

	// Result is the task result (optional, filled after execution)
	Result string `json:"result,omitempty"`

	// Error is the error message if failed
	Error string `json:"error,omitempty"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// NewTaskMessage creates a new task message
func NewTaskMessage(intent *IntentURI) *TaskMessage {
	return &TaskMessage{
		ID:        generateID(),
		Version:   Version,
		Timestamp: time.Now(),
		Intent:    intent,
		Status:    TaskStatusPending,
		Context:   make(map[string]interface{}),
	}
}

// ToJSON returns the message as JSON
func (m *TaskMessage) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseTaskMessage parses a JSON task message
func ParseTaskMessage(jsonStr string) (*TaskMessage, error) {
	var msg TaskMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// =============================================================================
// Workflow Registry - Standard workflow types
// =============================================================================

// WorkflowType represents a standard workflow type
type WorkflowType struct {
	// Name is the workflow type name
	Name string `json:"name"`

	// Description is the workflow description
	Description string `json:"description"`

	// Category is the workflow category
	Category string `json:"category"`

	// RequiredParams are required parameters
	RequiredParams []string `json:"required_params,omitempty"`

	// OptionalParams are optional parameters
	OptionalParams []string `json:"optional_params,omitempty"`

	// Examples are example intent URIs
	Examples []string `json:"examples,omitempty"`
}

// StandardWorkflowTypes defines the standard workflow types
var StandardWorkflowTypes = []WorkflowType{
	// Travel
	{Name: "book_flight", Description: "Book a flight", Category: "travel", RequiredParams: []string{"from", "to"}, OptionalParams: []string{"date", "passengers", "class"}, Examples: []string{"intent://workflow/book_flight?from=上海&to=北京&date=2024-01-15"}},
	{Name: "book_hotel", Description: "Book a hotel room", Category: "travel", RequiredParams: []string{"location"}, OptionalParams: []string{"checkin", "checkout", "guests"}, Examples: []string{"intent://workflow/book_hotel?location=北京&checkin=2024-01-15&checkout=2024-01-17"}},
	{Name: "book_train", Description: "Book a train ticket", Category: "travel", RequiredParams: []string{"from", "to"}, OptionalParams: []string{"date", "passengers"}, Examples: []string{"intent://workflow/book_train?from=上海&to=北京"}},

	// Communication
	{Name: "send_message", Description: "Send a message", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"body", "app"}, Examples: []string{"intent://workflow/send_message?to=张三&body=你好"}},
	{Name: "make_call", Description: "Make a phone call", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"app"}, Examples: []string{"intent://workflow/make_call?to=13800138000"}},
	{Name: "send_email", Description: "Send an email", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"subject", "body"}, Examples: []string{"intent://workflow/send_email?to=user@example.com&subject=测试&body=内容"}},

	// Food
	{Name: "order_food", Description: "Order food delivery", Category: "food", RequiredParams: []string{}, OptionalParams: []string{"restaurant", "dish", "address"}, Examples: []string{"intent://workflow/order_food?restaurant=麦当劳&dish=巨无霸套餐"}},
	{Name: "book_restaurant", Description: "Book a restaurant table", Category: "food", RequiredParams: []string{"restaurant"}, OptionalParams: []string{"date", "time", "guests"}, Examples: []string{"intent://workflow/book_restaurant?restaurant=海底捞&date=2024-01-15"}},

	// Shopping
	{Name: "shop_search", Description: "Search for products", Category: "shopping", RequiredParams: []string{"query"}, OptionalParams: []string{"category", "max_price"}, Examples: []string{"intent://workflow/shop_search?query=iPhone"}},
	{Name: "compare_prices", Description: "Compare product prices", Category: "shopping", RequiredParams: []string{"product"}, OptionalParams: []string{"stores"}, Examples: []string{"intent://workflow/compare_prices?product=iPhone%2015"}},

	// Entertainment
	{Name: "play_music", Description: "Play music", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"song", "artist", "playlist"}, Examples: []string{"intent://workflow/play_music?song=晴天&artist=周杰伦"}},
	{Name: "play_video", Description: "Play video/movie", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"title", "type"}, Examples: []string{"intent://workflow/play_video?title=流浪地球2"}},
	{Name: "play_game", Description: "Launch a game", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"game"}, Examples: []string{"intent://workflow/play_game?game=王者荣耀"}},

	// Work
	{Name: "schedule_meeting", Description: "Schedule a meeting", Category: "work", RequiredParams: []string{"title"}, OptionalParams: []string{"date", "time", "attendees"}, Examples: []string{"intent://workflow/schedule_meeting?title=项目评审&date=2024-01-15"}},
	{Name: "set_reminder", Description: "Set a reminder", Category: "work", RequiredParams: []string{"message"}, OptionalParams: []string{"time"}, Examples: []string{"intent://workflow/set_reminder?message=喝水&time=14:00"}},
	{Name: "create_task", Description: "Create a task", Category: "work", RequiredParams: []string{"title"}, OptionalParams: []string{"due_date", "priority"}, Examples: []string{"intent://workflow/create_task?title=完成报告&due_date=2024-01-20"}},

	// System
	{Name: "control_device", Description: "Control smart device", Category: "system", RequiredParams: []string{"device"}, OptionalParams: []string{"action", "value"}, Examples: []string{"intent://workflow/control_device?device=灯光&action=打开"}},
	{Name: "check_weather", Description: "Check weather", Category: "system", RequiredParams: []string{}, OptionalParams: []string{"location"}, Examples: []string{"intent://workflow/check_weather?location=北京"}},
	{Name: "set_alarm", Description: "Set an alarm", Category: "system", RequiredParams: []string{"time"}, OptionalParams: []string{"label"}, Examples: []string{"intent://workflow/set_alarm?time=07:00&label=起床"}},
	{Name: "take_photo", Description: "Take a photo", Category: "system", RequiredParams: []string{}, OptionalParams: []string{"mode", "flash"}, Examples: []string{"intent://workflow/take_photo?mode=portrait"}},
	{Name: "open_app", Description: "Open an app", Category: "system", RequiredParams: []string{"app"}, OptionalParams: []string{"action"}, Examples: []string{"intent://workflow/open_app?app=微信"}},

	// AI Agent
	{Name: "research_topic", Description: "Research a topic", Category: "agent", RequiredParams: []string{"topic"}, OptionalParams: []string{"depth"}, Examples: []string{"intent://workflow/research_topic?topic=AI发展趋势"}},
	{Name: "analyze_document", Description: "Analyze a document", Category: "agent", RequiredParams: []string{"document"}, OptionalParams: []string{"analysis_type"}, Examples: []string{"intent://workflow/analyze_document?document=report.pdf"}},
	{Name: "generate_content", Description: "Generate content", Category: "agent", RequiredParams: []string{"type"}, OptionalParams: []string{"topic", "length"}, Examples: []string{"intent://workflow/generate_content?type=文章&topic=科技"}},
}

// GetWorkflowType returns a workflow type by name
func GetWorkflowType(name string) *WorkflowType {
	for _, wt := range StandardWorkflowTypes {
		if wt.Name == name {
			return &wt
		}
	}
	return nil
}

// =============================================================================
// Helper functions
// =============================================================================

func generateID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
