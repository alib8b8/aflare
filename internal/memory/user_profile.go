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

package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type PreferenceCategory string

const (
	PrefCodingStyle  PreferenceCategory = "coding_style"
	PrefOutputFormat PreferenceCategory = "output_format"
	PrefModelChoice  PreferenceCategory = "model_choice"
	PrefVerbosity    PreferenceCategory = "verbosity"
	PrefLanguage     PreferenceCategory = "language"
	PrefSafety       PreferenceCategory = "safety"
	PrefWorkflow     PreferenceCategory = "workflow"
	PrefCustom       PreferenceCategory = "custom"
)

type PreferenceEntry struct {
	ID           string             `json:"id"`
	Category     PreferenceCategory `json:"category"`
	Key          string             `json:"key"`
	Value        string             `json:"value"`
	Confidence   float64            `json:"confidence"`
	Source       string             `json:"source"`
	Count        int                `json:"count"`
	LastObserved time.Time          `json:"last_observed"`
	CreatedAt    time.Time          `json:"created_at"`
}

type UserProfile struct {
	UserID      string                      `json:"user_id"`
	Preferences map[string]*PreferenceEntry `json:"preferences"`
	History     []ProfileEvent              `json:"history"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
	mu          sync.RWMutex
}

type ProfileEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Category  string    `json:"category"`
	Key       string    `json:"key"`
	OldValue  string    `json:"old_value,omitempty"`
	NewValue  string    `json:"new_value,omitempty"`
	Source    string    `json:"source"`
}

type UserProfileManager struct {
	profiles   map[string]*UserProfile
	storageDir string
	mu         sync.RWMutex
	maxPerUser int
}

var (
	defaultProfileManager *UserProfileManager
	profileManagerOnce    sync.Once
)

const (
	defaultMaxPrefsPerUser = 200
	maxPrefKeyLength       = 128
	maxPrefValueLength     = 1024
	maxHistoryEvents       = 500
)

func GetProfileManager() *UserProfileManager {
	profileManagerOnce.Do(func() {
		defaultProfileManager = &UserProfileManager{
			profiles:   make(map[string]*UserProfile),
			storageDir: defaultProfileStorageDir(),
			maxPerUser: defaultMaxPrefsPerUser,
		}
		defaultProfileManager.loadAllProfiles()
	})
	return defaultProfileManager
}

func defaultProfileStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.llm-box/profiles"
	}
	return filepath.Join(home, ".llm-box", "profiles")
}

func (m *UserProfileManager) GetProfile(userID string) *UserProfile {
	if userID == "" {
		userID = "default"
	}

	m.mu.RLock()
	p, exists := m.profiles[userID]
	m.mu.RUnlock()

	if exists {
		return p
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if p, exists = m.profiles[userID]; exists {
		return p
	}

	p = &UserProfile{
		UserID:      userID,
		Preferences: make(map[string]*PreferenceEntry),
		History:     make([]ProfileEvent, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.profiles[userID] = p
	m.saveProfile(userID)
	return p
}

func prefKey(category PreferenceCategory, key string) string {
	return fmt.Sprintf("%s:%s", category, key)
}

func (p *UserProfile) SetPreference(category PreferenceCategory, key, value, source string, confidence float64) {
	if len(key) > maxPrefKeyLength {
		key = key[:maxPrefKeyLength]
	}
	if len(value) > maxPrefValueLength {
		value = value[:maxPrefValueLength]
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	k := prefKey(category, key)
	now := time.Now()

	existing, exists := p.Preferences[k]
	oldValue := ""
	if exists {
		oldValue = existing.Value
		if existing.Value == value {
			existing.Count++
			existing.LastObserved = now
			if confidence > existing.Confidence {
				existing.Confidence = confidence
			}
			p.UpdatedAt = now
			p.addEventLocked("preference_reinforced", string(category), key, oldValue, value, source)
			return
		}
	}

	entry := &PreferenceEntry{
		ID:           generatePrefID(k),
		Category:     category,
		Key:          key,
		Value:        value,
		Confidence:   confidence,
		Source:       source,
		Count:        1,
		LastObserved: now,
		CreatedAt:    now,
	}

	if len(p.Preferences) >= defaultMaxPrefsPerUser {
		p.evictLeastImportantLocked()
	}

	p.Preferences[k] = entry
	eventType := "preference_set"
	if exists {
		eventType = "preference_changed"
	}
	p.addEventLocked(eventType, string(category), key, oldValue, value, source)
	p.UpdatedAt = now
}

func (p *UserProfile) GetPreference(category PreferenceCategory, key string) (string, float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	k := prefKey(category, key)
	entry, exists := p.Preferences[k]
	if !exists {
		return "", 0, false
	}
	return entry.Value, entry.Confidence, true
}

func (p *UserProfile) GetAllByCategory(category PreferenceCategory) map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]string)
	prefix := string(category) + ":"
	for k, v := range p.Preferences {
		if strings.HasPrefix(k, prefix) {
			result[v.Key] = v.Value
		}
	}
	return result
}

func (p *UserProfile) GetSummary() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	categories := make(map[string]int)
	for _, v := range p.Preferences {
		categories[string(v.Category)]++
	}

	return map[string]interface{}{
		"user_id":       p.UserID,
		"total_prefs":   len(p.Preferences),
		"categories":    categories,
		"history_count": len(p.History),
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
	}
}

func (p *UserProfile) BuildSystemPromptAddon() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.Preferences) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "\n\n[User Preferences - learned from past interactions]")

	catOrder := []PreferenceCategory{
		PrefCodingStyle, PrefOutputFormat, PrefModelChoice,
		PrefVerbosity, PrefLanguage, PrefSafety, PrefWorkflow,
	}

	for _, cat := range catOrder {
		prefix := string(cat) + ":"
		catItems := make([]string, 0)
		for k, v := range p.Preferences {
			if strings.HasPrefix(k, prefix) && v.Confidence >= 0.5 {
				catItems = append(catItems, fmt.Sprintf("  - %s: %s (confidence: %.0f%%)", v.Key, v.Value, v.Confidence*100))
			}
		}
		if len(catItems) > 0 {
			sort.Strings(catItems)
			parts = append(parts, fmt.Sprintf("%s:", string(cat)))
			parts = append(parts, catItems...)
		}
	}

	return strings.Join(parts, "\n")
}

func (p *UserProfile) LearnFromInteraction(userID, category, key, value, source string) {
	p.SetPreference(PreferenceCategory(category), key, value, source, 0.6)
}

func (p *UserProfile) evictLeastImportantLocked() {
	var leastKey string
	var leastScore float64 = 999

	for k, v := range p.Preferences {
		recency := time.Since(v.LastObserved).Hours()
		score := float64(v.Count)*v.Confidence - recency*0.01
		if score < leastScore {
			leastScore = score
			leastKey = k
		}
	}

	if leastKey != "" {
		delete(p.Preferences, leastKey)
	}
}

func (p *UserProfile) addEventLocked(eventType, category, key, oldValue, newValue, source string) {
	event := ProfileEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		Category:  category,
		Key:       key,
		OldValue:  oldValue,
		NewValue:  newValue,
		Source:    source,
	}
	p.History = append(p.History, event)
	if len(p.History) > maxHistoryEvents {
		p.History = p.History[len(p.History)-maxHistoryEvents:]
	}
}

func generatePrefID(key string) string {
	h := sha256.Sum256([]byte(key + time.Now().String()))
	return hex.EncodeToString(h[:8])
}

func (m *UserProfileManager) saveProfile(userID string) {
	if m.storageDir == "" {
		return
	}
	p, exists := m.profiles[userID]
	if !exists {
		return
	}

	if err := os.MkdirAll(m.storageDir, 0700); err != nil {
		return
	}

	filePath := filepath.Join(m.storageDir, sanitizeUserID(userID)+".json")
	p.mu.RLock()
	data, err := json.MarshalIndent(p, "", "  ")
	p.mu.RUnlock()
	if err != nil {
		return
	}

	if len(data) > 5*1024*1024 {
		return
	}

	_ = os.WriteFile(filePath, data, 0600)
}

func (m *UserProfileManager) loadAllProfiles() {
	if m.storageDir == "" {
		return
	}
	entries, err := os.ReadDir(m.storageDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		userID := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(m.storageDir, entry.Name())

		li, err := os.Lstat(filePath)
		if err != nil || li.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if li.Size() > 5*1024*1024 {
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var p UserProfile
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		p.Preferences = make(map[string]*PreferenceEntry)

		var temp struct {
			Preferences map[string]*PreferenceEntry `json:"preferences"`
		}
		if err := json.Unmarshal(data, &temp); err == nil {
			p.Preferences = temp.Preferences
		}

		m.profiles[userID] = &p
	}
}

func sanitizeUserID(id string) string {
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:16])
}

func (m *UserProfileManager) Save(userID string) {
	m.saveProfile(userID)
}

func (m *UserProfileManager) AutoSave(interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 吞掉 panic 避免拖垮进程;下一轮 ticker 仍会触发
				_ = r
			}
		}()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.mu.RLock()
			ids := make([]string, 0, len(m.profiles))
			for id := range m.profiles {
				ids = append(ids, id)
			}
			m.mu.RUnlock()
			for _, id := range ids {
				m.saveProfile(id)
			}
		}
	}()
}
