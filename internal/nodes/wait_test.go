// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​​‌‌‌​​‌‌‌​​​​‌​​‌‌‌​​​‌‌​​‌​‌​‌‌​‌​‌‌​‌‌​​​​​‌​​​‌​‌​​​​​​​​​​​​​​​​‌‌​​​‌‌​​‌​‌‌‌​‌⁠
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

package nodes

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWaitNode_RegisteredAndNamed(t *testing.T) {
	n := &WaitNode{}
	if n.Name() != "wait" {
		t.Fatalf("Name() = %q, want wait", n.Name())
	}
	// init() registration must have happened for `aflare list` discovery.
	if _, ok := GetGlobalRegistry().Get("wait"); !ok {
		t.Fatal("wait node not registered in the global registry")
	}
}

func TestWaitNode_PassesInputThroughAfterWait(t *testing.T) {
	n := &WaitNode{}
	start := time.Now()
	out, err := n.Execute(context.Background(), "payload", map[string]string{"duration": "50ms"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "payload" {
		t.Errorf("output = %q, want pass-through %q", out, "payload")
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("wait returned after %v, before the requested 50ms", elapsed)
	}
}

func TestWaitNode_ZeroDurationIsPassThrough(t *testing.T) {
	// duration "0s" is a no-op pipeline element, not an error: generators
	// may emit it for templates that have waiting disabled.
	n := &WaitNode{}
	out, err := n.Execute(context.Background(), "keep", map[string]string{"duration": "0s"})
	if err != nil {
		t.Fatalf("unexpected error for 0s: %v", err)
	}
	if out != "keep" {
		t.Errorf("output = %q, want %q", out, "keep")
	}
}

func TestWaitNode_MissingDuration(t *testing.T) {
	n := &WaitNode{}
	_, err := n.Execute(context.Background(), "x", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("expected missing-duration error, got %v", err)
	}
}

func TestWaitNode_InvalidDuration(t *testing.T) {
	n := &WaitNode{}
	// Bare numbers are the classic footgun ("10" means 10 nanoseconds in
	// Go, which is never what the user meant) — reject with guidance.
	for _, bad := range []string{"10", "abc", "1x"} {
		_, err := n.Execute(context.Background(), "x", map[string]string{"duration": bad})
		if err == nil {
			t.Errorf("duration %q: expected error, got nil", bad)
		}
	}
}

func TestWaitNode_NegativeDurationRejected(t *testing.T) {
	n := &WaitNode{}
	_, err := n.Execute(context.Background(), "x", map[string]string{"duration": "-5s"})
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected negative-duration error, got %v", err)
	}
}

func TestWaitNode_OverCapRejected(t *testing.T) {
	n := &WaitNode{}
	_, err := n.Execute(context.Background(), "x", map[string]string{"duration": "2h"})
	if err == nil || !strings.Contains(err.Error(), "schedule") {
		t.Fatalf("expected over-cap error pointing at aflare schedule, got %v", err)
	}
}

func TestWaitNode_ContextCancelInterruptsWait(t *testing.T) {
	// The core correctness property: a cancelled workflow (or an expired
	// per-step timeout) must interrupt a pending wait immediately, not
	// sleep the full duration.
	n := &WaitNode{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := n.Execute(ctx, "x", map[string]string{"duration": "10s"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if elapsed > time.Second {
		t.Fatalf("wait did not honor cancellation: took %v, want immediate return", elapsed)
	}
}
