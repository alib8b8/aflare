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

// This file implements an append-only Write-Ahead Log (WAL) for workflow
// checkpoint state. It replaces the non-atomic os.WriteFile checkpoint path
// with a durable, replayable log that survives process crashes.
//
// Design highlights:
//   - Each completed step appends a single record (no full-state rewrite).
//   - Records are length-prefixed and CRC32-protected so a reader can detect
//     torn tail writes (from a crash mid-append) and stop at the last complete
//     record during recovery.
//   - Periodic compaction collapses the log into a single snapshot record
//     using a tmp-file + atomic rename to bound replay time.
//   - Crash recovery is performed by ReplayWAL, which yields every complete
//     record in order; the last one holds the latest restorable state.
//
// Delivery semantics: the WAL provides at-least-once recovery of workflow
// state, not exactly-once side effects — replay may re-fire a side effect
// emitted just before a crash. Exactly-once requires combining the WAL with
// an IdempotencyKey; see idempotency.go in this package.

package workflow

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
)

// WALRecord is a single append-only entry in the write-ahead log.
//
// Records come in two flavours:
//   - Full snapshot (IsDelta=false): StepOutputs/Variables hold the complete
//     state after the step. This is the legacy format and what compaction
//     writes.
//   - Delta (IsDelta=true): StepOutputs/Variables hold only the entries added
//     or changed relative to the previous record. WALStateManager writes
//     these to keep an N-step workflow's log volume at O(N) instead of the
//     O(N²) of one cumulative snapshot per step. Recovery folds deltas back
//     into the latest full snapshot (see LoadStateWAL / Compact).
type WALRecord struct {
	Seq         int64             `json:"seq"`          // monotonically increasing sequence number
	StepIndex   int               `json:"step_index"`   // 0-based step index
	StepName    string            `json:"step_name"`    // step name (may be empty)
	NodeName    string            `json:"node_name"`    // node type
	Data        string            `json:"data"`         // flowing data after this step
	StepOutputs map[int]string    `json:"step_outputs"` // all (snapshot) or changed (delta) step outputs
	Variables   map[string]string `json:"variables"`    // all (snapshot) or changed (delta) variables
	Timestamp   time.Time         `json:"timestamp"`
	IsDelta     bool              `json:"is_delta,omitempty"` // true when outputs/variables are a delta
}

// walFrameLen converts a marshaled record length into the uint32 frame
// length prefix, rejecting sizes that would silently truncate (and so
// corrupt the frame) — gosec G115.
func walFrameLen(dataLen int) (uint32, error) {
	if uint64(dataLen) > math.MaxUint32 {
		return 0, fmt.Errorf("wal: record too large (%d bytes, max %d)", dataLen, math.MaxUint32)
	}
	return uint32(dataLen), nil
}

// WAL is an append-only write-ahead log for workflow checkpoint state.
// It writes records sequentially to a log file and supports crash recovery
// via replay. Periodic compaction collapses the log into a single snapshot
// record to bound replay time.
//
// On-disk format (binary, little-endian):
//
//	[4 bytes: record length (uint32)]
//	[N bytes: JSON-encoded WALRecord]
//	[4 bytes: CRC32 checksum of the N bytes]
//
// The length prefix + CRC32 allow a reader to detect torn writes (partial
// records at the tail) and stop replay at the last complete record.
type WAL struct {
	mu                   sync.Mutex
	path                 string        // path to the .wal file
	file                 *os.File      // underlying file handle
	writer               *bufio.Writer // buffered writer for appends
	seq                  int64         // last assigned sequence number
	bytesSinceCompaction int64         // bytes appended since the last compaction
	stopSyncer           chan struct{} // closed to stop the periodic syncer (nil when disabled)
	syncerOnce           sync.Once
	opts                 WALOptions
}

