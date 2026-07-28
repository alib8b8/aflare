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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxFSKeyLength     = 128
	maxFSContentLength = 64 * 1024
)

// FSStore is the unified context filesystem abstraction.
// Inspired by OpenViking, it uses a filesystem paradigm to unify
// memory/knowledge/skills management, so agents use ls/cat/write
// operations instead of calling 3 different APIs.
type FSStore struct {
	sessionMgr *SessionMemoryManager
	profileMgr *UserProfileManager
	skillsDir  string
	kgPath     string
	kgMu       sync.Mutex
	kgCache    *kgFile
	kgLoaded   bool
}

// FSEntry represents a single virtual file or directory entry.
type FSEntry struct {
	Path       string    `json:"path"`
	Type       string    `json:"type"` // "file" | "dir"
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Category   string    `json:"category"` // mem|profile|kg|skills
}

// kgEntity mirrors nodes.KGEntity to avoid a circular dependency on the
// nodes package. The JSON tags match so the on-disk format is compatible.
type kgEntity struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// kgRelation mirrors nodes.KGRelation to avoid a circular dependency.
type kgRelation struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence,omitempty"`
}

// kgFile is the on-disk JSON format for the knowledge graph, matching
// the structure persisted by nodes.KnowledgeGraph.Save.
type kgFile struct {
	Entities  map[string]kgEntity `json:"entities"`
	Relations []kgRelation        `json:"relations"`
}

// NewFSStore creates a unified context store.
// skillsDir is empty by default ~/.llm-box/skills.
func NewFSStore(skillsDir string) *FSStore {
	if skillsDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			skillsDir = filepath.Join(home, ".llm-box", "skills")
		} else {
			skillsDir = ".llm-box/skills"
		}
	}
	kgPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		kgPath = filepath.Join(home, ".llm-box", "knowledge_graph.json")
	}
	return &FSStore{
		sessionMgr: GlobalSessionManager,
		profileMgr: GetProfileManager(),
		skillsDir:  skillsDir,
		kgPath:     kgPath,
	}
}

// validateFSPath ensures path starts with / and contains no .. or //.
func validateFSPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must start with /: %s", path)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("path must not contain ..: %s", path)
	}
	if strings.Contains(path, "//") {
		return fmt.Errorf("path must not contain //: %s", path)
	}
	return nil
}

// splitFSPath trims leading/trailing slashes and splits by /.
func splitFSPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

// Read reads a virtual path (like cat).
// Supports:
//
//	/mem/<level>/             — list keys at this level
//	/mem/<level>/<key>        — read a single memory
//	/profile/<category>/      — list keys in this category
//	/profile/<category>/<key> — read a single preference
//	/kg/entities/             — list all entity names
//	/kg/entities/<name>       — read a single entity
//	/kg/relations             — read all relations
//	/skills/                  — (use List) list all skills
//	/skills/<name>            — read skill content
func (fs *FSStore) Read(path string) (string, error) {
	if err := validateFSPath(path); err != nil {
		return "", err
	}
	parts := splitFSPath(path)
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("cannot read root directory; use List instead")
	}
	switch parts[0] {
	case "mem":
		return fs.readMem(parts)
	case "profile":
		return fs.readProfile(parts)
	case "kg":
		return fs.readKG(parts)
	case "skills":
		return fs.readSkill(parts)
	default:
		return "", fmt.Errorf("unknown category: %s (supported: mem, profile, kg, skills)", parts[0])
	}
}

// Write writes content to a virtual path (like echo > file).
// The path determines which backend stores the content.
func (fs *FSStore) Write(path, content string) error {
	if err := validateFSPath(path); err != nil {
		return err
	}
	if len(content) > maxFSContentLength {
		return fmt.Errorf("content too large (max %d bytes)", maxFSContentLength)
	}
	parts := splitFSPath(path)
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("cannot write to root")
	}
	switch parts[0] {
	case "mem":
		return fs.writeMem(parts, content)
	case "profile":
		return fs.writeProfile(parts, content)
	case "kg":
		return fs.writeKG(parts, content)
	case "skills":
		return fs.writeSkill(parts, content)
	default:
		return fmt.Errorf("unknown category: %s (supported: mem, profile, kg, skills)", parts[0])
	}
}

