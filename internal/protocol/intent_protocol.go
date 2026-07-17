package protocol

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	Version = "1.0.0"

	maxIntentURLLength   = 4096
	maxTaskMessageLength = 65536
	maxParamKeyLength    = 100
	maxParamValueLength  = 1000
	maxDIDLength         = 256
	maxSignatureLength   = 1024
	maxMetadataEntries   = 50
	maxMetadataKeyLen    = 64
	maxMetadataValueLen  = 512
)

var (
	// validDIDMethod restricts method names to lowercase letters and digits.
	validDIDMethod = regexp.MustCompile(`^[a-z0-9]+$`)
	// validDIDIdentifier restricts identifiers to alphanumeric, underscore, hyphen, and dot.
	validDIDIdentifier = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)
)

type IntentURI struct {
	Type     string            `json:"type"`
	Params   map[string]string `json:"params"`
	Source   string            `json:"source,omitempty"`
	Target   string            `json:"target,omitempty"`
	Priority string            `json:"priority,omitempty"`
	Timeout  string            `json:"timeout,omitempty"`
	Callback string            `json:"callback,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

func ParseIntentURI(uri string) (*IntentURI, error) {
	if len(uri) > maxIntentURLLength {
		return nil, fmt.Errorf("intent URI too long")
	}

	if !strings.HasPrefix(uri, "intent://") {
		return nil, fmt.Errorf("invalid intent URI: must start with 'intent://'")
	}

	uri = strings.TrimPrefix(uri, "intent://")

	parts := strings.SplitN(uri, "?", 2)
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid intent URI: missing workflow type")
	}

	intent := &IntentURI{
		Params: make(map[string]string),
		Meta:   make(map[string]string),
	}

	typePath := parts[0]
	if strings.HasPrefix(typePath, "workflow/") {
		intent.Type = strings.TrimPrefix(typePath, "workflow/")
	} else {
		intent.Type = typePath
	}

	if err := validateWorkflowType(intent.Type); err != nil {
		return nil, err
	}

	if len(parts) > 1 {
		queryStr := parts[1]
		query, err := url.ParseQuery(queryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid query string: %w", err)
		}

		if len(query) > 50 {
			return nil, fmt.Errorf("too many parameters (max 50)")
		}

		for key, values := range query {
			if len(key) > maxParamKeyLength {
				return nil, fmt.Errorf("parameter key too long")
			}
			if len(values) > 0 {
				value := values[0]
				if len(value) > maxParamValueLength {
					return nil, fmt.Errorf("parameter value too long for key: %s", key)
				}
				intent.Params[key] = value
			}
		}

		if v := intent.Params["source"]; v != "" {
			if len(v) > maxParamValueLength {
				return nil, fmt.Errorf("source too long")
			}
			intent.Source = v
			delete(intent.Params, "source")
		}
		if v := intent.Params["target"]; v != "" {
			if len(v) > maxParamValueLength {
				return nil, fmt.Errorf("target too long")
			}
			intent.Target = v
			delete(intent.Params, "target")
		}
		if v := intent.Params["priority"]; v != "" {
			if !validatePriority(v) {
				return nil, fmt.Errorf("invalid priority: %s", v)
			}
			intent.Priority = v
			delete(intent.Params, "priority")
		}
		if v := intent.Params["timeout"]; v != "" {
			if err := validateTimeout(v); err != nil {
				return nil, err
			}
			intent.Timeout = v
			delete(intent.Params, "timeout")
		}
		if v := intent.Params["callback"]; v != "" {
			if err := validateCallback(v); err != nil {
				return nil, err
			}
			intent.Callback = v
			delete(intent.Params, "callback")
		}
	}

	return intent, nil
}

func validateWorkflowType(workflowType string) error {
	if len(workflowType) > 100 {
		return fmt.Errorf("workflow type too long")
	}

	for _, wt := range StandardWorkflowTypes {
		if wt.Name == workflowType {
			return nil
		}
	}

	return fmt.Errorf("unknown workflow type: %s", workflowType)
}

func validatePriority(priority string) bool {
	validPriorities := map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	return validPriorities[priority]
}

func validateTimeout(timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	if duration <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if duration > 24*time.Hour {
		return fmt.Errorf("timeout too long (max 24 hours)")
	}
	return nil
}

func validateCallback(callback string) error {
	if len(callback) > maxIntentURLLength {
		return fmt.Errorf("callback too long")
	}

	u, err := url.Parse(callback)
	if err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("callback must use http or https scheme")
	}

	if u.User != nil {
		return fmt.Errorf("callback URL cannot contain credentials")
	}

	return nil
}

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

func (i *IntentURI) ToJSON() (string, error) {
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DIDIdentity represents a W3C DID-based agent identity for cross-domain
// authentication and trust establishment, inspired by awiki.ai's agent-native
// identity system.
type DIDIdentity struct {
	DID        string            `json:"did"`
	Method     string            `json:"method"`
	Endpoint   string            `json:"endpoint,omitempty"`
	PublicKey  string            `json:"public_key,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	VerifiedAt time.Time         `json:"verified_at,omitempty"`
}

