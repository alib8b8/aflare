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

package core

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// mockNode is a minimal Node implementation for testing the Registry
// without depending on real node implementations.
type mockNode struct {
	name        string
	description string
	executeErr  error
	output      string
}

func (m *mockNode) Name() string        { return m.name }
func (m *mockNode) Description() string { return m.description }
func (m *mockNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        m.name,
		Description: m.description,
		Input:       "string",
		Output:      "string",
	}
}
func (m *mockNode) Execute(_ context.Context, _ string, _ map[string]string) (string, error) {
	if m.executeErr != nil {
		return "", m.executeErr
	}
	if m.output != "" {
		return m.output, nil
	}
	return "ok", nil
}

// --- Registry basics ---

func TestNewRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if got := r.List(); len(got) != 0 {
		t.Errorf("new registry should be empty, got %v", got)
	}
	if got := r.ListNodes(); len(got) != 0 {
		t.Errorf("new registry ListNodes should be empty, got %v", got)
	}
	if r.IsSafeMode() {
		t.Error("new registry should not be in safe mode")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "alpha", description: "first"})

	node, ok := r.Get("alpha")
	if !ok {
		t.Fatal("expected to find registered node")
	}
	if node.Name() != "alpha" {
		t.Errorf("got name %q, want alpha", node.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("does-not-exist"); ok {
		t.Error("expected ok=false for missing node")
	}
}

func TestRegistry_Register_OverwriteBySameName(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "dup", description: "v1"})
	r.Register(&mockNode{name: "dup", description: "v2"})
	node, ok := r.Get("dup")
	if !ok {
		t.Fatal("expected to find dup")
	}
	if node.Description() != "v2" {
		t.Errorf("expected overwrite to v2, got %q", node.Description())
	}
	if len(r.List()) != 1 {
		t.Errorf("expected 1 node, got %d", len(r.List()))
	}
}

func TestRegistry_List_Sorted(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "zebra"})
	r.Register(&mockNode{name: "alpha"})
	r.Register(&mockNode{name: "mango"})
	got := r.List()
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, n := range got {
		if n != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestRegistry_ListNodes_FieldsAndSorting(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "zebra", description: "striped animal"})
	r.Register(&mockNode{name: "alpha", description: "first letter"})
	infos := r.ListNodes()
	if len(infos) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(infos))
	}
	// Should be sorted by name.
	if infos[0].Name != "alpha" || infos[1].Name != "zebra" {
		t.Errorf("expected sorted by name, got %v then %v", infos[0].Name, infos[1].Name)
	}
	if infos[0].Description != "first letter" {
		t.Errorf("description mismatch: %q", infos[0].Description)
	}
}

// --- Search ---

func TestRegistry_Search(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "fetch_url", description: "Fetch content from a URL"})
	r.Register(&mockNode{name: "json_parse", description: "Parse JSON documents"})
	r.Register(&mockNode{name: "file_read", description: "Read a file"})

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"by name", "fetch", []string{"fetch_url"}},
		{"by description", "JSON", []string{"json_parse"}},
		{"case insensitive name", "FILE_READ", []string{"file_read"}},
		{"case insensitive desc", "parse json", []string{"json_parse"}},
		{"no match", "nonexistent_xyz", nil},
		{"matches multiple via 'a' in description", "a", nil}, // validated separately below
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Search(c.query)
			var names []string
			for _, info := range got {
				names = append(names, info.Name)
			}
			sort.Strings(names)
			if c.want != nil {
				sort.Strings(c.want)
				if !equalStringSlices(names, c.want) {
					t.Errorf("Search(%q) names = %v, want %v", c.query, names, c.want)
				}
			}
			// results must always be sorted by name
			for i := 1; i < len(got); i++ {
				if got[i-1].Name > got[i].Name {
					t.Errorf("Search results not sorted: %q before %q", got[i-1].Name, got[i].Name)
				}
			}
		})
	}
}

func TestRegistry_Search_EmptyQuery(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "alpha"})
	// Empty query is a substring of everything -> all nodes returned.
	got := r.Search("")
	if len(got) != 1 {
		t.Errorf("Search(\"\") returned %d, want 1", len(got))
	}
}