// List lists virtual directory entries (like ls).
// When path is a directory, returns its entries; when path is a file,
// returns the entry itself.
func (fs *FSStore) List(path string) ([]FSEntry, error) {
	if err := validateFSPath(path); err != nil {
		return nil, err
	}
	parts := splitFSPath(path)
	if len(parts) == 0 || parts[0] == "" {
		return []FSEntry{
			{Path: "/mem", Type: "dir", Category: "mem"},
			{Path: "/profile", Type: "dir", Category: "profile"},
			{Path: "/kg", Type: "dir", Category: "kg"},
			{Path: "/skills", Type: "dir", Category: "skills"},
		}, nil
	}
	switch parts[0] {
	case "mem":
		return fs.listMem(parts)
	case "profile":
		return fs.listProfile(parts)
	case "kg":
		return fs.listKG(parts)
	case "skills":
		return fs.listSkills(parts)
	default:
		return nil, fmt.Errorf("unknown category: %s (supported: mem, profile, kg, skills)", parts[0])
	}
}

// Delete removes a virtual path (like rm).
func (fs *FSStore) Delete(path string) error {
	if err := validateFSPath(path); err != nil {
		return err
	}
	parts := splitFSPath(path)
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("cannot delete root")
	}
	switch parts[0] {
	case "mem":
		return fs.deleteMem(parts)
	case "profile":
		return fs.deleteProfile(parts)
	case "kg":
		return fs.deleteKG(parts)
	case "skills":
		return fs.deleteSkill(parts)
	default:
		return fmt.Errorf("unknown category: %s (supported: mem, profile, kg, skills)", parts[0])
	}
}

