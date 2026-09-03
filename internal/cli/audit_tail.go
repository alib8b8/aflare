// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​​​​​​​‌‌​​‌​​​‌‌​‌‌​​​‌​‌​‌​‌​​​​​​​‌​​‌​​​‌​‌​‌​​​‌‌​​‌​​‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌‌​‌‌​​​‌‌​‌​​⁠
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

// audit_tail.go implements "aflare audit tail": a tail -f style live
// subscription to the append-only audit log. It prints the last N entries
// and then follows the file, emitting each newly appended record as it
// lands — the real-time view enterprise buyers ask for during due
// diligence, without touching the HMAC chain engine at all (pure I/O
// packaging; integrity checking stays in "aflare audit verify").
//
// Output modes:
//
//	human (default)  one formatted summary line per record
//	--json           the raw JSON line exactly as stored on disk, so SIEM
//	                 forwarders ingest the identical bytes the hash chain
//	                 covers
//
// The follower polls the file (auditPollInterval) rather than using
// fsnotify: append-only logs on local disks make polling cheap and it
// keeps the dependency surface at zero. A file that shrinks (manual
// truncation / rotation — both tamper events for a chain log) is re-read
// from the start with a stderr warning.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/history"
)

// auditPollInterval is how often the tail follower checks for new bytes.
const auditPollInterval = 500 * time.Millisecond

// auditTailChunk is the backward-read chunk size used to locate the last
// N entries without loading the whole file.
const auditTailChunk = 64 * 1024

// auditTailOptions carries the parsed "audit tail" arguments.
type auditTailOptions struct {
	auditPath string
	lines     int  // initial entries to print (-n), default 10
	jsonOut   bool // raw JSONL output for SIEM ingestion
	help      bool
}

// parseAuditTailArgs parses the arguments of "audit tail".
func parseAuditTailArgs(args []string) (auditTailOptions, error) {
	opts := auditTailOptions{lines: 10}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--file 需要一个路径参数")
			}
			i++
			opts.auditPath = args[i]
		case "-n", "--lines":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--lines 需要一个数字参数")
			}
			i++
			n, err := fmt.Sscanf(args[i], "%d", &opts.lines)
			if err != nil || n != 1 || opts.lines < 0 {
				return opts, fmt.Errorf("--lines 参数无效：%q（需要非负整数）", args[i])
			}
		case "--json":
			opts.jsonOut = true
		case "--help", "-h":
			opts.help = true
		default:
			switch {
			case strings.HasPrefix(args[i], "--file="):
				opts.auditPath = strings.TrimPrefix(args[i], "--file=")
			case strings.HasPrefix(args[i], "--lines="):
				n, err := fmt.Sscanf(strings.TrimPrefix(args[i], "--lines="), "%d", &opts.lines)
				if err != nil || n != 1 || opts.lines < 0 {
					return opts, fmt.Errorf("--lines 参数无效：%q（需要非负整数）", args[i])
				}
			default:
				return opts, fmt.Errorf("未知参数：%s", args[i])
			}
		}
	}
	return opts, nil
}

// HandleAuditTail handles the "audit tail" subcommand.
func HandleAuditTail(args []string) error {
	opts, err := parseAuditTailArgs(args)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	if opts.help {
		PrintAuditTailUsage()
		return nil
	}
	return runAuditTail(context.Background(), opts, os.Stdout)
}

// runAuditTail resolves the audit log, prints the last opts.lines entries
// and then follows the file until ctx is cancelled (Ctrl-C for the CLI).
func runAuditTail(ctx context.Context, opts auditTailOptions, w io.Writer) error {
	path := opts.auditPath
	if path == "" {
		path = history.GetAuditLogPath()
		if path == "" {
			return fmt.Errorf("无法定位审计日志路径，请使用 --file 指定审计日志文件")
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开审计日志失败（%s）：%w", path, err)
	}
	defer f.Close()

	fmt.Fprintf(os.Stderr, "==> tailing %s（Ctrl-C 停止；完整性校验请用 aflare audit verify）\n", path)

	// Initial snapshot: last N entries.
	offset, err := auditTailOffset(f, opts.lines)
	if err != nil {
		return fmt.Errorf("定位审计日志末尾失败：%w", err)
	}
	if _, err := emitAuditLines(f, &offset, w, opts.jsonOut); err != nil {
		return err
	}

	// Follow loop: poll for new bytes, emit every completed line.
	for {
		if _, err := emitAuditLines(f, &offset, w, opts.jsonOut); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(auditPollInterval):
		}
	}
}

