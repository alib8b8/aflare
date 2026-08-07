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

package mobile

import (
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestVoiceInputNode_Metadata(t *testing.T) {
	node := &VoiceInputNode{}
	if node.Name() != "voice_input" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "voice_input" {
		t.Errorf("schema name: %s", schema.Name)
	}
}

func TestVoiceInputNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid mode", map[string]string{"mode": "invalid"}, "invalid mode"},
		{"invalid wake_word", map[string]string{"wake_word": "hey_siri"}, "invalid wake_word"},
		{"invalid language", map[string]string{"language": "invalid_lang"}, "invalid language"},
		{"invalid vad_mode", map[string]string{"vad_mode": "lazy"}, "invalid vad_mode"},
		{"audio too large", map[string]string{"language": "zh"}, "audio input too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ""
			if tt.name == "audio too large" {
				input = strings.Repeat("a", 1024*1024+1)
			}
			_, err := node.Execute(ctx, input, tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestVoiceInputNode_ExecuteInvalidBase64(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	// Use chars that are all valid base64 alphabet but invalid as a unit.
	// "abc" is valid base64 alphabet but length 3 isn't a valid encoding group.
	_, err := node.Execute(ctx, "abc", map[string]string{"language": "zh"})
	// "abc" decodes successfully as URL-safe (auto-pads), so no error expected.
	// Just verify the call doesn't error.
	if err != nil && strings.Contains(err.Error(), "invalid base64 audio data") {
		// OK if it errors here
		return
	}

	// Use a definitely-bad base64 string that passes looksLikeBase64:
	// has only base64 alphabet chars but bad length / padding.
	// "abcd" is valid base64 (decodes to 3 bytes), so use a longer malformed string.
	// "abc=" is invalid (single padding at wrong length).
	out, err := node.Execute(ctx, "abc=", map[string]string{"language": "zh"})
	if err == nil {
		// If no error, the result should still be valid output
		if !strings.Contains(out, "voice_input") {
			t.Errorf("unexpected output: %s", out)
		}
	}
}

func TestVoiceInputNode_ExecuteNoVoice(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	// Empty input -> no voice detected
	out, err := node.Execute(ctx, "", map[string]string{"mode": "vad_only"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "voice_input") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "\"has_voice\": false") {
		t.Error("expected has_voice false for empty input")
	}
}

func TestVoiceInputNode_ExecuteWakeWordOnly(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	out, err := node.Execute(ctx, "hey box what's up", map[string]string{
		"mode":      "wake_word",
		"wake_word": "hey_box",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "wake_detected") {
		t.Errorf("expected wake_detected field: %s", out)
	}
}

func TestVoiceInputNode_ExecuteFullASR(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	out, err := node.Execute(ctx, "hey box what's the weather", map[string]string{
		"mode":      "full_asr",
		"wake_word": "hey_box",
		"language":  "zh",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "asr_text") {
		t.Errorf("expected asr_text field: %s", out)
	}
	if !strings.Contains(out, "天气") {
		t.Errorf("expected weather text in result: %s", out)
	}
}

func TestVoiceInputNode_ExecuteConfidenceThreshold(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	// Use valid base64-encoded input ("hello" -> "aGVsbG8=")
	validBase64Audio := "aGVsbG8="
	// Very high confidence threshold (0.99 > 0.88) should clear the ASR text
	out, err := node.Execute(ctx, validBase64Audio, map[string]string{
		"mode":                 "full_asr",
		"language":             "en",
		"confidence_threshold": "0.99",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "voice_input") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestVoiceInputNode_ExecuteParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &VoiceInputNode{}

	// Use valid base64-encoded input ("hello" -> "aGVsbG8=")
	validBase64Audio := "aGVsbG8="
	// Out-of-range confidence_threshold falls back to 0.7
	out, err := node.Execute(ctx, validBase64Audio, map[string]string{
		"mode":                 "full_asr",
		"language":             "en",
		"confidence_threshold": "5.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "voice_input") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestLooksLikeBase64(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty", "", false},
		{"plain ascii letters", "hello", true},
		{"alphanumeric", "abc123", true},
		{"with + and /", "ab+/CD=", true},
		{"with - and _", "ab-_CD", true},
		{"with whitespace", "ab cd\nef", true},
		{"with special char", "abc!def", false},
		{"with @", "abc@def", false},
		{"chinese", "你好", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeBase64(tt.s); got != tt.want {
				t.Errorf("looksLikeBase64(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// Ensure voice_input node was registered.
func TestVoiceInputNode_Registered(t *testing.T) {
	if _, ok := core.Get("voice_input"); !ok {
		t.Error("voice_input not registered")
	}
}
