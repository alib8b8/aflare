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

package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/memory"
)

// ------------------------------------------------------------------
// Memory tool implementations
// ------------------------------------------------------------------

func (s *Server) toolMemoryStore(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	value, err := requireString(args, "value")
	if err != nil {
		return nil, err
	}
	key := optionalString(args, "key")
	level := optionalString(args, "level")
	if level == "" {
		level = "medium"
	}
	memType := optionalString(args, "type")
	if memType == "" {
		memType = "fact"
	}
	tagsStr := optionalString(args, "tags")
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	confidence := 0.8
	switch v := args["confidence"].(type) {
	case float64:
		confidence = v
	case int:
		confidence = float64(v)
	}

	sess := memory.GetSession(sessionID)
	id, expiresAt, err := sess.Store(key, value, level, memType, tags, 72, confidence, "mcp")
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf("Stored: key=%s id=%s expires=%s", key, id, expiresAt.Format(time.RFC3339))}},
	}, nil
}

func (s *Server) toolMemoryRetrieve(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	key, err := requireString(args, "key")
	if err != nil {
		return nil, err
	}

	sess := memory.GetSession(sessionID)
	entry, err := sess.Retrieve(key)
	if err != nil {
		return nil, err
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	return &toolCallResult{
		Content: []content{{Type: "text", Text: string(data)}},
	}, nil
}

func (s *Server) toolMemorySearch(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	level := optionalString(args, "level")
	topK := optionalInt(args, "top_k", 10)

	sess := memory.GetSession(sessionID)
	results := sess.Search(query, level, topK, 0.3)

	if len(results) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No matching memory entries found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d memory entries:\n\n", len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (key: %s)\n", i+1, r.Type, r.Value, r.Key))
		if len(r.Value) > 150 {
			sb.WriteString(fmt.Sprintf("   Preview: %s...\n", r.Value[:147]))
		}
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolMemoryStats(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}

	if sessionID == "global" {
		gs := memory.GetGlobalStats()
		return &toolCallResult{
			Content: []content{{Type: "text", Text: fmt.Sprintf(
				"Global Memory: %d sessions, %d entries, %.2f MB total",
				gs.ActiveSessions, gs.TotalEntries, gs.TotalEstimatedMB,
			)}},
		}, nil
	}

	sess := memory.GetSession(sessionID)
	stats := sess.GetStats()
	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf(
			"Session '%s': %d entries (S:%d M:%d L:%d), %.2f KB, %d accesses",
			sessionID, stats.TotalEntries,
			stats.ShortTermCount, stats.MediumTermCount, stats.LongTermCount,
			float64(stats.EstimatedBytes)/1024, stats.TotalAccesses,
		)}},
	}, nil
}

func (s *Server) toolMemoryListSessions() (*toolCallResult, error) {
	sessions := memory.ListSessions()
	gs := memory.GetGlobalStats()

	if len(sessions) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No active memory sessions."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active memory sessions (%d):\n\n", len(sessions)))
	for _, id := range sessions {
		ss := gs.PerSession[id]
		sb.WriteString(fmt.Sprintf("  • %s: %d entries (%.2f KB)\n",
			id, ss.TotalEntries, float64(ss.EstimatedBytes)/1024))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

// ------------------------------------------------------------------
// Preference tool implementations (MemSlides-inspired user profiling)
// ------------------------------------------------------------------

func (s *Server) toolPreferenceGet(args map[string]interface{}) (*toolCallResult, error) {
	key, err := requireString(args, "key")
	if err != nil {
		return nil, err
	}
	userID := optionalString(args, "user_id")
	if userID == "" {
		userID = "default"
	}
	category := optionalString(args, "category")
	if category == "" {
		category = "custom"
	}

	pm := memory.GetProfileManager()
	profile := pm.GetProfile(userID)
	value, confidence, ok := profile.GetPreference(memory.PreferenceCategory(category), key)
	if !ok {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: fmt.Sprintf("No preference found for [%s:%s]", category, key)}},
		}, nil
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf("%s (confidence: %.0f%%)", value, confidence*100)}},
	}, nil
}

func (s *Server) toolPreferenceSet(args map[string]interface{}) (*toolCallResult, error) {
	key, err := requireString(args, "key")
	if err != nil {
		return nil, err
	}
	value, err := requireString(args, "value")
	if err != nil {
		return nil, err
	}
	userID := optionalString(args, "user_id")
	if userID == "" {
		userID = "default"
	}
	category := optionalString(args, "category")
	if category == "" {
		category = "custom"
	}
	isLearn := optionalBool(args, "learn", false)

	pm := memory.GetProfileManager()
	profile := pm.GetProfile(userID)

	if isLearn {
		profile.LearnFromInteraction(userID, category, key, value, "mcp")
	} else {
		profile.SetPreference(memory.PreferenceCategory(category), key, value, "mcp", 1.0)
	}
	pm.Save(userID)

	mode := "set"
	if isLearn {
		mode = "learned"
	}
	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf("Preference %s: [%s:%s] = %s", mode, category, key, value)}},
	}, nil
}