func TestRegistry_Search_EmptyRegistry(t *testing.T) {
	r := NewRegistry()
	if got := r.Search("anything"); len(got) != 0 {
		t.Errorf("Search on empty registry returned %v", got)
	}
}

// --- NodesByCategory ---

func TestRegistry_NodesByCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "ollama", description: "Local LLM"})
	r.Register(&mockNode{name: "openai", description: "OpenAI API"})
	r.Register(&mockNode{name: "fetch_url", description: "Fetch URL"})
	r.Register(&mockNode{name: "file_read", description: "Read file"})

	llm := r.NodesByCategory(CategoryLLM)
	if len(llm) != 2 {
		t.Fatalf("expected 2 LLM nodes, got %d", len(llm))
	}
	gotNames := []string{llm[0].Name, llm[1].Name}
	sort.Strings(gotNames)
	if gotNames[0] != "ollama" || gotNames[1] != "openai" {
		t.Errorf("LLM category = %v, want [ollama openai]", gotNames)
	}

	io := r.NodesByCategory(CategoryIO)
	if len(io) != 2 {
		t.Fatalf("expected 2 IO nodes, got %d", len(io))
	}

	// Unregistered category members -> empty result.
	if got := r.NodesByCategory(CategorySecurity); len(got) != 0 {
		t.Errorf("expected 0 security nodes, got %d", len(got))
	}
}

func TestRegistry_NodesByCategory_UnknownCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "ollama"})
	if got := r.NodesByCategory(NodeCategory("bogus")); got != nil {
		t.Errorf("expected nil for unknown category, got %v", got)
	}
}

// --- SafeMode ---

func TestRegistry_SetSafeMode(t *testing.T) {
	r := NewRegistry()
	r.SetSafeMode(true)
	if !r.IsSafeMode() {
		t.Error("expected safe mode on after SetSafeMode(true)")
	}
	r.SetSafeMode(false)
	if r.IsSafeMode() {
		t.Error("expected safe mode off after SetSafeMode(false)")
	}
}

// --- Stats ---

func TestRegistry_ExecuteWithStats_Success(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "n", output: "hello world"})
	out, err := r.ExecuteWithStats("n", context.Background(), "input", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("got %q, want hello world", out)
	}
	s := r.GetStats("n")
	if s == nil {
		t.Fatal("expected stats after execution")
	}
	if s.Calls != 1 {
		t.Errorf("Calls = %d, want 1", s.Calls)
	}
	if s.Errors != 0 {
		t.Errorf("Errors = %d, want 0", s.Errors)
	}
	if s.InputBytes != int64(len("input")) {
		t.Errorf("InputBytes = %d, want %d", s.InputBytes, len("input"))
	}
	if s.OutputBytes != int64(len("hello world")) {
		t.Errorf("OutputBytes = %d, want %d", s.OutputBytes, len("hello world"))
	}
}

func TestRegistry_ExecuteWithStats_ErrorRecorded(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "n", executeErr: context.DeadlineExceeded})
	_, err := r.ExecuteWithStats("n", context.Background(), "input", nil)
	if err == nil {
		t.Fatal("expected error from node")
	}
	s := r.GetStats("n")
	if s == nil {
		t.Fatal("expected stats after error execution")
	}
	if s.Calls != 1 {
		t.Errorf("Calls = %d, want 1", s.Calls)
	}
	if s.Errors != 1 {
		t.Errorf("Errors = %d, want 1", s.Errors)
	}
}

func TestRegistry_ExecuteWithStats_NodeNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.ExecuteWithStats("missing", context.Background(), "input", nil)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
	if r.GetStats("missing") != nil {
		t.Error("expected nil stats for never-executed node")
	}
}

func TestRegistry_GetStats_CopyNotAlias(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "n", output: "out"})
	_, _ = r.ExecuteWithStats("n", context.Background(), "in", nil)
	s1 := r.GetStats("n")
	s1.Calls = 999 // mutate the returned copy
	s2 := r.GetStats("n")
	if s2.Calls == 999 {
		t.Error("GetStats returned an alias, not a copy")
	}
}

