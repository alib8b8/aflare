// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​​​​‌‌‌‌‌​​‌​​​​​‌‌‌​​​​​‌‌​​‌​‌‌​‌‌​‌‌​‌​​​​​​​​​​​​​​​​​​​‌‌‌‌​​‌‌‌‌‌​​​​‌⁠
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

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// TestContextManager_Summary verifies the /history summary format and the
// ok/compressed status switch around the token budget.
func TestContextManager_Summary(t *testing.T) {
	cm := NewContextManager()
	cm.AddUser("hello")
	cm.AddAssistant("hi there")

	s := cm.Summary()
	if !strings.Contains(s, "Messages: 2") || !strings.Contains(s, "Status: ok") {
		t.Errorf("Summary() = %q, want Messages: 2 and Status: ok", s)
	}

	// CJK content is estimated at 1.5 tokens/char: 6 messages × 1000 chars
	// = 9000 tokens > MaxContextChars → status flips to "compressed".
	big := strings.Repeat("中", 1000)
	for i := 0; i < 6; i++ {
		cm.AddUser(big)
	}
	if s := cm.Summary(); !strings.Contains(s, "Status: compressed") {
		t.Errorf("Summary() over budget = %q, want Status: compressed", s)
	}
}

// TestContextManager_ContextUsage verifies usage reporting and the
// compressed flag derived from a summary marker message.
func TestContextManager_ContextUsage(t *testing.T) {
	cm := NewContextManager()
	cm.AddUser("plain question")

	chars, limit, compressed := cm.ContextUsage()
	if limit != MaxContextChars {
		t.Errorf("ContextUsage() limit = %d, want %d", limit, MaxContextChars)
	}
	if compressed {
		t.Error("ContextUsage() compressed = true on fresh history, want false")
	}
	if chars <= 0 {
		t.Errorf("ContextUsage() chars = %d, want > 0", chars)
	}

	// Simulate a post-compression state: history replaced by a summary marker.
	cm2 := NewContextManager()
	cm2.messages = []core.LLMMessage{
		{Role: "system", Content: "[Previous conversation summary]\nsomething\n[End summary]"},
		{Role: "user", Content: "follow-up"},
	}
	if _, _, compressed := cm2.ContextUsage(); !compressed {
		t.Error("ContextUsage() compressed = false with summary marker, want true")
	}
}

// TestContextManager_TotalChars verifies the raw character accounting
// (byte length of message contents).
func TestContextManager_TotalChars(t *testing.T) {
	cm := NewContextManager()
	if got := cm.TotalChars(); got != 0 {
		t.Errorf("TotalChars() on empty = %d, want 0", got)
	}
	cm.AddUser("hello")   // 5 bytes
	cm.AddAssistant("你好") // 6 bytes (UTF-8 CJK = 3 bytes each)
	if got := cm.TotalChars(); got != 11 {
		t.Errorf("TotalChars() = %d, want 11", got)
	}
}

// TestWorkflowCapability_ParseTemplateMeta covers metadata extraction from
// workflow YAML: full success, step extraction, and all rejection branches.
func TestWorkflowCapability_ParseTemplateMeta(t *testing.T) {
	w := NewWorkflowCapability()
	dir := t.TempDir()

	// Valid workflow with map-style steps (the real template format).
	valid := filepath.Join(dir, "valid.yaml")
	yaml := "# comment\nname: stock-monitor\ndescription: watch prices\ncategory: finance\nsteps:\n  - node: http_request\n    id: fetch\n  - node: feishu_notify\n"
	if err := os.WriteFile(valid, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := w.parseTemplateMeta(valid)
	if meta == nil {
		t.Fatal("parseTemplateMeta(valid) = nil, want meta")
	}
	if meta.Name != "stock-monitor" || meta.Category != "finance" {
		t.Errorf("meta = %+v, want name stock-monitor / category finance", meta)
	}
	if len(meta.Steps) != 2 || meta.Steps[0] != "fetch" || meta.Steps[1] != "feishu_notify" {
		t.Errorf("meta.Steps = %v, want [fetch feishu_notify] (id preferred, node fallback)", meta.Steps)
	}

	// Missing name → nil.
	noName := filepath.Join(dir, "noname.yaml")
	if err := os.WriteFile(noName, []byte("description: no name here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if m := w.parseTemplateMeta(noName); m != nil {
		t.Errorf("parseTemplateMeta(noName) = %+v, want nil", m)
	}

	// Invalid YAML → nil.
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte(":\n  - {"), 0o600); err != nil {
		t.Fatal(err)
	}
	if m := w.parseTemplateMeta(bad); m != nil {
		t.Errorf("parseTemplateMeta(bad) = %+v, want nil", m)
	}

	// Unreadable path → nil.
	if m := w.parseTemplateMeta(filepath.Join(dir, "missing.yaml")); m != nil {
		t.Errorf("parseTemplateMeta(missing) = %+v, want nil", m)
	}
}

// TestExtractDescription verifies comment-based description extraction,
// including the Usage:/Config: skip rules and the three-comment cap.
func TestExtractDescription(t *testing.T) {
	dir := t.TempDir()

	full := filepath.Join(dir, "full.yaml")
	content := "# First line of description\n# Second line\n# Usage: aflare run\n# Config: none\nname: tpl\nsteps: []\n"
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := extractDescription(full); got != "First line of description Second line" {
		t.Errorf("extractDescription(full) = %q, want combined comment lines without Usage", got)
	}

	// More than three comment lines → capped at three.
	capped := filepath.Join(dir, "capped.yaml")
	content = "# one\n# two\n# three\n# four\n# five\nname: tpl\n"
	if err := os.WriteFile(capped, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := extractDescription(capped); strings.Count(got, " ") != 2 || !strings.HasPrefix(got, "one") || !strings.HasSuffix(got, "three") {
		t.Errorf("extractDescription(capped) = %q, want first three comments only", got)
	}

	// No comments at all → empty string.
	bare := filepath.Join(dir, "bare.yaml")
	if err := os.WriteFile(bare, []byte("name: tpl\nsteps: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := extractDescription(bare); got != "" {
		t.Errorf("extractDescription(bare) = %q, want empty", got)
	}

	// Unreadable path → empty string.
	if got := extractDescription(filepath.Join(dir, "missing.yaml")); got != "" {
		t.Errorf("extractDescription(missing) = %q, want empty", got)
	}
}

// TestCapabilityRegistry_All verifies registration order is preserved.
func TestCapabilityRegistry_All(t *testing.T) {
	cr := NewCapabilityRegistry()
	cr.Register(NewReflectionCapability())
	cr.Register(NewMemoryCapability())

	all := cr.All()
	if len(all) != 2 {
		t.Fatalf("All() length = %d, want 2", len(all))
	}
	if all[0].Name() != "reflection" || all[1].Name() != "memory" {
		t.Errorf("All() order = [%s, %s], want [reflection, memory]", all[0].Name(), all[1].Name())
	}
	if cr.Get("memory") == nil || cr.Get("nonexistent") != nil {
		t.Error("Get() lookup mismatch")
	}
}
