// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​‌​‌‌‌​‌‌‌​‌‌‌​‌‌‌​​​‌​​​​‌​‌​​‌​​‌​‌‌‌‌​‌‌​​​​​​​​​​​​​​​​​​‌‌​​​​‌​‌‌​​‌‌‌⁠
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

// Package output provides concise output mode for workflows.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/strutil"
)

// Mode controls the output verbosity.
type Mode int

const (
	// ModeNormal shows full output.
	ModeNormal Mode = iota
	// ModeConcise shows minimal output (one-liners).
	ModeConcise
	// ModeQuiet suppresses all non-essential output.
	ModeQuiet
	// ModeADHD shows answer-first, numbered, no-fluff output (inspired by i-have-adhd).
	ModeADHD
)

// OutputManager manages output modes.
type OutputManager struct {
	mu   sync.RWMutex
	mode Mode
	out  io.Writer
	err  io.Writer
}

// Global is the default output manager.
var Global = &OutputManager{
	mode: ModeNormal,
	out:  os.Stdout,
	err:  os.Stderr,
}

// SetMode sets the output mode.
func (o *OutputManager) SetMode(mode Mode) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mode = mode
}

// GetMode returns the current output mode.
func (o *OutputManager) GetMode() Mode {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode
}

// IsConcise returns true if concise mode is enabled (includes ADHD mode).
func (o *OutputManager) IsConcise() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode == ModeConcise || o.mode == ModeQuiet || o.mode == ModeADHD
}

// IsQuiet returns true if quiet mode is enabled.
func (o *OutputManager) IsQuiet() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode == ModeQuiet
}

// IsADHD returns true if ADHD-friendly mode is enabled.
func (o *OutputManager) IsADHD() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode == ModeADHD
}

// Print prints a message (respects output mode).
func (o *OutputManager) Print(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	fmt.Fprint(o.out, msg)
}

// Println prints a message with newline (respects output mode).
func (o *OutputManager) Println(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	fmt.Fprintln(o.out, msg)
}

// Printf prints a formatted message (respects output mode).
func (o *OutputManager) Printf(format string, args ...interface{}) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	fmt.Fprintf(o.out, format, args...)
}

// Error prints an error message (always shown).
func (o *OutputManager) Error(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	fmt.Fprintln(o.err, msg)
}

// Errorf prints a formatted error message (always shown).
func (o *OutputManager) Errorf(format string, args ...interface{}) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	fmt.Fprintf(o.err, format, args...)
}

// Status prints a concise status line (shown in all modes).
func (o *OutputManager) Status(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	fmt.Fprintln(o.out, msg)
}

// Detail prints detailed output (only shown in normal mode).
func (o *OutputManager) Detail(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode != ModeNormal {
		return
	}

	fmt.Fprintln(o.out, msg)
}

// Detailf prints formatted detailed output (only shown in normal mode).
func (o *OutputManager) Detailf(format string, args ...interface{}) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode != ModeNormal {
		return
	}

	fmt.Fprintf(o.out, format, args...)
}

// Progress prints progress info (one line, overwritten in concise mode).
func (o *OutputManager) Progress(step, total int, msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeConcise {
		// Concise: show progress percentage
		percent := 0
		if total > 0 {
			percent = step * 100 / total
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
		}
		// Truncate msg to avoid display issues with long lines
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		fmt.Fprintf(o.out, "\r⏳ [%d%%] %s", percent, msg)
	} else if o.mode == ModeNormal {
		// Normal: show full progress
		fmt.Fprintf(o.out, "  [%d/%d] %s\n", step, total, msg)
	}
}

// ProgressDone completes progress display.
func (o *OutputManager) ProgressDone(msg string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	switch {
	case o.mode == ModeConcise:
		fmt.Fprintf(o.out, "\r✅ %s\n", msg)
	case o.mode == ModeNormal:
		fmt.Fprintf(o.out, "✅ %s\n", msg)
	case o.mode == ModeADHD:
		fmt.Fprintf(o.out, "✅ %s\n", msg)
	}
}

// Answer prints the direct answer first (ADHD mode: always short, no fluff).
// In ADHD mode this is the ONLY thing shown for results.
func (o *OutputManager) Answer(answer string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	if o.mode == ModeADHD {
		fmt.Fprintf(o.out, "⚡ %s\n", strings.TrimSpace(answer))
	} else {
		fmt.Fprintf(o.out, "%s\n", answer)
	}
}