// DIDDocument represents the resolvable document for a DID identity,
// containing public keys and service endpoints.
type DIDDocument struct {
	ID             string         `json:"id"`
	Context        []string       `json:"@context"`
	PublicKeys     []DIDPublicKey `json:"publicKey,omitempty"`
	Services       []DIDService   `json:"service,omitempty"`
	Authentication []string       `json:"authentication,omitempty"`
	Created        time.Time      `json:"created"`
	Updated        time.Time      `json:"updated"`
}

// DIDPublicKey represents a public key entry in a DID document.
type DIDPublicKey struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Controller      string `json:"controller"`
	PublicKeyBase64 string `json:"publicKeyBase64,omitempty"`
}

// DIDService represents a service endpoint in a DID document.
type DIDService struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// Validate checks if the DID identity is well-formed and valid.
func (d *DIDIdentity) Validate() error {
	if d.DID == "" {
		return fmt.Errorf("DID is required")
	}
	if len(d.DID) > maxDIDLength {
		return fmt.Errorf("DID too long (max %d)", maxDIDLength)
	}
	if !strings.HasPrefix(d.DID, "did:") {
		return fmt.Errorf("invalid DID format: must start with 'did:'")
	}
	parts := strings.Split(d.DID, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid DID format: expected did:method:identifier")
	}
	if !validDIDMethod.MatchString(parts[1]) {
		return fmt.Errorf("invalid DID method: %s", parts[1])
	}
	if !validDIDIdentifier.MatchString(parts[2]) {
		return fmt.Errorf("invalid DID identifier: %s", parts[2])
	}
	if d.Method != "" && d.Method != parts[1] {
		return fmt.Errorf("DID method mismatch")
	}
	if len(d.Signature) > maxSignatureLength {
		return fmt.Errorf("signature too long (max %d)", maxSignatureLength)
	}
	if d.Endpoint != "" {
		if err := validateDIDEndpoint(d.Endpoint); err != nil {
			return fmt.Errorf("invalid DID endpoint: %w", err)
		}
	}
	if len(d.Metadata) > maxMetadataEntries {
		return fmt.Errorf("too many metadata entries (max %d)", maxMetadataEntries)
	}
	for k, v := range d.Metadata {
		if len(k) > maxMetadataKeyLen {
			return fmt.Errorf("metadata key too long: %s", k)
		}
		if len(v) > maxMetadataValueLen {
			return fmt.Errorf("metadata value too long for key: %s", k)
		}
	}
	return nil
}

// validateDIDEndpoint validates an endpoint URL for DID documents,
// blocking private IP ranges to prevent SSRF.
func validateDIDEndpoint(endpoint string) error {
	if len(endpoint) > 2048 {
		return fmt.Errorf("endpoint too long")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	if u.User != nil {
		return fmt.Errorf("URL cannot contain credentials")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("private IP addresses are not allowed")
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve host: %w", err)
		}
		for _, resolvedIP := range ips {
			if resolvedIP.IsLoopback() || resolvedIP.IsPrivate() || resolvedIP.IsLinkLocalUnicast() || resolvedIP.IsLinkLocalMulticast() || resolvedIP.IsUnspecified() || resolvedIP.IsMulticast() {
				return fmt.Errorf("resolved to private IP address")
			}
		}
	}
	return nil
}

// VerifySignature performs a placeholder signature verification.
// In production this would use the public key from the DID document.
func (d *DIDIdentity) VerifySignature(payload string) error {
	if d.Signature == "" {
		return fmt.Errorf("signature is required for verification")
	}
	// Placeholder: real implementation would verify using public key
	if len(d.Signature) < 16 {
		return fmt.Errorf("signature too short")
	}
	return nil
}

// NewDIDIdentity creates a new DID identity with the given method and identifier.
func NewDIDIdentity(method, identifier string) *DIDIdentity {
	did := fmt.Sprintf("did:%s:%s", method, identifier)
	return &DIDIdentity{
		DID:      did,
		Method:   method,
		Metadata: make(map[string]string),
	}
}