func TestRegistry_GetAllStats(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockNode{name: "a", output: "x"})
	r.Register(&mockNode{name: "b", output: "yy"})
	_, _ = r.ExecuteWithStats("a", context.Background(), "in", nil)
	_, _ = r.ExecuteWithStats("b", context.Background(), "in", nil)

	all := r.GetAllStats()
	if len(all) != 2 {
		t.Fatalf("expected 2 stats entries, got %d", len(all))
	}
	if all["a"].Calls != 1 || all["b"].Calls != 1 {
		t.Errorf("stats = %+v", all)
	}
	// Mutating the returned map should not affect internal state.
	a := all["a"]
	a.Calls = 0
	all["a"] = a
	if r.GetStats("a").Calls != 1 {
		t.Error("GetAllStats returned an alias, not a copy")
	}
}

// --- Concurrency ---

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < 20; i++ {
		r.Register(&mockNode{name: "node", description: "d"})
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Get("node")
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.ListNodes()
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Register(&mockNode{name: "other", description: "d"})
		}(i)
	}
	wg.Wait()
	// Should not panic; verify state is still consistent.
	if _, ok := r.Get("node"); !ok {
		t.Error("node missing after concurrent access")
	}
}

// --- Global registry ---

func TestGetGlobalRegistry_NonNil(t *testing.T) {
	if r := GetGlobalRegistry(); r == nil {
		t.Fatal("GetGlobalRegistry returned nil")
	}
}

func TestGlobalRegistry_RegisterGet(t *testing.T) {
	// Use a unique name to avoid colliding with other tests/registered nodes.
	const name = "global_test_node_unique"
	Register(&mockNode{name: name, description: "global"})
	defer func() {
		// Best-effort cleanup: global registry is process-wide; we cannot
		// remove nodes, but using a unique name keeps it isolated.
	}()
	if _, ok := Get(name); !ok {
		t.Errorf("global Get(%q) returned ok=false", name)
	}
	if _, ok := Get("definitely-not-registered-xyz"); ok {
		t.Error("expected ok=false for unregistered global node")
	}
}

func TestGlobalSafeMode(t *testing.T) {
	prev := IsSafeMode()
	SetSafeMode(true)
	if !IsSafeMode() {
		t.Error("expected global safe mode on")
	}
	SetSafeMode(false)
	if IsSafeMode() {
		t.Error("expected global safe mode off")
	}
	// Restore prior state.
	SetSafeMode(prev)
}

// --- LoadExternalNodes ---

