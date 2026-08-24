// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​​​‌​‌‌‌​​‌‌‌​‌‌​​‌​​​​​‌‌‌‌‌‌​​​‌​​‌​​​‌​‌‌‌‌​‌​​​​​​​​​​​​​​​​​‌​‌‌​‌‌​‌​​‌‌‌‌⁠
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

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The review command's Analyze/Explain helpers are rule-based (no LLM call,
// no network), so the full review flow is testable offline.

func writeReviewFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

func TestReviewCmd_NoArgs(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleReview(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare review") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

func TestReviewCmd_FileNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	var err error
	out := captureOutput(func() {
		err = HandleReview([]string{missing})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Failed to stat workflow file") {
		t.Errorf("expected stat failure output, got: %s", out)
	}
}

// TestReviewCmd_FileTooLarge covers the size guard: a sparse file just over
// the 10 MB limit is rejected before it is ever read.
func TestReviewCmd_FileTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.yaml")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("creating file: %v", err)
	}
	if err := os.Truncate(path, 10*1024*1024+1); err != nil {
		t.Fatalf("growing file past the size limit: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = HandleReview([]string{path})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for oversized file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Workflow file too large") {
		t.Errorf("expected too-large output, got: %s", out)
	}
}

func TestReviewCmd_AnalyzesValidWorkflow(t *testing.T) {
	wf := writeReviewFixture(t, "review.yaml", `name: review-demo
description: workflow used by review command tests
steps:
  - name: save
    node: file_write
    params:
      path: out.txt
      content: hello
`)

	var err error
	out := captureOutput(func() {
		err = HandleReview([]string{wf})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for review of valid workflow, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "aflare Workflow Advisor Review") {
		t.Errorf("expected review banner, got: %s", out)
	}
	if !strings.Contains(out, "Optimization Score:") {
		t.Errorf("expected optimization score, got: %s", out)
	}
}

// TestReviewCmd_InvalidYAMLReported covers the analyze-failure branch: an
// unparseable workflow still prints the review report (with the parse error
// as an error-severity suggestion) and exits 0 — review is advisory.
func TestReviewCmd_InvalidYAMLReported(t *testing.T) {
	wf := writeReviewFixture(t, "broken.yaml", "name: broken\nsteps:\n  - node: [unterminated\n")

	var err error
	out := captureOutput(func() {
		err = HandleReview([]string{wf})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for review of invalid YAML, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Invalid workflow YAML") {
		t.Errorf("expected invalid-YAML suggestion, got: %s", out)
	}
}
