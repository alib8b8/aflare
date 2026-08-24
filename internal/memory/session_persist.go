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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

// sessionFilePath returns the persistent storage path for a session.
func (mgr *SessionMemoryManager) sessionFilePath(sessionID string) string {
	return filepath.Join(mgr.storageDir, fmt.Sprintf("session-%s.json", sessionID))
}

// sessionData is the serializable format for a session.
type sessionData struct {
	SessionID   string                  `json:"session_id"`
	Entries     map[string]*MemoryEntry `json:"entries"`
	Graph       MemoryGraph             `json:"graph"`
	AccessCount int                     `json:"access_count"`
	MaxEntries  int                     `json:"max_entries"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	LastUsedAt  time.Time               `json:"last_used_at"`
	// C-3: persisted memory↔KG linkage. Keyed by memory key, values
	// are KG entity names. The vector index is NOT persisted: vectors
	// are recomputed on load via ReindexVectors (embeddings are cheap
	// and model versions may change).
	KGNodeRefs map[string][]string `json:"kg_node_refs,omitempty"`
}

// SaveAll persists all active sessions to storage.
func (mgr *SessionMemoryManager) SaveAll() {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	for _, s := range mgr.sessions {
		mgr.saveSessionLocked(s)
	}
}

// saveSessionLocked persists a session (must hold read or write lock on mgr).
func (mgr *SessionMemoryManager) saveSessionLocked(session *SessionMemory) {
	if mgr.storageDir == "" {
		return
	}

	session.mu.RLock()
	// Deep-copy KGNodeRefs AND Entries so the marshalled JSON is stable
	// even if a concurrent writer mutates the maps while json.Marshal
	// runs. Entries stores *MemoryEntry pointers; without copying the
	// pointed-to struct, a concurrent Retrieve (which mutates
	// AccessedAt under the write lock) would race with marshalling.
	kgRefs := make(map[string][]string, len(session.KGNodeRefs))
	for k, v := range session.KGNodeRefs {
		cp := make([]string, len(v))
		copy(cp, v)
		kgRefs[k] = cp
	}
	data := sessionData{
		SessionID:   session.SessionID,
		Entries:     make(map[string]*MemoryEntry, len(session.entries)),
		Graph:       session.graph,
		AccessCount: session.accessCount,
		MaxEntries:  session.maxEntries,
		CreatedAt:   session.createdAt,
		UpdatedAt:   session.updatedAt,
		LastUsedAt:  session.lastUsedAt,
		KGNodeRefs:  kgRefs,
	}
	for k, v := range session.entries {
		ec := *v // value copy so concurrent writers can't race the marshal
		data.Entries[k] = &ec
	}
	session.mu.RUnlock()

	path := mgr.sessionFilePath(session.SessionID)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	// Atomic write: write to tmp then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		logger.Warn("failed to atomically persist session file", "path", path, "err", err)
	}
}

// loadSessionLocked loads a session from persistent storage (must hold write lock).
func (mgr *SessionMemoryManager) loadSessionLocked(session *SessionMemory) {
	path := mgr.sessionFilePath(session.SessionID)

	data, err := os.ReadFile(path) // #nosec G304 -- internally generated session path
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var sd sessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if sd.Entries == nil {
		sd.Entries = make(map[string]*MemoryEntry)
	}
	if sd.KGNodeRefs == nil {
		sd.KGNodeRefs = make(map[string][]string)
	}
	session.entries = sd.Entries
	session.graph = sd.Graph
	session.accessCount = sd.AccessCount
	session.maxEntries = sd.MaxEntries
	session.createdAt = sd.CreatedAt
	session.updatedAt = sd.UpdatedAt
	session.lastUsedAt = sd.LastUsedAt
	session.KGNodeRefs = sd.KGNodeRefs
	// Vectors are intentionally left empty: callers must invoke
	// ReindexVectors after load if they want semantic search.
}

// StartAutoSave starts periodic auto-saving of sessions.
func (mgr *SessionMemoryManager) StartAutoSave(ctx context.Context, interval time.Duration) {
	if mgr.storageDir == "" {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// saveAll wraps SaveAll with panic recovery so the auto-save
		// loop keeps running even if a single save panics.
		saveAll := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("session auto-save panicked",
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
			}()
			mgr.SaveAll()
		}

		for {
			select {
			case <-ctx.Done():
				saveAll()
				return
			case <-ticker.C:
				saveAll()
			}
		}
	}()
}
