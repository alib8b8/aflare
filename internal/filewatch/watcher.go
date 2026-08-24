// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​‌​‌‌​​‌‌​‌​‌‌‌‌​​‌‌​​‌​‌​‌​‌‌‌​​‌‌‌‌​​‌​​‌‌​​​​​​​​​​​​​​​​​​​‌​​​‌​​‌‌​‌‌​​⁠
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

// filewatch provides a real-time file system watcher that feeds change events
// into the agent daemon loop. It uses polling-based comparison (no fsnotify
// dependency) and emits events on a channel consumed by the agent.
package filewatch

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultPollInterval is the default interval between directory scans.
	DefaultPollInterval = 1 * time.Second

	// MaxDepth is the maximum directory recursion depth.
	MaxDepth = 10

	// MaxFiles is the maximum number of files tracked per watch.
	MaxFiles = 50000
)

// Event represents a file system change event.
type Event struct {
	Type      string    // "create", "modify", "delete"
	Path      string    // file path relative to watch root
	Timestamp time.Time // when the event was detected
	Size      int64     // file size in bytes
}

// Watcher monitors a directory for file changes and emits events on a channel.
type Watcher struct {
	rootPath string
	interval time.Duration
	events   chan<- Event
}

// NewWatcher creates a new file watcher for the given directory.
// Events are sent to the provided channel. The caller must ensure the channel
// is consumed promptly to avoid blocking.
func NewWatcher(rootPath string, interval time.Duration, events chan<- Event) (*Watcher, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %q: %w", rootPath, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot watch %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", absPath)
	}

	if interval <= 0 {
		interval = DefaultPollInterval
	}

	return &Watcher{
		rootPath: absPath,
		interval: interval,
		events:   events,
	}, nil
}

// Start begins watching the directory. It blocks until ctx is cancelled.
// Returns when the watcher stops.
func (w *Watcher) Start(ctx context.Context) {
	log.Printf("[filewatch] watching %s (interval: %s)", w.rootPath, w.interval)

	// Take initial snapshot
	current, err := w.snapshot()
	if err != nil {
		log.Printf("[filewatch] initial snapshot failed: %v", err)
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[filewatch] stopped watching %s", w.rootPath)
			return
		case <-ticker.C:
			next, err := w.snapshot()
			if err != nil {
				log.Printf("[filewatch] snapshot error: %v", err)
				continue
			}
			events := w.diff(current, next)
			for _, e := range events {
				select {
				case w.events <- e:
				case <-ctx.Done():
					return
				default:
					log.Printf("[filewatch] event channel full, dropping event: %s %s", e.Type, e.Path)
				}
			}
			current = next
		}
	}
}

// fileMeta holds file metadata for snapshot comparison.
type fileMeta struct {
	ModTime time.Time
	Size    int64
}

// snapshot captures the current state of all files under the watch root.
func (w *Watcher) snapshot() (map[string]fileMeta, error) {
	result := make(map[string]fileMeta)

	err := filepath.WalkDir(w.rootPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible files in WalkDir callback
		}
		// Skip symlinks
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip root
		if p == w.rootPath {
			return nil
		}
		// Check depth
		rel, err := filepath.Rel(w.rootPath, p)
		if err != nil {
			return nil //nolint:nilerr
		}
		if strings.Count(filepath.ToSlash(rel), "/") > MaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// File count limit
		if len(result) >= MaxFiles {
			return filepath.SkipDir
		}
		// Only track files
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr
		}
		result[filepath.ToSlash(rel)] = fileMeta{
			ModTime: fi.ModTime(),
			Size:    fi.Size(),
		}
		return nil
	})

	return result, err
}

// diff compares two snapshots and returns the change events.
func (w *Watcher) diff(old, cur map[string]fileMeta) []Event {
	now := time.Now()
	var events []Event

	// Create and modify
	for path, curMeta := range cur {
		oldMeta, existed := old[path]
		if !existed {
			events = append(events, Event{
				Type:      "create",
				Path:      path,
				Timestamp: now,
				Size:      curMeta.Size,
			})
		} else if oldMeta.ModTime != curMeta.ModTime || oldMeta.Size != curMeta.Size {
			events = append(events, Event{
				Type:      "modify",
				Path:      path,
				Timestamp: now,
				Size:      curMeta.Size,
			})
		}
	}
	// Delete
	for path := range old {
		if _, existed := cur[path]; !existed {
			events = append(events, Event{
				Type:      "delete",
				Path:      path,
				Timestamp: now,
			})
		}
	}

	// Sort for determinism
	sort.Slice(events, func(i, j int) bool {
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].Path < events[j].Path
	})

	return events
}

// FormatEvent formats a filewatch event as a human-readable agent input message.
func FormatEvent(e Event) string {
	return fmt.Sprintf("[filewatch] %s: %s (size: %d, at: %s)",
		e.Type, e.Path, e.Size, e.Timestamp.Format("15:04:05"))
}