func writeExternalNodeDir(t *testing.T, parent, name string) string {
	t.Helper()
	nodeDir := filepath.Join(parent, name)
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "name: " + name + "\ndescription: external node\nentry: entry.sh\n"
	if err := os.WriteFile(filepath.Join(nodeDir, "metadata.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "entry.sh"), []byte("#!/bin/bash\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return nodeDir
}

func TestLoadExternalNodes_LoadsNode(t *testing.T) {
	r := NewRegistry()
	dir := t.TempDir()
	writeExternalNodeDir(t, dir, "myext")

	if err := r.LoadExternalNodes(dir); err != nil {
		t.Fatalf("LoadExternalNodes failed: %v", err)
	}
	node, ok := r.Get("myext")
	if !ok {
		t.Fatal("expected external node 'myext' to be registered")
	}
	if node.Name() != "myext" {
		t.Errorf("got name %q, want myext", node.Name())
	}
}

func TestLoadExternalNodes_SafeModeSkips(t *testing.T) {
	r := NewRegistry()
	r.SetSafeMode(true)
	dir := t.TempDir()
	writeExternalNodeDir(t, dir, "myext2")

	if err := r.LoadExternalNodes(dir); err != nil {
		t.Fatalf("LoadExternalNodes failed: %v", err)
	}
	if _, ok := r.Get("myext2"); ok {
		t.Error("expected external node NOT registered in safe mode")
	}
}

func TestLoadExternalNodes_NonExistentDir(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadExternalNodes(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("expected nil error for non-existent dir, got: %v", err)
	}
}

func TestLoadExternalNodes_SkipsUnderscoreDirs(t *testing.T) {
	r := NewRegistry()
	dir := t.TempDir()
	writeExternalNodeDir(t, dir, "_skipme")
	writeExternalNodeDir(t, dir, "keepme")

	if err := r.LoadExternalNodes(dir); err != nil {
		t.Fatalf("LoadExternalNodes failed: %v", err)
	}
	if _, ok := r.Get("_skipme"); ok {
		t.Error("directories starting with _ should be skipped")
	}
	if _, ok := r.Get("keepme"); !ok {
		t.Error("expected 'keepme' to be loaded")
	}
}

// --- ExternalNode ---

func TestNewExternalNode(t *testing.T) {
	meta := NodeMetadata{Name: "ext", Description: "desc", Entry: "entry.sh"}
	n := NewExternalNode(meta, "/path")
	if n.Name() != "ext" {
		t.Errorf("Name = %q, want ext", n.Name())
	}
	if n.Description() != "desc" {
		t.Errorf("Description = %q, want desc", n.Description())
	}
	schema := n.Schema()
	if schema.Name != "ext" || schema.Input != "string" || schema.Output != "string" {
		t.Errorf("Schema = %+v", schema)
	}
}

func TestExternalNode_Execute_ShScript(t *testing.T) {
	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "entry.sh"), []byte("#!/bin/bash\necho external-output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "entry.sh"}, nodeDir)
	out, err := node.Execute(context.Background(), "input", map[string]string{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "external-output\n" {
		t.Errorf("got %q, want external-output\\n", out)
	}
}

func TestExternalNode_Execute_DisallowedExtension(t *testing.T) {
	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "entry.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "entry.txt"}, nodeDir)
	_, err := node.Execute(context.Background(), "input", map[string]string{})
	if err == nil {
		t.Fatal("expected error for .txt entry, got nil")
	}
}

func TestExternalNode_Execute_EntryNotFound(t *testing.T) {
	nodeDir := t.TempDir()
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "missing.sh"}, nodeDir)
	_, err := node.Execute(context.Background(), "input", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

func TestExternalNode_Execute_SymlinkRejected(t *testing.T) {
	nodeDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "real.sh")
	if err := os.WriteFile(target, []byte("#!/bin/bash\necho bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(nodeDir, "entry.sh")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "entry.sh"}, nodeDir)
	_, err := node.Execute(context.Background(), "input", map[string]string{})
	if err == nil {
		t.Fatal("expected error for symlink entry, got nil")
	}
}

func TestExternalNode_Execute_PathEscapeRejected(t *testing.T) {
	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "entry.sh"), []byte("#!/bin/bash\necho ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Entry name with traversal that escapes nodeDir.
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "../../entry.sh"}, nodeDir)
	_, err := node.Execute(context.Background(), "input", map[string]string{})
	if err == nil {
		t.Fatal("expected error for traversal entry, got nil")
	}
}

func TestExternalNode_Execute_RedactsSensitiveParams(t *testing.T) {
	// The external script echoes its stdin payload; verify sensitive keys
	// are stripped before being passed to the script.
	nodeDir := t.TempDir()
	script := "#!/bin/bash\ncat\n"
	if err := os.WriteFile(filepath.Join(nodeDir, "entry.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	node := NewExternalNode(NodeMetadata{Name: "ext", Description: "d", Entry: "entry.sh"}, nodeDir)
	out, err := node.Execute(context.Background(), "input", map[string]string{
		"safe_param":  "visible",
		"api_key":     "sk-secret",
		"token":       "tok-secret",
		"description": "shown",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "safe_param") || !strings.Contains(out, "visible") {
		t.Errorf("safe param should be present in payload: %s", out)
	}
	if strings.Contains(out, "sk-secret") || strings.Contains(out, "tok-secret") {
		t.Errorf("sensitive values should be redacted from payload: %s", out)
	}
}

// --- NodeMetadata / categories constants ---

func TestNodeCategoryConstants(t *testing.T) {
	cases := []struct {
		cat   NodeCategory
		value string
	}{
		{CategoryLLM, "llm"},
		{CategoryAgent, "agent"},
		{CategoryIO, "io"},
		{CategoryTransform, "transform"},
		{CategoryFlow, "flow"},
		{CategoryData, "data"},
		{CategorySecurity, "security"},
		{CategoryUtility, "utility"},
	}
	for _, c := range cases {
		if string(c.cat) != c.value {
			t.Errorf("category = %q, want %q", c.cat, c.value)
		}
	}
}

// --- helpers ---

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
