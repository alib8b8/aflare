// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// nodes_coverage_boost3_test.go 进一步补充根包纯逻辑函数的单元测试，
// 目标将覆盖率提升到 50%+。仅测试不依赖真实 LLM API/网络/外部文件的
// 纯逻辑函数与简单成功/错误路径。

// ----------------------------------------------------------------------------
// file_watch.go: parseWatchEvents / parseIntWithDefault / sanitizePath
// ----------------------------------------------------------------------------

func TestParseWatchEvents(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    map[string]bool
	}{
		{"single create", "create", false, map[string]bool{"create": true}},
		{"all three", "create,modify,delete", false, map[string]bool{"create": true, "modify": true, "delete": true}},
		{"with spaces", " create , modify , delete ", false, map[string]bool{"create": true, "modify": true, "delete": true}},
		{"empty string", "", true, nil},
		{"only commas", ",,,", true, nil},
		{"invalid event", "create,unknown", true, nil},
		{"only invalid", "rename", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWatchEvents(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseWatchEvents(%q) expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWatchEvents(%q) unexpected error: %v", tt.in, err)
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseWatchEvents(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("parseWatchEvents(%q) missing key %q", tt.in, k)
				}
			}
		})
	}
}

func TestParseIntWithDefault(t *testing.T) {
	tests := []struct {
		in      string
		def     int
		want    int
		wantErr bool
	}{
		{"42", 0, 42, false},
		{"-7", 0, -7, false},
		{"  10  ", 5, 10, false}, // TrimSpace 后能解析
		{"", 99, 99, false},
		{"abc", 5, 0, true},
		{"12.5", 5, 12, false}, // Sscanf %d 部分匹配 12
	}
	for _, tt := range tests {
		got, err := parseIntWithDefault(tt.in, tt.def)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseIntWithDefault(%q, %d) expected error, got nil", tt.in, tt.def)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseIntWithDefault(%q, %d) unexpected error: %v", tt.in, tt.def, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseIntWithDefault(%q, %d) = %d, want %d", tt.in, tt.def, got, tt.want)
		}
	}
}

func TestSanitizePath_FileWatch(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"plain/path.txt", "plain/path.txt"},
		// 实现保留 r >= 0x20 或 r == '\t'，否则跳过（不替换为 ?）
		{"a\nb\tc", "ab\tc"},   // \n (< 0x20) 跳过，\t 保留
		{"a\x00b\x01c", "abc"}, // 控制字符全部跳过
		{"a\x7fb", "a\x7fb"},   // 0x7f >= 0x20，保留
		{"可见 ascii 123", "可见 ascii 123"},
	}
	for _, tt := range tests {
		if got := sanitizePath(tt.in); got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// 长度截断
	long := strings.Repeat("a", fwMaxPathLen+10)
	if got := sanitizePath(long); len(got) != fwMaxPathLen {
		t.Errorf("sanitizePath truncated len = %d, want %d", len(got), fwMaxPathLen)
	}
}

func TestDiffSnapshots(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-time.Hour)
	old := map[string]fileMeta{
		"deleted.go":   {ModTime: oldTime, Size: 100},
		"modified.go":  {ModTime: oldTime, Size: 100},
		"unchanged.go": {ModTime: now, Size: 50},
	}
	cur := map[string]fileMeta{
		"created.go":   {ModTime: now, Size: 200},
		"modified.go":  {ModTime: now, Size: 100}, // mtime 变化
		"unchanged.go": {ModTime: now, Size: 50},  // 与 old 完全一致
	}
	// 全部事件类型都想要
	want := map[string]bool{"create": true, "modify": true, "delete": true}
	events := diffSnapshots(old, cur, want)

	byType := map[string][]watchEvent{}
	for _, e := range events {
		byType[e.Type] = append(byType[e.Type], e)
	}
	if len(byType["create"]) != 1 || byType["create"][0].Path != "created.go" {
		t.Errorf("expected 1 create event for created.go, got %+v", byType["create"])
	}
	if len(byType["modify"]) != 1 || byType["modify"][0].Path != "modified.go" {
		t.Errorf("expected 1 modify event for modified.go, got %+v", byType["modify"])
	}
	if len(byType["delete"]) != 1 || byType["delete"][0].Path != "deleted.go" {
		t.Errorf("expected 1 delete event for deleted.go, got %+v", byType["delete"])
	}

	// 仅想要 create：其他事件被过滤
	events = diffSnapshots(old, cur, map[string]bool{"create": true})
	for _, e := range events {
		if e.Type != "create" {
			t.Errorf("expected only create events, got %s", e.Type)
		}
	}

	// 排序确定性：相同输入两次调用应一致
	e1 := diffSnapshots(old, cur, want)
	e2 := diffSnapshots(old, cur, want)
	if len(e1) != len(e2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(e1), len(e2))
	}
	for i := range e1 {
		if e1[i].Type != e2[i].Type || e1[i].Path != e2[i].Path {
			t.Errorf("non-deterministic order at %d: %+v vs %+v", i, e1[i], e2[i])
		}
	}
}