// auditTailOffset returns the byte offset at which the last n entries
// start, so the initial snapshot does not need to load the whole file. It
// scans backwards in chunks counting newlines: the offset sits right after
// the (n+1)-th newline from the end. n == 0 seeks to end-of-file ("follow
// only, print no history"); a file with n or fewer entries yields 0 (start
// of file). A file without a trailing newline (a torn final line from a
// crashed writer) may yield one extra entry — harmless, since only
// complete lines are ever emitted.
func auditTailOffset(f *os.File, n int) (int64, error) {
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		// n == 0: no initial history — follow only.
		return st.Size(), nil
	}
	size := st.Size()
	if size == 0 {
		return 0, nil
	}

	var offset = size
	newlines := 0
	for offset > 0 {
		start := offset - auditTailChunk
		if start < 0 {
			start = 0
		}
		buf := make([]byte, offset-start)
		if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
			return 0, err
		}
		for i := len(buf) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				newlines++
				if newlines > n {
					return start + int64(i) + 1, nil
				}
			}
		}
		offset = start
	}
	return 0, nil
}

// emitAuditLines reads from *offset to the end of the file and writes every
// newly completed line to w. Only bytes up to the last newline are
// consumed — a partially written final line stays in the file and is
// emitted once the writer completes it. Returns the number of lines
// emitted. A file that shrank (truncation/rotation) is re-read from the
// start with a warning on stderr: for a hash-chain log that is itself a
// tamper signal, and SIEM consumers can dedupe.
func emitAuditLines(f *os.File, offset *int64, w io.Writer, jsonOut bool) (int, error) {
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if st.Size() < *offset {
		fmt.Fprintln(os.Stderr, "warning: 审计日志变小（被截断/轮转？），从头重读——对哈希链日志这本身就是篡改信号")
		*offset = 0
	}
	if st.Size() == *offset {
		return 0, nil
	}

	buf := make([]byte, st.Size()-*offset)
	if _, err := f.ReadAt(buf, *offset); err != nil && err != io.EOF {
		return 0, err
	}
	lastNL := bytes.LastIndexByte(buf, '\n')
	if lastNL < 0 {
		return 0, nil // no complete new line yet
	}
	*offset += int64(lastNL) + 1

	emitted := 0
	for _, line := range bytes.Split(buf[:lastNL], []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if jsonOut {
			// Raw passthrough: SIEM forwarders ingest the identical bytes
			// the hash chain covers.
			if _, err := w.Write(append(line, '\n')); err != nil {
				return emitted, err
			}
		} else if err := writeAuditHumanLine(w, line); err != nil {
			return emitted, err
		}
		emitted++
	}
	return emitted, nil
}

// writeAuditHumanLine formats one JSONL audit record as a human-readable
// summary line. An unparsable line (torn write caught between polls) is
// still surfaced, prefixed so it is visually distinguishable.
func writeAuditHumanLine(w io.Writer, line []byte) error {
	var e history.AuditLog
	if err := json.Unmarshal(line, &e); err != nil {
		_, werr := fmt.Fprintf(w, "[unparsable] %s\n", line)
		return werr
	}
	status := "FAIL"
	if e.Success {
		status = "ok"
	}
	s := fmt.Sprintf("%s\t%s\t%s", e.Timestamp.Format(time.RFC3339), e.Action, status)
	if e.User != "" {
		s += "\tuser=" + e.User
	}
	if e.Resource != "" {
		s += "\tresource=" + e.Resource
	}
	if e.ID != "" {
		s += "\tid=" + e.ID
	}
	if e.Detail != "" {
		s += "\t" + e.Detail
	}
	_, err := fmt.Fprintln(w, s)
	return err
}

// PrintAuditTailUsage prints usage information for "audit tail".
func PrintAuditTailUsage() {
	fmt.Println("Usage: aflare audit tail [--lines <n>] [--json] [--file <path>]")
	fmt.Println()
	fmt.Println("Live tail of the append-only audit log: print the last <n> entries, then")
	fmt.Println("follow the file and stream every new record as it is appended (tail -f).")
	fmt.Println("This is a pure transport — chain integrity is checked by 'audit verify'.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -n, --lines <n>   Number of initial entries to print (default: 10; 0 = none)")
	fmt.Println("  --json            Emit the raw JSONL record per line, exactly as stored on")
	fmt.Println("                    disk — the ingest format for SIEM/forwarders")
	fmt.Println("  --file, -f <path> Audit log file (defaults to the standard location)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare audit tail                        # last 10 entries, then follow")
	fmt.Println("  aflare audit tail -n 0 --json | vector   # stream to a SIEM forwarder")
}
