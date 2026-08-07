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

package mobile

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestHashString(t *testing.T) {
	h1 := hashString("hello")
	if len(h1) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(h1))
	}
	h2 := hashString("hello")
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	h3 := hashString("world")
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}

func TestComputeRecordHash(t *testing.T) {
	r := AuditRecord{
		WorkflowID: "wf1",
		NodeID:     "node1",
		ActorDID:   "did:test:123",
		AuditLevel: "workflow",
		InputHash:  hashString("input"),
		Timestamp:  time.Now(),
		Metadata:   map[string]interface{}{"key": "value"},
	}
	h := computeRecordHash(r)
	if len(h) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(h))
	}
	h2 := computeRecordHash(r)
	if h != h2 {
		t.Error("same record should produce same hash")
	}
	r.WorkflowID = "wf2"
	h3 := computeRecordHash(r)
	if h == h3 {
		t.Error("different record should produce different hash")
	}
}

func TestIsValidDID(t *testing.T) {
	tests := []struct {
		did    string
		expect bool
	}{
		{"", false},
		{"did", false},
		{"did:", false},
		{"did:method", false},
		{"did:method:", false},
		{"did:method:id", true},
		{"did:example:123456789", true},
		{"did:key:z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoANm3PaZ5fS", true},
		{"DID:method:id", false},
		{strings.Repeat("a", 2049), false},
		{"xyz:method:id", false},
	}
	for _, tt := range tests {
		got := isValidDID(tt.did)
		if got != tt.expect {
			t.Errorf("isValidDID(%q) = %v, want %v", tt.did, got, tt.expect)
		}
	}
}

func TestSafeParseMetadata(t *testing.T) {
	tests := []struct {
		input string
		check func(map[string]interface{}) bool
	}{
		{"", func(m map[string]interface{}) bool { return m == nil }},
		{`{"key":"value"}`, func(m map[string]interface{}) bool {
			return m != nil && m["key"] == "value"
		}},
		{"not json", func(m map[string]interface{}) bool {
			return m != nil && m["raw"] == "not json"
		}},
	}
	for i, tt := range tests {
		got := safeParseMetadata(tt.input)
		if !tt.check(got) {
			t.Errorf("case %d: safeParseMetadata(%q) failed check", i, tt.input)
		}
	}
}

func TestVerifyAuditChain(t *testing.T) {
	ok, errs := VerifyAuditChain()
	if !ok || len(errs) != 0 {
		t.Error("empty chain should be valid")
	}
}

func TestQueryAuditLog(t *testing.T) {
	results := QueryAuditLog("", time.Time{})
	if len(results) != 0 {
		t.Error("empty log should return no results")
	}
	results = QueryAuditLog("nonexistent", time.Time{})
	if len(results) != 0 {
		t.Error("no match should return no results")
	}
}

func TestBlockchainAuditNode_Metadata(t *testing.T) {
	node := &BlockchainAuditNode{}
	if node.Name() != "blockchain_audit" {
		t.Errorf("expected blockchain_audit, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Name != "blockchain_audit" {
		t.Errorf("expected schema name blockchain_audit, got %s", schema.Name)
	}
}

func TestBlockchainAuditNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &BlockchainAuditNode{}

	_, err := node.Execute(ctx, "test", map[string]string{"chain_type": "invalid"})
	if err == nil {
		t.Error("expected error for invalid chain_type")
	}

	_, err = node.Execute(ctx, "test", map[string]string{"audit_level": "invalid"})
	if err == nil {
		t.Error("expected error for invalid audit_level")
	}

	_, err = node.Execute(ctx, "test", map[string]string{})
	if err == nil {
		t.Error("expected error for missing workflow_id")
	}

	_, err = node.Execute(ctx, "test", map[string]string{"workflow_id": strings.Repeat("a", 129)})
	if err == nil {
		t.Error("expected error for too long workflow_id")
	}

	_, err = node.Execute(ctx, "test", map[string]string{"workflow_id": "wf1", "node_id": strings.Repeat("a", 129)})
	if err == nil {
		t.Error("expected error for too long node_id")
	}
}

