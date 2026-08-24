// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​‌‌‌‌‌‌​‌‌‌​‌‌​​‌​‌‌‌‌​‌​​‌​‌‌​‌‌‌​​​‌​‌‌​‌​​​​​​​​​​​​​​​​​​​‌​‌​​​​​‌‌​​​​⁠
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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alib8b8/aflare/internal/memory"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// SessionManagerNode exposes the cross-session memory manager to
// workflows. Inspired by jcode's multi-session persistence model: each
// agent can create/switch/fork/merge isolated sessions while sharing
// long-term facts via the shared namespace.
//
// Supported actions:
//   - create:  create a new session, optionally inheriting from a parent
//   - switch:  mark a session as active
//   - list:    list all sessions
//   - delete:  remove a session
//   - merge:   copy memory from src session into dst session
//   - shared_get / shared_put: read/write the shared namespace
//   - recall:  retrieve a key from a session (falls back to shared)
//   - search:  search across a session + shared
type SessionManagerNode struct{}

func init() {
	Register(&SessionManagerNode{})
}

func (n *SessionManagerNode) Name() string { return "session_manager" }

func (n *SessionManagerNode) Description() string {
	return "Manage multiple isolated agent memory sessions with fork/merge and a shared namespace (jcode-inspired)"
}

func (n *SessionManagerNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "session_manager",
		Description: "Multi-session memory management. Create isolated sessions, fork a session from a parent, merge sessions, and share facts across sessions via the shared namespace.",
		Input:       "string - value to store (for shared_put action)",
		Output:      "string - JSON result of the action",
		Params: []core.ParamSchema{
			{Name: "action", Type: "string", Description: "create | switch | list | delete | merge | shared_get | shared_put | recall | search", Required: true, Default: "list"},
			{Name: "session_id", Type: "string", Description: "Target session id (required for most actions)"},
			{Name: "parent", Type: "string", Description: "Parent session id to inherit memory from (create action only)"},
			{Name: "src", Type: "string", Description: "Source session id (merge action only)"},
			{Name: "dst", Type: "string", Description: "Destination session id (merge action only)"},
			{Name: "key", Type: "string", Description: "Memory key (shared_get/shared_put/recall actions)"},
			{Name: "value", Type: "string", Description: "Memory value (shared_put action). Overrides input when set."},
			{Name: "level", Type: "string", Description: "Memory level: short|medium|long (default short)", Default: "short"},
			{Name: "type", Type: "string", Description: "Memory type tag (default fact)", Default: "fact"},
			{Name: "query", Type: "string", Description: "Search query (search action)"},
			{Name: "top_k", Type: "string", Description: "Max search results (default 10)", Default: "10"},
		},
	}
}

func (n *SessionManagerNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	mgr := memory.GlobalCrossSessionManager
	action := core.GetParam(params, "action", "list")

	switch action {
	case "list":
		return n.actionList(mgr)
	case "create":
		return n.actionCreate(mgr, params)
	case "switch":
		return n.actionSwitch(mgr, params)
	case "delete":
		return n.actionDelete(mgr, params)
	case "merge":
		return n.actionMerge(mgr, params)
	case "shared_get":
		return n.actionSharedGet(mgr, params)
	case "shared_put":
		return n.actionSharedPut(mgr, input, params)
	case "recall":
		return n.actionRecall(mgr, params)
	case "search":
		return n.actionSearch(mgr, params)
	default:
		return "", fmt.Errorf("unknown action: %s (supported: create, switch, list, delete, merge, shared_get, shared_put, recall, search)", action)
	}
}

func (n *SessionManagerNode) actionList(mgr *memory.SessionManager) (string, error) {
	ids := mgr.List()
	active := mgr.Active()
	out, _ := json.MarshalIndent(map[string]interface{}{
		"sessions": ids,
		"count":    len(ids),
		"active":   active,
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionCreate(mgr *memory.SessionManager, params map[string]string) (string, error) {
	sessionID := core.GetParam(params, "session_id", "")
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required for create")
	}
	parent := core.GetParam(params, "parent", "")
	if _, err := mgr.Create(sessionID, parent); err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"action":    "create",
		"session":   sessionID,
		"parent":    parent,
		"active":    sessionID,
		"inherited": parent != "",
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionSwitch(mgr *memory.SessionManager, params map[string]string) (string, error) {
	sessionID := core.GetParam(params, "session_id", "")
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required for switch")
	}
	if _, err := mgr.Switch(sessionID); err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"action":  "switch",
		"session": sessionID,
		"active":  sessionID,
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionDelete(mgr *memory.SessionManager, params map[string]string) (string, error) {
	sessionID := core.GetParam(params, "session_id", "")
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required for delete")
	}
	if err := mgr.Delete(sessionID); err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"action":  "delete",
		"session": sessionID,
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionMerge(mgr *memory.SessionManager, params map[string]string) (string, error) {
	src := core.GetParam(params, "src", "")
	dst := core.GetParam(params, "dst", "")
	if src == "" || dst == "" {
		return "", fmt.Errorf("src and dst are required for merge")
	}
	count, err := mgr.Merge(src, dst)
	if err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"action":       "merge",
		"src":          src,
		"dst":          dst,
		"merged_count": count,
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionSharedGet(mgr *memory.SessionManager, params map[string]string) (string, error) {
	key := core.GetParam(params, "key", "")
	if key == "" {
		return "", fmt.Errorf("key is required for shared_get")
	}
	entry, err := mgr.RetrieveShared(key)
	if err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(entry, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionSharedPut(mgr *memory.SessionManager, input string, params map[string]string) (string, error) {
	key := core.GetParam(params, "key", "")
	if key == "" {
		return "", fmt.Errorf("key is required for shared_put")
	}
	value := core.GetParam(params, "value", input)
	if value == "" {
		return "", fmt.Errorf("value (param or input) is required for shared_put")
	}
	level := core.GetParam(params, "level", "short")
	memType := core.GetParam(params, "type", "fact")
	id, expires, err := mgr.StoreShared(key, value, level, memType, nil, 0, 1.0, "session_manager")
	if err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"action":     "shared_put",
		"key":        key,
		"id":         id,
		"expires_at": expires,
	}, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionRecall(mgr *memory.SessionManager, params map[string]string) (string, error) {
	sessionID := core.GetParam(params, "session_id", "")
	key := core.GetParam(params, "key", "")
	if key == "" {
		return "", fmt.Errorf("key is required for recall")
	}
	entry, err := mgr.RetrieveWithFallback(sessionID, key)
	if err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(entry, "", "  ")
	return string(out), nil
}

func (n *SessionManagerNode) actionSearch(mgr *memory.SessionManager, params map[string]string) (string, error) {
	sessionID := core.GetParam(params, "session_id", "")
	query := core.GetParam(params, "query", "")
	if query == "" {
		return "", fmt.Errorf("query is required for search")
	}
	topK := core.ParamInt(params, "top_k", 10, 1, 100)
	results := mgr.SearchAcross(sessionID, query, "", topK, 0)
	out, _ := json.MarshalIndent(map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, "", "  ")
	return string(out), nil
}