// Search searches the virtual filesystem.
// query is the keyword; scope optionally limits to "/mem", "/profile",
// "/kg", or "/skills". Returns matching paths sorted by relevance.
func (fs *FSStore) Search(query, scope string, topK int) ([]FSEntry, error) {
	if topK <= 0 {
		topK = 10
	}
	var results []FSEntry
	if scope == "" || scope == "/mem" {
		results = append(results, fs.searchMem(query, topK)...)
	}
	if scope == "" || scope == "/profile" {
		results = append(results, fs.searchProfile(query, topK)...)
	}
	if scope == "" || scope == "/kg" {
		results = append(results, fs.searchKG(query, topK)...)
	}
	if scope == "" || scope == "/skills" {
		results = append(results, fs.searchSkills(query, topK)...)
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// --- memory backend ---

// readMem handles /mem/<level>/[<key>] reads.
func (fs *FSStore) readMem(parts []string) (string, error) {
	if len(parts) < 2 || parts[1] == "" {
		return "", fmt.Errorf("memory level required: /mem/<level>/[<key>]")
	}
	level := parts[1]
	if !validMemoryLevel(level) {
		return "", fmt.Errorf("invalid memory level: %s (supported: short, medium, long)", level)
	}
	if len(parts) <= 2 {
		return fs.listMemKeys(level)
	}
	key := parts[2]
	if len(key) > maxFSKeyLength {
		return "", fmt.Errorf("key too long (max %d)", maxFSKeyLength)
	}
	session := fs.sessionMgr.GetSession("default")
	entry, err := session.Retrieve(key)
	if err != nil {
		return "", err
	}
	if entry.Level != level {
		return "", fmt.Errorf("memory not found at level %s: %s", level, key)
	}
	return entry.Value, nil
}

// listMemKeys returns a newline-separated list of keys at the given level.
func (fs *FSStore) listMemKeys(level string) (string, error) {
	session := fs.sessionMgr.GetSession("default")
	session.mu.RLock()
	defer session.mu.RUnlock()
	var keys []string
	now := time.Now()
	for key, entry := range session.entries {
		if entry.Level != level {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\n"), nil
}

// writeMem handles /mem/<level>/<key> writes.
func (fs *FSStore) writeMem(parts []string, content string) error {
	if len(parts) < 3 || parts[2] == "" {
		return fmt.Errorf("memory key required: /mem/<level>/<key>")
	}
	level := parts[1]
	if !validMemoryLevel(level) {
		return fmt.Errorf("invalid memory level: %s (supported: short, medium, long)", level)
	}
	key := parts[2]
	if len(key) > maxFSKeyLength {
		return fmt.Errorf("key too long (max %d)", maxFSKeyLength)
	}
	ttlHours := 72
	switch level {
	case "short":
		ttlHours = 1
	case "long":
		ttlHours = 24 * 365
	}
	session := fs.sessionMgr.GetSession("default")
	_, _, err := session.Store(key, content, level, "fact", nil, ttlHours, 0.8, "fs_store")
	return err
}

// deleteMem handles /mem/<level>/<key> deletes.
func (fs *FSStore) deleteMem(parts []string) error {
	if len(parts) < 3 || parts[2] == "" {
		return fmt.Errorf("memory key required: /mem/<level>/<key>")
	}
	key := parts[2]
	if len(key) > maxFSKeyLength {
		return fmt.Errorf("key too long (max %d)", maxFSKeyLength)
	}
	session := fs.sessionMgr.GetSession("default")
	return session.Delete(key)
}

// listMem lists entries under /mem or /mem/<level>.
func (fs *FSStore) listMem(parts []string) ([]FSEntry, error) {
	if len(parts) <= 1 {
		return []FSEntry{
			{Path: "/mem/short", Type: "dir", Category: "mem"},
			{Path: "/mem/medium", Type: "dir", Category: "mem"},
			{Path: "/mem/long", Type: "dir", Category: "mem"},
		}, nil
	}
	level := parts[1]
	if !validMemoryLevel(level) {
		return nil, fmt.Errorf("invalid memory level: %s (supported: short, medium, long)", level)
	}
	session := fs.sessionMgr.GetSession("default")
	session.mu.RLock()
	defer session.mu.RUnlock()
	var entries []FSEntry
	now := time.Now()
	for key, entry := range session.entries {
		if entry.Level != level {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		entries = append(entries, FSEntry{
			Path:       fmt.Sprintf("/mem/%s/%s", level, key),
			Type:       "file",
			Size:       int64(len(entry.Value)),
			ModifiedAt: entry.CreatedAt,
			Category:   "mem",
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// searchMem searches memory entries by query across all levels.
func (fs *FSStore) searchMem(query string, topK int) []FSEntry {
	session := fs.sessionMgr.GetSession("default")
	var results []FSEntry
	for _, level := range []string{"short", "medium", "long"} {
		entries := session.Search(query, level, topK, 0.3)
		for _, entry := range entries {
			results = append(results, FSEntry{
				Path:       fmt.Sprintf("/mem/%s/%s", entry.Level, entry.Key),
				Type:       "file",
				Size:       int64(len(entry.Value)),
				ModifiedAt: entry.CreatedAt,
				Category:   "mem",
			})
		}
	}
	return results
}

// validMemoryLevel returns true if level is short, medium, or long.
func validMemoryLevel(level string) bool {
	return level == "short" || level == "medium" || level == "long"
}

// --- profile backend ---

// readProfile handles /profile/<category>/[<key>] reads.
func (fs *FSStore) readProfile(parts []string) (string, error) {
	if len(parts) < 2 || parts[1] == "" {
		return "", fmt.Errorf("profile category required: /profile/<category>/[<key>]")
	}
	category := parts[1]
	profile := fs.profileMgr.GetProfile("default")
	if len(parts) <= 2 {
		prefs := profile.GetAllByCategory(PreferenceCategory(category))
		var keys []string
		for k := range prefs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return strings.Join(keys, "\n"), nil
	}
	key := parts[2]
	if len(key) > maxFSKeyLength {
		return "", fmt.Errorf("key too long (max %d)", maxFSKeyLength)
	}
	val, _, ok := profile.GetPreference(PreferenceCategory(category), key)
	if !ok {
		return "", fmt.Errorf("preference not found: %s/%s", category, key)
	}
	return val, nil
}

// writeProfile handles /profile/<category>/<key> writes.
func (fs *FSStore) writeProfile(parts []string, content string) error {
	if len(parts) < 3 || parts[2] == "" {
		return fmt.Errorf("profile key required: /profile/<category>/<key>")
	}
	category := parts[1]
	key := parts[2]
	if len(key) > maxFSKeyLength {
		return fmt.Errorf("key too long (max %d)", maxFSKeyLength)
	}
	profile := fs.profileMgr.GetProfile("default")
	profile.SetPreference(PreferenceCategory(category), key, content, "fs_store", 1.0)
	fs.profileMgr.Save("default")
	return nil
}

// deleteProfile is not supported by the underlying UserProfile store.
func (fs *FSStore) deleteProfile(parts []string) error {
	return fmt.Errorf("delete not supported for profile; overwrite via Write instead")
}

// listProfile lists entries under /profile or /profile/<category>.
func (fs *FSStore) listProfile(parts []string) ([]FSEntry, error) {
	profile := fs.profileMgr.GetProfile("default")
	if len(parts) <= 1 {
		profile.mu.RLock()
		defer profile.mu.RUnlock()
		catSet := make(map[string]bool)
		for _, p := range profile.Preferences {
			catSet[string(p.Category)] = true
		}
		entries := make([]FSEntry, 0, len(catSet))
		for cat := range catSet {
			entries = append(entries, FSEntry{
				Path:     fmt.Sprintf("/profile/%s", cat),
				Type:     "dir",
				Category: "profile",
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})
		return entries, nil
	}
	category := parts[1]
	if len(parts) <= 2 {
		prefs := profile.GetAllByCategory(PreferenceCategory(category))
		entries := make([]FSEntry, 0, len(prefs))
		for k, v := range prefs {
			entries = append(entries, FSEntry{
				Path:     fmt.Sprintf("/profile/%s/%s", category, k),
				Type:     "file",
				Size:     int64(len(v)),
				Category: "profile",
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})
		return entries, nil
	}
	key := parts[2]
	val, _, ok := profile.GetPreference(PreferenceCategory(category), key)
	if !ok {
		return nil, fmt.Errorf("preference not found: %s/%s", category, key)
	}
	return []FSEntry{{
		Path:     fmt.Sprintf("/profile/%s/%s", category, key),
		Type:     "file",
		Size:     int64(len(val)),
		Category: "profile",
	}}, nil
}

// searchProfile searches preferences by key or value substring.
func (fs *FSStore) searchProfile(query string, topK int) []FSEntry {
	profile := fs.profileMgr.GetProfile("default")
	profile.mu.RLock()
	defer profile.mu.RUnlock()
	queryLower := strings.ToLower(query)
	var results []FSEntry
	for _, p := range profile.Preferences {
		if strings.Contains(strings.ToLower(p.Key), queryLower) ||
			strings.Contains(strings.ToLower(p.Value), queryLower) {
			results = append(results, FSEntry{
				Path:       fmt.Sprintf("/profile/%s/%s", p.Category, p.Key),
				Type:       "file",
				Size:       int64(len(p.Value)),
				ModifiedAt: p.LastObserved,
				Category:   "profile",
			})
		}
		if len(results) >= topK {
			break
		}
	}
	return results
}

// --- knowledge graph backend ---

// ensureKGLocked loads the kg cache from disk if not yet loaded.
// Caller must hold fs.kgMu.
func (fs *FSStore) ensureKGLocked() *kgFile {
	if fs.kgLoaded {
		return fs.kgCache
	}
	fs.kgCache = &kgFile{
		Entities:  make(map[string]kgEntity),
		Relations: make([]kgRelation, 0),
	}
	if fs.kgPath != "" {
		if data, err := os.ReadFile(fs.kgPath); err == nil { // #nosec G304 -- internally generated storage path
			if err := json.Unmarshal(data, fs.kgCache); err == nil {
				if fs.kgCache.Entities == nil {
					fs.kgCache.Entities = make(map[string]kgEntity)
				}
				if fs.kgCache.Relations == nil {
					fs.kgCache.Relations = make([]kgRelation, 0)
				}
			}
		}
	}
	fs.kgLoaded = true
	return fs.kgCache
}

// saveKGLocked persists the kg cache to disk. Caller must hold fs.kgMu.
func (fs *FSStore) saveKGLocked() error {
	if fs.kgPath == "" {
		return fmt.Errorf("knowledge graph path not configured")
	}
	if fs.kgCache == nil {
		return fmt.Errorf("knowledge graph not loaded")
	}
	data, err := json.MarshalIndent(fs.kgCache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fs.kgPath), 0755); err != nil {
		return fmt.Errorf("failed to create kg dir: %w", err)
	}
	return os.WriteFile(fs.kgPath, data, 0644)
}

// readKG handles /kg/entities/[<name>] and /kg/relations reads.
func (fs *FSStore) readKG(parts []string) (string, error) {
	fs.kgMu.Lock()
	defer fs.kgMu.Unlock()
	kg := fs.ensureKGLocked()
	if len(parts) < 2 || parts[1] == "" {
		return "", fmt.Errorf("kg subpath required: /kg/entities/[<name>] or /kg/relations")
	}
	switch parts[1] {
	case "entities":
		if len(parts) <= 2 {
			names := make([]string, 0, len(kg.Entities))
			for name := range kg.Entities {
				names = append(names, name)
			}
			sort.Strings(names)
			return strings.Join(names, "\n"), nil
		}
		name := parts[2]
		if len(name) > maxFSKeyLength {
			return "", fmt.Errorf("name too long (max %d)", maxFSKeyLength)
		}
		entity, ok := kg.Entities[name]
		if !ok {
			return "", fmt.Errorf("entity not found: %s", name)
		}
		data, err := json.MarshalIndent(entity, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal entity: %w", err)
		}
		return string(data), nil
	case "relations":
		data, err := json.MarshalIndent(kg.Relations, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal relations: %w", err)
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unknown kg subpath: %s (supported: entities, relations)", parts[1])
	}
}

// writeKG handles /kg/entities/<name> and /kg/relations writes.
func (fs *FSStore) writeKG(parts []string, content string) error {
	if len(parts) < 2 || parts[1] == "" {
		return fmt.Errorf("kg subpath required: /kg/entities/<name> or /kg/relations")
	}
	fs.kgMu.Lock()
	defer fs.kgMu.Unlock()
	kg := fs.ensureKGLocked()
	switch parts[1] {
	case "entities":
		if len(parts) < 3 || parts[2] == "" {
			return fmt.Errorf("entity name required: /kg/entities/<name>")
		}
		name := parts[2]
		if len(name) > maxFSKeyLength {
			return fmt.Errorf("name too long (max %d)", maxFSKeyLength)
		}
		var entity kgEntity
		if err := json.Unmarshal([]byte(content), &entity); err != nil {
			// Non-JSON content: treat as entity type string.
			entity = kgEntity{Type: content}
		}
		entity.Name = name
		kg.Entities[name] = entity
		return fs.saveKGLocked()
	case "relations":
		var rel kgRelation
		if err := json.Unmarshal([]byte(content), &rel); err != nil {
			return fmt.Errorf("invalid relation JSON: %w", err)
		}
		kg.Relations = append(kg.Relations, rel)
		return fs.saveKGLocked()
	default:
		return fmt.Errorf("unknown kg subpath: %s (supported: entities, relations)", parts[1])
	}
}

// deleteKG handles /kg/entities/<name> deletes.
func (fs *FSStore) deleteKG(parts []string) error {
	if len(parts) < 2 || parts[1] == "" {
		return fmt.Errorf("kg subpath required: /kg/entities/<name>")
	}
	fs.kgMu.Lock()
	defer fs.kgMu.Unlock()
	kg := fs.ensureKGLocked()
	switch parts[1] {
	case "entities":
		if len(parts) < 3 || parts[2] == "" {
			return fmt.Errorf("entity name required: /kg/entities/<name>")
		}
		name := parts[2]
		if _, ok := kg.Entities[name]; !ok {
			return fmt.Errorf("entity not found: %s", name)
		}
		delete(kg.Entities, name)
		return fs.saveKGLocked()
	default:
		return fmt.Errorf("delete not supported for kg/%s", parts[1])
	}
}

// listKG lists entries under /kg, /kg/entities, or /kg/entities/<name>.
func (fs *FSStore) listKG(parts []string) ([]FSEntry, error) {
	fs.kgMu.Lock()
	defer fs.kgMu.Unlock()
	kg := fs.ensureKGLocked()
	if len(parts) <= 1 {
		relSize := int64(len(kg.Relations))
		return []FSEntry{
			{Path: "/kg/entities", Type: "dir", Size: int64(len(kg.Entities)), Category: "kg"},
			{Path: "/kg/relations", Type: "file", Size: relSize, Category: "kg"},
		}, nil
	}
	switch parts[1] {
	case "entities":
		if len(parts) <= 2 {
			entries := make([]FSEntry, 0, len(kg.Entities))
			for name, entity := range kg.Entities {
				entries = append(entries, FSEntry{
					Path:     fmt.Sprintf("/kg/entities/%s", name),
					Type:     "file",
					Size:     int64(len(name) + len(entity.Type)),
					Category: "kg",
				})
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Path < entries[j].Path
			})
			return entries, nil
		}
		name := parts[2]
		entity, ok := kg.Entities[name]
		if !ok {
			return nil, fmt.Errorf("entity not found: %s", name)
		}
		return []FSEntry{{
			Path:     fmt.Sprintf("/kg/entities/%s", name),
			Type:     "file",
			Size:     int64(len(name) + len(entity.Type)),
			Category: "kg",
		}}, nil
	case "relations":
		return []FSEntry{{
			Path:     "/kg/relations",
			Type:     "file",
			Size:     int64(len(kg.Relations)),
			Category: "kg",
		}}, nil
	default:
		return nil, fmt.Errorf("unknown kg subpath: %s (supported: entities, relations)", parts[1])
	}
}

// searchKG searches entities by name or type substring.
func (fs *FSStore) searchKG(query string, topK int) []FSEntry {
	fs.kgMu.Lock()
	defer fs.kgMu.Unlock()
	kg := fs.ensureKGLocked()
	queryLower := strings.ToLower(query)
	var results []FSEntry
	for name, entity := range kg.Entities {
		if strings.Contains(strings.ToLower(name), queryLower) ||
			strings.Contains(strings.ToLower(entity.Type), queryLower) {
			results = append(results, FSEntry{
				Path:     fmt.Sprintf("/kg/entities/%s", name),
				Type:     "file",
				Size:     int64(len(name) + len(entity.Type)),
				Category: "kg",
			})
		}
		if len(results) >= topK {
			break
		}
	}
	return results
}

// --- skills backend ---

// skillFilePath returns the on-disk path for a skill name.
func (fs *FSStore) skillFilePath(name string) string {
	return filepath.Join(fs.skillsDir, name+".md")
}

// readSkill handles /skills/<name> reads.
func (fs *FSStore) readSkill(parts []string) (string, error) {
	if len(parts) < 2 || parts[1] == "" {
		return "", fmt.Errorf("skill name required: /skills/<name>")
	}
	name := parts[1]
	if len(name) > maxFSKeyLength {
		return "", fmt.Errorf("name too long (max %d)", maxFSKeyLength)
	}
	data, err := os.ReadFile(fs.skillFilePath(name)) // #nosec G304 -- internally generated storage path
	if err != nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return string(data), nil
}

// writeSkill handles /skills/<name> writes.
func (fs *FSStore) writeSkill(parts []string, content string) error {
	if len(parts) < 2 || parts[1] == "" {
		return fmt.Errorf("skill name required: /skills/<name>")
	}
	name := parts[1]
	if len(name) > maxFSKeyLength {
		return fmt.Errorf("name too long (max %d)", maxFSKeyLength)
	}
	if err := os.MkdirAll(fs.skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills dir: %w", err)
	}
	return os.WriteFile(fs.skillFilePath(name), []byte(content), 0644)
}

// deleteSkill handles /skills/<name> deletes.
func (fs *FSStore) deleteSkill(parts []string) error {
	if len(parts) < 2 || parts[1] == "" {
		return fmt.Errorf("skill name required: /skills/<name>")
	}
	name := parts[1]
	if err := os.Remove(fs.skillFilePath(name)); err != nil {
		return fmt.Errorf("skill not found: %s", name)
	}
	return nil
}

// listSkills lists skill files under /skills.
func (fs *FSStore) listSkills(parts []string) ([]FSEntry, error) {
	if len(parts) <= 1 {
		entries, err := os.ReadDir(fs.skillsDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		var result []FSEntry
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			result = append(result, FSEntry{
				Path:       fmt.Sprintf("/skills/%s", strings.TrimSuffix(name, ".md")),
				Type:       "file",
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
				Category:   "skills",
			})
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i].Path < result[j].Path
		})
		return result, nil
	}
	name := parts[1]
	info, err := os.Stat(fs.skillFilePath(name))
	if err != nil {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return []FSEntry{{
		Path:       fmt.Sprintf("/skills/%s", name),
		Type:       "file",
		Size:       info.Size(),
		ModifiedAt: info.ModTime(),
		Category:   "skills",
	}}, nil
}

// searchSkills searches skill files by name substring.
func (fs *FSStore) searchSkills(query string, topK int) []FSEntry {
	entries, err := os.ReadDir(fs.skillsDir)
	if err != nil {
		return nil
	}
	queryLower := strings.ToLower(query)
	var results []FSEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if strings.Contains(strings.ToLower(name), queryLower) {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			results = append(results, FSEntry{
				Path:       fmt.Sprintf("/skills/%s", name),
				Type:       "file",
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
				Category:   "skills",
			})
		}
		if len(results) >= topK {
			break
		}
	}
	return results
}
