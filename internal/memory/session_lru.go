// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌​‌​‌​‌‌​​‌‌‌‌​​‌​​‌​‌​‌​​​​​‌​‌​‌‌​​​​‌​‌‌‌​​‌​​​​​​​​​​​​​​​​​‌​‌‌‌​‌‌​‌​‌‌‌​‌⁠
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
	"time"
)

// evictLRULocked removes the least recently used entry (must hold write lock).
func (sm *SessionMemory) evictLRULocked() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range sm.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(sm.entries, oldestKey)
		delete(sm.KGNodeRefs, oldestKey)
		sm.vectors.Remove(oldestKey)
	}
}

// evictOldestLocked evicts the least recently used session (must hold write lock).
func (mgr *SessionMemoryManager) evictOldestLocked() {
	var oldestID string
	var oldestTime time.Time

	for id, s := range mgr.sessions {
		s.mu.RLock()
		lu := s.lastUsedAt
		s.mu.RUnlock()
		if oldestID == "" || lu.Before(oldestTime) {
			oldestID = id
			oldestTime = lu
		}
	}

	if oldestID != "" {
		// Save before evicting
		if mgr.storageDir != "" {
			if s, ok := mgr.sessions[oldestID]; ok {
				mgr.saveSessionLocked(s)
			}
		}
		delete(mgr.sessions, oldestID)
	}
}
