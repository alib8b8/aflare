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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/history"
)

// setupAuditChain points the history package at a temp dir, appends one audit
// record per timestamp (chained), and returns the audit log path.
func setupAuditChain(t *testing.T, stamps []time.Time) string {
	t.Helper()
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	history.SetHistoryDir(t.TempDir())
	for i, ts := range stamps {
		log := history.AuditLog{
			ID:        fmt.Sprintf("rec-%02d", i),
			Timestamp: ts,
			Action:    history.AuditActionConfigChange,
			User:      "alice",
			Detail:    fmt.Sprintf("change-%02d", i),
			Success:   true,
		}
		if err := history.AppendAuditLog(log); err != nil {
			t.Fatalf("failed to append audit log %d: %v", i, err)
		}
	}
	path := history.GetAuditLogPath()
	if path == "" {
		t.Fatal("audit log path is empty")
	}
	return path
}

func TestParseAuditExportArgs(t *testing.T) {
	opts, err := parseAuditExportArgs([]string{"--out", "/tmp/b.json", "--since", "2026-08-01", "--until", "2026-08-31", "--file", "/var/log/audit.jsonl"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.outPath != "/tmp/b.json" || opts.since != "2026-08-01" || opts.until != "2026-08-31" || opts.auditPath != "/var/log/audit.jsonl" {
		t.Errorf("unexpected options: %+v", opts)
	}

	opts, err = parseAuditExportArgs([]string{"--out=/b2.json", "--since=2026-08-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.outPath != "/b2.json" || opts.since != "2026-08-01T00:00:00Z" {
		t.Errorf("unexpected options: %+v", opts)
	}

	if _, err := parseAuditExportArgs([]string{"--bogus"}); err == nil {
		t.Error("expected error for unknown argument")
	}
	if _, err := parseAuditExportArgs([]string{"--out"}); err == nil {
		t.Error("expected error for --out without value")
	}
	if _, err := parseAuditExportArgs([]string{"--since"}); err == nil {
		t.Error("expected error for --since without value")
	}
	if _, err := parseAuditExportArgs([]string{"--until"}); err == nil {
		t.Error("expected error for --until without value")
	}

	opts, err = parseAuditExportArgs([]string{"-o", "/tmp/x.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.outPath != "/tmp/x.json" {
		t.Errorf("expected -o shorthand to set outPath, got %+v", opts)
	}
}

func TestParseAuditTimeArg(t *testing.T) {
	ts, dateOnly, err := parseAuditTimeArg("--since", "2026-08-15")
	if err != nil || !dateOnly {
		t.Fatalf("expected date-only parse, got dateOnly=%v err=%v", dateOnly, err)
	}
	if ts.Location() != time.Local {
		t.Errorf("expected local location for date-only value")
	}

	ts, dateOnly, err = parseAuditTimeArg("--since", "2026-08-15T09:30:00+08:00")
	if err != nil || dateOnly {
		t.Fatalf("expected RFC3339 parse, got dateOnly=%v err=%v", dateOnly, err)
	}
	if got := ts.Format(time.RFC3339); got != "2026-08-15T09:30:00+08:00" {
		t.Errorf("unexpected parsed time: %s", got)
	}

	_, _, err = parseAuditTimeArg("--since", "2026/08/15")
	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Errorf("expected --since format error, got: %v", err)
	}

	ts, _, err = parseAuditTimeArg("--since", "")
	if err != nil || !ts.IsZero() {
		t.Errorf("empty value should parse to zero time, got %v err=%v", ts, err)
	}
}

func TestRunAuditExport_RoundtripVerifyBundle(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour)})

	outPath := filepath.Join(t.TempDir(), "bundle.json")
	opts := auditExportOptions{outPath: outPath}
	bundle, gotPath, err := runAuditExport(opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if gotPath != outPath {
		t.Errorf("expected output path %s, got %s", outPath, gotPath)
	}
	if bundle.RecordCount != 3 {
		t.Errorf("expected 3 records, got %d", bundle.RecordCount)
	}
	if bundle.HeadHash != bundle.Records[2].CurrHash {
		t.Errorf("head_hash should equal last record's curr_hash")
	}
	if bundle.Manifest.Filter != nil {
		t.Errorf("expected nil filter for unfiltered export, got %+v", bundle.Manifest.Filter)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("bundle file not written: %v", err)
	}

	// verify --bundle roundtrip: runAuditVerifyBundle must succeed and agree
	// on the head hash.
	verified, err := runAuditVerifyBundle(outPath)
	if err != nil {
		t.Fatalf("bundle verification failed: %v", err)
	}
	if verified.HeadHash != bundle.HeadHash || verified.RecordCount != 3 {
		t.Errorf("verified bundle mismatch: %+v", verified)
	}
}

func TestRunAuditExport_DefaultOutPathInCwd(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{base})

	tmpCwd := t.TempDir()
	t.Chdir(tmpCwd)

	_, outPath, err := runAuditExport(auditExportOptions{})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if filepath.Dir(outPath) != tmpCwd {
		t.Errorf("expected bundle in %s, got %s", tmpCwd, outPath)
	}
	if !strings.HasPrefix(filepath.Base(outPath), "audit-bundle-") || !strings.HasSuffix(outPath, ".json") {
		t.Errorf("unexpected default bundle name: %s", outPath)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("bundle file not written: %v", err)
	}
}

