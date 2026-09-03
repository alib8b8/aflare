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

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/history"
)

// writeAuditLines appends JSONL-encoded audit entries to path.
func writeAuditLines(t *testing.T, path string, entries ...history.AuditLog) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
}

func auditTailTestEntry(id string, ts time.Time) history.AuditLog {
	return history.AuditLog{
		ID:        id,
		Timestamp: ts,
		Action:    history.AuditActionWorkflowStart,
		Resource:  "wf-" + id,
		Success:   true,
	}
}

func TestAuditTailOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log.jsonl")
	base := time.Date(2026, 9, 3, 4, 0, 0, 0, time.UTC)
	var entries []history.AuditLog
	for i := 0; i < 5; i++ {
		entries = append(entries, auditTailTestEntry(string(rune('a'+i)), base.Add(time.Duration(i)*time.Minute)))
	}
	writeAuditLines(t, path, entries...)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	st, _ := f.Stat()
	size := st.Size()

	cases := []struct {
		n         int
		wantLines int // number of lines reachable from the returned offset
	}{
		{0, 0},  // follow-only: seek to EOF
		{1, 1},  // last entry only
		{3, 3},  // last three
		{5, 5},  // exactly the whole file
		{99, 5}, // more than the file has → whole file
	}
	for _, tc := range cases {
		offset, err := auditTailOffset(f, tc.n)
		if err != nil {
			t.Fatalf("auditTailOffset(%d): %v", tc.n, err)
		}
		buf := make([]byte, size-offset)
		if len(buf) > 0 {
			if _, err := f.ReadAt(buf, offset); err != nil {
				t.Fatalf("ReadAt: %v", err)
			}
		}
		got := len(bytes.Split(bytes.TrimSuffix(buf, []byte("\n")), []byte("\n")))
		if len(buf) == 0 {
			got = 0
		}
		if got != tc.wantLines {
			t.Errorf("auditTailOffset(n=%d) exposes %d lines, want %d", tc.n, got, tc.wantLines)
		}
	}
}

func TestEmitAuditLines_JSONPassthroughAndPartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log.jsonl")
	ts := time.Date(2026, 9, 3, 4, 0, 0, 0, time.UTC)
	e1 := auditTailTestEntry("one", ts)
	writeAuditLines(t, path, e1)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	// Append a torn (newline-less) partial line: must NOT be emitted, and
	// the offset must not consume it.
	partial := []byte(`{"id":"torn","timestamp":"2026-09-03T04:00:00Z"`)
	if err := os.WriteFile(path, append(mustReadFile(t, path), partial...), 0o644); err != nil {
		t.Fatalf("append partial: %v", err)
	}

	var jsonOut bytes.Buffer
	var offset int64
	n, err := emitAuditLines(f, &offset, &jsonOut, true)
	if err != nil {
		t.Fatalf("emitAuditLines: %v", err)
	}
	if n != 1 {
		t.Fatalf("emitted %d lines, want 1 (torn line withheld)", n)
	}
	want, _ := json.Marshal(e1)
	if got := strings.TrimSpace(jsonOut.String()); got != string(want) {
		t.Errorf("json passthrough =\n%s\nwant\n%s", got, want)
	}

	// The torn line is completed now — the next emit must produce it.
	if err := os.WriteFile(path, append(mustReadFile(t, path), '}', '\n'), 0o644); err != nil {
		t.Fatalf("complete torn line: %v", err)
	}
	n, err = emitAuditLines(f, &offset, &jsonOut, true)
	if err != nil {
		t.Fatalf("emitAuditLines (2): %v", err)
	}
	if n != 1 {
		t.Fatalf("emitted %d lines after completion, want 1", n)
	}
	if !strings.Contains(jsonOut.String(), `"torn"`) {
		t.Error("completed line was not emitted")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestEmitAuditLines_HumanFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log.jsonl")
	ts := time.Date(2026, 9, 3, 4, 5, 6, 0, time.UTC)
	writeAuditLines(t, path, history.AuditLog{
		ID:        "e1",
		Timestamp: ts,
		Action:    history.AuditActionWorkflowEnd,
		User:      "ops",
		Resource:  "billing-run",
		Success:   false,
		Detail:    "step 3 failed",
	})

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var out bytes.Buffer
	var offset int64
	if _, err := emitAuditLines(f, &offset, &out, false); err != nil {
		t.Fatalf("emitAuditLines: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"2026-09-03T04:05:06Z",
		"workflow_end",
		"FAIL",
		"user=ops",
		"resource=billing-run",
		"id=e1",
		"step 3 failed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human line missing %q:\n%s", want, got)
		}
	}
}

func TestParseAuditTailArgs(t *testing.T) {
	opts, err := parseAuditTailArgs([]string{})
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if opts.lines != 10 || opts.jsonOut || opts.auditPath != "" {
		t.Errorf("defaults = %+v, want lines=10 json=false path=''", opts)
	}

	opts, err = parseAuditTailArgs([]string{"-n", "3", "--json", "--file", "/tmp/a.jsonl"})
	if err != nil {
		t.Fatalf("flags: %v", err)
	}
	if opts.lines != 3 || !opts.jsonOut || opts.auditPath != "/tmp/a.jsonl" {
		t.Errorf("flags = %+v", opts)
	}

	opts, err = parseAuditTailArgs([]string{"--lines=0", "--file=/tmp/b.jsonl"})
	if err != nil {
		t.Fatalf("equals-form: %v", err)
	}
	if opts.lines != 0 || opts.auditPath != "/tmp/b.jsonl" {
		t.Errorf("equals-form = %+v", opts)
	}

	if _, err := parseAuditTailArgs([]string{"-n", "-1"}); err == nil {
		t.Error("negative -n must be rejected")
	}
	if _, err := parseAuditTailArgs([]string{"--wat"}); err == nil {
		t.Error("unknown flag must be rejected")
	}
}

// syncBuffer is a mutex-guarded buffer: the tail goroutine writes while the
// test polls the content.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestRunAuditTail_FollowsNewEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log.jsonl")
	ts := time.Date(2026, 9, 3, 4, 0, 0, 0, time.UTC)
	writeAuditLines(t, path, auditTailTestEntry("one", ts), auditTailTestEntry("two", ts))

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- runAuditTail(ctx, auditTailOptions{auditPath: path, lines: 10, jsonOut: true}, out)
	}()

	// Wait for the initial snapshot (both existing entries).
	waitFor(t, func() bool {
		return strings.Contains(out.String(), `"one"`) && strings.Contains(out.String(), `"two"`)
	}, "initial snapshot")

	// Append a third entry — the follower must stream it.
	writeAuditLines(t, path, auditTailTestEntry("three", ts))
	waitFor(t, func() bool {
		return strings.Contains(out.String(), `"three"`)
	}, "followed entry")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAuditTail: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runAuditTail did not return after context cancel")
	}
}

func TestRunAuditTail_MissingFile(t *testing.T) {
	err := runAuditTail(context.Background(), auditTailOptions{auditPath: "/nonexistent/audit.log.jsonl"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing audit log file")
	}
}

// waitFor polls cond until it holds or the 3s deadline passes.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; output so far:\n%s", what, "<see buffer>")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
