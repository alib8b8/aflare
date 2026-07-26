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

package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseIntentURIBasic(t *testing.T) {
	uri := "intent://workflow/send_message?to=%E5%BC%A0%E4%B8%89&body=hello"
	intent, err := ParseIntentURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Type != "send_message" {
		t.Errorf("expected type send_message, got %s", intent.Type)
	}
	if intent.Params["to"] != "张三" {
		t.Errorf("expected to=张三, got %s", intent.Params["to"])
	}
	if intent.Params["body"] != "hello" {
		t.Errorf("expected body=hello, got %s", intent.Params["body"])
	}
}

func TestParseIntentURIOhos(t *testing.T) {
	uri := "ohos://workflow/send_message?to=test&body=hello"
	intent, err := ParseIntentURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Type != "send_message" {
		t.Errorf("expected type send_message, got %s", intent.Type)
	}
}

func TestParseIntentURIInvalidScheme(t *testing.T) {
	_, err := ParseIntentURI("http://workflow/send_message")
	if err == nil {
		t.Fatal("expected error for http scheme")
	}
}

func TestParseIntentURIMissingType(t *testing.T) {
	_, err := ParseIntentURI("intent://?foo=bar")
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestParseIntentURITooLong(t *testing.T) {
	longURI := "intent://workflow/send_message?body=" + strings.Repeat("a", 5000)
	_, err := ParseIntentURI(longURI)
	if err == nil {
		t.Fatal("expected error for too long URI")
	}
}

func TestParseIntentURIPriority(t *testing.T) {
	for _, p := range []string{"low", "normal", "high", "urgent"} {
		uri := "intent://workflow/send_message?to=test&priority=" + p
		intent, err := ParseIntentURI(uri)
		if err != nil {
			t.Fatalf("unexpected error for priority %s: %v", p, err)
		}
		if intent.Priority != p {
			t.Errorf("expected priority %s, got %s", p, intent.Priority)
		}
	}
}

func TestParseIntentURITimeout(t *testing.T) {
	validCases := []string{"30s", "5m", "1h"}
	for _, tc := range validCases {
		uri := "intent://workflow/send_message?to=test&timeout=" + tc
		_, err := ParseIntentURI(uri)
		if err != nil {
			t.Errorf("unexpected error for timeout %s: %v", tc, err)
		}
	}

	invalidCases := []string{"invalid", "0s", "-1m", "48h"}
	for _, tc := range invalidCases {
		uri := "intent://workflow/send_message?to=test&timeout=" + tc
		_, err := ParseIntentURI(uri)
		if err == nil {
			t.Errorf("expected error for invalid timeout %s", tc)
		}
	}
}

func TestParseIntentURICallback(t *testing.T) {
	validCases := []string{
		"http://example.com/callback",
		"https://example.com/callback",
	}
	for _, cb := range validCases {
		uri := "intent://workflow/send_message?to=test&callback=" + cb
		_, err := ParseIntentURI(uri)
		if err != nil {
			t.Errorf("unexpected error for callback %s: %v", cb, err)
		}
	}

	invalidCases := []string{
		"ftp://example.com/callback",
		"http://user:pass@example.com/callback",
	}
	for _, cb := range invalidCases {
		uri := "intent://workflow/send_message?to=test&callback=" + cb
		_, err := ParseIntentURI(uri)
		if err == nil {
			t.Errorf("expected error for invalid callback %s", cb)
		}
	}
}

func TestIntentURIString(t *testing.T) {
	uri := "intent://workflow/send_message?body=hello&to=test"
	intent, err := ParseIntentURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str := intent.String()
	if !strings.HasPrefix(str, "intent://workflow/send_message?") {
		t.Errorf("unexpected string output: %s", str)
	}

	parsed2, err := ParseIntentURI(str)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if parsed2.Type != intent.Type {
		t.Errorf("type mismatch after round-trip")
	}
}

func TestIntentURIToJSON(t *testing.T) {
	uri := "intent://workflow/send_message?to=test&body=hello"
	intent, err := ParseIntentURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jsonStr, err := intent.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var decoded IntentURI
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if decoded.Type != "send_message" {
		t.Errorf("type mismatch in JSON")
	}
}

func TestDIDIdentityValidate(t *testing.T) {
	did := &DIDIdentity{
		DID:    "did:key:abc123",
		Method: "key",
	}
	if err := did.Validate(); err != nil {
		t.Errorf("expected valid DID, got error: %v", err)
	}
}

func TestDIDIdentityInvalidFormat(t *testing.T) {
	did := &DIDIdentity{DID: "notadid:key:abc"}
	err := did.Validate()
	if err == nil {
		t.Fatal("expected error for missing did: prefix")
	}
}

func TestDIDIdentityInvalidMethod(t *testing.T) {
	did := &DIDIdentity{DID: "did:KEY:abc123"}
	err := did.Validate()
	if err == nil {
		t.Fatal("expected error for uppercase method")
	}
}

func TestDIDIdentityValidateEndpoint(t *testing.T) {
	did := &DIDIdentity{
		DID:      "did:key:abc123",
		Method:   "key",
		Endpoint: "http://127.0.0.1:8080/endpoint",
	}
	err := did.Validate()
	if err == nil {
		t.Fatal("expected error for private IP endpoint (SSRF protection)")
	}
}

func TestDIDIdentityVerifySignature(t *testing.T) {
	did := &DIDIdentity{
		DID:       "did:key:abc123",
		Signature: "",
	}
	err := did.VerifySignature("test payload")
	if err == nil {
		t.Fatal("expected error for empty signature")
	}

	did.Signature = "short"
	err = did.VerifySignature("test payload")
	if err == nil {
		t.Fatal("expected error for too short signature")
	}

	did.Signature = "valid_signature_12345"
	err = did.VerifySignature("test payload")
	if err != nil {
		t.Errorf("unexpected error for valid signature: %v", err)
	}
}

func TestNewDIDIdentity(t *testing.T) {
	did := NewDIDIdentity("key", "abc123")
	if did.DID != "did:key:abc123" {
		t.Errorf("expected did:key:abc123, got %s", did.DID)
	}
	if did.Method != "key" {
		t.Errorf("expected method key, got %s", did.Method)
	}
}

func TestDIDToDocument(t *testing.T) {
	did := NewDIDIdentity("key", "abc123")
	did.PublicKey = "testpubkey"
	did.Endpoint = "https://example.com/agent"

	doc := did.ToDIDDocument()
	if doc.ID != "did:key:abc123" {
		t.Errorf("expected doc ID did:key:abc123, got %s", doc.ID)
	}
	if len(doc.PublicKeys) != 1 {
		t.Errorf("expected 1 public key, got %d", len(doc.PublicKeys))
	}
	if len(doc.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(doc.Services))
	}
	if len(doc.Authentication) != 1 {
		t.Errorf("expected 1 authentication, got %d", len(doc.Authentication))
	}
}