// WALOptions configures WAL behaviour, including its durability level.
//
// Durability levels (like SQLite's synchronous modes):
//   - SyncEveryWrite=true: fsync after every append. Most durable, slowest.
//   - SyncEveryWrite=false && SyncInterval>0: a background goroutine fsyncs
//     at the given interval. Bounds the loss window to ~SyncInterval while
//     keeping appends off the fsync critical path.
//   - Neither set: relies on the OS page cache + fsync on Close. Fastest;
//     a crash may lose everything buffered since the last flush.
type WALOptions struct {
	// CompactionThreshold is the log size in bytes that triggers compaction.
	// 0 means use default (1 MB).
	CompactionThreshold int64
	// SyncEveryWrite calls file.Sync() after every append (durable but slow).
	// When false, relies on the OS page cache + SyncOnClose.
	SyncEveryWrite bool
	// SyncInterval starts a background fsync loop with the given period
	// (ignored when SyncEveryWrite is set). Values <= 0 disable the loop.
	SyncInterval time.Duration
}

const (
	defaultWALCompactionThreshold = 1 << 20 // 1 MB
	walCRCLen                     = 4
	walLenFieldLen                = 4
	walMaxRecordSize              = 64 * 1024 * 1024 // sanity cap: 64MB per record
)

// NewWAL opens (or creates) a WAL at the given path. If the file already
// exists, it is opened for append; existing records can be recovered via
// Replay(). The parent directory is created with 0700 permissions.
func NewWAL(path string, opts WALOptions) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("wal: create dir: %w", err)
	}
	if opts.CompactionThreshold <= 0 {
		opts.CompactionThreshold = defaultWALCompactionThreshold
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("wal: open: %w", err)
	}
	w := &WAL{
		path:   path,
		file:   f,
		writer: bufio.NewWriterSize(f, 64*1024),
		opts:   opts,
	}
	// Periodic syncer for the interval durability level.
	if opts.SyncInterval > 0 && !opts.SyncEveryWrite {
		w.stopSyncer = make(chan struct{})
		go w.syncLoop(opts.SyncInterval)
	}
	// Recover the high-water seq and bytes-since-compaction from existing
	// records (if any).
	if err := w.recoverSeq(); err != nil {
		if cerr := w.Close(); cerr != nil {
			logger.Error("wal: close during failed init", "path", w.path, "error", cerr)
		}
		return nil, err
	}
	return w, nil
}

// syncLoop periodically fsyncs the log. It exits when stopSyncer is closed
// (from Close). A Sync racing with a concurrent Close may observe a closed
// file; that is logged and the loop exits on the next tick.
func (w *WAL) syncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := w.Sync(); err != nil {
				logger.Warn("wal: periodic sync failed", "path", w.path, "error", err)
			}
		case <-w.stopSyncer:
			return
		}
	}
}

// stopSyncLoop terminates the periodic syncer (no-op when disabled). It is
// safe to call more than once.
func (w *WAL) stopSyncLoop() {
	if w.stopSyncer == nil {
		return
	}
	w.syncerOnce.Do(func() { close(w.stopSyncer) })
}

// Append writes a single record to the WAL. The record's Seq is assigned
// automatically (incremented from the last known seq). The write is buffered
// and flushed to the OS on the next Flush/Sync or when the buffer fills.
func (w *WAL) Append(rec WALRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	start := time.Now()

	w.seq++
	rec.Seq = w.seq
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(rec)
	if err != nil {
		w.seq-- // roll back seq on marshal failure
		return fmt.Errorf("wal: marshal: %w", err)
	}
	frameLen, err := walFrameLen(len(data))
	if err != nil {
		w.seq-- // roll back seq: the record never reached the log
		return err
	}

	// Write: [length][json][crc32]
	var lenBuf [walLenFieldLen]byte
	binary.LittleEndian.PutUint32(lenBuf[:], frameLen)
	if _, err := w.writer.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("wal: write len: %w", err)
	}
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("wal: write data: %w", err)
	}
	var crcBuf [walCRCLen]byte
	binary.LittleEndian.PutUint32(crcBuf[:], crc32.ChecksumIEEE(data))
	if _, err := w.writer.Write(crcBuf[:]); err != nil {
		return fmt.Errorf("wal: write crc: %w", err)
	}

	w.bytesSinceCompaction += int64(walLenFieldLen + len(data) + walCRCLen)

	if w.opts.SyncEveryWrite {
		if err := w.writer.Flush(); err != nil {
			return fmt.Errorf("wal: flush: %w", err)
		}
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("wal: sync: %w", err)
		}
		metrics.IncWALSync()
	}

	metrics.RecordWALAppend(rec.IsDelta, time.Since(start))
	return nil
}

