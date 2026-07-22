// Package output provides concise output mode for workflows.
package output

import (
	"fmt"
	"io"
	"os"
	"sync"
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
)

// OutputManager manages output modes.
type OutputManager struct {
	mu    sync.RWMutex
	mode  Mode
	out   io.Writer
	err   io.Writer
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

// IsConcise returns true if concise mode is enabled.
func (o *OutputManager) IsConcise() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode == ModeConcise || o.mode == ModeQuiet
}

// IsQuiet returns true if quiet mode is enabled.
func (o *OutputManager) IsQuiet() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode == ModeQuiet
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
	
	if o.mode == ModeConcise {
		fmt.Fprintf(o.out, "\r✅ %s\n", msg)
	} else if o.mode == ModeNormal {
		fmt.Fprintf(o.out, "✅ %s\n", msg)
	}
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