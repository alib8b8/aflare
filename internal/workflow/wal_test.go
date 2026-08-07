// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package workflow

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// collectReplay reads all complete records from a WAL file via ReplayWAL.
func collectReplay(t *testing.T, path string) []WALRecord {
	t.Helper()
	var got []WALRecord
	if err := ReplayWAL(path, func(r WALRecord) error {
		got = append(got, r)
		return nil
	}); err != nil {
		t.Fatalf("ReplayWAL failed: %v", err)
	}
	return got
}

func TestWAL_AppendAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "append.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer wal.Close()

	records := []WALRecord{
		{StepIndex: 0, StepName: "s0", NodeName: "llm", Data: "d0",
			StepOutputs: map[int]string{0: "o0"}, Variables: map[string]string{"v0": "x"}},
		{StepIndex: 1, StepName: "s1", NodeName: "http", Data: "d1",
			StepOutputs: map[int]string{0: "o0", 1: "o1"}, Variables: map[string]string{"v0": "x", "v1": "y"}},
		{StepIndex: 2, StepName: "s2", NodeName: "map", Data: "d2",
			StepOutputs: map[int]string{2: "o2"}, Variables: map[string]string{"v2": "z"}},
	}
	for _, r := range records {
		if err := wal.Append(r); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	got := collectReplay(t, path)
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	for i, r := range got {
		if r.Seq != int64(i+1) {
			t.Errorf("record %d: expected seq %d, got %d", i, i+1, r.Seq)
		}
		if r.StepIndex != records[i].StepIndex {
			t.Errorf("record %d: expected step_index %d, got %d", i, records[i].StepIndex, r.StepIndex)
		}
		if r.StepName != records[i].StepName {
			t.Errorf("record %d: expected step_name %q, got %q", i, records[i].StepName, r.StepName)
		}
		if r.NodeName != records[i].NodeName {
			t.Errorf("record %d: expected node_name %q, got %q", i, records[i].NodeName, r.NodeName)
		}
		if r.Data != records[i].Data {
			t.Errorf("record %d: expected data %q, got %q", i, records[i].Data, r.Data)
		}
		if r.StepOutputs[records[i].StepIndex] != records[i].StepOutputs[records[i].StepIndex] {
			t.Errorf("record %d: step_outputs mismatch", i)
		}
		if r.Timestamp.IsZero() {
			t.Errorf("record %d: expected non-zero timestamp", i)
		}
	}
}

