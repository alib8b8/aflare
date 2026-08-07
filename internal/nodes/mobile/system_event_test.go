// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestSystemEventNode_Metadata(t *testing.T) {
	node := &SystemEventNode{}
	if node.Name() != "system_event" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "system_event" {
		t.Errorf("schema name: %s", schema.Name)
	}
}

func TestSystemEventNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &SystemEventNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing event_type", map[string]string{}, "event_type parameter is required"},
		{"invalid event_type", map[string]string{"event_type": "unknown_event"}, "invalid event_type"},
		{"invalid trigger_mode", map[string]string{"event_type": "notification", "trigger_mode": "fast"}, "invalid trigger_mode"},
		{"invalid filter_app", map[string]string{"event_type": "notification", "filter_app": "bad app!"}, "invalid filter_app"},
		{"filter_keyword too long", map[string]string{"event_type": "notification", "filter_keyword": strings.Repeat("a", 201)}, "filter_keyword too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestSystemEventNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &SystemEventNode{}

	tests := []struct {
		name        string
		params      map[string]string
		wantSub     string
		wantMatched bool
	}{
		{
			"notification matched",
			map[string]string{"event_type": "notification"},
			"system_event",
			true,
		},
		{
			"incoming_call",
			map[string]string{"event_type": "incoming_call"},
			"incoming_call",
			true,
		},
		{
			"sms_received",
			map[string]string{"event_type": "sms_received"},
			"sms_received",
			true,
		},
		{
			"battery_low matched (threshold=20)",
			map[string]string{"event_type": "battery_low", "battery_threshold": "20"},
			"battery_low",
			true,
		},
		{
			"battery_low NOT matched (threshold=10, level=15)",
			map[string]string{"event_type": "battery_low", "battery_threshold": "10"},
			"\"matched\": false",
			false,
		},
		{
			"location_changed",
			map[string]string{"event_type": "location_changed", "location_radius_m": "200"},
			"location_changed",
			true,
		},
		{
			"alarm_triggered",
			map[string]string{"event_type": "alarm_triggered"},
			"alarm_triggered",
			true,
		},
		{
			"wifi_connected",
			map[string]string{"event_type": "wifi_connected"},
			"wifi_connected",
			true,
		},
		{
			"default event type",
			map[string]string{"event_type": "screen_on"},
			"screen_on",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := node.Execute(ctx, "input", tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.wantSub) {
				t.Errorf("expected %q in output: %s", tt.wantSub, out)
			}
		})
	}
}

func TestSystemEventNode_ExecuteParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &SystemEventNode{}

	// Out-of-range debounce_ms, battery_threshold, location_radius_m should be clamped
	out, err := node.Execute(ctx, "", map[string]string{
		"event_type":        "notification",
		"debounce_ms":       "99999", // > 60000, falls back to 1000
		"battery_threshold": "200",   // > 100, falls back to 20
		"location_radius_m": "99999", // > 10000, falls back to 100
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"debounce_ms\": 1000") {
		t.Errorf("expected clamped debounce_ms 1000: %s", out)
	}
}

// -----------------------------------------------------------------
// Subscription management
// -----------------------------------------------------------------

func TestSubscribeAndUnsubscribeEvent(t *testing.T) {
	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "immediate",
	}

	id := SubscribeEvent(sub)
	if id == "" {
		t.Fatal("expected non-empty subscription id")
	}
	if !sub.Active {
		t.Error("expected subscription to be active")
	}
	if sub.ID != id {
		t.Errorf("ID: got %q, want %q", sub.ID, id)
	}

	// Unsubscribe
	if !UnsubscribeEvent(id) {
		t.Error("expected UnsubscribeEvent to return true")
	}
	if sub.Active {
		t.Error("expected subscription to be inactive after unsubscribe")
	}

	// Unsubscribing again should return false
	if UnsubscribeEvent(id) {
		t.Error("expected UnsubscribeEvent to return false for already-removed subscription")
	}
}

func TestDispatchEvent_Immediate(t *testing.T) {
	var triggered atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "immediate",
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
			wg.Done()
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	DispatchEvent("notification", map[string]interface{}{
		"package_name": "com.test",
		"title":        "Hello",
		"content":      "World",
	})

	// Wait for callback (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback not triggered within 2s")
	}
	if triggered.Load() != 1 {
		t.Errorf("expected 1 trigger, got %d", triggered.Load())
	}
}

func TestDispatchEvent_FilterByApp(t *testing.T) {
	var triggered atomic.Int32
	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "immediate",
		FilterApp:   "com.target",
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	// Non-matching app: callback should not fire
	DispatchEvent("notification", map[string]interface{}{
		"package_name": "com.other",
	})

	// Give the goroutine time to (not) run
	time.Sleep(100 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Errorf("expected 0 triggers for non-matching app, got %d", triggered.Load())
	}

	// Matching app: callback should fire
	var wg sync.WaitGroup
	wg.Add(1)
	sub.Callback = func(event map[string]interface{}) {
		triggered.Add(1)
		wg.Done()
	}
	DispatchEvent("notification", map[string]interface{}{
		"package_name": "com.target.app",
	})
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback not triggered for matching app")
	}
	if triggered.Load() != 1 {
		t.Errorf("expected 1 trigger for matching app, got %d", triggered.Load())
	}
}