func TestRunAuditExport_OutPathDirectory(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{base})

	dir := t.TempDir()
	_, outPath, err := runAuditExport(auditExportOptions{outPath: dir})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if filepath.Dir(outPath) != dir {
		t.Errorf("expected bundle inside %s, got %s", dir, outPath)
	}
}

func TestRunAuditExport_BrokenChainRefuses(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	path := setupAuditChain(t, []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour)})

	// Tamper with line 2, leaving curr_hash unchanged.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	tampered := strings.Replace(lines[1], "change-01", "change-EVIL", 1)
	if tampered == lines[1] {
		t.Fatal("tampering did not modify the line")
	}
	lines[1] = tampered
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("failed to write audit log: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "refused.json")
	_, _, err = runAuditExport(auditExportOptions{outPath: outPath})
	if err == nil {
		t.Fatal("expected export to be refused on broken chain")
	}
	var broken *auditChainBrokenError
	if !errors.As(err, &broken) {
		t.Fatalf("expected auditChainBrokenError, got: %v", err)
	}
	if broken.line != 2 {
		t.Errorf("expected broken line 2, got %d", broken.line)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Errorf("no bundle file should be written when export is refused")
	}
}

func TestRunAuditExport_SinceUntilInclusiveRFC3339(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{
		base,                    // 08:00 excluded (before since)
		base.Add(time.Hour),     // 09:00 == since boundary, included
		base.Add(2 * time.Hour), // 10:00 included
		base.Add(3 * time.Hour), // 11:00 == until boundary, included
		base.Add(4 * time.Hour), // 12:00 excluded (after until)
	})

	outPath := filepath.Join(t.TempDir(), "filtered.json")
	opts := auditExportOptions{
		outPath: outPath,
		since:   "2026-08-15T09:00:00Z",
		until:   "2026-08-15T11:00:00Z",
	}
	bundle, _, err := runAuditExport(opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if bundle.RecordCount != 3 {
		t.Fatalf("expected 3 records (bounds inclusive), got %d", bundle.RecordCount)
	}
	if bundle.Records[0].ID != "rec-01" || bundle.Records[2].ID != "rec-03" {
		t.Errorf("unexpected selected records: %s..%s", bundle.Records[0].ID, bundle.Records[2].ID)
	}
	if bundle.HeadHash != bundle.Records[2].CurrHash {
		t.Errorf("head_hash must follow the filtered chain tail")
	}
	if bundle.Manifest.Filter == nil ||
		bundle.Manifest.Filter.Since != "2026-08-15T09:00:00Z" ||
		bundle.Manifest.Filter.Until != "2026-08-15T11:00:00Z" {
		t.Errorf("manifest filter must echo raw args, got %+v", bundle.Manifest.Filter)
	}
	if bundle.TimeRange == nil ||
		bundle.TimeRange.From != base.Add(time.Hour).Format(time.RFC3339Nano) ||
		bundle.TimeRange.To != base.Add(3*time.Hour).Format(time.RFC3339Nano) {
		t.Errorf("time_range must cover actual records, got %+v", bundle.TimeRange)
	}

	// The filtered bundle must itself verify (chain subset stays linked).
	if err := runAuditVerifyBundleCheck(outPath); err != nil {
		t.Errorf("filtered bundle should verify: %v", err)
	}
}

// runAuditVerifyBundleCheck loads and verifies a bundle, returning an error on
// any failure (test helper around runAuditVerifyBundle).
func runAuditVerifyBundleCheck(path string) error {
	_, err := runAuditVerifyBundle(path)
	return err
}

func TestRunAuditExport_SinceUntilDateOnlyWholeDay(t *testing.T) {
	day := time.Date(2026, 8, 16, 0, 0, 0, 0, time.Local)
	setupAuditChain(t, []time.Time{
		day.Add(-time.Second), // 08-15 23:59:59 excluded
		day.Add(time.Hour),    // 08-16 01:00 included
		day.Add(23*time.Hour + 59*time.Minute + 59*time.Second + 500*time.Millisecond), // 08-16 23:59:59.5 included
		day.Add(24 * time.Hour), // 08-17 00:00 excluded
	})

	outPath := filepath.Join(t.TempDir(), "day.json")
	bundle, _, err := runAuditExport(auditExportOptions{outPath: outPath, since: "2026-08-16", until: "2026-08-16"})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if bundle.RecordCount != 2 {
		t.Fatalf("expected 2 records for whole-day range, got %d", bundle.RecordCount)
	}
	if bundle.Records[0].ID != "rec-01" || bundle.Records[1].ID != "rec-02" {
		t.Errorf("unexpected selected records: %s, %s", bundle.Records[0].ID, bundle.Records[1].ID)
	}
	if bundle.Manifest.Filter.Since != "2026-08-16" || bundle.Manifest.Filter.Until != "2026-08-16" {
		t.Errorf("manifest filter must echo raw date strings, got %+v", bundle.Manifest.Filter)
	}
}

