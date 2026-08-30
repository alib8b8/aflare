// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​‌‌‌‌​​​​​‌​​‌‌‌‌‌​​‌​‌‌‌​‌​‌​‌​‌​‌​‌‌​​​​‌​​​​​​​​​​​​​​​​​​​​‌​‌​‌‌​​​​​​​‌​⁠
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

package output

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func newTestManager() (*OutputManager, *bytes.Buffer, *bytes.Buffer) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	om := &OutputManager{
		mode: ModeNormal,
		out:  outBuf,
		err:  errBuf,
	}
	return om, outBuf, errBuf
}

func TestModeGetSet(t *testing.T) {
	om, _, _ := newTestManager()

	if om.GetMode() != ModeNormal {
		t.Errorf("default mode = %v, want ModeNormal", om.GetMode())
	}

	om.SetMode(ModeConcise)
	if om.GetMode() != ModeConcise {
		t.Errorf("mode after SetMode(Concise) = %v, want ModeConcise", om.GetMode())
	}

	om.SetMode(ModeQuiet)
	if om.GetMode() != ModeQuiet {
		t.Errorf("mode after SetMode(Quiet) = %v, want ModeQuiet", om.GetMode())
	}

	om.SetMode(ModeADHD)
	if om.GetMode() != ModeADHD {
		t.Errorf("mode after SetMode(ADHD) = %v, want ModeADHD", om.GetMode())
	}
}

func TestIsConciseVariants(t *testing.T) {
	om, _, _ := newTestManager()

	om.SetMode(ModeNormal)
	if om.IsConcise() {
		t.Error("IsConcise() should be false for ModeNormal")
	}

	om.SetMode(ModeConcise)
	if !om.IsConcise() {
		t.Error("IsConcise() should be true for ModeConcise")
	}

	om.SetMode(ModeQuiet)
	if !om.IsConcise() {
		t.Error("IsConcise() should be true for ModeQuiet")
	}

	om.SetMode(ModeADHD)
	if !om.IsConcise() {
		t.Error("IsConcise() should be true for ModeADHD")
	}
}

func TestPrintSuppression(t *testing.T) {
	om, outBuf, _ := newTestManager()

	om.SetMode(ModeQuiet)
	om.Print("hello")
	om.Println("world")
	om.Printf("fmt %s", "test")

	if outBuf.Len() != 0 {
		t.Errorf("quiet mode should suppress Print/Println/Printf, got %q", outBuf.String())
	}
}

func TestErrorAlwaysShown(t *testing.T) {
	modes := []Mode{ModeNormal, ModeConcise, ModeQuiet, ModeADHD}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			om, _, errBuf := newTestManager()
			om.SetMode(mode)

			errBuf.Reset()
			om.Error("error msg")
			if !strings.Contains(errBuf.String(), "error msg") {
				t.Errorf("Error() should show in mode %v, got %q", mode, errBuf.String())
			}

			errBuf.Reset()
			om.Errorf("fmt %s", "error")
			if !strings.Contains(errBuf.String(), "fmt error") {
				t.Errorf("Errorf() should show in mode %v, got %q", mode, errBuf.String())
			}
		})
	}
}

func TestDetailOnlyNormal(t *testing.T) {
	modes := []struct {
		mode  Mode
		shown bool
	}{
		{ModeNormal, true},
		{ModeConcise, false},
		{ModeQuiet, false},
		{ModeADHD, false},
	}

	for _, tt := range modes {
		t.Run(tt.mode.String(), func(t *testing.T) {
			om, outBuf, _ := newTestManager()
			om.SetMode(tt.mode)

			om.Detail("detail msg")
			got := outBuf.String()

			if tt.shown && !strings.Contains(got, "detail msg") {
				t.Errorf("Detail() should show in mode %v, got %q", tt.mode, got)
			}
			if !tt.shown && strings.Contains(got, "detail msg") {
				t.Errorf("Detail() should be suppressed in mode %v, got %q", tt.mode, got)
			}

			outBuf.Reset()
			om.Detailf("fmt %s", "detail")
			got = outBuf.String()

			if tt.shown && !strings.Contains(got, "fmt detail") {
				t.Errorf("Detailf() should show in mode %v, got %q", tt.mode, got)
			}
			if !tt.shown && strings.Contains(got, "fmt detail") {
				t.Errorf("Detailf() should be suppressed in mode %v, got %q", tt.mode, got)
			}
		})
	}
}

func TestProgressConcise(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeConcise)

	om.Progress(50, 100, "doing work")
	got := outBuf.String()

	if !strings.Contains(got, "[50%]") {
		t.Errorf("Progress() in concise mode should show percentage, got %q", got)
	}
	if !strings.Contains(got, "doing work") {
		t.Errorf("Progress() in concise mode should show message, got %q", got)
	}
}