func TestDispatchEvent_FilterByKeyword(t *testing.T) {
	var triggered atomic.Int32
	sub := &EventSubscription{
		EventType:     "notification",
		TriggerMode:   "immediate",
		FilterKeyword: "urgent",
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	// Non-matching keyword
	DispatchEvent("notification", map[string]interface{}{
		"package_name": "com.test",
		"content":      "regular message",
	})
	time.Sleep(100 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Errorf("expected 0 triggers for non-matching keyword, got %d", triggered.Load())
	}

	// Matching keyword
	var wg sync.WaitGroup
	wg.Add(1)
	sub.Callback = func(event map[string]interface{}) {
		triggered.Add(1)
		wg.Done()
	}
	DispatchEvent("notification", map[string]interface{}{
		"package_name": "com.test",
		"content":      "urgent: please respond",
	})
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback not triggered for matching keyword")
	}
}

func TestDispatchEvent_Throttle(t *testing.T) {
	var triggered atomic.Int32
	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "throttle",
		DebounceMs:  1000,
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	// First dispatch triggers
	DispatchEvent("notification", map[string]interface{}{"content": "first"})
	// Wait for first callback to complete
	time.Sleep(100 * time.Millisecond)

	// Second immediate dispatch should be throttled (not fire)
	DispatchEvent("notification", map[string]interface{}{"content": "second"})
	time.Sleep(100 * time.Millisecond)

	if triggered.Load() != 1 {
		t.Errorf("expected 1 trigger (throttled), got %d", triggered.Load())
	}
}

func TestDispatchEvent_DebounceAlwaysFires(t *testing.T) {
	var triggered atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2) // both should fire

	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "debounce",
		DebounceMs:  1000,
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
			wg.Done()
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	DispatchEvent("notification", map[string]interface{}{"content": "first"})
	DispatchEvent("notification", map[string]interface{}{"content": "second"})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("debounce callbacks not triggered")
	}
	if triggered.Load() != 2 {
		t.Errorf("expected 2 triggers (debounce always fires), got %d", triggered.Load())
	}
}

func TestDispatchEvent_WrongEventType(t *testing.T) {
	var triggered atomic.Int32
	sub := &EventSubscription{
		EventType:   "notification",
		TriggerMode: "immediate",
		Callback: func(event map[string]interface{}) {
			triggered.Add(1)
		},
	}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	DispatchEvent("sms_received", map[string]interface{}{})
	time.Sleep(100 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Errorf("expected 0 triggers for non-matching event type, got %d", triggered.Load())
	}
}

func TestSafeCallback_PanicRecovery(t *testing.T) {
	// Should not panic even if callback panics
	cb := func(event map[string]interface{}) {
		panic("intentional panic")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("safeCallback should not propagate panic: %v", r)
		}
	}()
	safeCallback(cb, map[string]interface{}{"k": "v"})
}

func TestSafeCallback_NilCallback(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil callback should not panic: %v", r)
		}
	}()
	safeCallback(nil, map[string]interface{}{})
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty IDs")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if !strings.HasPrefix(id1, "sub_") {
		t.Errorf("expected ID to start with 'sub_', got %q", id1)
	}
}

func TestCleanupExpiredSubscriptionsLocked(t *testing.T) {
	// Add an expired subscription manually
	eventSubscriptionsMu.Lock()
	oldCount := len(eventSubscriptions)
	eventSubscriptions["expired_sub"] = &EventSubscription{
		ID:        "expired_sub",
		EventType: "notification",
		Active:    true,
		createdAt: time.Now().Add(-25 * time.Hour), // expired
	}
	eventSubscriptionsMu.Unlock()

	// Trigger cleanup by subscribing a new event
	sub := &EventSubscription{EventType: "notification", TriggerMode: "immediate"}
	id := SubscribeEvent(sub)
	defer UnsubscribeEvent(id)

	eventSubscriptionsMu.RLock()
	_, stillExists := eventSubscriptions["expired_sub"]
	eventSubscriptionsMu.RUnlock()

	if stillExists {
		t.Error("expected expired subscription to be removed")
	}

	// Ensure we didn't lose subscriptions beyond the expired one
	eventSubscriptionsMu.RLock()
	newCount := len(eventSubscriptions)
	eventSubscriptionsMu.RUnlock()
	if newCount < oldCount {
		t.Errorf("subscription count should not have decreased below original (excluding expired): old=%d, new=%d", oldCount, newCount)
	}
}

// Ensure system_event node was registered.
func TestSystemEventNode_Registered(t *testing.T) {
	if _, ok := core.Get("system_event"); !ok {
		t.Error("system_event not registered")
	}
}