// Flush flushes the buffered writer to the underlying file.
func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("wal: flush: %w", err)
	}
	return nil
}

// Sync flushes the buffer and fsyncs the file to durable storage. Both
// operations are performed under the same lock so no appends can interleave
// between the flush and the fsync.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("wal: flush: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}
	metrics.IncWALSync()
	return nil
}

// Close stops the periodic syncer (if any), flushes, syncs, and closes the WAL.
func (w *WAL) Close() error {
	// Stop the syncer BEFORE taking the lock: syncLoop's Sync() needs the
	// same lock, so stopping under it would deadlock.
	w.stopSyncLoop()
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.writer.Flush(); err != nil {
		if cerr := w.file.Close(); cerr != nil {
			logger.Error("wal: close after flush failure", "path", w.path, "error", cerr)
		}
		return fmt.Errorf("wal: flush on close: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		if cerr := w.file.Close(); cerr != nil {
			logger.Error("wal: close after sync failure", "path", w.path, "error", cerr)
		}
		return fmt.Errorf("wal: sync on close: %w", err)
	}
	return w.file.Close()
}

// Replay reads the WAL and yields each complete record in order. Torn or
// corrupted records at the tail (from a crash mid-write) are silently
// truncated — replay stops at the last complete record.
func (w *WAL) Replay(fn func(WALRecord) error) error {
	return ReplayWAL(w.path, fn)
}

// ReplayWAL reads a WAL file and yields complete records in order.
// Used for crash recovery without opening the WAL for writes.
func ReplayWAL(path string, fn func(WALRecord) error) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // no log = nothing to replay
		}
		return fmt.Errorf("wal: open for replay: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		// Read length prefix.
		var lenBuf [walLenFieldLen]byte
		if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil // clean end
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return nil // torn tail, stop
			}
			return fmt.Errorf("wal: read len: %w", err)
		}
		recLen := binary.LittleEndian.Uint32(lenBuf[:])
		if recLen == 0 || recLen > walMaxRecordSize {
			return nil // corrupt, stop
		}

		// Read JSON data.
		data := make([]byte, recLen)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil // torn, stop
		}

		// Read CRC.
		var crcBuf [walCRCLen]byte
		if _, err := io.ReadFull(reader, crcBuf[:]); err != nil {
			return nil // torn, stop
		}
		expectedCRC := binary.LittleEndian.Uint32(crcBuf[:])
		if crc32.ChecksumIEEE(data) != expectedCRC {
			return nil // corrupt, stop
		}

		var rec WALRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return nil // corrupt, stop
		}
		if err := fn(rec); err != nil {
			return err // caller requested stop
		}
	}
}

