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

package badge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContributorID(t *testing.T) {
	id1 := ContributorID("alice", "alice@example.com")
	id2 := ContributorID("alice", "alice@example.com")
	id3 := ContributorID("bob", "bob@example.com")

	if id1 != id2 {
		t.Errorf("same name+email should produce same ID, got %s != %s", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("different name+email should produce different IDs")
	}
	if len(id1) != 32 {
		t.Errorf("ID should be 32 hex chars, got %d", len(id1))
	}
}

func TestTierForCount(t *testing.T) {
	tests := []struct {
		count int
		want  Tier
	}{
		{0, ""},
		{1, TierBronze},
		{2, TierBronze},
		{3, TierSilver},
		{5, TierSilver},
		{6, TierGold},
		{9, TierGold},
		{10, TierPlatinum},
		{100, TierPlatinum},
	}

	for _, tt := range tests {
		got := TierForCount(tt.count)
		if got != tt.want {
			t.Errorf("TierForCount(%d) = %s, want %s", tt.count, got, tt.want)
		}
	}
}

func TestStore_RecordContribution_Bronze(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	cid := ContributorID("alice", "alice@example.com")
	badge, awarded := store.RecordContribution(cid, "Alice", "Submitted template: stock-analyzer", ContributionTemplate)

	if !awarded {
		t.Fatal("expected bronze badge to be awarded on first contribution")
	}
	if badge.Tier != TierBronze {
		t.Errorf("expected bronze, got %s", badge.Tier)
	}

	// Save and reload to verify persistence.
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	store2, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}

	rec := store2.GetContributor(cid)
	if rec == nil {
		t.Fatal("contributor not found after reload")
	}
	if rec.ContributionCount != 1 {
		t.Errorf("expected 1 contribution, got %d", rec.ContributionCount)
	}
	if len(rec.Badges) != 1 {
		t.Errorf("expected 1 badge, got %d", len(rec.Badges))
	}
}

func TestStore_RecordContribution_Progression(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	cid := ContributorID("bob", "bob@example.com")

	// 1st contribution → Bronze
	b, awarded := store.RecordContribution(cid, "Bob", "template 1", ContributionTemplate)
	if !awarded || b.Tier != TierBronze {
		t.Errorf("contribution 1: awarded=%v tier=%s", awarded, b.Tier)
	}

	// 2nd contribution → no new badge (still bronze)
	b, awarded = store.RecordContribution(cid, "Bob", "template 2", ContributionTemplate)
	if awarded {
		t.Errorf("contribution 2: should not award new badge, got %s", b.Tier)
	}

	// 3rd contribution → Silver
	b, awarded = store.RecordContribution(cid, "Bob", "template 3", ContributionTemplate)
	if !awarded || b.Tier != TierSilver {
		t.Errorf("contribution 3: awarded=%v tier=%s", awarded, b.Tier)
	}

	rec := store.GetContributor(cid)
	if rec.ContributionCount != 3 {
		t.Errorf("expected 3 contributions, got %d", rec.ContributionCount)
	}
	if len(rec.Badges) != 2 {
		t.Errorf("expected 2 badges, got %d", len(rec.Badges))
	}
}

func TestStore_RecordContribution_Platinum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	cid := ContributorID("charlie", "charlie@example.com")

	for i := 0; i < 10; i++ {
		store.RecordContribution(cid, "Charlie", "contribution", ContributionCode)
	}

	rec := store.GetContributor(cid)
	if rec.ContributionCount != 10 {
		t.Errorf("expected 10 contributions, got %d", rec.ContributionCount)
	}
	if rec.CurrentTier() != TierPlatinum {
		t.Errorf("expected platinum tier, got %s", rec.CurrentTier())
	}

	// Should have all 4 badges (bronze, silver, gold, platinum).
	if len(rec.Badges) != 4 {
		t.Errorf("expected 4 badges, got %d", len(rec.Badges))
	}
}

func TestStore_RecordContribution_DifferentTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	cid := ContributorID("dave", "dave@example.com")

	store.RecordContribution(cid, "Dave", "code review fix", ContributionBugFix)
	store.RecordContribution(cid, "Dave", "doc update", ContributionDocs)
	store.RecordContribution(cid, "Dave", "new template", ContributionTemplate)

	rec := store.GetContributor(cid)
	if rec.ContributionCount != 3 {
		t.Errorf("expected 3 contributions, got %d", rec.ContributionCount)
	}
	if rec.CurrentTier() != TierSilver {
		t.Errorf("expected silver tier, got %s", rec.CurrentTier())
	}
}

func TestStore_ListContributors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// Add contributors with different counts.
	store.RecordContribution(ContributorID("a", "a@x.com"), "A", "t1", ContributionTemplate)
	store.RecordContribution(ContributorID("b", "b@x.com"), "B", "t1", ContributionTemplate)
	store.RecordContribution(ContributorID("b", "b@x.com"), "B", "t2", ContributionTemplate)
	store.RecordContribution(ContributorID("b", "b@x.com"), "B", "t3", ContributionTemplate)

	list := store.ListContributors()
	if len(list) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(list))
	}

	// B should be first (3 contributions), A second (1 contribution).
	if list[0].Name != "B" {
		t.Errorf("expected B first, got %s", list[0].Name)
	}
	if list[1].Name != "A" {
		t.Errorf("expected A second, got %s", list[1].Name)
	}
}

func TestStore_LoadStore_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badges.json")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore empty file: %v", err)
	}
	if store.Contributors == nil {
		t.Error("Contributors map should be initialized")
	}
}

func TestStore_LoadStore_NonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "badges.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore nonexistent: %v", err)
	}
	if store.Contributors == nil {
		t.Error("Contributors map should be initialized for new store")
	}
}

func TestFormatBadge(t *testing.T) {
	badge := Badge{
		ID:       "test-bronze",
		Tier:     TierBronze,
		Reason:   "Submitted template: hello-world",
		ContType: ContributionTemplate,
		Project:  "aflare",
	}

	result := FormatBadge(badge)
	if result == "" {
		t.Error("FormatBadge returned empty string")
	}
	// Should contain the tier name and reason.
	if !strings.Contains(result, "bronze") || !strings.Contains(result, "hello-world") {
		t.Errorf("unexpected FormatBadge output: %s", result)
	}
}

func TestContributorRecord_HasBadge(t *testing.T) {
	rec := &ContributorRecord{
		Badges: []Badge{
			{Tier: TierBronze},
			{Tier: TierSilver},
		},
	}

	if !rec.HasBadge(TierBronze) {
		t.Error("should have bronze badge")
	}
	if !rec.HasBadge(TierSilver) {
		t.Error("should have silver badge")
	}
	if rec.HasBadge(TierGold) {
		t.Error("should not have gold badge")
	}
}

func TestDefaultStorePath(t *testing.T) {
	path := DefaultStorePath()
	if path == "" {
		t.Error("DefaultStorePath returned empty string")
	}
	if !strings.Contains(path, ".aflare") || !strings.Contains(path, "badges.json") {
		t.Errorf("unexpected DefaultStorePath: %s", path)
	}
}
