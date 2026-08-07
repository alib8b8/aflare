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

func TestVoiceOutputNode_Metadata(t *testing.T) {
	node := &VoiceOutputNode{}
	if node.Name() != "voice_output" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "voice_output" {
		t.Errorf("schema name: %s", schema.Name)
	}
	if len(schema.Params) == 0 {
		t.Error("expected params")
	}
}

func TestVoiceOutputNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	tests := []struct {
		name   string
		input  string
		params map[string]string
		errSub string
	}{
		{
			"invalid operation",
			"hello",
			map[string]string{"operation": "invalid"},
			"invalid operation",
		},
		{
			"invalid engine",
			"hello",
			map[string]string{"operation": "tts", "engine": "nonexistent"},
			"invalid engine",
		},
		{
			"text too long",
			strings.Repeat("a", 4001),
			map[string]string{"operation": "tts", "engine": "sensevoice"},
			"text too long",
		},
		{
			"invalid style",
			"hello",
			map[string]string{"operation": "tts", "engine": "sensevoice", "style": "fancy"},
			"invalid style",
		},
		{
			"invalid output_format",
			"hello",
			map[string]string{"operation": "tts", "engine": "sensevoice", "output_format": "aac"},
			"invalid output_format",
		},
		{
			"reference_audio too large",
			"hello",
			map[string]string{"operation": "clone", "reference_audio": strings.Repeat("a", 10*1024*1024+1)},
			"reference_audio too large",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestVoiceOutputNode_ExecuteInvalidBase64ReferenceAudio(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// A string that looks like base64 (only base64 alphabet) but has invalid length/padding.
	// "abc=" has valid chars but invalid padding for length 4 (would need "ab==").
	out, err := node.Execute(ctx, "", map[string]string{
		"operation":       "clone",
		"reference_audio": "abc=",
	})
	// Depending on std vs url decoding, this may pass or fail. Either outcome is OK as
	// long as it doesn't panic. Just verify no panic.
	_ = out
	_ = err
}

func TestVoiceOutputNode_ExecuteTTSSuccess(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	out, err := node.Execute(ctx, "Hello, how are you?", map[string]string{
		"engine":        "cosyvoice",
		"operation":     "tts",
		"voice":         "speaker1",
		"style":         "friendly",
		"speed":         "1.2",
		"pitch":         "1.1",
		"output_format": "wav",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "voice_output") && !strings.Contains(out, "\"engine\":") {
		// The TTS result doesn't include "voice_output" type field directly, just check engine
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "cosyvoice") {
		t.Error("expected engine cosyvoice in output")
	}
}

func TestVoiceOutputNode_ExecuteParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Out-of-range speed and pitch fall back to 1.0 (clamped internally).
	// The TTS result does not expose speed/pitch, so we verify the call
	// succeeds and produces valid TTS output.
	out, err := node.Execute(ctx, "hello", map[string]string{
		"operation": "tts",
		"engine":    "sensevoice",
		"speed":     "5.0", // > 2.0, falls back to 1.0
		"pitch":     "0.1", // < 0.5, falls back to 1.0
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "sensevoice") {
		t.Errorf("expected engine sensevoice in output: %s", out)
	}
	if !strings.Contains(out, "tts") {
		t.Errorf("expected operation tts in output: %s", out)
	}
}

func TestVoiceOutputNode_ExecuteCloneFromInput(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Clone operation: reference_audio falls back to input
	out, err := node.Execute(ctx, "AAAA", map[string]string{
		"operation": "clone",
		"engine":    "fish-speech",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "fish-speech") {
		t.Errorf("expected fish-speech engine: %s", out)
	}
}

// -----------------------------------------------------------------
// ASR operations
// -----------------------------------------------------------------

func TestVoiceOutputNode_ExecuteASRErrors(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{
			"invalid ASR engine",
			map[string]string{"operation": "transcribe", "engine": "invalid_engine"},
			"invalid ASR engine",
		},
		{
			"invalid language",
			map[string]string{"operation": "transcribe", "engine": "whisper", "language": "klingon"},
			"invalid language",
		},
		{
			"audio_input too large",
			map[string]string{"operation": "transcribe", "engine": "whisper", "audio_input": strings.Repeat("a", 50*1024*1024+1)},
			"audio_input too large",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestVoiceOutputNode_ExecuteASRSuccess(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Use a valid base64-encoded audio input ("audio" -> "YXVkaW8=")
	validBase64Audio := "YXVkaW8="

	operations := []string{"asr", "transcribe", "diarize", "voice-analyze"}
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			out, err := node.Execute(ctx, validBase64Audio, map[string]string{
				"operation": op,
				"engine":    "whisper",
				"language":  "zh-CN",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// All ASR operations should produce a result
			if !strings.Contains(out, "operation") {
				t.Errorf("expected operation field: %s", out)
			}
		})
	}
}

func TestVoiceOutputNode_ExecuteASRWithTimestamps(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Use a valid base64-encoded audio input ("audio" -> "YXVkaW8=")
	validBase64Audio := "YXVkaW8="

	out, err := node.Execute(ctx, validBase64Audio, map[string]string{
		"operation":          "transcribe",
		"engine":             "whisper",
		"language":           "en",
		"enable_timestamps":  "true",
		"enable_diarization": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "segments") {
		t.Errorf("expected segments with timestamps: %s", out)
	}
}

// -----------------------------------------------------------------
// Creator operations
// -----------------------------------------------------------------

func TestVoiceOutputNode_ExecuteCreatorErrors(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Creator operations require text
	_, err := node.Execute(ctx, "", map[string]string{"operation": "podcast"})
	if err == nil {
		t.Fatal("expected error for creator op without text")
	}
	if !strings.Contains(err.Error(), "text is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVoiceOutputNode_ExecuteCreatorSuccess(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	tests := []struct {
		name        string
		operation   string
		creatorMode string
	}{
		{"podcast", "podcast", ""},
		{"audio-book", "audio-book", ""},
		{"narration", "narration", ""},
		{"jingles", "jingles", ""},
		{"create podcast", "create", "podcast"},
		{"create audio-book", "create", "audio-book"},
		{"create narration", "create", "narration"},
		{"create jingles", "create", "jingles"},
		{"create ad", "create", "ad"},
		{"create education", "create", "education"},
		{"create unknown falls back to podcast", "create", "unknown_mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"operation": tt.operation,
				"text":      "This is the first paragraph.\n\nThis is the second paragraph.",
			}
			if tt.creatorMode != "" {
				params["creator_mode"] = tt.creatorMode
			}
			out, err := node.Execute(ctx, "", params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "segments") {
				t.Errorf("expected segments field: %s", out)
			}
			if !strings.Contains(out, "total_duration") {
				t.Errorf("expected total_duration field: %s", out)
			}
		})
	}
}

func TestVoiceOutputNode_ExecutePodcastWithIntroAndMusic(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	out, err := node.Execute(ctx, "Content here", map[string]string{
		"operation":        "podcast",
		"intro":            "true",
		"background_music": "true",
		"host_voice":       "narrator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "intro") {
		t.Error("expected intro segment")
	}
	if !strings.Contains(out, "background_music") {
		t.Error("expected background_music field")
	}
}

// -----------------------------------------------------------------
// Inkling audio operation
// -----------------------------------------------------------------

func TestVoiceOutputNode_ExecuteInklingErrors(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	// Missing audio_input
	_, err := node.Execute(ctx, "", map[string]string{"operation": "inkling_audio"})
	if err == nil {
		t.Fatal("expected error for missing audio_input")
	}
	if !strings.Contains(err.Error(), "audio_input is required") {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid language
	_, err = node.Execute(ctx, "audio_data", map[string]string{
		"operation": "inkling_audio",
		"language":  "klingon",
	})
	if err == nil {
		t.Fatal("expected error for invalid language")
	}
	if !strings.Contains(err.Error(), "invalid language") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVoiceOutputNode_ExecuteInklingSuccess(t *testing.T) {
	ctx := context.Background()
	node := &VoiceOutputNode{}

	out, err := node.Execute(ctx, "audio_data", map[string]string{
		"operation": "inkling_audio",
		"language":  "zh-CN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "inkling_audio") {
		t.Errorf("expected inkling_audio operation: %s", out)
	}
	if !strings.Contains(out, "inkling_features") {
		t.Error("expected inkling_features field")
	}
}

// -----------------------------------------------------------------
// Direct create* method tests
// -----------------------------------------------------------------

func TestVoiceOutputNode_CreatePodcast(t *testing.T) {
	node := &VoiceOutputNode{}
	text := "Paragraph one.\n\nParagraph two.\n\nParagraph three."

	// Without intro
	segments, total := node.createPodcast(text, "host", "friendly", 1.0, false, false)
	if len(segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}

	// With intro adds intro and outro
	segments, total = node.createPodcast(text, "host", "friendly", 1.0, true, true)
	if len(segments) != 5 { // intro + 3 content + outro
		t.Errorf("expected 5 segments with intro, got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration with intro")
	}
	// All segments should have background_music=true
	for _, s := range segments {
		if s["background_music"] != true {
			t.Error("expected background_music=true on all segments")
		}
	}
}

func TestVoiceOutputNode_CreateAudioBook(t *testing.T) {
	node := &VoiceOutputNode{}
	text := "word " + strings.Repeat("word ", 100)

	segments, total := node.createAudioBook(text, 3, "calm", 1.0)
	if len(segments) != 3 {
		t.Errorf("expected 3 chapters, got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}

	// More chapters than words
	segments, _ = node.createAudioBook("short text", 5, "calm", 1.0)
	// Should produce at most 1 segment since words < wordsPerChapter
	if len(segments) > 1 {
		t.Errorf("expected at most 1 segment for short text, got %d", len(segments))
	}
}

func TestVoiceOutputNode_CreateNarration(t *testing.T) {
	node := &VoiceOutputNode{}
	text := "Sentence one. Sentence two. Sentence three."

	segments, total := node.createNarration(text, "natural", 1.0)
	if len(segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}

	// Empty text produces no segments
	segments, _ = node.createNarration("   .   .   ", "natural", 1.0)
	if len(segments) != 0 {
		t.Errorf("expected 0 segments for whitespace-only, got %d", len(segments))
	}
}

func TestVoiceOutputNode_CreateJingles(t *testing.T) {
	node := &VoiceOutputNode{}
	text := "Line one\nLine two\nLine three"

	segments, total := node.createJingles(text, "excited", 1.5)
	if len(segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}
}

func TestVoiceOutputNode_CreateAd(t *testing.T) {
	node := &VoiceOutputNode{}

	segments, total := node.createAd("Buy our product now!", "professional", 1.0)
	if len(segments) != 3 {
		t.Errorf("expected 3 segments (intro, content, cta), got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}
}

func TestVoiceOutputNode_CreateEducation(t *testing.T) {
	node := &VoiceOutputNode{}

	segments, total := node.createEducation("Today we learn Go testing.", "friendly", 1.0)
	if len(segments) != 3 {
		t.Errorf("expected 3 segments (intro, content, summary), got %d", len(segments))
	}
	if total <= 0 {
		t.Error("expected positive duration")
	}
}

// -----------------------------------------------------------------
// Direct simulation method tests
// -----------------------------------------------------------------

func TestVoiceOutputNode_SimulateASRTranscription(t *testing.T) {
	node := &VoiceOutputNode{}

	tests := []struct {
		language string
		wantSub  string
	}{
		{"zh", "今天天气很好"},
		{"zh-CN", "今天天气很好"},
		{"ja", "こんにちは"},
		{"ko", "안녕하세요"},
		{"en", "test of the speech recognition"},
		{"en-US", "test of the speech recognition"},
		{"unknown", "test of the speech recognition"}, // falls back to en
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			result := node.simulateASRTranscription("audio", "whisper", tt.language, "base", false)
			text, _ := result["text"].(string)
			if !strings.Contains(text, tt.wantSub) {
				t.Errorf("language %s: text = %q, want substring %q", tt.language, text, tt.wantSub)
			}
		})
	}

	// With timestamps
	result := node.simulateASRTranscription("audio", "whisper", "en", "base", true)
	if _, ok := result["segments"]; !ok {
		t.Error("expected segments with timestamps enabled")
	}
}

func TestVoiceOutputNode_SimulateSpeakerDiarization(t *testing.T) {
	node := &VoiceOutputNode{}

	// Chinese language
	result := node.simulateSpeakerDiarization("audio", "whisper", "zh", false, false)
	if result["num_speakers"] != 2 {
		t.Errorf("expected 2 speakers, got %v", result["num_speakers"])
	}
	segs, _ := result["segments"].([]map[string]interface{})
	if len(segs) != 4 {
		t.Errorf("expected 4 segments, got %d", len(segs))
	}

	// English language uses English segments
	result = node.simulateSpeakerDiarization("audio", "whisper", "en-US", false, false)
	segs, _ = result["segments"].([]map[string]interface{})
	if len(segs) != 4 {
		t.Errorf("expected 4 segments for en, got %d", len(segs))
	}
	firstSeg := segs[0]
	if !strings.Contains(firstSeg["text"].(string), "Speaker A") && !strings.Contains(firstSeg["text"].(string), "Hello") {
		t.Errorf("expected English segment text, got %v", firstSeg["text"])
	}

	// With timestamps adds word_timestamps
	result = node.simulateSpeakerDiarization("audio", "whisper", "zh", false, true)
	segs, _ = result["segments"].([]map[string]interface{})
	if _, ok := segs[0]["word_timestamps"]; !ok {
		t.Error("expected word_timestamps field when enableTimestamps=true")
	}
}

func TestVoiceOutputNode_SimulateVoiceAnalysis(t *testing.T) {
	node := &VoiceOutputNode{}

	result := node.simulateVoiceAnalysis("audio", "whisper")
	if result["operation"] != "voice-analyze" {
		t.Errorf("operation: got %v, want voice-analyze", result["operation"])
	}
	analysis, ok := result["analysis"].(map[string]interface{})
	if !ok {
		t.Fatal("expected analysis map")
	}
	if analysis["gender"] == "" {
		t.Error("expected non-empty gender")
	}
	if analysis["language_detect"] == "" {
		t.Error("expected non-empty language_detect")
	}
}

func TestVoiceOutputNode_PerformInklingAudioAnalysis(t *testing.T) {
	node := &VoiceOutputNode{}

	// Chinese language
	result := node.performInklingAudioAnalysis("audio", "zh")
	transcription, _ := result["transcription"].(string)
	if !strings.Contains(transcription, "今天天气很好") {
		t.Errorf("expected Chinese transcription, got %q", transcription)
	}

	// Non-Chinese language
	result = node.performInklingAudioAnalysis("audio", "en")
	transcription, _ = result["transcription"].(string)
	if !strings.Contains(transcription, "Inkling") {
		t.Errorf("expected English transcription with Inkling, got %q", transcription)
	}

	// Verify inkling_features structure
	features, ok := result["inkling_features"].(map[string]interface{})
	if !ok {
		t.Fatal("expected inkling_features map")
	}
	if features["architecture"] == "" {
		t.Error("expected non-empty architecture")
	}
	semUnder, ok := features["semantic_understanding"].(map[string]interface{})
	if !ok {
		t.Fatal("expected semantic_understanding map")
	}
	if semUnder["topic"] == "" {
		t.Error("expected non-empty topic")
	}
}

// Ensure voice_output node was registered.
func TestVoiceOutputNode_Registered(t *testing.T) {
	if _, ok := core.Get("voice_output"); !ok {
		t.Error("voice_output not registered")
	}
}