func TestNewTaskMessage(t *testing.T) {
	intent := &IntentURI{
		Type:   "send_message",
		Params: map[string]string{"to": "test"},
	}

	msg := NewTaskMessage(intent)
	if msg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if msg.Version != Version {
		t.Errorf("expected version %s, got %s", Version, msg.Version)
	}
	if msg.Status != TaskStatusPending {
		t.Errorf("expected pending status, got %s", msg.Status)
	}
	if msg.Intent != intent {
		t.Error("expected intent to be set")
	}
}

func TestParseTaskMessage(t *testing.T) {
	intent := &IntentURI{
		Type:   "send_message",
		Params: map[string]string{"to": "test"},
	}
	msg := NewTaskMessage(intent)

	jsonStr, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	parsed, err := ParseTaskMessage(jsonStr)
	if err != nil {
		t.Fatalf("ParseTaskMessage failed: %v", err)
	}
	if parsed.ID != msg.ID {
		t.Errorf("ID mismatch: %s vs %s", parsed.ID, msg.ID)
	}
	if parsed.Intent.Type != "send_message" {
		t.Errorf("type mismatch: %s", parsed.Intent.Type)
	}
}

func TestParseTaskMessageInvalid(t *testing.T) {
	intent := &IntentURI{Type: "send_message"}
	msg := NewTaskMessage(intent)

	jsonStr, _ := msg.ToJSON()
	var decoded map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &decoded)

	delete(decoded, "id")
	data, _ := json.Marshal(decoded)
	_, err := ParseTaskMessage(string(data))
	if err == nil {
		t.Error("expected error for missing ID")
	}

	jsonStr, _ = msg.ToJSON()
	var decoded2 map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &decoded2)
	delete(decoded2, "intent")
	data, _ = json.Marshal(decoded2)
	_, err = ParseTaskMessage(string(data))
	if err == nil {
		t.Error("expected error for missing intent")
	}

	jsonStr, _ = msg.ToJSON()
	var decoded3 map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &decoded3)
	decoded3["status"] = "invalid_status"
	data, _ = json.Marshal(decoded3)
	_, err = ParseTaskMessage(string(data))
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestParseTaskMessageCrossDomain(t *testing.T) {
	intent := &IntentURI{Type: "send_message"}
	msg := NewTaskMessage(intent)
	msg.CrossDomain = true

	sender := NewDIDIdentity("key", "sender123")
	receiver := NewDIDIdentity("key", "receiver456")
	msg.Sender = sender
	msg.Receiver = receiver

	jsonStr, _ := msg.ToJSON()
	parsed, err := ParseTaskMessage(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.CrossDomain {
		t.Error("expected cross_domain=true")
	}
}

func TestParseTaskMessageLocalNoDID(t *testing.T) {
	intent := &IntentURI{Type: "send_message"}
	msg := NewTaskMessage(intent)
	msg.CrossDomain = false
	msg.Sender = NewDIDIdentity("key", "sender123")

	jsonStr, _ := msg.ToJSON()
	_, err := ParseTaskMessage(jsonStr)
	if err == nil {
		t.Error("expected error when sender set without cross_domain=true")
	}
}

func TestGetWorkflowType(t *testing.T) {
	wt := GetWorkflowType("book_flight")
	if wt == nil {
		t.Fatal("expected book_flight to exist")
	}
	if wt.Name != "book_flight" {
		t.Errorf("expected name book_flight, got %s", wt.Name)
	}

	wt = GetWorkflowType("nonexistent_workflow")
	if wt != nil {
		t.Error("expected nil for unknown workflow type")
	}
}

func TestGenerateSecureID(t *testing.T) {
	id1 := generateSecureID()
	id2 := generateSecureID()

	if id1 == "" {
		t.Error("expected non-empty ID")
	}
	if len(id1) == 0 {
		t.Error("expected consistent non-zero length")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
}