func TestWAL_CrashRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}

	// Append two records, then flush to push them into the OS page cache.
	// A process crash preserves page-cached data even without Close.
	if err := wal.Append(WALRecord{StepIndex: 0, Data: "first"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Append(WALRecord{StepIndex: 1, Data: "second"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Simulate crash: do NOT call Close/Sync. Drop the handle and recover
	// using ReplayWAL on the raw file.
	got := collectReplay(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 recovered records, got %d", len(got))
	}
	if got[0].Data != "first" || got[0].StepIndex != 0 {
		t.Errorf("record 0: %+v", got[0])
	}
	if got[1].Data != "second" || got[1].StepIndex != 1 {
		t.Errorf("record 1: %+v", got[1])
	}
}

func TestWAL_TornWriteRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	if err := wal.Append(WALRecord{StepIndex: 0, Data: "good0"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Append(WALRecord{StepIndex: 1, Data: "good1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate a torn tail write: a length prefix claiming a large record
	// followed by only a few bytes of data (no complete record, no CRC).
	garbage := make([]byte, 0, 4+5)
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], 100) // claims 100-byte payload
	garbage = append(garbage, lenBuf[:]...)
	garbage = append(garbage, []byte("parti")...) // only 5 bytes, then crash
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("open for garbage: %v", err)
	}
	if _, err := f.Write(garbage); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close garbage: %v", err)
	}

	got := collectReplay(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 complete records (torn tail truncated), got %d", len(got))
	}
	if got[0].Data != "good0" || got[1].Data != "good1" {
		t.Errorf("unexpected records: %q %q", got[0].Data, got[1].Data)
	}
}

func TestWAL_Compaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer wal.Close()

	for i := 0; i < 5; i++ {
		if err := wal.Append(WALRecord{
			StepIndex:   i,
			StepName:    "s",
			NodeName:    "n",
			Data:        "d",
			StepOutputs: map[int]string{i: "o"},
			Variables:   map[string]string{"k": "v"},
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	if err := wal.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	got := collectReplay(t, path)
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot record after compaction, got %d", len(got))
	}
	if got[0].StepIndex != 4 {
		t.Errorf("expected snapshot of latest record (step_index 4), got %d", got[0].StepIndex)
	}
	if got[0].Seq != 5 {
		t.Errorf("expected seq preserved as 5, got %d", got[0].Seq)
	}

	// The WAL must remain usable for further appends after compaction.
	if err := wal.Append(WALRecord{StepIndex: 5, Data: "after"}); err != nil {
		t.Fatalf("Append after compact: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got = collectReplay(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 records after post-compact append, got %d", len(got))
	}
	if got[1].Seq != 6 {
		t.Errorf("expected seq 6 after compact, got %d", got[1].Seq)
	}
	if got[1].Data != "after" {
		t.Errorf("expected data 'after', got %q", got[1].Data)
	}
}

func TestWAL_CRCMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crc.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	if err := wal.Append(WALRecord{StepIndex: 0, Data: "keep"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Append(WALRecord{StepIndex: 1, Data: "corrupt"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Corrupt the CRC of the last record (final 4 bytes of the file).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(raw) < 4 {
		t.Fatalf("file too small: %d bytes", len(raw))
	}
	// Flip the last byte, which is part of the second record's CRC32.
	raw[len(raw)-1] ^= 0xFF
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := collectReplay(t, path)
	if len(got) != 1 {
		t.Fatalf("expected 1 good record (replay stops at CRC mismatch), got %d", len(got))
	}
	if got[0].Data != "keep" {
		t.Errorf("expected to keep first record 'keep', got %q", got[0].Data)
	}
}

func TestWAL_Empty(t *testing.T) {
	dir := t.TempDir()

	// A freshly created (empty) WAL file replays without error and yields
	// no records.
	path := filepath.Join(dir, "empty.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := collectReplay(t, path)
	if len(got) != 0 {
		t.Fatalf("expected 0 records from empty WAL, got %d", len(got))
	}

	// A non-existent WAL file also replays without error.
	missing := filepath.Join(dir, "does-not-exist.wal")
	got = collectReplay(t, missing)
	if len(got) != 0 {
		t.Fatalf("expected 0 records from missing WAL, got %d", len(got))
	}

	// LoadStateWAL on an empty log returns nil state with no error.
	state, err := LoadStateWAL(missing)
	if err != nil {
		t.Fatalf("LoadStateWAL missing: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state for empty WAL, got %+v", state)
	}
}

func TestWAL_SequenceRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seq.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := wal.Append(WALRecord{StepIndex: i, Data: "d"}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen: recovered seq should be 3 (last assigned), so the next append
	// continues at seq 4.
	wal2, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL reopen: %v", err)
	}
	defer wal2.Close()
	if wal2.seq != 3 {
		t.Errorf("expected recovered seq 3, got %d", wal2.seq)
	}
	if err := wal2.Append(WALRecord{StepIndex: 3, Data: "next"}); err != nil {
		t.Fatalf("Append after reopen: %v", err)
	}
	if wal2.seq != 4 {
		t.Errorf("expected seq 4 after append, got %d", wal2.seq)
	}
	if err := wal2.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	got := collectReplay(t, path)
	if len(got) != 4 {
		t.Fatalf("expected 4 records after reopen+append, got %d", len(got))
	}
	if got[3].Seq != 4 {
		t.Errorf("expected last record seq 4, got %d", got[3].Seq)
	}
	if got[3].Data != "next" {
		t.Errorf("expected last record data 'next', got %q", got[3].Data)
	}
}

// TestWAL_MaybeCompact verifies that MaybeCompact triggers compaction once the
// log exceeds the configured threshold and is a no-op otherwise.
func TestWAL_MaybeCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "maybe.wal")
	wal, err := NewWAL(path, WALOptions{CompactionThreshold: 1}) // tiny threshold
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer wal.Close()
	for i := 0; i < 3; i++ {
		if err := wal.Append(WALRecord{StepIndex: i, Data: "d"}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	// First MaybeCompact after threshold exceeded collapses the log.
	if err := wal.MaybeCompact(); err != nil {
		t.Fatalf("MaybeCompact (compact): %v", err)
	}
	got := collectReplay(t, path)
	if len(got) != 1 {
		t.Fatalf("expected 1 record after compaction, got %d", len(got))
	}
	if got[0].StepIndex != 2 {
		t.Errorf("expected latest step_index 2, got %d", got[0].StepIndex)
	}
	// Second call is a no-op (log smaller than threshold again).
	if err := wal.MaybeCompact(); err != nil {
		t.Fatalf("MaybeCompact (noop): %v", err)
	}
	got = collectReplay(t, path)
	if len(got) != 1 {
		t.Fatalf("expected still 1 record after noop compaction, got %d", len(got))
	}
}

// TestWAL_LoadStateWAL exercises the LoadStateWAL helper end-to-end: it must
// return the latest cumulative state from the log.
func TestWAL_LoadStateWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "load.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	if err := wal.Append(WALRecord{
		StepIndex:   0,
		Data:        "old",
		StepOutputs: map[int]string{0: "o0"},
		Variables:   map[string]string{"a": "1"},
		Timestamp:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Append(WALRecord{
		StepIndex:   1,
		Data:        "new",
		StepOutputs: map[int]string{0: "o0", 1: "o1"},
		Variables:   map[string]string{"a": "2", "b": "3"},
		Timestamp:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	state, err := LoadStateWAL(path)
	if err != nil {
		t.Fatalf("LoadStateWAL: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.StepIndex != 1 {
		t.Errorf("expected step_index 1, got %d", state.StepIndex)
	}
	if state.Data != "new" {
		t.Errorf("expected data 'new', got %q", state.Data)
	}
	if state.StepOutputs[1] != "o1" {
		t.Errorf("expected step output 1 'o1', got %q", state.StepOutputs[1])
	}
	if state.Variables["a"] != "2" || state.Variables["b"] != "3" {
		t.Errorf("expected variables a=2 b=3, got %+v", state.Variables)
	}
}