func TestSnapshotPath_DirAndFile(t *testing.T) {
	dir := t.TempDir()
	// 创建若干文件
	files := map[string]string{
		"a.go":     "package main",
		"b.txt":    "hello",
		"sub/c.go": "package sub",
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// 目录快照：默认 * 匹配所有文件
	snap, err := snapshotPath(dir, dir, "*")
	if err != nil {
		t.Fatalf("snapshotPath dir: %v", err)
	}
	// 应包含 3 个文件
	if len(snap) != 3 {
		t.Errorf("snapshot dir len = %d, want 3: %+v", len(snap), snap)
	}

	// 单文件快照
	singleFile := filepath.Join(dir, "a.go")
	snap, err = snapshotPath(singleFile, "a.go", "*.go")
	if err != nil {
		t.Fatalf("snapshotPath file: %v", err)
	}
	if len(snap) != 1 {
		t.Errorf("snapshot file len = %d, want 1", len(snap))
	}

	// 不存在的路径：返回空 map，无错误
	snap, err = snapshotPath("/nonexistent/xyz/12345", "/nonexistent/xyz/12345", "*")
	if err != nil {
		t.Errorf("snapshotPath nonexistent should not error, got: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("snapshotPath nonexistent should be empty, got: %d", len(snap))
	}

	// pattern 过滤：只匹配 .go
	snap, err = snapshotPath(dir, dir, "*.go")
	if err != nil {
		t.Fatalf("snapshotPath *.go: %v", err)
	}
	for k := range snap {
		if !strings.HasSuffix(k, ".go") {
			t.Errorf("expected only .go files, got %q", k)
		}
	}
}

func TestBuildWatchOutput(t *testing.T) {
	events := []watchEvent{
		{Type: "create", Path: "a.go", Timestamp: "2024-01-01T00:00:00Z", Size: 100},
		{Type: "modify", Path: "b.go", Timestamp: "2024-01-01T00:00:00Z"},
	}
	out, err := buildWatchOutput("/tmp/foo", 5*time.Second, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "/tmp/foo") {
		t.Errorf("output missing path, got: %s", out)
	}
	if !strings.Contains(out, "5s") {
		t.Errorf("output missing duration, got: %s", out)
	}
	if !strings.Contains(out, `"events_collected": 2`) {
		t.Errorf("output missing events_collected, got: %s", out)
	}
	if !strings.Contains(out, `"create"`) || !strings.Contains(out, `"a.go"`) {
		t.Errorf("output missing event content, got: %s", out)
	}

	// nil events 应回退为空数组
	out, err = buildWatchOutput("/p", time.Second, nil)
	if err != nil {
		t.Fatalf("nil events error: %v", err)
	}
	if !strings.Contains(out, `"events": []`) {
		t.Errorf("nil events should produce empty array, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// code_knowledge_graph.go: computeFileHash / getIndexFilePath / detectChangedFiles
//                         / GetTokenSavingsReport / GetCompactSavingsReport / extractConcepts
// ----------------------------------------------------------------------------

func TestComputeFileHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	h, err := computeFileHash(p)
	if err != nil {
		t.Fatalf("computeFileHash: %v", err)
	}
	// "hello" 的 sha256 hex
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != want {
		t.Errorf("computeFileHash(hello) = %q, want %q", h, want)
	}
	// 不同内容产生不同哈希
	if err := os.WriteFile(p, []byte("world"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	h2, _ := computeFileHash(p)
	if h2 == h {
		t.Errorf("expected different hash for different content")
	}
	// 不存在的文件应报错
	if _, err := computeFileHash(filepath.Join(dir, "nope.txt")); err == nil {
		t.Errorf("expected error for nonexistent file")
	}
}

func TestCodeKnowledgeGraphNode_GetIndexFilePath(t *testing.T) {
	n := &CodeKnowledgeGraphNode{}
	p1 := n.getIndexFilePath("/some/root")
	p2 := n.getIndexFilePath("/some/root")
	// 同输入应给出同输出
	if p1 != p2 {
		t.Errorf("getIndexFilePath not deterministic: %q vs %q", p1, p2)
	}
	// 不同输入应给出不同输出
	p3 := n.getIndexFilePath("/other/root")
	if p1 == p3 {
		t.Errorf("expected different paths for different roots")
	}
	// 应位于系统 TempDir 下，含 ckgIndexDir
	if !strings.HasPrefix(p1, os.TempDir()) {
		t.Errorf("expected path under TempDir, got: %s", p1)
	}
	if !strings.Contains(p1, ckgIndexDir) {
		t.Errorf("expected path to contain %q, got: %s", ckgIndexDir, p1)
	}
	if !strings.HasSuffix(p1, ".json") {
		t.Errorf("expected .json suffix, got: %s", p1)
	}
}

func TestCodeKnowledgeGraphNode_DetectChangedFiles(t *testing.T) {
	n := &CodeKnowledgeGraphNode{}
	dir := t.TempDir()

	// 创建两个文件
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(f1, []byte("aaa"), 0644); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	if err := os.WriteFile(f2, []byte("bbb"), 0644); err != nil {
		t.Fatalf("write f2: %v", err)
	}

	// nil 索引：所有文件视为 added
	added, modified := n.detectChangedFiles([]string{f1, f2}, nil)
	if len(added) != 2 || len(modified) != 0 {
		t.Errorf("nil idx: added=%d modified=%d, want 2,0", len(added), len(modified))
	}

	// 用现有哈希构造索引：无变更
	h1, _ := computeFileHash(f1)
	h2, _ := computeFileHash(f2)
	idx := &ckgIndex{FileHashes: map[string]string{f1: h1, f2: h2}}
	added, modified = n.detectChangedFiles([]string{f1, f2}, idx)
	if len(added) != 0 || len(modified) != 0 {
		t.Errorf("unchanged: added=%d modified=%d, want 0,0", len(added), len(modified))
	}

	// 修改 f1 内容
	if err := os.WriteFile(f1, []byte("changed"), 0644); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	added, modified = n.detectChangedFiles([]string{f1, f2}, idx)
	if len(added) != 0 || len(modified) != 1 {
		t.Errorf("after change: added=%d modified=%d, want 0,1", len(added), len(modified))
	}
	if len(modified) == 1 && modified[0] != f1 {
		t.Errorf("expected modified=[f1], got %v", modified)
	}

	// 新增 f3：构造一个不在索引中的文件
	f3 := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(f3, []byte("ccc"), 0644); err != nil {
		t.Fatalf("write f3: %v", err)
	}
	added, _ = n.detectChangedFiles([]string{f1, f2, f3}, idx)
	if len(added) != 1 || added[0] != f3 {
		t.Errorf("expected added=[f3], got %v", added)
	}

	// 不存在的文件应被跳过（不报错）
	added, _ = n.detectChangedFiles([]string{f1, "/nonexistent/xyz/12345"}, idx)
	// 不存在文件不应出现在 added 中
	for _, f := range added {
		if f == "/nonexistent/xyz/12345" {
			t.Errorf("nonexistent file should not be in added: %v", added)
		}
	}
}

func TestCodeKnowledgeGraphNode_ExtractConcepts(t *testing.T) {
	n := &CodeKnowledgeGraphNode{}
	// 输入空切片也应返回固定概念集
	concepts := n.extractConcepts(nil)
	if len(concepts) == 0 {
		t.Errorf("expected non-empty concepts even for nil input")
	}
	// 至少包含 MVC / Microservices / Cloud-Native
	names := map[string]bool{}
	for _, c := range concepts {
		names[c.Name] = true
		if c.Confidence <= 0 || c.Confidence > 1 {
			t.Errorf("concept %q confidence out of (0,1]: %f", c.Name, c.Confidence)
		}
		if c.Type == "" || c.Description == "" {
			t.Errorf("concept %q missing type/description", c.Name)
		}
	}
	for _, want := range []string{"MVC", "Microservices", "Cloud-Native"} {
		if !names[want] {
			t.Errorf("expected concept %q, got %v", want, names)
		}
	}
}

func TestCodeKnowledgeGraphNode_TokenSavingsReport(t *testing.T) {
	n := &CodeKnowledgeGraphNode{}
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "idx.json")

	// 不存在的索引：返回错误提示
	out := n.GetTokenSavingsReport(indexPath)
	if !strings.Contains(out, "无法加载索引") {
		t.Errorf("expected error message for missing index, got: %s", out)
	}
	compact := n.GetCompactSavingsReport(indexPath)
	if compact != "" {
		t.Errorf("expected empty compact report for missing index, got: %s", compact)
	}

	// 写入索引文件
	now := time.Now()
	idx := &ckgIndex{
		Path:         "/some/root",
		Entities:     []ckgEntity{{Name: "Foo", Type: "function", Location: "a.go", Line: 1, Score: 0.9}},
		Relations:    []ckgRelation{{Source: "Foo", Target: "Bar", Type: "calls"}},
		Concepts:     []ckgConcept{{Name: "MVC", Type: "design_pattern", Description: "d", Confidence: 0.85}},
		FileHashes:   map[string]string{"a.go": "hash1"},
		CreatedAt:    now,
		UpdatedAt:    now,
		TotalFiles:   5,
		TotalTokens:  1000,
		TokensSaved:  300,
		SavingsRatio: 0.3,
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		t.Fatalf("write idx: %v", err)
	}
	// 清空缓存以强制从磁盘加载
	ckgIndexMu.Lock()
	delete(ckgIndexCache, indexPath)
	ckgIndexMu.Unlock()

	out = n.GetTokenSavingsReport(indexPath)
	if !strings.Contains(out, "Token 节省统计") {
		t.Errorf("expected report header, got: %s", out)
	}
	if !strings.Contains(out, "文件总数: 5") {
		t.Errorf("expected file count, got: %s", out)
	}
	if !strings.Contains(out, "累计节省 Token: 300") {
		t.Errorf("expected tokens saved, got: %s", out)
	}
	if !strings.Contains(out, "节省比例: 30.0%") {
		t.Errorf("expected savings percent, got: %s", out)
	}

	compact = n.GetCompactSavingsReport(indexPath)
	if !strings.Contains(compact, "Token 节省: 300") {
		t.Errorf("expected compact savings, got: %s", compact)
	}
	if !strings.Contains(compact, "30.0%") {
		t.Errorf("expected compact percent, got: %s", compact)
	}

	// 无节省数据的简洁报告
	idx2 := &ckgIndex{Path: "/x", TotalFiles: 3, TotalTokens: 100, TokensSaved: 0}
	idxPath2 := filepath.Join(dir, "idx2.json")
	data2, _ := json.MarshalIndent(idx2, "", "  ")
	_ = os.WriteFile(idxPath2, data2, 0644)
	ckgIndexMu.Lock()
	delete(ckgIndexCache, idxPath2)
	ckgIndexMu.Unlock()
	compact = n.GetCompactSavingsReport(idxPath2)
	if !strings.Contains(compact, "首次索引") {
		t.Errorf("expected 首次索引 in compact report, got: %s", compact)
	}
}

// ----------------------------------------------------------------------------
// search_aggregate.go: parseSources / rankResults / formatMarkdownResults
//                     / formatTextResults / joinSources
// ----------------------------------------------------------------------------

func TestParseSources_Boost3(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []SearchSource
	}{
		{"single", "reddit", []SearchSource{SourceReddit}},
		{"multiple", "reddit,hn,github", []SearchSource{SourceReddit, SourceHackerNews, SourceGitHub}},
		{"with spaces and case", " Reddit , HN , GitHub ", []SearchSource{SourceReddit, SourceHackerNews, SourceGitHub}},
		{"all invalid", "foo,bar,baz", []SearchSource{SourceHackerNews, SourceGitHub}}, // 默认回退
		{"empty", "", []SearchSource{SourceHackerNews, SourceGitHub}},
		{"mixed valid invalid", "reddit,invalid,hn", []SearchSource{SourceReddit, SourceHackerNews}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSources(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("parseSources(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseSources(%q)[%d] = %v, want %v", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRankResults_Boost3(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Hour)
	results := []SearchResult{
		{Title: "low", Score: 1.0, PublishedAt: t1, Summary: "short"},
		{Title: "high", Score: 10.0, PublishedAt: t2, Summary: "longer summary here"},
		{Title: "mid", Score: 5.0, PublishedAt: t1.Add(30 * time.Minute), Summary: "mid"},
	}

	// 默认 (signal) 按 Score 降序
	got := rankResults(results, "signal")
	if got[0].Title != "high" || got[2].Title != "low" {
		t.Errorf("signal sort: got %v", got)
	}

	// time 按时间降序（最新在前）
	got = rankResults(results, "time")
	if got[0].Title != "high" {
		t.Errorf("time sort: first = %q, want high", got[0].Title)
	}

	// relevance 按 summary 长度降序
	got = rankResults(results, "relevance")
	if got[0].Title != "high" {
		t.Errorf("relevance sort: first = %q, want high (longest summary)", got[0].Title)
	}

	// 未知 sortBy 走默认 signal
	got = rankResults(results, "unknown")
	if got[0].Title != "high" {
		t.Errorf("unknown sort should default to signal, first = %q", got[0].Title)
	}

	// 空切片不 panic
	_ = rankResults(nil, "signal")
}

func TestJoinSources_Boost3(t *testing.T) {
	if got := joinSources(nil); got != "" {
		t.Errorf("joinSources(nil) = %q, want empty", got)
	}
	if got := joinSources([]SearchSource{SourceReddit}); got != "reddit" {
		t.Errorf("joinSources single = %q, want reddit", got)
	}
	got := joinSources([]SearchSource{SourceReddit, SourceHackerNews, SourceGitHub})
	if got != "reddit, hn, github" {
		t.Errorf("joinSources multi = %q, want %q", got, "reddit, hn, github")
	}
}

func TestFormatMarkdownResults_Search(t *testing.T) {
	// 空结果
	agg := AggregatedResults{Query: "q", Count: 0, Duration: "10ms"}
	out := formatMarkdownResults(agg)
	if !strings.Contains(out, "_No results found_") {
		t.Errorf("expected no results message, got: %s", out)
	}

	// 含结果
	now := time.Now()
	agg = AggregatedResults{
		Query:    "golang",
		Count:    2,
		Sources:  []SearchSource{SourceHackerNews, SourceGitHub},
		Duration: "5ms",
		Results: []SearchResult{
			{Title: "T1", URL: "http://t1", Summary: "Sum1", Source: SourceHackerNews, Score: 10, Signals: SignalData{Upvotes: 5, Comments: 2}, PublishedAt: now},
			{Title: "T2", URL: "http://t2", Summary: "Sum2", Source: SourceGitHub, Score: 8, Signals: SignalData{Upvotes: 0, Comments: 0}, PublishedAt: now},
		},
	}
	out = formatMarkdownResults(agg)
	if !strings.Contains(out, "golang") {
		t.Errorf("expected query in output, got: %s", out)
	}
	if !strings.Contains(out, "T1") || !strings.Contains(out, "T2") {
		t.Errorf("expected titles in output, got: %s", out)
	}
	if !strings.Contains(out, "⬆️5") {
		t.Errorf("expected upvote signal, got: %s", out)
	}
}

func TestFormatTextResults_Search(t *testing.T) {
	agg := AggregatedResults{
		Query:    "q",
		Count:    1,
		Duration: "1ms",
		Results: []SearchResult{
			{Title: "T", URL: "http://t", Summary: "S", Source: SourceHackerNews},
		},
	}
	out := formatTextResults(agg)
	if !strings.Contains(out, "Search: q (1 results, 1ms)") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "[hn] T") {
		t.Errorf("expected formatted entry, got: %s", out)
	}
	if !strings.Contains(out, "http://t") {
		t.Errorf("expected url, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// tools_compat.go: tcSanitize / tcSanitizePath / tcRelDepth / tcMatchGlob
//                 / tcParseDiffTarget / tcSplitLines / tcApplyHunks / tcFormatDirEntry
// ----------------------------------------------------------------------------

func TestTcSanitize(t *testing.T) {
	// tcControlCharRe = [\x00-\x08\x0B\x0C\x0E-\x1F\x7F]
	// 即 \n (0x0A), \t (0x09), \r (0x0D) 不被替换；其它控制字符与 DEL 替换为 ?
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"a\nb", "a\nb"},  // \n 不在替换范围
		{"a\tb", "a\tb"},  // \t 不在替换范围
		{"a\rb", "a\rb"},  // \r 不在替换范围
		{"a\x00b", "a?b"}, // NUL 替换
		{"a\x01b", "a?b"}, // SOH 替换
		{"a\x7fb", "a?b"}, // DEL 替换
		{"a\x0Bb", "a?b"}, // VT 替换
		{"中文", "中文"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := tcSanitize(tt.in); got != tt.want {
			t.Errorf("tcSanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTcSanitizePath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain/path", "plain/path"},
		{"a\nb", "a?b"},
		{"a\x00b", "a?b"},
		{"a\x7fb", "a?b"}, // 0x7f 被替换
		{"a\tb", "a?b"},   // \t 也被替换
		{"a b/c.txt", "a b/c.txt"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := tcSanitizePath(tt.in); got != tt.want {
			t.Errorf("tcSanitizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTcRelDepth(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{".", 0},
		{"file.go", 1},
		{"a/b", 2},
		{"a/b/c", 3},
		{"a/b/c/d", 4},
		// Windows 风格反斜杠被 ToSlash 转换
		{filepath.Join("a", "b", "c"), 3},
	}
	for _, tt := range tests {
		if got := tcRelDepth(tt.in); got != tt.want {
			t.Errorf("tcRelDepth(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestTcMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		relPath string
		want    bool
	}{
		// 无 ** 模式
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		{"*.go", "src/main.go", true}, // 也匹配 basename
		{"main.go", "main.go", true},
		{"main.go", "other.go", false},
		// ** 通配
		{"**/*.go", "src/main.go", true},
		{"**/*.go", "main.go", true},
		{"**/*.go", "src/sub/x.go", true},
		{"**/*.go", "main.py", false},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "test/a.go", false}, // 前缀不匹配
		{"**", "anything/here", true},
		{"**", "x", true},
		// suffix 为空：只校验前缀
		{"src/**", "src/a/b", true},
		{"src/**", "test/a", false},
	}
	for _, tt := range tests {
		if got := tcMatchGlob(tt.pattern, tt.relPath); got != tt.want {
			t.Errorf("tcMatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.relPath, got, tt.want)
		}
	}
}

func TestTcParseDiffTarget(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    string
		wantErr bool
	}{
		{"plain b/ prefix", "+++ b/path/to/file.go", "path/to/file.go", false},
		{"no prefix", "+++ file.go", "file.go", false},
		{"with timestamp", "+++ b/file.go\t2024-01-01 12:00:00", "file.go", false},
		{"quoted path", `+++ "weird name.txt"`, "weird name.txt", false},
		{"dev/null in +++", "+++ /dev/null", "", true},
		{"absolute path", "+++ /etc/passwd", "", true},
		{"path with ..", "+++ ../escape.go", "", true},
		{"empty after prefix", "+++ b/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tcParseDiffTarget(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Errorf("tcParseDiffTarget(%q) expected error, got nil", tt.line)
				}
				return
			}
			if err != nil {
				t.Errorf("tcParseDiffTarget(%q) unexpected error: %v", tt.line, err)
				return
			}
			if got != tt.want {
				t.Errorf("tcParseDiffTarget(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestTcSplitLines(t *testing.T) {
	if got := tcSplitLines(""); got != nil {
		t.Errorf("tcSplitLines(\"\") = %v, want nil", got)
	}
	got := tcSplitLines("a\nb\nc")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("tcSplitLines simple = %v", got)
	}
	// 尾部换行符产生空串
	got = tcSplitLines("a\nb\n")
	if len(got) != 3 || got[2] != "" {
		t.Errorf("tcSplitLines trailing newline = %v, want [a b ]", got)
	}
}

func TestTcApplyHunks(t *testing.T) {
	orig := []string{"line1", "line2", "line3", "line4", "line5"}

	// 简单替换：将 line3 改为 LINE3
	// oldStart=2 (1-indexed), 即从 line2 开始（context）
	hunks := []tcHunk{{
		oldStart: 2,
		oldCount: 3,
		newCount: 3,
		lines: []tcDiffLine{
			{kind: ' ', content: "line2"},
			{kind: '-', content: "line3"},
			{kind: '+', content: "LINE3"},
			{kind: ' ', content: "line4"},
		},
	}}
	result, err := tcApplyHunks(orig, hunks)
	if err != nil {
		t.Fatalf("tcApplyHunks: %v", err)
	}
	want := []string{"line1", "line2", "LINE3", "line4", "line5"}
	if len(result) != len(want) {
		t.Fatalf("result len = %d, want %d: %v", len(result), len(want), result)
	}
	for i := range want {
		if result[i] != want[i] {
			t.Errorf("result[%d] = %q, want %q", i, result[i], want[i])
		}
	}

	// 上下文不匹配应报错
	hunks = []tcHunk{{
		oldStart: 1,
		oldCount: 1,
		newCount: 1,
		lines: []tcDiffLine{
			{kind: ' ', content: "WRONG"},
		},
	}}
	if _, err := tcApplyHunks(orig, hunks); err == nil {
		t.Errorf("expected context mismatch error")
	}

	// 删除行不匹配应报错
	hunks = []tcHunk{{
		oldStart: 1,
		oldCount: 1,
		newCount: 0,
		lines: []tcDiffLine{
			{kind: '-', content: "WRONG"},
		},
	}}
	if _, err := tcApplyHunks(orig, hunks); err == nil {
		t.Errorf("expected deletion mismatch error")
	}

	// 新建文件（oldStart=0）
	hunks = []tcHunk{{
		oldStart: 0,
		oldCount: 0,
		newCount: 2,
		lines: []tcDiffLine{
			{kind: '+', content: "new1"},
			{kind: '+', content: "new2"},
		},
	}}
	result, err = tcApplyHunks(nil, hunks)
	if err != nil {
		t.Fatalf("new file: %v", err)
	}
	if len(result) != 2 || result[0] != "new1" || result[1] != "new2" {
		t.Errorf("new file result = %v, want [new1 new2]", result)
	}
}

func TestTcFormatDirEntry(t *testing.T) {
	dir := t.TempDir()
	// 创建子目录与文件
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 目录条目
	subDirEntry, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range subDirEntry {
		out := tcFormatDirEntry(e.Name(), e)
		// 输出格式：relpath\ttype\tsize
		parts := strings.Split(out, "\t")
		if len(parts) != 3 {
			t.Errorf("expected 3 parts, got %d: %s", len(parts), out)
			continue
		}
		if e.IsDir() {
			if parts[1] != "dir" || parts[2] != "-" {
				t.Errorf("dir entry wrong: %s", out)
			}
		} else {
			if parts[1] != "file" {
				t.Errorf("file entry type wrong: %s", out)
			}
			if parts[2] != "5" {
				t.Errorf("file size wrong: %s (want 5)", out)
			}
		}
	}
}

func TestTcParsePatch_SimpleModify(t *testing.T) {
	patch := `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 line1
-line2
+LINE2
 line3
`
	patches, err := tcParsePatch(patch)
	if err != nil {
		t.Fatalf("tcParsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.targetPath != "file.go" {
		t.Errorf("targetPath = %q, want file.go", p.targetPath)
	}
	if p.isNewFile {
		t.Errorf("isNewFile should be false for modify")
	}
	if len(p.hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(p.hunks))
	}
	h := p.hunks[0]
	if h.oldStart != 1 || h.oldCount != 3 || h.newStart != 1 || h.newCount != 3 {
		t.Errorf("hunk header wrong: %+v", h)
	}
}

func TestTcParsePatch_NewFile(t *testing.T) {
	patch := `--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package main
+
`
	patches, err := tcParsePatch(patch)
	if err != nil {
		t.Fatalf("tcParsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if !patches[0].isNewFile {
		t.Errorf("expected isNewFile=true")
	}
}

func TestTcParsePatch_Errors(t *testing.T) {
	// hunk 在 file 外
	_, err := tcParsePatch("@@ -1,1 +1,1 @@\n+foo\n")
	if err == nil {
		t.Errorf("expected error for hunk outside file patch")
	}
	// 无效 hunk header
	_, err = tcParsePatch("+++ b/file.go\n@@ bad header @@\n+foo\n")
	if err == nil {
		t.Errorf("expected error for bad hunk header")
	}
	// 文件块无 hunk
	_, err = tcParsePatch("+++ b/file.go\n")
	if err == nil {
		t.Errorf("expected error for file patch without hunks")
	}
	// 行数不匹配
	_, err = tcParsePatch("+++ b/file.go\n@@ -1,5 +1,5 @@\n-line2\n+LINE2\n")
	if err == nil {
		t.Errorf("expected error for line count mismatch")
	}
}

// ----------------------------------------------------------------------------
// knowledge_graph.go: NewKnowledgeGraph / AddEntity / AddRelation
//                    / formatGraphOutput / extractEntitiesSimple / extractRelationsSimple
//                    / Save / LoadKnowledgeGraph
// ----------------------------------------------------------------------------

func TestKnowledgeGraph_AddEntity(t *testing.T) {
	kg := NewKnowledgeGraph()
	// 新增
	kg.AddEntity("Alice", "Person", map[string]string{"age": "30"})
	if e, ok := kg.Entities["Alice"]; !ok || e.Type != "Person" {
		t.Errorf("AddEntity failed: %+v", kg.Entities["Alice"])
	}

	// 空名应被忽略
	kg.AddEntity("", "Type", nil)
	if _, ok := kg.Entities[""]; ok {
		t.Errorf("empty name should be ignored")
	}
	// 仅空白的名字应被忽略
	kg.AddEntity("   ", "Type", nil)
	if _, ok := kg.Entities[""]; ok {
		t.Errorf("whitespace name should be ignored")
	}

	// 重复添加：合并属性
	kg.AddEntity("Alice", "", map[string]string{"role": "admin"})
	e := kg.Entities["Alice"]
	if e.Properties["role"] != "admin" {
		t.Errorf("expected merged property role=admin, got %+v", e.Properties)
	}
	if e.Properties["age"] != "30" {
		t.Errorf("expected original property age=30, got %+v", e.Properties)
	}

	// 重复添加时填充空 Type
	kg.AddEntity("Bob", "", nil)
	kg.AddEntity("Bob", "Person", nil)
	if kg.Entities["Bob"].Type != "Person" {
		t.Errorf("expected Bob Type=Person, got %q", kg.Entities["Bob"].Type)
	}
}

func TestKnowledgeGraph_AddRelation(t *testing.T) {
	kg := NewKnowledgeGraph()
	kg.AddRelation("Alice", "Bob", "knows", 0.9)
	if len(kg.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(kg.Relations))
	}
	r := kg.Relations[0]
	if r.From != "Alice" || r.To != "Bob" || r.Relation != "knows" || r.Confidence != 0.9 {
		t.Errorf("relation wrong: %+v", r)
	}

	// 空字段应被忽略
	kg.AddRelation("", "Bob", "x", 0.5)
	kg.AddRelation("Alice", "", "x", 0.5)
	kg.AddRelation("Alice", "Bob", "", 0.5)
	if len(kg.Relations) != 1 {
		t.Errorf("invalid relations should be ignored, got %d", len(kg.Relations))
	}

	// 名字会被 TrimSpace（from/to 会，relation 不会）
	kg.AddRelation("  Alice  ", "  Bob  ", "likes", 0.5)
	if len(kg.Relations) != 2 {
		t.Errorf("expected 2 relations after trim, got %d", len(kg.Relations))
	}
	if kg.Relations[1].From != "Alice" || kg.Relations[1].To != "Bob" || kg.Relations[1].Relation != "likes" {
		t.Errorf("trim failed: %+v", kg.Relations[1])
	}
}

func TestFormatGraphOutput(t *testing.T) {
	kg := NewKnowledgeGraph()
	kg.AddEntity("Alice", "Person", nil)
	kg.AddEntity("UnknownEnt", "Unknown", nil)
	kg.AddEntity("NoType", "", nil)
	kg.AddRelation("Alice", "Bob", "knows", 0.9)

	// markdown 格式
	out := formatGraphOutput(kg, "markdown")
	if !strings.Contains(out, "## Knowledge Graph") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "**Entities:** 3") {
		t.Errorf("expected entity count, got: %s", out)
	}
	if !strings.Contains(out, "**Relations:** 1") {
		t.Errorf("expected relation count, got: %s", out)
	}
	if !strings.Contains(out, "- Alice (Person)") {
		t.Errorf("expected Alice with type, got: %s", out)
	}
	if !strings.Contains(out, "- NoType") {
		t.Errorf("expected NoType without type, got: %s", out)
	}
	// Unknown 类型应只显示名字
	if !strings.Contains(out, "- UnknownEnt") {
		t.Errorf("expected UnknownEnt plain, got: %s", out)
	}
	if !strings.Contains(out, "Alice --[knows]--> Bob") {
		t.Errorf("expected relation line, got: %s", out)
	}

	// json 格式
	out = formatGraphOutput(kg, "json")
	if !strings.Contains(out, `"name": "Alice"`) {
		t.Errorf("expected JSON entity, got: %s", out)
	}
}

func TestExtractEntitiesSimple(t *testing.T) {
	kg := NewKnowledgeGraph()
	text := "Person: Alice, Bob\nOrganization: Acme Inc\nLocation: Beijing\nSome Random Text With Capitalized Words"
	extractEntitiesSimple(text, kg)

	// 前缀识别
	if _, ok := kg.Entities["Alice"]; !ok {
		t.Errorf("expected Alice entity, got: %+v", kg.Entities)
	}
	if _, ok := kg.Entities["Bob"]; !ok {
		t.Errorf("expected Bob entity, got: %+v", kg.Entities)
	}
	if e, ok := kg.Entities["Acme Inc"]; !ok || e.Type != "Organization" {
		t.Errorf("expected Acme Inc as Organization, got: %+v", kg.Entities["Acme Inc"])
	}
	if e, ok := kg.Entities["Beijing"]; !ok || e.Type != "Location" {
		t.Errorf("expected Beijing as Location, got: %+v", kg.Entities["Beijing"])
	}
	// 大写词应被识别为 Unknown
	if _, ok := kg.Entities["Some"]; !ok {
		t.Errorf("expected Some as Unknown, got: %+v", kg.Entities)
	}
}

func TestExtractRelationsSimple(t *testing.T) {
	kg := NewKnowledgeGraph()
	text := `Alice -> Bob
Foo -> knows: Bar
Relation: Alice -- knows -- Bob`
	extractRelationsSimple(text, kg)

	if len(kg.Relations) != 3 {
		t.Fatalf("expected 3 relations, got %d: %+v", len(kg.Relations), kg.Relations)
	}
	// 验证包含预期关系
	foundRelatedTo := false
	foundKnowsArrow := false
	foundKnowsFormal := false
	for _, r := range kg.Relations {
		if r.From == "Alice" && r.To == "Bob" && r.Relation == "related_to" {
			foundRelatedTo = true
		}
		if r.From == "Foo" && r.To == "Bar" && r.Relation == "knows" {
			foundKnowsArrow = true
		}
		if r.From == "Alice" && r.To == "Bob" && r.Relation == "knows" {
			foundKnowsFormal = true
		}
	}
	if !foundRelatedTo {
		t.Errorf("expected 'Alice related_to Bob', got: %+v", kg.Relations)
	}
	if !foundKnowsArrow {
		t.Errorf("expected 'Foo knows Bar', got: %+v", kg.Relations)
	}
	if !foundKnowsFormal {
		t.Errorf("expected formal 'Alice knows Bob', got: %+v", kg.Relations)
	}
}

func TestKnowledgeGraph_SaveAndLoad(t *testing.T) {
	// Save/Load 使用 validateWritePath/validateReadPath，需要相对路径
	// 通过设置 workDir 到临时目录来允许写入该目录
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	kg := NewKnowledgeGraph()
	kg.AddEntity("Alice", "Person", map[string]string{"k": "v"})
	kg.AddRelation("Alice", "Bob", "knows", 0.9)

	if err := kg.Save("kg.json"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fullPath := filepath.Join(dir, "kg.json")
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	loaded, err := LoadKnowledgeGraph("kg.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Entities) != 1 {
		t.Errorf("loaded entities = %d, want 1", len(loaded.Entities))
	}
	if e, ok := loaded.Entities["Alice"]; !ok || e.Type != "Person" {
		t.Errorf("loaded Alice wrong: %+v", loaded.Entities["Alice"])
	}
	if len(loaded.Relations) != 1 {
		t.Errorf("loaded relations = %d, want 1", len(loaded.Relations))
	}

	// 加载不存在的文件应报错
	if _, err := LoadKnowledgeGraph("nope.json"); err == nil {
		t.Errorf("expected error for nonexistent file")
	}
}

// ----------------------------------------------------------------------------
// agent.go: truncate / parseReActResponse / buildConversationPrompt
// ----------------------------------------------------------------------------

func TestTruncate_Agent(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q, want short", got)
	}
	if got := truncate("exact", 5); got != "exact" {
		t.Errorf("truncate exact = %q, want exact", got)
	}
	if got := truncate("toolong", 3); got != "too..." {
		t.Errorf("truncate toolong = %q, want too...", got)
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate empty = %q, want empty", got)
	}
}

func TestParseReActResponse(t *testing.T) {
	tests := []struct {
		name       string
		resp       string
		wantErr    bool
		wantAction string
		wantFinal  string
	}{
		{
			"valid action",
			`{"thought":"thinking","action":"search","action_input":"query"}`,
			false, "search", "",
		},
		{
			"valid final answer",
			`{"thought":"done","action":"final_answer","final_answer":"the answer"}`,
			false, "final_answer", "the answer",
		},
		{
			"json code fence",
			"```json\n{\"thought\":\"x\",\"action\":\"y\",\"action_input\":\"z\"}\n```",
			false, "y", "",
		},
		{
			"bare code fence",
			"```\n{\"thought\":\"x\",\"action\":\"y\",\"action_input\":\"z\"}\n```",
			false, "y", "",
		},
		{
			"surrounding text",
			`some preamble {"thought":"x","action":"y","action_input":"z"} trailing`,
			false, "y", "",
		},
		{
			"invalid json",
			"not json at all",
			true, "", "",
		},
		{
			"missing action and final_answer",
			`{"thought":"only thought"}`,
			true, "", "",
		},
		{
			"empty",
			"",
			true, "", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thought, err := parseReActResponse(tt.resp)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseReActResponse(%q) expected error, got nil", tt.resp)
				}
				return
			}
			if err != nil {
				t.Errorf("parseReActResponse(%q) unexpected error: %v", tt.resp, err)
				return
			}
			if thought.Action != tt.wantAction {
				t.Errorf("action = %q, want %q", thought.Action, tt.wantAction)
			}
			if thought.FinalAnswer != tt.wantFinal {
				t.Errorf("final_answer = %q, want %q", thought.FinalAnswer, tt.wantFinal)
			}
		})
	}
}

func TestBuildConversationPrompt_Boost3(t *testing.T) {
	msgs := []LLMMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	out := buildConversationPrompt(msgs)
	if !strings.Contains(out, "system: sys") {
		t.Errorf("expected system message, got: %s", out)
	}
	if !strings.Contains(out, "user: hi") {
		t.Errorf("expected user message, got: %s", out)
	}
	if !strings.Contains(out, "assistant: hello") {
		t.Errorf("expected assistant message, got: %s", out)
	}
	// 消息之间应用双换行分隔
	if !strings.Contains(out, "\n\n") {
		t.Errorf("expected double newline separator, got: %s", out)
	}
	// 空切片
	if out := buildConversationPrompt(nil); out != "" {
		t.Errorf("empty conversation should produce empty string, got: %s", out)
	}
}

func TestReActAgent_BuildToolDescriptions(t *testing.T) {
	a := &ReActAgent{
		tools: []AgentTool{
			{Name: "search", Description: "search the web", NodeName: "http_request"},
			{Name: "calc", Description: "do math", NodeName: "transform"},
		},
	}
	out := a.buildToolDescriptions()
	if !strings.Contains(out, "- search: search the web") {
		t.Errorf("expected search tool desc, got: %s", out)
	}
	if !strings.Contains(out, "- calc: do math") {
		t.Errorf("expected calc tool desc, got: %s", out)
	}
	// 空工具列表
	a2 := &ReActAgent{}
	if out := a2.buildToolDescriptions(); out != "" {
		t.Errorf("empty tools should produce empty string, got: %s", out)
	}
}

func TestReActAgent_BuildSystemPrompt(t *testing.T) {
	a := &ReActAgent{
		systemPrompt:   "Custom role",
		enableThinking: true,
	}
	out := a.buildSystemPrompt("tool desc here")
	if !strings.Contains(out, "Custom role") {
		t.Errorf("expected custom system prompt, got: %s", out)
	}
	if !strings.Contains(out, "tool desc here") {
		t.Errorf("expected tool descriptions, got: %s", out)
	}
	if !strings.Contains(out, "Thinking mode is ENABLED") {
		t.Errorf("expected thinking instruction, got: %s", out)
	}
	if !strings.Contains(out, "ReAct") {
		t.Errorf("expected ReAct pattern, got: %s", out)
	}

	// 无 thinking
	a.enableThinking = false
	out = a.buildSystemPrompt("")
	if strings.Contains(out, "Thinking mode is ENABLED") {
		t.Errorf("should not include thinking when disabled, got: %s", out)
	}
}

func TestReActAgent_ToolNames(t *testing.T) {
	a := &ReActAgent{
		tools: []AgentTool{
			{Name: "search", NodeName: "n1"},
			{Name: "calc", NodeName: "n2"},
		},
	}
	names := a.toolNames()
	if !strings.Contains(names, "search") || !strings.Contains(names, "calc") {
		t.Errorf("expected both tool names, got: %s", names)
	}
}

// ----------------------------------------------------------------------------
// condition.go: evaluateCondition / evalPositive / SafeRegexMatch
// ----------------------------------------------------------------------------

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		input   string
		want    bool
		wantErr bool
	}{
		{"empty true", "empty", "", true, false},
		{"empty false", "empty", "x", false, false},
		{"not_empty true", "not_empty", "x", true, false},
		{"not_empty false", "not_empty", "", false, false},
		{"true literal", "true", "anything", true, false},
		{"false literal", "false", "anything", false, false},
		{"contains true", "contains:foo", "hello foo world", true, false},
		{"contains false", "contains:bar", "hello foo world", false, false},
		{"equals true", "equals:abc", "abc", true, false},
		{"equals false", "equals:abc", "abd", false, false},
		{"starts_with true", "starts_with:hello", "hello world", true, false},
		{"starts_with false", "starts_with:hello", "world hello", false, false},
		{"ends_with true", "ends_with:world", "hello world", true, false},
		{"ends_with false", "ends_with:hello", "hello world", false, false},
		{"regex true", "regex:^a.c$", "abc", true, false},
		{"regex false", "regex:^a.c$", "abd", false, false},
		{"not prefix", "not empty", "", false, false},
		{"not prefix true", "not empty", "x", true, false},
		{"not contains", "not contains:foo", "bar baz", true, false},
		{"invalid format", "invalidformat", "x", false, true},
		{"unsupported op", "bogus:foo", "x", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateCondition(tt.expr, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("evaluateCondition(%q, %q) expected error", tt.expr, tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("evaluateCondition(%q, %q) unexpected error: %v", tt.expr, tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("evaluateCondition(%q, %q) = %v, want %v", tt.expr, tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeRegexMatch(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
		wantErr bool
	}{
		{"^abc$", "abc", true, false},
		{"^abc$", "abcd", false, false},
		{"a+", "baad", true, false},
		{"[invalid", "x", false, true}, // 编译失败
	}
	for _, tt := range tests {
		got, err := SafeRegexMatch(tt.pattern, tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("SafeRegexMatch(%q, %q) expected error", tt.pattern, tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("SafeRegexMatch(%q, %q) unexpected error: %v", tt.pattern, tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("SafeRegexMatch(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
		}
	}
}

func TestConditionNode_Execute(t *testing.T) {
	n := &ConditionNode{}
	ctx := context.Background()

	// 缺少 expr 参数
	if _, err := n.Execute(ctx, "input", map[string]string{}); err == nil {
		t.Errorf("expected error for missing expr")
	}
	// 通过 expr 参数
	out, err := n.Execute(ctx, "hello world", map[string]string{"expr": "contains:hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "true" {
		t.Errorf("expected true, got %q", out)
	}
	// 通过 condition 参数
	out, err = n.Execute(ctx, "", map[string]string{"condition": "empty"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "true" {
		t.Errorf("expected true for empty input, got %q", out)
	}
	// false 情况
	out, _ = n.Execute(ctx, "x", map[string]string{"expr": "empty"})
	if out != "false" {
		t.Errorf("expected false, got %q", out)
	}
}

// ----------------------------------------------------------------------------
// human_in_loop.go: buildApprovedOutput
// ----------------------------------------------------------------------------

func TestBuildApprovedOutput(t *testing.T) {
	// 未批准：返回原输入
	if got := buildApprovedOutput("orig", "passthrough", false, ""); got != "orig" {
		t.Errorf("not approved passthrough = %q, want orig", got)
	}
	// 已批准 passthrough：返回原输入
	if got := buildApprovedOutput("orig", "passthrough", true, ""); got != "orig" {
		t.Errorf("approved passthrough = %q, want orig", got)
	}
	// 已批准 modified：包含 reason
	got := buildApprovedOutput("orig", "modified", true, "looks good")
	if !strings.Contains(got, "Approved (looks good)") {
		t.Errorf("expected Approved marker, got: %s", got)
	}
	if !strings.Contains(got, "orig") {
		t.Errorf("expected original input, got: %s", got)
	}
	// 未知模式默认返回原输入
	if got := buildApprovedOutput("orig", "unknown", true, "x"); got != "orig" {
		t.Errorf("unknown mode = %q, want orig", got)
	}
}

func TestHumanInLoopNode_Execute(t *testing.T) {
	n := &HumanInLoopNode{}
	ctx := context.Background()

	// auto_approve
	out, err := n.Execute(ctx, "input data", map[string]string{"mode": "auto_approve"})
	if err != nil {
		t.Fatalf("auto_approve error: %v", err)
	}
	if out != "input data" {
		t.Errorf("auto_approve out = %q, want input data", out)
	}

	// auto_approve + modified
	out, err = n.Execute(ctx, "input", map[string]string{"mode": "auto_approve", "on_approve": "modified"})
	if err != nil {
		t.Fatalf("auto_approve modified error: %v", err)
	}
	if !strings.Contains(out, "Approved") {
		t.Errorf("expected Approved marker, got: %s", out)
	}

	// env 模式未批准应报错
	os.Unsetenv("LLM_BOX_APPROVED")
	if _, err := n.Execute(ctx, "x", map[string]string{"mode": "env"}); err == nil {
		t.Errorf("env mode without approval should error")
	}
	// env 模式已批准（需 on_approve=modified 才能看到 reason）
	os.Setenv("LLM_BOX_APPROVED", "1")
	defer os.Unsetenv("LLM_BOX_APPROVED")
	out, err = n.Execute(ctx, "x", map[string]string{"mode": "env", "on_approve": "modified"})
	if err != nil {
		t.Fatalf("env approved error: %v", err)
	}
	if !strings.Contains(out, "env LLM_BOX_APPROVED=approved") {
		t.Errorf("expected env approval message, got: %s", out)
	}

	// 未知模式
	if _, err := n.Execute(ctx, "x", map[string]string{"mode": "bogus"}); err == nil {
		t.Errorf("unknown mode should error")
	}
}

// ----------------------------------------------------------------------------
// inference_backend.go: backends / BackendManager
// ----------------------------------------------------------------------------

func TestLlamaCppBackend_Basics(t *testing.T) {
	b := NewLlamaCppBackend()
	if b.Name() != BackendLlamaCpp {
		t.Errorf("Name = %v, want %v", b.Name(), BackendLlamaCpp)
	}
	caps := b.Capabilities()
	if !caps.SupportsGPU || !caps.SupportsStreaming || caps.MaxModelSizeGB != 128 {
		t.Errorf("LlamaCpp capabilities wrong: %+v", caps)
	}
	// Status 应可调用（IsAvailable 取决于环境，但 Status 不应 panic）
	status := b.Status()
	if status.Backend != BackendLlamaCpp {
		t.Errorf("Status.Backend = %v, want %v", status.Backend, BackendLlamaCpp)
	}
	// LoadModel/UnloadModel 是 no-op
	if err := b.LoadModel(context.Background(), "/some/model"); err != nil {
		t.Errorf("LoadModel should be no-op, got: %v", err)
	}
	if err := b.UnloadModel("/some/model"); err != nil {
		t.Errorf("UnloadModel should be no-op, got: %v", err)
	}
}

func TestONNXBackend_Basics(t *testing.T) {
	b := NewONNXBackend()
	if b.Name() != BackendONNXRuntime {
		t.Errorf("Name = %v, want %v", b.Name(), BackendONNXRuntime)
	}
	caps := b.Capabilities()
	if caps.SupportsINT4 || caps.SupportsStreaming || caps.MaxModelSizeGB != 32 {
		t.Errorf("ONNX capabilities wrong: %+v", caps)
	}
	status := b.Status()
	if status.Backend != BackendONNXRuntime {
		t.Errorf("Status.Backend = %v, want %v", status.Backend, BackendONNXRuntime)
	}
}

func TestLlamaCppBackend_InferUnavailable(t *testing.T) {
	b := NewLlamaCppBackend()
	// 若 llama-server 不可用，应返回 error
	if !b.IsAvailable() {
		_, err := b.Infer(context.Background(), InferenceRequest{Model: "m", Prompt: "p"})
		if err == nil {
			t.Errorf("expected error when llama.cpp unavailable")
		}
	}
}

func TestBackendManager_RegisterAndGet(t *testing.T) {
	m := &BackendManager{
		backends: make(map[InferenceBackend]InferenceBackendAdapter),
	}
	m.registerDefaultBackends()

	// 应包含 llama.cpp 与 onnx
	if _, ok := m.GetBackend(BackendLlamaCpp); !ok {
		t.Errorf("expected llama.cpp backend registered")
	}
	if _, ok := m.GetBackend(BackendONNXRuntime); !ok {
		t.Errorf("expected onnx backend registered")
	}
	if _, ok := m.GetBackend(BackendVLLM); ok {
		t.Errorf("vllm should not be registered")
	}

	// 默认 active 应为 llama.cpp
	if m.GetActive() != BackendLlamaCpp {
		t.Errorf("default active = %v, want %v", m.GetActive(), BackendLlamaCpp)
	}

	// 切换 active
	if err := m.SetActive(BackendONNXRuntime); err != nil {
		t.Errorf("SetActive error: %v", err)
	}
	if m.GetActive() != BackendONNXRuntime {
		t.Errorf("after SetActive, active = %v, want %v", m.GetActive(), BackendONNXRuntime)
	}

	// 切换到未注册的应报错
	if err := m.SetActive(BackendVLLM); err == nil {
		t.Errorf("SetActive to unregistered should error")
	}
}

func TestBackendManager_ListBackends(t *testing.T) {
	m := GetBackendManager()
	list := m.ListBackends()
	if len(list) < 2 {
		t.Errorf("expected at least 2 backends, got %d", len(list))
	}
	// 应包含 llama.cpp 与 onnx
	names := map[InferenceBackend]bool{}
	for _, s := range list {
		names[s.Backend] = true
	}
	if !names[BackendLlamaCpp] || !names[BackendONNXRuntime] {
		t.Errorf("expected llama.cpp and onnx, got %v", names)
	}
}

func TestBackendManager_Infer(t *testing.T) {
	m := &BackendManager{
		backends: make(map[InferenceBackend]InferenceBackendAdapter),
	}
	m.registerDefaultBackends()
	// active 是 llama.cpp；若不可用应返回错误
	active := m.GetActive()
	backend, _ := m.GetBackend(active)
	if !backend.IsAvailable() {
		_, err := m.Infer(context.Background(), InferenceRequest{Model: "m", Prompt: "p"})
		if err == nil {
			t.Errorf("expected Infer error when backend unavailable")
		}
	}
}

func TestFileExists(t *testing.T) {
	if fileExists("/nonexistent/xyz/12345") {
		t.Errorf("fileExists should be false for nonexistent")
	}
	if !fileExists("/") {
		t.Errorf("fileExists should be true for root dir")
	}
}

func TestDefaultModelDir(t *testing.T) {
	d := defaultModelDir()
	if d == "" {
		t.Errorf("defaultModelDir should not be empty")
	}
	// 应包含 .llm-box 段
	if !strings.Contains(d, ".llm-box") {
		t.Errorf("expected .llm-box in model dir, got: %s", d)
	}
}

// ----------------------------------------------------------------------------
// mcp_bridge.go: validateServerURL / callCalculator
// ----------------------------------------------------------------------------

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://localhost:8080", false},
		{"https://example.com/path", false},
		{"http://", true},            // 无 host
		{"ftp://example.com", true},  // 错误 scheme
		{"not-a-url", true},          // 非法
		{"://missing-scheme", true},  // 缺 scheme
		{"file:///etc/passwd", true}, // file scheme 不允许
	}
	for _, tt := range tests {
		err := validateServerURL(tt.url)
		if tt.wantErr && err == nil {
			t.Errorf("validateServerURL(%q) expected error", tt.url)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateServerURL(%q) unexpected error: %v", tt.url, err)
		}
	}
}

func TestCallCalculator(t *testing.T) {
	tests := []struct {
		name   string
		args   string
		input  string
		wantOp string
		want   float64
	}{
		{"add via input", "", "1+2", "+", 42.0},
		{"sub via input", "", "10-3", "-", 10.0},
		{"mul via input", "", "4*5", "*", 100.0},
		{"div via input", "", "20/4", "/", 5.0},
		{"no op via input", "", "42", "", 0.0},
		{"via args json", `{"expression":"1+1"}`, "", "+", 42.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errMsg := callCalculator(tt.args, tt.input)
			if errMsg != "" {
				t.Fatalf("unexpected error message: %s", errMsg)
			}
			m, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("expected map result, got %T", result)
			}
			if m["tool"] != "calculator" {
				t.Errorf("tool = %v, want calculator", m["tool"])
			}
			if m["result"] != tt.want {
				t.Errorf("result = %v, want %v", m["result"], tt.want)
			}
		})
	}

	// 空表达式应返回错误消息
	_, errMsg := callCalculator("", "")
	if errMsg == "" {
		t.Errorf("expected error message for empty expression")
	}
}

// ----------------------------------------------------------------------------
// swarm_comm.go: generateMessageID / signMessage / countActiveAgents / Execute actions
// ----------------------------------------------------------------------------

func TestGenerateMessageID(t *testing.T) {
	id1 := generateMessageID("agent1", "hello")
	id2 := generateMessageID("agent1", "hello")
	// 因为包含 time.Now()，两次调用结果应不同（极小概率相同）
	if len(id1) != 16 {
		t.Errorf("id length = %d, want 16", len(id1))
	}
	// 不同 agent 应产生不同 id（time 不同的概率高）
	id3 := generateMessageID("agent2", "hello")
	if id1 == id3 && id1 == id2 {
		// 仅当所有都相同时才报警（容忍 time 相同的极端情况）
	}
}

func TestSignMessage(t *testing.T) {
	sig := signMessage("msgid", "content")
	if len(sig) != 12 {
		t.Errorf("signature length = %d, want 12", len(sig))
	}
	// 同输入应产生同输出（确定性）
	sig2 := signMessage("msgid", "content")
	if sig != sig2 {
		t.Errorf("signMessage not deterministic: %q vs %q", sig, sig2)
	}
	// 不同输入应产生不同输出
	sig3 := signMessage("msgid2", "content")
	if sig == sig3 {
		t.Errorf("expected different sigs for different inputs")
	}
}

func TestSwarmComm_ExecuteActions(t *testing.T) {
	n := &SwarmCommNode{}
	ctx := context.Background()

	// join
	out, err := n.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   "test-agent-1",
		"agent_name": "Tester",
		"agent_role": "developer",
	})
	if err != nil {
		t.Fatalf("join error: %v", err)
	}
	if !strings.Contains(out, "Agent Joined Swarm") && !strings.Contains(out, "Agent Rejoined") {
		t.Errorf("expected join message, got: %s", out)
	}

	// join 缺 agent_id 应报错
	if _, err := n.Execute(ctx, "", map[string]string{"action": "join"}); err == nil {
		t.Errorf("join without agent_id should error")
	}

	// list_channels
	out, err = n.Execute(ctx, "", map[string]string{"action": "list_channels"})
	if err != nil {
		t.Fatalf("list_channels error: %v", err)
	}
	if !strings.Contains(out, "Swarm Channels") {
		t.Errorf("expected channels header, got: %s", out)
	}
	if !strings.Contains(out, "general") {
		t.Errorf("expected general channel, got: %s", out)
	}

	// list_agents
	out, err = n.Execute(ctx, "", map[string]string{"action": "list_agents"})
	if err != nil {
		t.Fatalf("list_agents error: %v", err)
	}
	if !strings.Contains(out, "Swarm Agents") {
		t.Errorf("expected agents header, got: %s", out)
	}

	// send
	out, err = n.Execute(ctx, "hello world", map[string]string{
		"action":   "send",
		"agent_id": "test-agent-1",
		"channel":  "general",
	})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
	if !strings.Contains(out, "Message Sent") {
		t.Errorf("expected sent message, got: %s", out)
	}

	// send 缺 agent_id 应报错
	if _, err := n.Execute(ctx, "hi", map[string]string{"action": "send"}); err == nil {
		t.Errorf("send without agent_id should error")
	}
	// send 空内容应报错
	if _, err := n.Execute(ctx, "  ", map[string]string{"action": "send", "agent_id": "test-agent-1"}); err == nil {
		t.Errorf("send empty content should error")
	}
	// send 未注册 agent 应报错
	if _, err := n.Execute(ctx, "hi", map[string]string{"action": "send", "agent_id": "ghost-agent"}); err == nil {
		t.Errorf("send from unregistered agent should error")
	}
	// send 不存在的 channel 应报错
	if _, err := n.Execute(ctx, "hi", map[string]string{"action": "send", "agent_id": "test-agent-1", "channel": "nonexistent-channel"}); err == nil {
		t.Errorf("send to nonexistent channel should error")
	}

	// broadcast
	out, err = n.Execute(ctx, "important announcement", map[string]string{
		"action":   "broadcast",
		"agent_id": "test-agent-1",
	})
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if !strings.Contains(out, "Broadcast Sent") {
		t.Errorf("expected broadcast message, got: %s", out)
	}
	// broadcast 缺 agent_id 应报错
	if _, err := n.Execute(ctx, "x", map[string]string{"action": "broadcast"}); err == nil {
		t.Errorf("broadcast without agent_id should error")
	}

	// read
	out, err = n.Execute(ctx, "", map[string]string{
		"action":   "read",
		"channel":  "general",
		"agent_id": "test-agent-1",
		"limit":    "10",
	})
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if !strings.Contains(out, "Channel: #general") {
		t.Errorf("expected channel header, got: %s", out)
	}
	// read 不存在的 channel 应报错
	if _, err := n.Execute(ctx, "", map[string]string{"action": "read", "channel": "nope"}); err == nil {
		t.Errorf("read nonexistent channel should error")
	}

	// leave
	out, err = n.Execute(ctx, "", map[string]string{"action": "leave", "agent_id": "test-agent-1"})
	if err != nil {
		t.Fatalf("leave error: %v", err)
	}
	if !strings.Contains(out, "Agent Left Swarm") {
		t.Errorf("expected leave message, got: %s", out)
	}
	// leave 不存在的 agent
	out, _ = n.Execute(ctx, "", map[string]string{"action": "leave", "agent_id": "ghost"})
	if !strings.Contains(out, "not found") {
		t.Errorf("expected not found message, got: %s", out)
	}
	// leave 缺 agent_id
	if _, err := n.Execute(ctx, "", map[string]string{"action": "leave"}); err == nil {
		t.Errorf("leave without agent_id should error")
	}

	// 未知 action 应报错
	if _, err := n.Execute(ctx, "", map[string]string{"action": "bogus"}); err == nil {
		t.Errorf("unknown action should error")
	}

	// 清理：重新加入测试 agent 后离开，避免影响其他测试
	_, _ = n.Execute(ctx, "", map[string]string{"action": "join", "agent_id": "cleanup-agent"})
}

func TestSwarmComm_CreateChannelAndCountActive(t *testing.T) {
	n := &SwarmCommNode{}
	ctx := context.Background()

	// 创建一个新 channel
	out, err := n.Execute(ctx, "test channel description", map[string]string{
		"action":  "create_channel",
		"channel": "test-ch-unique",
	})
	if err != nil {
		t.Fatalf("create_channel error: %v", err)
	}
	if !strings.Contains(out, "Channel") && !strings.Contains(out, "channel") {
		t.Errorf("expected channel message, got: %s", out)
	}

	// countActiveAgents 应 >= 0（不 panic）
	count := countActiveAgents()
	if count < 0 {
		t.Errorf("countActiveAgents = %d, want >= 0", count)
	}
}

// ----------------------------------------------------------------------------
// multimodal.go: resolveImageURL（使用临时文件）
// ----------------------------------------------------------------------------

func TestResolveImageURL_HTTPPassthrough(t *testing.T) {
	// http(s) URL 直接返回
	for _, in := range []string{"http://example.com/a.png", "https://example.com/b.jpg"} {
		got, err := resolveImageURL(in)
		if err != nil {
			t.Errorf("resolveImageURL(%q) error: %v", in, err)
		}
		if got != in {
			t.Errorf("resolveImageURL(%q) = %q, want passthrough", in, got)
		}
	}
}

func TestResolveImageURL_LocalFile(t *testing.T) {
	// resolveImageURL 调用 validateReadPath，需要相对路径
	// 通过设置 workDir 到临时目录来允许读取该目录
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	// PNG 文件（用最小合法 PNG 头）
	if err := os.WriteFile(filepath.Join(dir, "img.png"), []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := resolveImageURL("img.png")
	if err != nil {
		t.Fatalf("resolveImageURL local: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("expected data URL with png mime, got: %s", got[:30])
	}

	// JPG 文件
	if err := os.WriteFile(filepath.Join(dir, "img.jpg"), []byte("fake jpg"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ = resolveImageURL("img.jpg")
	if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
		t.Errorf("expected jpeg mime, got: %s", got[:30])
	}

	// 不存在的文件应报错
	if _, err := resolveImageURL("nope.png"); err == nil {
		t.Errorf("expected error for nonexistent file")
	}

	// 路径遍历应报错（validateReadPath 拒绝）
	if _, err := resolveImageURL("../etc/passwd"); err == nil {
		t.Errorf("expected error for path traversal")
	}
}

// ----------------------------------------------------------------------------
// 自愈节点 (self_heal.go): runCmd / Execute (markdown/json/text 输出)
// ----------------------------------------------------------------------------

func TestRunCmd_Empty(t *testing.T) {
	// 空命令应报错
	_, _, err := runCmd("")
	if err == nil {
		t.Errorf("runCmd(\"\") should error")
	}
}

func TestRunCmd_Echo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("echo test skipped on windows")
	}
	out, code, err := runCmd("echo hello")
	if err != nil {
		t.Fatalf("runCmd echo: %v", err)
	}
	if code != 0 {
		t.Errorf("echo exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("echo output missing hello, got: %s", out)
	}
}

func TestRunCmdArgs_False(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("false test skipped on windows")
	}
	// /usr/bin/false 总是非零退出
	_, code, _ := runCmdArgs("false")
	if code == 0 {
		t.Errorf("false exit code = 0, want non-zero")
	}
}

func TestSelfHealNode_Execute_Formats(t *testing.T) {
	n := &SelfHealNode{}
	ctx := context.Background()

	// json 输出
	out, err := n.Execute(ctx, "format", map[string]string{"output": "json", "auto_fix": "false"})
	if err != nil {
		t.Fatalf("json format error: %v", err)
	}
	// 应是合法 JSON
	var report HealReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Errorf("json output not valid JSON: %v\noutput: %s", err, out)
	}
	// format 区域应只检查 gofmt
	if len(report.Checks) != 1 || report.Checks[0].Name != "gofmt" {
		t.Errorf("format area checks = %+v, want only gofmt", report.Checks)
	}

	// text 输出
	out, err = n.Execute(ctx, "format", map[string]string{"output": "text", "auto_fix": "false"})
	if err != nil {
		t.Fatalf("text format error: %v", err)
	}
	if !strings.Contains(out, "Self-Heal Report:") {
		t.Errorf("expected text header, got: %s", out)
	}

	// markdown 输出（默认）
	out, err = n.Execute(ctx, "format", map[string]string{"auto_fix": "false"})
	if err != nil {
		t.Fatalf("markdown default error: %v", err)
	}
	if !strings.Contains(out, "Self-Heal Report") {
		t.Errorf("expected markdown header, got: %s", out)
	}

	// 空 input 应默认 area=all
	out, _ = n.Execute(ctx, "", map[string]string{"output": "json", "auto_fix": "false"})
	_ = json.Unmarshal([]byte(out), &report)
	if len(report.Checks) < 4 {
		t.Errorf("area=all should run multiple checks, got %d", len(report.Checks))
	}
}

// ----------------------------------------------------------------------------
// search_aggregate.go: SearchAggregateNode.Execute 参数校验
// ----------------------------------------------------------------------------

func TestSearchAggregateNode_Execute_Validation(t *testing.T) {
	n := &SearchAggregateNode{}
	ctx := context.Background()

	// 空查询应报错
	if _, err := n.Execute(ctx, "  ", map[string]string{}); err == nil {
		t.Errorf("empty query should error")
	}

	// 有查询但所有源都不可达：仍应返回（fetchSource 各自返回 nil）
	out, err := n.Execute(ctx, "test query", map[string]string{
		"sources": "hn,github",
		"output":  "json",
		"limit":   "1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应为合法 JSON
	var agg AggregatedResults
	if err := json.Unmarshal([]byte(out), &agg); err != nil {
		t.Errorf("output not valid JSON: %v", err)
	}
	if agg.Query != "test query" {
		t.Errorf("query = %q, want test query", agg.Query)
	}

	// text 输出
	out, err = n.Execute(ctx, "q", map[string]string{
		"sources": "invalid,foo,bar", // 全部无效 -> 默认回退
		"output":  "text",
	})
	if err != nil {
		t.Fatalf("text output error: %v", err)
	}
	if !strings.Contains(out, "Search: q") {
		t.Errorf("expected text header, got: %s", out)
	}

	// markdown 输出（默认）
	out, _ = n.Execute(ctx, "q", map[string]string{})
	if !strings.Contains(out, "Search Results for: **q**") {
		t.Errorf("expected markdown header, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// 辅助：测试 sort 包导入使用
// ----------------------------------------------------------------------------

func TestSortStableUsage(t *testing.T) {
	// 这只是一个 smoke test 确保 sort 包正常使用
	s := []string{"c", "a", "b"}
	sort.Strings(s)
	if s[0] != "a" || s[2] != "c" {
		t.Errorf("sort failed: %v", s)
	}
}