// Numbered prints items as a numbered list (ADHD-friendly: skips fluff).
func (o *OutputManager) Numbered(items []string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if o.mode == ModeADHD {
			if len(item) > 120 {
				// Byte-level item[:117] tears CJK runes and emits invalid
				// UTF-8 into step output — cut on a rune boundary instead.
				item = strutil.Truncate(item, 117) + "..."
			}
			fmt.Fprintf(o.out, "%d. %s\n", i+1, item)
		} else {
			fmt.Fprintf(o.out, "  %d. %s\n", i+1, item)
		}
	}
}

// KeyPoints prints key takeaways (ADHD mode: max 3 bullet points).
func (o *OutputManager) KeyPoints(points []string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.mode == ModeQuiet {
		return
	}

	maxPoints := len(points)
	if o.mode == ModeADHD {
		maxPoints = 3
	}
	if len(points) < maxPoints {
		maxPoints = len(points)
	}

	if o.mode == ModeADHD {
		fmt.Fprintln(o.out, "🔑 Key:")
	} else {
		fmt.Fprintln(o.out, "Key points:")
	}

	for i := 0; i < maxPoints; i++ {
		p := strings.TrimSpace(points[i])
		if p == "" {
			continue
		}
		if o.mode == ModeADHD && len(p) > 100 {
			p = p[:97] + "..."
		}
		fmt.Fprintf(o.out, "  • %s\n", p)
	}
}

// NoFluff removes common filler phrases (optimized for ADHD mode).
func NoFluff(text string) string {
	fluff := []string{
		"First of all, ",
		"First, ",
		"It's important to note that ",
		"It should be noted that ",
		"As you may know, ",
		"As we all know, ",
		"Basically, ",
		"Essentially, ",
		"Actually, ",
		"Honestly, ",
		"To be honest, ",
		"For what it's worth, ",
		"At the end of the day, ",
		"That being said, ",
		"With that being said, ",
		"in order to ",
		"in order for ",
		"the fact that ",
	}
	result := text
	for _, f := range fluff {
		result = strings.ReplaceAll(result, f, "")
		result = strings.ReplaceAll(result, strings.ToLower(f), "")
	}
	return strings.TrimSpace(result)
}

// Convenience functions using the global manager

// SetMode sets the global output mode.
func SetMode(mode Mode) {
	Global.SetMode(mode)
}

// GetMode returns the global output mode.
func GetMode() Mode {
	return Global.GetMode()
}

// IsConcise returns true if global concise mode is enabled.
func IsConcise() bool {
	return Global.IsConcise()
}

// IsQuiet returns true if global quiet mode is enabled.
func IsQuiet() bool {
	return Global.IsQuiet()
}

// Print prints using the global manager.
func Print(msg string) {
	Global.Print(msg)
}

// Println prints with newline using the global manager.
func Println(msg string) {
	Global.Println(msg)
}

// Printf prints formatted using the global manager.
func Printf(format string, args ...interface{}) {
	Global.Printf(format, args...)
}

// Error prints an error using the global manager.
func Error(msg string) {
	Global.Error(msg)
}

// Errorf prints a formatted error using the global manager.
func Errorf(format string, args ...interface{}) {
	Global.Errorf(format, args...)
}

// Status prints a status line using the global manager.
func Status(msg string) {
	Global.Status(msg)
}

// Detail prints detailed output using the global manager.
func Detail(msg string) {
	Global.Detail(msg)
}

// Detailf prints formatted detailed output using the global manager.
func Detailf(format string, args ...interface{}) {
	Global.Detailf(format, args...)
}

// Progress prints progress using the global manager.
func Progress(step, total int, msg string) {
	Global.Progress(step, total, msg)
}

// ProgressDone completes progress using the global manager.
func ProgressDone(msg string) {
	Global.ProgressDone(msg)
}

// IsADHD returns true if global ADHD mode is enabled.
func IsADHD() bool {
	return Global.IsADHD()
}

// Answer prints the direct answer using the global manager.
func Answer(answer string) {
	Global.Answer(answer)
}

// Numbered prints items as a numbered list using the global manager.
func Numbered(items []string) {
	Global.Numbered(items)
}

// KeyPoints prints key takeaways using the global manager.
func KeyPoints(points []string) {
	Global.KeyPoints(points)
}