func TestSimulatedSubmit(t *testing.T) {
	record := AuditRecord{
		WorkflowID: "wf1",
		NodeID:     "n1",
		RecordHash: hashString("test"),
	}
	receipt, err := simulatedSubmit(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receipt == nil {
		t.Fatal("expected non-nil receipt")
	}
	if receipt.Status != "confirmed" {
		t.Errorf("expected confirmed status, got %s", receipt.Status)
	}
	if receipt.TxHash == "" {
		t.Error("expected non-empty tx hash")
	}
}

func TestSimulateBatteryAndTemp(t *testing.T) {
	bat := simulateBatteryLevel()
	if bat < 0 || bat > 100 {
		t.Errorf("battery level out of range: %d", bat)
	}

	temp := simulateCPUTemperature()
	if temp < 0 || temp > 100 {
		t.Errorf("CPU temp out of range: %f", temp)
	}

	_ = simulateChargingStatus()
}

func TestSimulateInference(t *testing.T) {
	tests := []struct {
		input   string
		wantSub string
	}{
		{"请翻译这段话", "Translation"},
		{"summarize this", "Summary"},
		{"写一段代码", "```"},
		{"hello", "On-device"},
	}
	for _, tt := range tests {
		got := simulateInference("qwen2.5-3b", tt.input, "", 100)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateInference(%q) = %q, want substring %q", tt.input, got, tt.wantSub)
		}
	}
}

func TestGetMapKeys(t *testing.T) {
	m := map[string]bool{"a": true, "b": true, "c": true}
	keys := getMapKeys(m)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	empty := getMapKeys(map[string]bool{})
	if len(empty) != 0 {
		t.Errorf("expected 0 keys, got %d", len(empty))
	}
}

func TestSimulateVideoProcessing(t *testing.T) {
	ops := []string{"smart_cut", "merge", "effects", "subtitle", "storyboard", "upscale"}
	for _, op := range ops {
		_, result := simulateVideoProcessing(op, []string{"input.mp4"}, "cinematic", 30.0, "1080p", "zh")
		if result == "" {
			t.Errorf("empty result for op %s", op)
		}
	}
}

func TestSimulateVAD(t *testing.T) {
	result := simulateVAD("some audio", "normal")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasVoice {
		t.Error("expected voice detected for non-empty input")
	}

	result = simulateVAD("", "aggressive")
	if result.HasVoice {
		t.Error("expected no voice for empty input")
	}

	_ = simulateVAD("test", "fixed")
}

func TestSimulateWakeWordDetection(t *testing.T) {
	result := simulateWakeWordDetection("hey box, what's up", "hey_box")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Detected {
		t.Error("expected wake word detected")
	}

	result = simulateWakeWordDetection("hello", "hey_box")
	if result.Detected {
		t.Error("expected no wake word detected")
	}

	result = simulateWakeWordDetection("unknown wake word", "custom_wake")
	if result == nil {
		t.Fatal("expected non-nil result for unknown wake word")
	}
}

func TestSimulateASR(t *testing.T) {
	langs := []string{"zh", "en", "ja", "ko", "fr", "de", "es", "unknown"}
	for _, lang := range langs {
		result := simulateASR("audio data", lang, false, 0.5)
		if result == nil {
			t.Fatalf("expected non-nil result for lang %s", lang)
		}
		if result.Text == "" {
			t.Errorf("empty text for lang %s", lang)
		}
	}

	result := simulateASR("weather query", "zh", false, 0.5)
	if !strings.Contains(result.Text, "天气") {
		t.Errorf("expected weather in result, got %q", result.Text)
	}

	result = simulateASR("remind me", "en", true, 0.5)
	if !strings.Contains(strings.ToLower(result.Text), "remind") {
		t.Errorf("expected remind in result, got %q", result.Text)
	}
}

func TestSimulateTTSGeneration(t *testing.T) {
	result, duration := simulateTTSGeneration("hello world", "local", "speak", "default", "normal", 1.0, 1.0, "", "wav")
	if result == "" {
		t.Error("expected non-empty result")
	}
	if duration <= 0 {
		t.Errorf("expected positive duration, got %f", duration)
	}
}

func TestSimulateEventData(t *testing.T) {
	eventTypes := []string{"notification", "incoming_call", "sms_received", "battery_low", "location_changed", "alarm_triggered", "wifi_connected", "unknown"}
	for _, et := range eventTypes {
		result := simulateEventData(et, "test input", "", "", 20, 100)
		if result == nil {
			t.Fatalf("expected non-nil result for event type %s", et)
		}
	}

	result := simulateEventData("battery_low", "test", "", "", 10, 100)
	if result != nil {
		t.Error("expected nil when battery level above threshold")
	}

	result = simulateEventData("notification", "test", "nonexistent", "", 20, 100)
	if result != nil {
		t.Error("expected nil when app filter doesn't match")
	}
}

