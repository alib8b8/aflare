// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​​‌​‌​​‌​‌​‌‌​‌‌​​​‌‌​​‌​‌‌​​​​‌‌‌‌​‌‌​​‌​‌​​​‌‌‌​​​​​​​​​​​​​​​​‌​‌‌​‌‌​​​‌​‌​​​⁠
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

// cleanupExpiredLocked removes expired entries (must hold write lock).
func (sm *SessionMemory) cleanupExpiredLocked() {
	now := time.Now()
	for key, entry := range sm.entries {
		if now.After(entry.ExpiresAt) {
			delete(sm.entries, key)
			delete(sm.KGNodeRefs, key)
			sm.vectors.Remove(key)
		}
	}
}
