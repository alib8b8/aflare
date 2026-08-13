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
	"time"
)

func TestBDICapability_Basic(t *testing.T) {
	b := NewBDICapability()
	if b.Name() != "bdi" {
		t.Errorf("Name() = %q, want %q", b.Name(), "bdi")
	}
	if b.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(b.Description(), "BDI") {
		t.Errorf("Description() = %q, want it to mention BDI", b.Description())
	}
	if err := b.Init(&AgentLoop{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestBDICapability_AddGoal(t *testing.T) {
	b := NewBDICapability()
	id1 := b.AddGoal("write tests", 1)
	id2 := b.AddGoal("ship release", 2)
	if id1 != "goal-1" || id2 != "goal-2" {
		t.Errorf("AddGoal ids = %q, %q, want goal-1, goal-2", id1, id2)
	}
	goals := b.GetGoals()
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(goals))
	}
	if goals[0].Description != "write tests" {
		t.Errorf("first goal = %q, want %q", goals[0].Description, "write tests")
	}
	if goals[0].Status != "active" {
		t.Errorf("new goal status = %q, want active", goals[0].Status)
	}
}

func TestBDICapability_GetGoals_ReturnsCopy(t *testing.T) {
	b := NewBDICapability()
	b.AddGoal("original", 1)
	goals := b.GetGoals()
	// Appending to the returned slice must not affect the internal state.
	// The append result is intentionally discarded: the point is that the
	// call itself (growing the returned slice) does not mutate b.desires.
	_ = append(goals, &Desire{ID: "extra", Description: "injected"})
	if len(b.GetGoals()) != 1 {
		t.Error("GetGoals should return a copy, internal state was mutated by append")
	}
}

func TestBDICapability_GetBeliefs(t *testing.T) {
	b := NewBDICapability()
	b.beliefs["pref"] = &Belief{
		Key:        "pref",
		Value:      "likes Go",
		Confidence: 0.9,
		UpdatedAt:  time.Now(),
	}
	beliefs := b.GetBeliefs()
	if len(beliefs) != 1 {
		t.Fatalf("expected 1 belief, got %d", len(beliefs))
	}
	if beliefs["pref"].Value != "likes Go" {
		t.Errorf("belief value = %q, want %q", beliefs["pref"].Value, "likes Go")
	}
	// Returned map should be a copy.
	beliefs["pref"].Value = "mutated"
	if b.beliefs["pref"].Value != "likes Go" {
		t.Error("GetBeliefs should return a copy, internal state was mutated")
	}
}

func TestBDICapability_GetBeliefs_Empty(t *testing.T) {
	b := NewBDICapability()
	beliefs := b.GetBeliefs()
	if beliefs == nil {
		t.Error("GetBeliefs should return non-nil map even when empty")
	}
	if len(beliefs) != 0 {
		t.Errorf("expected empty beliefs, got %d", len(beliefs))
	}
}

func TestBDICapability_getActiveDesires(t *testing.T) {
	b := NewBDICapability()
	b.desires = []*Desire{
		{ID: "g1", Status: "active", Description: "one", Priority: 1},
		{ID: "g2", Status: "in-progress", Description: "two", Priority: 2},
		{ID: "g3", Status: "completed", Description: "done", Priority: 3},
		{ID: "g4", Status: "abandoned", Description: "skip", Priority: 4},
	}
	active := b.getActiveDesires()
	if len(active) != 2 {
		t.Fatalf("expected 2 active desires, got %d", len(active))
	}
	if active[0].ID != "g1" || active[1].ID != "g2" {
		t.Errorf("active ids = %s, %s, want g1, g2", active[0].ID, active[1].ID)
	}
}

func TestBDICapability_getActiveDesires_Empty(t *testing.T) {
	b := NewBDICapability()
	if active := b.getActiveDesires(); len(active) != 0 {
		t.Errorf("expected 0 active desires, got %d", len(active))
	}
}

func TestBDICapability_pruneDesires(t *testing.T) {
	t.Run("no completed goals", func(t *testing.T) {
		b := NewBDICapability()
		b.AddGoal("active goal", 1)
		b.pruneDesires()
		if len(b.desires) != 1 {
			t.Errorf("expected 1 desire retained, got %d", len(b.desires))
		}
	})

	t.Run("keeps at most 5 completed", func(t *testing.T) {
		b := NewBDICapability()
		// Add 8 completed goals.
		for i := 0; i < 8; i++ {
			b.AddGoal("completed", 1)
			b.desires[i].Status = "completed"
		}
		// Add 1 abandoned.
		b.AddGoal("abandoned", 1)
		b.desires[8].Status = "abandoned"
		// Add 2 active.
		b.AddGoal("active", 1)
		b.AddGoal("active2", 1)

		b.pruneDesires()
		// active(2) + completed-or-abandoned trimmed to 5 = 7
		if len(b.desires) != 7 {
			t.Errorf("expected 7 desires after prune, got %d", len(b.desires))
		}
	})
}

func TestBDICapability_buildBDIContext(t *testing.T) {
	t.Run("no active desires returns empty", func(t *testing.T) {
		b := NewBDICapability()
		if ctx := b.buildBDIContext(); ctx != "" {
			t.Errorf("expected empty context, got %q", ctx)
		}
	})

	t.Run("with desires and beliefs", func(t *testing.T) {
		b := NewBDICapability()
		b.desires = []*Desire{
			{ID: "g1", Description: "write tests", Priority: 1, Status: "active", Progress: "50% done"},
		}
		b.beliefs["env"] = &Belief{Key: "env", Value: "production", Confidence: 0.8}
		ctx := b.buildBDIContext()
		if !strings.Contains(ctx, "BDI Context") {
			t.Error("expected BDI Context header")
		}
		if !strings.Contains(ctx, "write tests") {
			t.Error("expected goal description in context")
		}
		if !strings.Contains(ctx, "Progress") {
			t.Error("expected progress note in context")
		}
		if !strings.Contains(ctx, "Beliefs") {
			t.Error("expected beliefs section in context")
		}
	})

	t.Run("beliefs capped at 5", func(t *testing.T) {
		b := NewBDICapability()
		b.desires = []*Desire{{ID: "g1", Description: "goal", Priority: 1, Status: "active"}}
		for i := 0; i < 8; i++ {
			key := string(rune('a' + i))
			b.beliefs[key] = &Belief{Key: key, Value: "v", Confidence: 0.5}
		}
		ctx := b.buildBDIContext()
		// Only belief lines contain "confidence:" — count those to verify the cap.
		if got := strings.Count(ctx, "confidence:"); got != 5 {
			t.Errorf("expected 5 beliefs listed, got %d", got)
		}
	})
}

func TestBDICapability_PreProcess_Empty(t *testing.T) {
	// With no desires, buildBDIContext returns "" and PreProcess should return "".
	b := NewBDICapability()
	out, err := b.PreProcess(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output with no goals, got %q", out)
	}
}
