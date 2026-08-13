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

package agent

import (
	"context"
	"strings"
	"testing"
)

// setupLearningStore redirects the shared learning store to a temp HOME so
// adaptive tests that call appendAdaptiveFeedback / loadRecentAdaptiveFeedback
// do not pollute the real ~/.config/aflare/learning.json.
//
// We redirect $HOME rather than sharedLearning.path because loadEntries()
// unconditionally calls initLearningStore(), which rebuilds the path from
// os.UserHomeDir() — so any direct path mutation would be overwritten on the
// next read. t.Setenv restores $HOME automatically on cleanup.
func setupLearningStore(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	// Reset sharedLearning state so each test starts clean. The path will be
	// rebuilt to point inside the temp HOME on the next initLearningStore().
	sharedLearning.mu.Lock()
	sharedLearning.path = ""
	sharedLearning.appendCount = 0
	sharedLearning.recentKeys = nil
	sharedLearning.mu.Unlock()
}

func TestAdaptiveCapability_Basic(t *testing.T) {
	setupLearningStore(t)
	a := NewAdaptiveCapability()
	if a.Name() != "adaptive" {
		t.Errorf("Name() = %q, want %q", a.Name(), "adaptive")
	}
	if a.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(a.Description(), "adaptation") {
		t.Errorf("Description() = %q, want it to mention adaptation", a.Description())
	}
	if err := a.Shutdown(); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestAdaptiveCapability_Init_Empty(t *testing.T) {
	setupLearningStore(t)
	a := NewAdaptiveCapability()
	// No learning file yet → loadRecentAdaptiveFeedback returns nil.
	if err := a.Init(&AgentLoop{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if len(a.feedback) != 0 {
		t.Errorf("expected 0 feedback entries with empty store, got %d", len(a.feedback))
	}
}

func TestAdaptiveCapability_Init_LoadsPast(t *testing.T) {
	setupLearningStore(t)
	// Seed the store with past adaptive feedback by running a PostProcess
	// that records an error pattern.
	seed := NewAdaptiveCapability()
	_ = seed.Init(&AgentLoop{})
	_, _ = seed.PostProcess(context.Background(), "task", "error: something failed")

	// A fresh capability should load the past feedback on Init.
	a := NewAdaptiveCapability()
	if err := a.Init(&AgentLoop{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if len(a.feedback) == 0 {
		t.Fatal("expected past feedback to be loaded on Init")
	}
}

func TestAdaptiveCapability_PreProcess(t *testing.T) {
	t.Run("empty feedback returns empty", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		out, err := a.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output with no feedback, got %q", out)
		}
	})

	t.Run("feedback within range injects context", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		a.feedback = []string{"avoid errors", "be concise"}
		out, err := a.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Adaptive Learning") {
			t.Errorf("expected adaptive context injection, got %q", out)
		}
		if !strings.Contains(out, "hello") {
			t.Error("expected original input preserved")
		}
		if !strings.Contains(out, "avoid errors") {
			t.Error("expected feedback content in context")
		}
	})

	t.Run("too much feedback returns empty", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		a.feedback = make([]string, 11) // > 10 → skip injection
		out, err := a.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output when feedback > 10, got %q", out)
		}
	})
}

func TestAdaptiveCapability_PostProcess(t *testing.T) {
	t.Run("error output records feedback", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		out, err := a.PostProcess(context.Background(), "task", "error: bad thing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if len(a.feedback) == 0 {
			t.Fatal("expected feedback to be recorded for error output")
		}
	})

	t.Run("failed output records feedback", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		_, _ = a.PostProcess(context.Background(), "task", "operation failed")
		if len(a.feedback) == 0 {
			t.Fatal("expected feedback to be recorded for failed output")
		}
	})

	t.Run("normal output returns empty", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		out, err := a.PostProcess(context.Background(), "task", "success: all good")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output for normal response, got %q", out)
		}
		if len(a.feedback) != 0 {
			t.Error("normal output should not record feedback")
		}
	})

	t.Run("feedback trimmed beyond 20", func(t *testing.T) {
		setupLearningStore(t)
		a := NewAdaptiveCapability()
		// Each error output records one feedback entry; after 25 it should trim to 20.
		for i := 0; i < 25; i++ {
			_, _ = a.PostProcess(context.Background(), "task", "error occurred")
		}
		if len(a.feedback) != 20 {
			t.Errorf("expected feedback trimmed to 20, got %d", len(a.feedback))
		}
	})
}