// Compact collapses the WAL into a single snapshot record. It:
//  1. Replays the log and folds delta records into the latest cumulative
//     state (the last raw record may itself be a delta).
//  2. Writes the merged state as a new full snapshot to a tmp file.
//  3. Atomically renames the tmp file over the WAL file.
//  4. Resets the WAL for fresh appends.
//
// If the log is empty, the WAL is truncated to an empty file.
func (w *WAL) Compact() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	metrics.IncWALCompaction()

	// Flush current buffer first so on-disk state is current.
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("wal: compact flush: %w", err)
	}

	// Replay and merge: fold delta records into the latest full snapshot.
	var merged *WALRecord
	if err := ReplayWAL(w.path, func(r WALRecord) error {
		if merged == nil || !r.IsDelta {
			m := r
			merged = &m
			merged.IsDelta = false // the snapshot we write is always full
		} else {
			if merged.StepOutputs == nil {
				merged.StepOutputs = make(map[int]string, len(r.StepOutputs))
			}
			for k, v := range r.StepOutputs {
				merged.StepOutputs[k] = v
			}
			if merged.Variables == nil {
				merged.Variables = make(map[string]string, len(r.Variables))
			}
			for k, v := range r.Variables {
				merged.Variables[k] = v
			}
			merged.StepIndex = r.StepIndex
			merged.StepName = r.StepName
			merged.NodeName = r.NodeName
			merged.Data = r.Data
			merged.Timestamp = r.Timestamp
			merged.Seq = r.Seq
		}
		return nil
	}); err != nil {
		return fmt.Errorf("wal: compact replay: %w", err)
	}
	lastRec := merged

	// Close current write handle before replacing the file.
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("wal: compact close: %w", err)
	}

	// Write snapshot to tmp file with atomic rename.
	tmpPath := w.path + ".tmp"
	snapFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("wal: compact create tmp: %w", err)
	}

	snapWriter := bufio.NewWriterSize(snapFile, 64*1024)
	if lastRec != nil {
		// Write single snapshot record (seq preserved).
		data, err := json.Marshal(*lastRec)
		if err != nil {
			if cerr := snapFile.Close(); cerr != nil {
				logger.Warn("wal: compact snap close error", "stage", "marshal", "path", tmpPath, "error", cerr)
			}
			return fmt.Errorf("wal: compact marshal: %w", err)
		}
		frameLen, err := walFrameLen(len(data))
		if err != nil {
			if cerr := snapFile.Close(); cerr != nil {
				logger.Warn("wal: compact snap close error", "stage", "too_large", "path", tmpPath, "error", cerr)
			}
			return fmt.Errorf("wal: compact: %w", err)
		}
		var lenBuf [walLenFieldLen]byte
		binary.LittleEndian.PutUint32(lenBuf[:], frameLen)
		if _, err := snapWriter.Write(lenBuf[:]); err != nil {
			if cerr := snapFile.Close(); cerr != nil {
				logger.Warn("wal: compact snap close error", "stage", "write_len", "path", tmpPath, "error", cerr)
			}
			return fmt.Errorf("wal: compact write len: %w", err)
		}
		if _, err := snapWriter.Write(data); err != nil {
			if cerr := snapFile.Close(); cerr != nil {
				logger.Warn("wal: compact snap close error", "stage", "write_data", "path", tmpPath, "error", cerr)
			}
			return fmt.Errorf("wal: compact write data: %w", err)
		}
		var crcBuf [walCRCLen]byte
		binary.LittleEndian.PutUint32(crcBuf[:], crc32.ChecksumIEEE(data))
		if _, err := snapWriter.Write(crcBuf[:]); err != nil {
			if cerr := snapFile.Close(); cerr != nil {
				logger.Warn("wal: compact snap close error", "stage", "write_crc", "path", tmpPath, "error", cerr)
			}
			return fmt.Errorf("wal: compact write crc: %w", err)
		}
	}
	if err := snapWriter.Flush(); err != nil {
		if cerr := snapFile.Close(); cerr != nil {
			logger.Warn("wal: compact snap close error", "stage", "flush", "path", tmpPath, "error", cerr)
		}
		return fmt.Errorf("wal: compact snap flush: %w", err)
	}
	if err := snapFile.Sync(); err != nil {
		if cerr := snapFile.Close(); cerr != nil {
			logger.Warn("wal: compact snap close error", "stage", "sync", "path", tmpPath, "error", cerr)
		}
		return fmt.Errorf("wal: compact snap sync: %w", err)
	}
	if err := snapFile.Close(); err != nil {
		return fmt.Errorf("wal: compact snap close: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, w.path); err != nil {
		return fmt.Errorf("wal: compact rename: %w", err)
	}

	// Reopen for appends.
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("wal: compact reopen: %w", err)
	}
	w.file = f
	w.writer = bufio.NewWriterSize(f, 64*1024)
	w.bytesSinceCompaction = 0
	if lastRec != nil {
		w.seq = lastRec.Seq
	}

	return nil
}

// MaybeCompact triggers compaction if the log has grown past the threshold.
func (w *WAL) MaybeCompact() error {
	w.mu.Lock()
	threshold := w.opts.CompactionThreshold
	bytes := w.bytesSinceCompaction
	w.mu.Unlock()
	if bytes >= threshold {
		return w.Compact()
	}
	return nil
}

// recoverSeq scans existing records to find the highest seq number and seeds
// bytesSinceCompaction from the current on-disk file size.
func (w *WAL) recoverSeq() error {
	if info, err := os.Stat(w.path); err == nil {
		w.bytesSinceCompaction = info.Size()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("wal: stat: %w", err)
	}
	return ReplayWAL(w.path, func(r WALRecord) error {
		if r.Seq > w.seq {
			w.seq = r.Seq
		}
		return nil
	})
}
