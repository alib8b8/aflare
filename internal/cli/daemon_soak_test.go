// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​‌‌‌​​​​‌‌​‌‌‌​‌‌​​‌​​​‌​‌‌‌‌‌​​​‌‌‌​‌​​​​​‌​​​‌‌​​​​​​‌​​​​​​​​​​​​​​​​​​​​​‌​​​‌​​‌​​‌‌​​​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// buildAflare compiles the real aflare binary once per test run and
// returns its path. These tests exercise process-level behavior (signal
// delivery, graceful teardown) that cannot be observed in-process.
func buildAflare(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("signal-based daemon tests are POSIX-only")
	}
	if testing.Short() {
		t.Skip("daemon integration tests are skipped in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "aflare-test")
	start := time.Now()
	cmd := exec.Command("go", "build", "-o", bin, "github.com/alib8b8/aflare/cmd/aflare")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed (%s): %v\n%s", time.Since(start), err, out)
	}
	return bin
}

// repoRoot walks up from the test file to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found above test directory")
	return ""
}

// daemonEnv builds an isolated HOME for the daemon: no user config, a
// per-minute cron schedule and an empty watch directory, so scheduler,
// filewatch and taskqueue are all live even without an LLM backend.
func daemonEnv(t *testing.T) (home string, watchDir string) {
	t.Helper()
	base := t.TempDir()
	home = filepath.Join(base, "home")
	watchDir = filepath.Join(base, "watch")
	for _, d := range []string{filepath.Join(home, ".aflare"), watchDir} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	schedules := `[{"id":"soak-1","cron":"* * * * *","workflow_path":"","description":"soak tick"}]`
	if err := os.WriteFile(filepath.Join(home, ".aflare", "schedules.json"), []byte(schedules), 0o600); err != nil {
		t.Fatalf("write schedules: %v", err)
	}
	return home, watchDir
}

// startDaemon spawns the daemon with stdin held open (idle — the exact
// condition that used to deadlock signal handling) and returns the
// command plus its log file.
func startDaemon(t *testing.T, bin, home, watchDir string) (*exec.Cmd, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create log: %v", err)
	}

	// Hold stdin open forever so the daemon's stdin reader stays idle —
	// this is the condition under which SIGINT used to be ignored.
	neverCloses, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	t.Cleanup(func() { _ = neverCloses.Close() })

	cmd := exec.Command(bin, "agent", "--watch", watchDir)
	cmd.Stdin = neverCloses
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start daemon: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		_ = logFile.Close()
	})
	return cmd, logPath
}

// waitDaemonReady blocks until the daemon prints its banner or the
// timeout hits.
func waitDaemonReady(t *testing.T, logPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(data), "daemon mode") {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("daemon did not reach ready state within %s; log:\n%s", timeout, readLog(t, logPath))
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(unreadable: %v)", err)
	}
	return string(data)
}

// TestDaemon_SIGINTGracefulShutdown is the regression test for the
// deadlock fixed in the daemon main loop: with stdin idle (no input
// pending), SIGINT must shut the daemon down promptly and cleanly.
// Before the fix, the signal was captured but never consumed and only
// SIGKILL could stop the process.
func TestDaemon_SIGINTGracefulShutdown(t *testing.T) {
	bin := buildAflare(t)
	home, watchDir := daemonEnv(t)
	cmd, logPath := startDaemon(t, bin, home, watchDir)
	waitDaemonReady(t, logPath, 30*time.Second)

	// stdin is idle — the exact deadlock condition.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		// exited — good
	case <-time.After(15 * time.Second):
		t.Fatalf("daemon still running 15s after SIGINT (deadlock regression); log:\n%s", readLog(t, logPath))
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "Goodbye!") {
		t.Errorf("daemon exited but skipped graceful teardown (no Goodbye! marker); log:\n%s", log)
	}
}

// procStats samples RSS (KB) and thread count of a live process.
func procStats(t *testing.T, pid int) (rssKB int, threads int) {
	t.Helper()
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	data, err := os.ReadFile(statusPath) // #nosec G306 -- /proc status, world-readable
	if err != nil {
		t.Fatalf("read %s: %v", statusPath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "VmRSS:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				rssKB, _ = strconv.Atoi(fields[1])
			}
		case strings.HasPrefix(line, "Threads:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				threads, _ = strconv.Atoi(fields[1])
			}
		}
	}
	if rssKB == 0 {
		t.Fatalf("could not parse VmRSS from %s", statusPath)
	}
	return rssKB, threads
}

// TestDaemon_SoakStability runs the daemon long enough for the
// scheduler tick to fire and filewatch events to be processed, sampling
// RSS and thread count along the way. Fail conditions: runaway memory
// growth, thread explosion, or failure to shut down gracefully at the
// end. Skipped unless AFLARE_SOAK=1 (nightly workflow) because of its
// runtime.
func TestDaemon_SoakStability(t *testing.T) {
	if os.Getenv("AFLARE_SOAK") != "1" {
		t.Skip("set AFLARE_SOAK=1 to run the daemon soak test (nightly CI)")
	}

	bin := buildAflare(t)
	home, watchDir := daemonEnv(t)
	cmd, logPath := startDaemon(t, bin, home, watchDir)
	waitDaemonReady(t, logPath, 30*time.Second)

	// Sample at 10s, 30s, 50s. The first sample sits after runtime
	// warmup; growth between the later samples is what matters.
	type sample struct {
		t       time.Duration
		rssKB   int
		threads int
	}
	var samples []sample
	for _, at := range []time.Duration{10 * time.Second, 30 * time.Second, 50 * time.Second} {
		time.Sleep(at - func() time.Duration {
			if len(samples) == 0 {
				return 0
			}
			return samples[len(samples)-1].t
		}())
		rss, threads := procStats(t, cmd.Process.Pid)
		samples = append(samples, sample{t: at, rssKB: rss, threads: threads})
		t.Logf("t=%v rss=%dKB threads=%d", at, rss, threads)
	}

	// Filewatch events: touch files to force task processing churn.
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(watchDir, fmt.Sprintf("churn-%d.txt", i)), []byte("x"), 0o600); err != nil {
			t.Fatalf("touch watch file: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(5 * time.Second)

	// Memory assertion: between the 30s and 50s samples (post-warmup),
	// RSS growth must stay bounded. Go's GC makes RSS sawtooth, so allow
	// generous headroom: the manual soak measured ~27MB plateau; anything
	// growing >32MB between samples signals a leak.
	first, last := samples[1], samples[2]
	if growth := last.rssKB - first.rssKB; growth > 32*1024 {
		t.Errorf("RSS grew %dKB between %v and %v (leak?); samples: %+v", growth, first.t, last.t, samples)
	}
	// Thread explosion: manual soak held 7-8 threads; >32 is pathological.
	if last.threads > 32 {
		t.Errorf("thread count %d at %v exceeds 32 (goroutine/thread leak?); samples: %+v", last.threads, last.t, samples)
	}

	// Graceful shutdown after ~1 minute under load.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("daemon did not exit within 15s of SIGINT after soak; log tail:\n%s", tailStr(readLog(t, logPath), 2000))
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "Goodbye!") {
		t.Errorf("graceful teardown marker missing after soak; log tail:\n%s", tailStr(log, 2000))
	}
}

func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