// ToDIDDocument generates a DID document for this identity.
func (d *DIDIdentity) ToDIDDocument() *DIDDocument {
	now := time.Now()
	return &DIDDocument{
		ID:      d.DID,
		Context: []string{"https://www.w3.org/ns/did/v1"},
		PublicKeys: []DIDPublicKey{
			{
				ID:              d.DID + "#keys-1",
				Type:            "Ed25519VerificationKey2020",
				Controller:      d.DID,
				PublicKeyBase64: d.PublicKey,
			},
		},
		Services: []DIDService{
			{
				ID:              d.DID + "#agent",
				Type:            "AgentService",
				ServiceEndpoint: d.Endpoint,
			},
		},
		Authentication: []string{d.DID + "#keys-1"},
		Created:        now,
		Updated:        now,
	}
}

type TaskMessage struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Timestamp   time.Time              `json:"timestamp"`
	Intent      *IntentURI             `json:"intent"`
	Workflow    string                 `json:"workflow,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Status      TaskStatus             `json:"status"`
	Result      string                 `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Sender      *DIDIdentity           `json:"sender,omitempty"`
	Receiver    *DIDIdentity           `json:"receiver,omitempty"`
	CrossDomain bool                   `json:"cross_domain,omitempty"`
}

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

func NewTaskMessage(intent *IntentURI) *TaskMessage {
	return &TaskMessage{
		ID:        generateSecureID(),
		Version:   Version,
		Timestamp: time.Now(),
		Intent:    intent,
		Status:    TaskStatusPending,
		Context:   make(map[string]interface{}),
	}
}

func (m *TaskMessage) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseTaskMessage(jsonStr string) (*TaskMessage, error) {
	if len(jsonStr) > maxTaskMessageLength {
		return nil, fmt.Errorf("task message too large")
	}

	var msg TaskMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, err
	}

	if msg.ID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	if msg.Intent == nil {
		return nil, fmt.Errorf("intent is required")
	}

	if err := validateWorkflowType(msg.Intent.Type); err != nil {
		return nil, err
	}

	if !validateTaskStatus(msg.Status) {
		return nil, fmt.Errorf("invalid task status: %s", msg.Status)
	}

	// Validate DID identities for cross-domain messages
	if msg.CrossDomain {
		if msg.Sender == nil {
			return nil, fmt.Errorf("cross-domain message requires sender DID")
		}
		if err := msg.Sender.Validate(); err != nil {
			return nil, fmt.Errorf("invalid sender DID: %w", err)
		}
		if msg.Receiver == nil {
			return nil, fmt.Errorf("cross-domain message requires receiver DID")
		}
		if err := msg.Receiver.Validate(); err != nil {
			return nil, fmt.Errorf("invalid receiver DID: %w", err)
		}
		if msg.Sender.DID == msg.Receiver.DID {
			return nil, fmt.Errorf("sender and receiver DIDs must be different")
		}
	} else {
		// When CrossDomain is false, Sender and Receiver should not be set
		// to prevent accidental DID leakage in non-cross-domain contexts.
		if msg.Sender != nil || msg.Receiver != nil {
			return nil, fmt.Errorf("sender/receiver DIDs require cross_domain=true")
		}
	}

	return &msg, nil
}

func validateTaskStatus(status TaskStatus) bool {
	validStatuses := map[TaskStatus]bool{
		TaskStatusPending:   true,
		TaskStatusQueued:    true,
		TaskStatusRunning:   true,
		TaskStatusCompleted: true,
		TaskStatusFailed:    true,
		TaskStatusCancelled: true,
	}
	return validStatuses[status]
}

type WorkflowType struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	RequiredParams []string `json:"required_params,omitempty"`
	OptionalParams []string `json:"optional_params,omitempty"`
	Examples       []string `json:"examples,omitempty"`
}