func TestQuantizationForProfile(t *testing.T) {
	tests := []struct {
		profile PowerProfile
		want    string
	}{
		{PowerProfileEco, "int4"},
		{PowerProfileBalanced, "int8"},
		{PowerProfileHigh, "fp16"},
		{PowerProfile("unknown"), "int8"},
	}
	for _, tt := range tests {
		got := quantizationForProfile(tt.profile)
		if got != tt.want {
			t.Errorf("quantizationForProfile(%v) = %q, want %q", tt.profile, got, tt.want)
		}
	}
}

func TestThreadsForProfile(t *testing.T) {
	profiles := []PowerProfile{PowerProfileEco, PowerProfileBalanced, PowerProfileHigh, PowerProfile("unknown")}
	for _, p := range profiles {
		got := threadsForProfile(p)
		if got < 1 {
			t.Errorf("threadsForProfile(%v) = %d, want >= 1", p, got)
		}
	}
}

func TestContextSizeForProfile(t *testing.T) {
	tests := []struct {
		profile PowerProfile
		want    int
	}{
		{PowerProfileEco, 2048},
		{PowerProfileBalanced, 4096},
		{PowerProfileHigh, 8192},
		{PowerProfile("unknown"), 4096},
	}
	for _, tt := range tests {
		got := contextSizeForProfile(tt.profile)
		if got != tt.want {
			t.Errorf("contextSizeForProfile(%v) = %d, want %d", tt.profile, got, tt.want)
		}
	}
}

func TestSimulateBatteryTempCharging(t *testing.T) {
	bat := simulateBatteryLevel()
	if bat < 0 || bat > 100 {
		t.Errorf("invalid battery level: %d", bat)
	}

	temp := simulateCPUTemperature()
	if temp < -50 || temp > 200 {
		t.Errorf("invalid CPU temperature: %f", temp)
	}

	_ = simulateChargingStatus()
}

func TestRegexpFindAllStringSubmatch(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`(\w+)=(\d+)`, "a=1 b=2 c=3", -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0][1] != "a" || matches[0][2] != "1" {
		t.Errorf("expected first match [a, 1], got %v", matches[0])
	}
}

func TestRegexpFindAllStringSubmatch_Limit(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`\d+`, "1 2 3 4 5", 2)
	if len(matches) != 2 {
		t.Errorf("expected 2 matches with limit 2, got %d", len(matches))
	}
}

func TestRegexpFindAllStringSubmatch_InvalidPattern(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`[invalid`, "test", -1)
	if matches != nil {
		t.Errorf("expected nil for invalid pattern, got %v", matches)
	}
}

func TestRegexpFindAllStringSubmatch_NoMatch(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`\d+`, "no digits here", -1)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestExtractLinksFromText_MarkdownLinks(t *testing.T) {
	text := "Check [Google](https://google.com) and [GitHub](https://github.com)"
	links := extractLinksFromText(text, "")

	if len(links) < 2 {
		t.Fatalf("expected at least 2 links, got %d", len(links))
	}
	if links[0].Title != "Google" || links[0].URL != "https://google.com" {
		t.Errorf("unexpected first link: %+v", links[0])
	}
}

func TestExtractLinksFromText_RawURLs(t *testing.T) {
	text := "Visit https://example.com for more info"
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %q", links[0].URL)
	}
}

func TestExtractLinksFromText_Deduplication(t *testing.T) {
	text := "Visit https://same.com twice https://same.com"
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Errorf("expected 1 deduplicated link, got %d", len(links))
	}
}

func TestExtractLinksFromText_LongURLTitle(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", 100)
	text := "Check " + longURL
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if len(links[0].Title) > 83 {
		t.Errorf("expected title truncated to 80+3 chars, got %d", len(links[0].Title))
	}
	if !strings.HasSuffix(links[0].Title, "...") {
		t.Errorf("expected title to end with '...', got %q", links[0].Title)
	}
}

func TestExtractLinksFromText_NoLinks(t *testing.T) {
	text := "Just plain text without any links"
	links := extractLinksFromText(text, "")
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func BenchmarkRegexCache_Hit(b *testing.B) {
	pattern := `https?://[^\s)]+`
	text := strings.Repeat("Visit https://example.com for more info. ", 10)
	for i := 0; i < b.N; i++ {
		regexpFindAllStringSubmatch(pattern, text, -1)
	}
}

func BenchmarkExtractLinksFromText(b *testing.B) {
	text := strings.Repeat("Check [Google](https://google.com) and https://github.com for info. ", 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromText(text, "")
	}
}
