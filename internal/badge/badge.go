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

// Package badge provides a virtual badge system for recognizing aflare
// community contributors. Badges are awarded when contributors submit
// high-quality templates or code contributions.
//
// Badge tiers:
//   - Bronze: 1-2 accepted contributions
//   - Silver: 3-5 accepted contributions
//   - Gold: 6-10 accepted contributions
//   - Platinum: 10+ accepted contributions
//
// Badges are stored persistently as JSON in the user's aflare data directory
// and can be displayed via the CLI.
package badge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Tier represents a badge achievement level.
type Tier string

const (
	TierBronze   Tier = "bronze"
	TierSilver   Tier = "silver"
	TierGold     Tier = "gold"
	TierPlatinum Tier = "platinum"
)

// TierThresholds maps each tier to the minimum contribution count required.
var TierThresholds = map[Tier]int{
	TierBronze:   1,
	TierSilver:   3,
	TierGold:     6,
	TierPlatinum: 10,
}

// TierOrder defines the display order of tiers.
var TierOrder = []Tier{TierBronze, TierSilver, TierGold, TierPlatinum}

// TierEmoji maps each tier to a display emoji.
var TierEmoji = map[Tier]string{
	TierBronze:   "\U0001F949", // 🥉
	TierSilver:   "\U0001F948", // 🥈
	TierGold:     "\U0001F947", // 🥇
	TierPlatinum: "\U0001F48E", // 💎
}

// ContributionType classifies the kind of contribution.
type ContributionType string

const (
	ContributionTemplate ContributionType = "template"
	ContributionCode     ContributionType = "code"
	ContributionDocs     ContributionType = "docs"
	ContributionBugFix   ContributionType = "bugfix"
)

// Badge represents a single awarded badge.
type Badge struct {
	ID        string           `json:"id"`         // unique badge ID (hash of contributor + tier)
	Tier      Tier             `json:"tier"`       // badge tier
	Reason    string           `json:"reason"`     // human-readable reason
	ContType  ContributionType `json:"cont_type"`  // type of contribution
	AwardedAt time.Time        `json:"awarded_at"` // when the badge was awarded
	Project   string           `json:"project"`    // project name (always "aflare")
}

// ContributorRecord tracks a single contributor's badges and history.
type ContributorRecord struct {
	ID                string    `json:"id"`            // unique contributor identifier (email or name hash)
	Name              string    `json:"name"`          // display name
	Badges            []Badge   `json:"badges"`        // all awarded badges
	ContributionCount int       `json:"contrib_count"` // total accepted contributions
	FirstContribution time.Time `json:"first_cont"`    // date of first contribution
	LastContribution  time.Time `json:"last_cont"`     // date of most recent contribution
}

// CurrentTier returns the contributor's current badge tier based on count.
func (c *ContributorRecord) CurrentTier() Tier {
	return TierForCount(c.ContributionCount)
}

// HasBadge checks if the contributor already has a specific badge.
func (c *ContributorRecord) HasBadge(tier Tier) bool {
	for _, b := range c.Badges {
		if b.Tier == tier {
			return true
		}
	}
	return false
}

// Store is the persistent badge store backed by a JSON file.
type Store struct {
	mu           sync.RWMutex
	path         string
	Contributors map[string]*ContributorRecord `json:"contributors"`
}

// DefaultStorePath returns the default path for the badge store JSON file.
func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".aflare", "badges.json")
}

// LoadStore loads the badge store from the given path, creating a new one
// if the file does not exist.
func LoadStore(path string) (*Store, error) {
	s := &Store{
		path:         path,
		Contributors: make(map[string]*ContributorRecord),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read badge store: %w", err)
	}

	if len(data) == 0 {
		return s, nil
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse badge store: %w", err)
	}

	if s.Contributors == nil {
		s.Contributors = make(map[string]*ContributorRecord)
	}

	return s, nil
}

// Save persists the badge store to disk.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create badge store dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal badge store: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write badge store: %w", err)
	}

	return nil
}

// ContributorID generates a deterministic contributor ID from name and email.
func ContributorID(name, email string) string {
	h := sha256.Sum256([]byte(name + ":" + email))
	return fmt.Sprintf("%x", h[:16])
}

// RecordContribution records a new contribution and awards badges if earned.
// Returns the newly awarded badge (if any) and whether a badge was awarded.
func (s *Store) RecordContribution(contributorID, name, reason string, contType ContributionType) (*Badge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	rec, exists := s.Contributors[contributorID]
	if !exists {
		rec = &ContributorRecord{
			ID:                contributorID,
			Name:              name,
			FirstContribution: now,
		}
		s.Contributors[contributorID] = rec
	}

	rec.ContributionCount++
	rec.LastContribution = now

	newTier := rec.CurrentTier()
	if rec.HasBadge(newTier) {
		// Already has this tier badge — no new badge awarded.
		return nil, false
	}

	// Award all lower-tier badges they haven't received yet.
	var awarded *Badge
	shortID := contributorID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	for _, tier := range TierOrder {
		if rec.ContributionCount >= TierThresholds[tier] && !rec.HasBadge(tier) {
			badge := Badge{
				ID:        fmt.Sprintf("%s-%s", shortID, tier),
				Tier:      tier,
				Reason:    reason,
				ContType:  contType,
				AwardedAt: now,
				Project:   "aflare",
			}
			rec.Badges = append(rec.Badges, badge)
			awarded = &badge
		}
	}

	return awarded, awarded != nil
}

// GetContributor returns the record for a contributor, or nil if not found.
func (s *Store) GetContributor(id string) *ContributorRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Contributors[id]
}

// ListContributors returns all contributor records sorted by contribution count descending.
func (s *Store) ListContributors() []*ContributorRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recs := make([]*ContributorRecord, 0, len(s.Contributors))
	for _, r := range s.Contributors {
		recs = append(recs, r)
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].ContributionCount > recs[j].ContributionCount
	})

	return recs
}

// TierForCount returns the badge tier for a given contribution count.
func TierForCount(count int) Tier {
	if count >= TierThresholds[TierPlatinum] {
		return TierPlatinum
	}
	if count >= TierThresholds[TierGold] {
		return TierGold
	}
	if count >= TierThresholds[TierSilver] {
		return TierSilver
	}
	if count >= TierThresholds[TierBronze] {
		return TierBronze
	}
	return ""
}

// FormatBadge returns a display string for a badge.
func FormatBadge(b Badge) string {
	emoji := TierEmoji[b.Tier]
	return fmt.Sprintf("%s %s — %s (%s)", emoji, string(b.Tier), b.Reason, b.AwardedAt.Format("2006-01-02"))
}