var StandardWorkflowTypes = []WorkflowType{
	{Name: "book_flight", Description: "Book a flight", Category: "travel", RequiredParams: []string{"from", "to"}, OptionalParams: []string{"date", "passengers", "class"}, Examples: []string{"intent://workflow/book_flight?from=上海&to=北京&date=2024-01-15"}},
	{Name: "book_hotel", Description: "Book a hotel room", Category: "travel", RequiredParams: []string{"location"}, OptionalParams: []string{"checkin", "checkout", "guests"}, Examples: []string{"intent://workflow/book_hotel?location=北京&checkin=2024-01-15&checkout=2024-01-17"}},
	{Name: "book_train", Description: "Book a train ticket", Category: "travel", RequiredParams: []string{"from", "to"}, OptionalParams: []string{"date", "passengers"}, Examples: []string{"intent://workflow/book_train?from=上海&to=北京"}},
	{Name: "send_message", Description: "Send a message", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"body", "app"}, Examples: []string{"intent://workflow/send_message?to=张三&body=你好"}},
	{Name: "make_call", Description: "Make a phone call", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"app"}, Examples: []string{"intent://workflow/make_call?to=13800138000"}},
	{Name: "send_email", Description: "Send an email", Category: "communication", RequiredParams: []string{"to"}, OptionalParams: []string{"subject", "body"}, Examples: []string{"intent://workflow/send_email?to=user@example.com&subject=测试&body=内容"}},
	{Name: "order_food", Description: "Order food delivery", Category: "food", RequiredParams: []string{}, OptionalParams: []string{"restaurant", "dish", "address"}, Examples: []string{"intent://workflow/order_food?restaurant=麦当劳&dish=巨无霸套餐"}},
	{Name: "book_restaurant", Description: "Book a restaurant table", Category: "food", RequiredParams: []string{"restaurant"}, OptionalParams: []string{"date", "time", "guests"}, Examples: []string{"intent://workflow/book_restaurant?restaurant=海底捞&date=2024-01-15"}},
	{Name: "shop_search", Description: "Search for products", Category: "shopping", RequiredParams: []string{"query"}, OptionalParams: []string{"category", "max_price"}, Examples: []string{"intent://workflow/shop_search?query=iPhone"}},
	{Name: "compare_prices", Description: "Compare product prices", Category: "shopping", RequiredParams: []string{"product"}, OptionalParams: []string{"stores"}, Examples: []string{"intent://workflow/compare_prices?product=iPhone%2015"}},
	{Name: "play_music", Description: "Play music", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"song", "artist", "playlist"}, Examples: []string{"intent://workflow/play_music?song=晴天&artist=周杰伦"}},
	{Name: "play_video", Description: "Play video/movie", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"title", "type"}, Examples: []string{"intent://workflow/play_video?title=流浪地球2"}},
	{Name: "play_game", Description: "Launch a game", Category: "entertainment", RequiredParams: []string{}, OptionalParams: []string{"game"}, Examples: []string{"intent://workflow/play_game?game=王者荣耀"}},
	{Name: "schedule_meeting", Description: "Schedule a meeting", Category: "work", RequiredParams: []string{"title"}, OptionalParams: []string{"date", "time", "attendees"}, Examples: []string{"intent://workflow/schedule_meeting?title=项目评审&date=2024-01-15"}},
	{Name: "set_reminder", Description: "Set a reminder", Category: "work", RequiredParams: []string{"message"}, OptionalParams: []string{"time"}, Examples: []string{"intent://workflow/set_reminder?message=喝水&time=14:00"}},
	{Name: "create_task", Description: "Create a task", Category: "work", RequiredParams: []string{"title"}, OptionalParams: []string{"due_date", "priority"}, Examples: []string{"intent://workflow/create_task?title=完成报告&due_date=2024-01-20"}},
	{Name: "control_device", Description: "Control smart device", Category: "system", RequiredParams: []string{"device"}, OptionalParams: []string{"action", "value"}, Examples: []string{"intent://workflow/control_device?device=灯光&action=打开"}},
	{Name: "check_weather", Description: "Check weather", Category: "system", RequiredParams: []string{}, OptionalParams: []string{"location"}, Examples: []string{"intent://workflow/check_weather?location=北京"}},
	{Name: "set_alarm", Description: "Set an alarm", Category: "system", RequiredParams: []string{"time"}, OptionalParams: []string{"label"}, Examples: []string{"intent://workflow/set_alarm?time=07:00&label=起床"}},
	{Name: "take_photo", Description: "Take a photo", Category: "system", RequiredParams: []string{}, OptionalParams: []string{"mode", "flash"}, Examples: []string{"intent://workflow/take_photo?mode=portrait"}},
	{Name: "open_app", Description: "Open an app", Category: "system", RequiredParams: []string{"app"}, OptionalParams: []string{"action"}, Examples: []string{"intent://workflow/open_app?app=微信"}},
	{Name: "research_topic", Description: "Research a topic", Category: "agent", RequiredParams: []string{"topic"}, OptionalParams: []string{"depth"}, Examples: []string{"intent://workflow/research_topic?topic=AI发展趋势"}},
	{Name: "analyze_document", Description: "Analyze a document", Category: "agent", RequiredParams: []string{"document"}, OptionalParams: []string{"analysis_type"}, Examples: []string{"intent://workflow/analyze_document?document=report.pdf"}},
	{Name: "generate_content", Description: "Generate content", Category: "agent", RequiredParams: []string{"type"}, OptionalParams: []string{"topic", "length"}, Examples: []string{"intent://workflow/generate_content?type=文章&topic=科技"}},
}

func GetWorkflowType(name string) *WorkflowType {
	for _, wt := range StandardWorkflowTypes {
		if wt.Name == name {
			return &wt
		}
	}
	return nil
}

func generateID() string {
	return generateSecureID()
}

func generateSecureID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}
