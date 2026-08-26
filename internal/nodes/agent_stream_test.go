// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​​‌‌‌‌​‌​​​‌‌​‌‌‌​​​​​​‌‌​​‌​‌​‌​​​​‌‌‌‌​​​‌​​​​​​​​​​​​​​​​​‌‌​‌‌​​‌​‌‌​‌​​⁠
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

package nodes

import (
	"strings"
	"testing"
)

// ── ollamaStreamFilter tests ──────────────────────────────────────────────

func TestOllamaStreamFilter_ThoughtExtraction(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Simulate ollama streaming the thought field
	f.feed(`{"thought": "`)
	f.feed(`I need to`)
	f.feed(` search for`)
	f.feed(` weather data`)
	f.feed(`"`)

	got := strings.Join(chunks, "")
	want := "I need to search for weather data"
	if got != want {
		t.Errorf("thought = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_FinalAnswerExtraction(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	f.feed(`{"final_answer": "`)
	f.feed(`The weather`)
	f.feed(` is sunny`)
	f.feed(`"`)

	got := strings.Join(chunks, "")
	want := "The weather is sunny"
	if got != want {
		t.Errorf("final_answer = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_ThoughtThenAnswer(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Full ReAct response: thought followed by action, then final_answer
	f.feed(`{"thought": "I will search", "action": "search", "action_input": "weather", "final_answer": "Sunny 22C"}`)

	got := strings.Join(chunks, "")
	want := "I will searchSunny 22C"
	if got != want {
		t.Errorf("combined = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_EscapedCharacters(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// thought with escaped double-quote and backslash
	f.feed(`{"thought": "`)
	f.feed(`He said \\\"hello\\\"`)
	f.feed(`"`)

	got := strings.Join(chunks, "")
	// escaped \" becomes " in the output, \\ becomes \
	want := `He said \"hello\"`
	if got != want {
		t.Errorf("escaped = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_JSONStructureSuppressed(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Pure JSON structure — no field values entered
	f.feed(`{`)
	f.feed(`"thought":`)
	f.feed(` "`)

	if len(chunks) != 0 {
		t.Errorf("expected no chunks from JSON structure, got %d: %v", len(chunks), chunks)
	}
}

func TestOllamaStreamFilter_EmptyChunks(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	f.feed("") // empty chunk should not crash
	f.feed(`{"thought": "hello"}`)

	if len(chunks) == 0 {
		t.Error("expected chunks after valid input")
	}
}

func TestOllamaStreamFilter_PartialPrefix(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Partial match should not trigger field entry
	f.feed(`"tho`) // partial prefix
	f.feed(`ught": "real content"`)

	got := strings.Join(chunks, "")
	want := "real content"
	if got != want {
		t.Errorf("partial prefix = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_MultibyteRuneContent(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Non-ASCII (CJK) content must stream through unmodified — regression
	// test for the rune→byte truncation that previously corrupted it.
	f.feed(`{"thought": "`)
	f.feed(`我需要搜索天气数据`)
	f.feed(`"`)
	f.feed(`,"final_answer": "今天晴，22 度"}`)

	got := strings.Join(chunks, "")
	want := "我需要搜索天气数据今天晴，22 度"
	if got != want {
		t.Errorf("multibyte = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_MultibyteRuneNoFalsePrefix(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// U+0174 (Ŵ) truncates to byte 0x74 ('t'). The sequence `"ŴhoughŴ": "`
	// spells `"thought": "` under that truncation — a naive rune→byte cast
	// in the prefix matcher would falsely enter field mode and stream the
	// value ("LEAK") as thought content.
	f.feed(`{"note": "ŴhoughŴ": "LEAK"`)
	f.feed(`, "final_answer": "ok"}`)

	got := strings.Join(chunks, "")
	if got != "ok" {
		t.Errorf("low-byte collision = %q, want %q", got, "ok")
	}
}

func TestOllamaStreamFilter_Flush(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Enter a field but don't close it
	f.feed(`{"thought": "incomplete`)

	// flush should add a newline
	f.flush()

	got := strings.Join(chunks, "")
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("flush should end with newline, got %q", got)
	}
}

func TestOllamaStreamFilter_FlushWhenNotInField(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Complete field — flush should not add extra newline
	f.feed(`{"thought": "done"}`)
	before := len(chunks)
	f.flush()

	if len(chunks) != before {
		t.Errorf("flush when not in field should not add chunks, got %d extra", len(chunks)-before)
	}
}

func TestOllamaStreamFilter_RepeatedFields(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// Two thought/answer pairs (simulating multi-turn ReAct)
	f.feed(`{"thought": "first", "final_answer": "done"}`)
	f.feed(`{"thought": "second", "final_answer": "also done"}`)

	got := strings.Join(chunks, "")
	want := "firstdonesecondalso done"
	if got != want {
		t.Errorf("repeated fields = %q, want %q", got, want)
	}
}

func TestOllamaStreamFilter_OnlyFinalAnswer(t *testing.T) {
	var chunks []string
	f := newOllamaStreamFilter(func(s string) { chunks = append(chunks, s) })

	// No thought field, only final_answer
	f.feed(`{"action": "final_answer", "final_answer": "Direct answer"}`)

	got := strings.Join(chunks, "")
	want := "Direct answer"
	if got != want {
		t.Errorf("only final_answer = %q, want %q", got, want)
	}
}
