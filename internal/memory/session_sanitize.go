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

package memory

import (
	"encoding/hex"
	"hash/fnv"
	"regexp"
)

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// sanitizeSessionID maps an arbitrary session identifier onto a
// filesystem-safe one. IDs that already match the strict pattern pass
// through unchanged; anything else (path separators, ".." traversal,
// over-long or exotic identifiers) is replaced by a stable short digest.
// Every code path that derives a storage path from a session ID MUST go
// through this, so a hostile ID such as "x/../../victim" can never steer
// file operations outside the session storage directory.
//
// The digest is FNV-1a 128-bit — a NON-cryptographic hash, chosen
// deliberately: the input is an untrusted identifier, not a secret, and
// the output is only a namespace key (session map entry + storage file
// name). Two hostile IDs colliding onto the same sanitized key merely
// makes them share one session, which crosses no trust boundary, and a
// sanitized "s_<hex>" key can never collide with a pass-through ID
// unless that ID was itself hostile. No password or key material is ever
// stored or verified through this digest, so a fast hash is the right
// tool here (a slow/KDF hash would only waste CPU).
func sanitizeSessionID(sessionID string) string {
	if sessionIDPattern.MatchString(sessionID) {
		return sessionID
	}
	h := fnv.New128a()
	_, _ = h.Write([]byte(sessionID))
	return "s_" + hex.EncodeToString(h.Sum(nil))
}