func TestRunAuditExport_EmptyLog(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	history.SetHistoryDir(t.TempDir())

	outPath := filepath.Join(t.TempDir(), "empty.json")
	bundle, _, err := runAuditExport(auditExportOptions{outPath: outPath})
	if err != nil {
		t.Fatalf("export of empty log should succeed: %v", err)
	}
	if bundle.RecordCount != 0 || bundle.HeadHash != history.AuditZeroHash {
		t.Errorf("unexpected empty bundle: %+v", bundle)
	}
	if err := runAuditVerifyBundleCheck(outPath); err != nil {
		t.Errorf("empty bundle should verify: %v", err)
	}
}

func TestRunAuditExport_BadDate(t *testing.T) {
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{base})

	_, _, err := runAuditExport(auditExportOptions{outPath: filepath.Join(t.TempDir(), "x.json"), since: "not-a-date"})
	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Errorf("expected --since format error, got: %v", err)
	}
}

// tamperBundleJSON loads a bundle file, applies fn to the parsed raw object and
// writes it back, simulating an attacker editing the export.
func tamperBundleJSON(t *testing.T, path string, fn func(raw map[string]interface{})) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read bundle: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse bundle: %v", err)
	}
	fn(raw)
	out, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal bundle: %v", err)
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		t.Fatalf("failed to write bundle: %v", err)
	}
}

func exportTestBundle(t *testing.T) string {
	t.Helper()
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	setupAuditChain(t, []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour)})
	outPath := filepath.Join(t.TempDir(), "bundle.json")
	if _, _, err := runAuditExport(auditExportOptions{outPath: outPath}); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	return outPath
}

func TestRunAuditVerifyBundle_TamperedRecord(t *testing.T) {
	path := exportTestBundle(t)

	tamperBundleJSON(t, path, func(raw map[string]interface{}) {
		records := raw["records"].([]interface{})
		record := records[1].(map[string]interface{})
		record["detail"] = "tampered"
	})

	err := runAuditVerifyBundleCheck(path)
	if !errors.Is(err, history.ErrAuditBundleRecordsHash) {
		t.Errorf("expected ErrAuditBundleRecordsHash, got: %v", err)
	}
}

func TestRunAuditVerifyBundle_DeletedRecord(t *testing.T) {
	path := exportTestBundle(t)

	tamperBundleJSON(t, path, func(raw map[string]interface{}) {
		records := raw["records"].([]interface{})
		raw["records"] = append(records[:1], records[2:]...)
	})

	err := runAuditVerifyBundleCheck(path)
	if !errors.Is(err, history.ErrAuditBundleRecordsHash) {
		t.Errorf("expected ErrAuditBundleRecordsHash for deleted record, got: %v", err)
	}
}

func TestRunAuditVerifyBundle_TamperedSignature(t *testing.T) {
	path := exportTestBundle(t)

	tamperBundleJSON(t, path, func(raw map[string]interface{}) {
		raw["signature"] = strings.Repeat("ab", 32)
	})

	err := runAuditVerifyBundleCheck(path)
	if !errors.Is(err, history.ErrAuditBundleSignature) {
		t.Errorf("expected ErrAuditBundleSignature, got: %v", err)
	}
}

func TestRunAuditVerifyBundle_TamperedManifest(t *testing.T) {
	path := exportTestBundle(t)

	// A manifest field is inside the signature scope, so editing it must be
	// caught by the signature check.
	tamperBundleJSON(t, path, func(raw map[string]interface{}) {
		manifest := raw["manifest"].(map[string]interface{})
		manifest["count"] = 999
	})

	err := runAuditVerifyBundleCheck(path)
	if !errors.Is(err, history.ErrAuditBundleSignature) {
		t.Errorf("expected ErrAuditBundleSignature for manifest tamper, got: %v", err)
	}
}

func TestRunAuditVerifyBundle_WrongKey(t *testing.T) {
	path := exportTestBundle(t)

	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "another-key")
	err := runAuditVerifyBundleCheck(path)
	if !errors.Is(err, history.ErrAuditBundleSignature) {
		t.Errorf("expected ErrAuditBundleSignature with wrong key, got: %v", err)
	}
}

func TestRunAuditVerifyBundle_MissingFile(t *testing.T) {
	err := runAuditVerifyBundleCheck(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || !strings.Contains(err.Error(), "读取导出包失败") {
		t.Errorf("expected read error, got: %v", err)
	}
}