func TestProgressNormal(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeNormal)

	om.Progress(3, 10, "step 3")
	got := outBuf.String()

	if !strings.Contains(got, "[3/10]") {
		t.Errorf("Progress() in normal mode should show [step/total], got %q", got)
	}
	if !strings.Contains(got, "step 3") {
		t.Errorf("Progress() in normal mode should show message, got %q", got)
	}
}

func TestProgressDoneModes(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeConcise, "✅ done\n"},
		{ModeNormal, "✅ done\n"},
		{ModeADHD, "✅ done\n"},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			om, outBuf, _ := newTestManager()
			om.SetMode(tt.mode)

			om.ProgressDone("done")
			got := outBuf.String()

			if !strings.Contains(got, tt.want) {
				t.Errorf("ProgressDone() in mode %v = %q, want to contain %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestAnswerADHD(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeADHD)

	om.Answer("  the answer  ")
	got := outBuf.String()

	if !strings.HasPrefix(got, "⚡") {
		t.Errorf("Answer() in ADHD mode should have ⚡ prefix, got %q", got)
	}
	if !strings.Contains(got, "the answer") {
		t.Errorf("Answer() should contain answer text, got %q", got)
	}
}

func TestAnswerNormal(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeNormal)

	om.Answer("the answer")
	got := outBuf.String()

	if strings.HasPrefix(got, "⚡") {
		t.Errorf("Answer() in normal mode should not have ⚡ prefix, got %q", got)
	}
	if !strings.Contains(got, "the answer") {
		t.Errorf("Answer() should contain answer text, got %q", got)
	}
}

func TestNumbered(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeNormal)

	items := []string{"first", "second", "third"}
	om.Numbered(items)
	got := outBuf.String()

	for i, item := range items {
		expected := string(rune('0'+i+1)) + ". " + item
		if !strings.Contains(got, expected) {
			t.Errorf("Numbered() should contain %q, got %q", expected, got)
		}
	}
}

// TestNumberedADHDCJKTruncation pins the runtime half of the byte-truncation
// bug: ADHD-mode items over 120 bytes used to be cut with item[:117], which
// tears CJK runes and ships invalid UTF-8 into step output and logs.
func TestNumberedADHDCJKTruncation(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeADHD)

	// 50 × 3-byte runes = 150 bytes, well over the 120-byte budget. The cut
	// must land on a rune boundary: 39 whole runes + "..." = 120 bytes.
	item := strings.Repeat("模块描述", 12) + strings.Repeat("尾", 2)
	om.Numbered([]string{item})
	got := outBuf.String()

	if !utf8.ValidString(got) {
		t.Fatalf("ADHD-mode truncation emitted invalid UTF-8: %q", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "...") {
		t.Errorf("truncated item should end with \"...\", got %q", got)
	}
	if strings.Contains(got, "\ufffd") {
		t.Errorf("truncated item contains U+FFFD replacement char: %q", got)
	}
}

func TestKeyPointsMax3(t *testing.T) {
	om, outBuf, _ := newTestManager()
	om.SetMode(ModeADHD)

	points := []string{"p1", "p2", "p3", "p4", "p5"}
	om.KeyPoints(points)
	got := outBuf.String()

	if !strings.Contains(got, "p1") || !strings.Contains(got, "p2") || !strings.Contains(got, "p3") {
		t.Errorf("KeyPoints() in ADHD mode should include first 3 points, got %q", got)
	}
	if strings.Contains(got, "p4") || strings.Contains(got, "p5") {
		t.Errorf("KeyPoints() in ADHD mode should limit to 3 points, got %q", got)
	}
}

func TestNoFluff(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"First of all, here is the answer.", "here is the answer."},
		{"First, let me explain.", "let me explain."},
		{"It's important to note that this works.", "this works."},
		{"Basically, the result is 42.", "the result is 42."},
		{"Actually, you need to do X.", "you need to do X."},
	}

	for _, tt := range tests {
		got := NoFluff(tt.input)
		if got != tt.want {
			t.Errorf("NoFluff(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGlobalConvenience(t *testing.T) {
	origGlobal := Global
	origOut := Global.out
	origErr := Global.err
	origMode := Global.GetMode()
	defer func() {
		Global = origGlobal
		Global.out = origOut
		Global.err = origErr
		Global.SetMode(origMode)
	}()

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	Global.out = outBuf
	Global.err = errBuf

	SetMode(ModeNormal)
	if GetMode() != ModeNormal {
		t.Errorf("GetMode() = %v, want ModeNormal", GetMode())
	}

	outBuf.Reset()
	Print("global print")
	if !strings.Contains(outBuf.String(), "global print") {
		t.Errorf("Print() on global failed, got %q", outBuf.String())
	}

	errBuf.Reset()
	Error("global error")
	if !strings.Contains(errBuf.String(), "global error") {
		t.Errorf("Error() on global failed, got %q", errBuf.String())
	}
}

func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "Normal"
	case ModeConcise:
		return "Concise"
	case ModeQuiet:
		return "Quiet"
	case ModeADHD:
		return "ADHD"
	default:
		return "Unknown"
	}
}

var _ io.Writer = (*bytes.Buffer)(nil)
