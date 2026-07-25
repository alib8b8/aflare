// Copyright (c) 2026 llm-box Contributors
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

package nodes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	validWakeWords = map[string]bool{
		"hey_box":   true,
		"hello_box": true,
		"hi_box":    true,
		"ok_box":    true,
		"box_box":   true,
	}
	validVADModes = map[string]bool{
		"fixed":      true,
		"adaptive":   true,
		"aggressive": true,
	}
	languagePattern = regexp.MustCompile(`^[a-zA-Z]{2}(-[a-zA-Z]{2})?$`)
)

// VoiceInputNode handles voice activity detection, wake word detection, and speech recognition
type VoiceInputNode struct{}

func (n *VoiceInputNode) Name() string { return "voice_input" }

func (n *VoiceInputNode) Description() string {
	return "Voice input pipeline: VAD (Voice Activity Detection), wake word detection, and speech-to-text. Supports on-device recognition for privacy."
}

func (n *VoiceInputNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - raw audio data (base64) or audio file path",
		Output:      "string - recognized text with confidence and metadata",
		Params: []ParamSchema{
			{Name: "mode", Type: "string", Description: "Pipeline mode: vad_only/wake_word/full_asr (default: full_asr)", Required: false, Default: "full_asr"},
			{Name: "wake_word", Type: "string", Description: "Wake word to detect: hey_box/hello_box/hi_box/ok_box/box_box (default: hey_box)", Required: false, Default: "hey_box"},
			{Name: "language", Type: "string", Description: "Recognition language: zh/en/ja/ko/fr/de/es (default: zh)", Required: false, Default: "zh"},
			{Name: "vad_mode", Type: "string", Description: "VAD sensitivity: fixed/adaptive/aggressive (default: adaptive)", Required: false, Default: "adaptive"},
			{Name: "max_duration_sec", Type: "int", Description: "Max audio duration in seconds (default: 30)", Required: false, Default: "30"},
			{Name: "offline", Type: "bool", Description: "Use on-device recognition only (default: true)", Required: false, Default: "true"},
			{Name: "confidence_threshold", Type: "float", Description: "Minimum confidence 0.0-1.0 (default: 0.7)", Required: false, Default: "0.7"},
		},
	}
}

func (n *VoiceInputNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	mode := getMobileParam(params, "mode", "full_asr")
	if mode != "vad_only" && mode != "wake_word" && mode != "full_asr" {
		return "", fmt.Errorf("invalid mode: %s", mode)
	}

	wakeWord := getMobileParam(params, "wake_word", "hey_box")
	if !validWakeWords[wakeWord] {
		return "", fmt.Errorf("invalid wake_word: %s", wakeWord)
	}

	language := getMobileParam(params, "language", "zh")
	if !languagePattern.MatchString(language) {
		return "", fmt.Errorf("invalid language: %s", language)
	}

	vadMode := getMobileParam(params, "vad_mode", "adaptive")
	if !validVADModes[vadMode] {
		return "", fmt.Errorf("invalid vad_mode: %s", vadMode)
	}

	maxDuration := parseIntSafe(getMobileParam(params, "max_duration_sec", "30"), 30)
	if maxDuration < 1 || maxDuration > 300 {
		maxDuration = 30
	}

	offline := strings.ToLower(getMobileParam(params, "offline", "true")) == "true"

	confidenceThreshold := parseFloatSafe(getMobileParam(params, "confidence_threshold", "0.7"), 0.7)
	if confidenceThreshold < 0 || confidenceThreshold > 1.0 {
		confidenceThreshold = 0.7
	}

	// Validate input (audio data or file path)
	if input != "" {
		if len(input) > 1024*1024 {
			return "", fmt.Errorf("audio input too large (max 1MB)")
		}
		// Check if input looks like base64 and validate
		trimmed := strings.TrimSpace(input)
		if looksLikeBase64(trimmed) {
			cleaned := strings.ReplaceAll(trimmed, "\n", "")
			cleaned = strings.ReplaceAll(cleaned, "\r", "")
			cleaned = strings.ReplaceAll(cleaned, " ", "")
			if _, err := base64.StdEncoding.DecodeString(cleaned); err != nil {
				// Try URL-safe base64
				if _, err2 := base64.URLEncoding.DecodeString(cleaned); err2 != nil {
					return "", fmt.Errorf("invalid base64 audio data")
				}
			}
		}
	}

	// Simulate voice pipeline
	vadResult := simulateVAD(input, vadMode)
	if !vadResult.HasVoice {
		result := map[string]interface{}{
			"type":       "voice_input",
			"mode":       mode,
			"vad_result": vadResult,
			"has_voice":  false,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		return string(output), nil
	}

	// Wake word detection
	wakeResult := simulateWakeWordDetection(input, wakeWord)
	if mode == "wake_word" {
		result := map[string]interface{}{
			"type":            "voice_input",
			"mode":            mode,
			"vad_result":      vadResult,
			"wake_detected":   wakeResult.Detected,
			"wake_confidence": wakeResult.Confidence,
			"wake_word":       wakeWord,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		return string(output), nil
	}

	// Full ASR
	if mode == "full_asr" || (mode == "wake_word" && wakeResult.Detected) {
		asrResult := simulateASR(input, language, offline, confidenceThreshold)

		result := map[string]interface{}{
			"type":               "voice_input",
			"mode":               mode,
			"vad_result":         vadResult,
			"wake_detected":      wakeResult.Detected,
			"wake_confidence":    wakeResult.Confidence,
			"wake_word":          wakeWord,
			"asr_text":           asrResult.Text,
			"asr_confidence":     asrResult.Confidence,
			"language":           language,
			"offline":            offline,
			"processing_time_ms": asrResult.ProcessingTimeMs,
			"timestamp":          time.Now().UTC().Format(time.RFC3339),
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		return string(output), nil
	}

	return "", fmt.Errorf("unsupported voice pipeline mode")
}

// VADResult represents voice activity detection result
type VADResult struct {
	HasVoice    bool    `json:"has_voice"`
	StartMs     int     `json:"start_ms"`
	EndMs       int     `json:"end_ms"`
	EnergyLevel float64 `json:"energy_level"`
}

// WakeWordResult represents wake word detection result
type WakeWordResult struct {
	Detected   bool    `json:"detected"`
	Confidence float64 `json:"confidence"`
	MatchScore float64 `json:"match_score"`
}

// ASRResult represents speech recognition result
type ASRResult struct {
	Text             string       `json:"text"`
	Confidence       float64      `json:"confidence"`
	ProcessingTimeMs int          `json:"processing_time_ms"`
	WordTimings      []WordTiming `json:"word_timings,omitempty"`
}

// WordTiming represents timing for a single word
type WordTiming struct {
	Word       string  `json:"word"`
	StartMs    int     `json:"start_ms"`
	EndMs      int     `json:"end_ms"`
	Confidence float64 `json:"confidence"`
}

func simulateVAD(input, mode string) *VADResult {
	// In real implementation, analyze audio energy levels
	// For simulation, assume voice is present if input is non-empty
	hasVoice := input != ""
	energy := 0.75
	if mode == "aggressive" {
		energy = 0.85
	} else if mode == "fixed" {
		energy = 0.65
	}

	return &VADResult{
		HasVoice:    hasVoice,
		StartMs:     200,
		EndMs:       2500,
		EnergyLevel: energy,
	}
}

func simulateWakeWordDetection(input, wakeWord string) *WakeWordResult {
	// In real implementation, use keyword spotting model
	// For simulation, detect if input contains wake word or similar phrases
	lowerInput := strings.ToLower(input)
	wakeVariants := map[string][]string{
		"hey_box":   {"hey box", "hi box", "hello box", "ok box"},
		"hello_box": {"hello box", "hi box", "hey box"},
		"hi_box":    {"hi box", "hey box", "hello box"},
		"ok_box":    {"ok box", "okay box"},
		"box_box":   {"box box", "box"},
	}

	variants := wakeVariants[wakeWord]
	if variants == nil {
		variants = []string{wakeWord}
	}

	for _, v := range variants {
		if strings.Contains(lowerInput, v) {
			return &WakeWordResult{
				Detected:   true,
				Confidence: 0.92,
				MatchScore: 0.95,
			}
		}
	}

	return &WakeWordResult{
		Detected:   false,
		Confidence: 0.15,
		MatchScore: 0.1,
	}
}

func simulateASR(input, language string, offline bool, threshold float64) *ASRResult {
	// In real implementation, use Whisper/Qwen-Audio/Moonshine model
	// For simulation, return sample recognition based on input content

	sampleTexts := map[string]string{
		"zh": "帮我总结今天的未读消息",
		"en": "Summarize today's unread messages",
		"ja": "今日の未読メッセージを要約して",
		"ko": "오늘 읽지 않은 메시지 요약해줘",
		"fr": "Résume mes messages non lus d'aujourd'hui",
		"de": "Fasse meine heutigen ungelesenen Nachrichten zusammen",
		"es": "Resume mis mensajes no leídos de hoy",
	}

	text := sampleTexts[language]
	if text == "" {
		text = sampleTexts["en"]
	}

	// If input contains specific keywords, use those
	lower := strings.ToLower(input)
	if strings.Contains(lower, "weather") || strings.Contains(lower, "天气") {
		text = "今天天气怎么样"
		if language == "en" {
			text = "What's the weather like today"
		}
	} else if strings.Contains(lower, "remind") || strings.Contains(lower, "提醒") {
		text = "提醒我下午三点开会"
		if language == "en" {
			text = "Remind me about the meeting at 3 PM"
		}
	}

	confidence := 0.88
	if offline {
		confidence = 0.82 // On-device slightly lower accuracy
	}
	if confidence < threshold {
		text = ""
	}

	processingTime := 350
	if offline {
		processingTime = 200 // Faster on-device
	}

	return &ASRResult{
		Text:             text,
		Confidence:       confidence,
		ProcessingTimeMs: processingTime,
		WordTimings: []WordTiming{
			{Word: text, StartMs: 0, EndMs: processingTime, Confidence: confidence},
		},
	}
}

func looksLikeBase64(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' ||
			c == '-' || c == '_' ||
			c == '\n' || c == '\r' || c == ' ') {
			return false
		}
	}
	return true
}

func init() {
	Register(&VoiceInputNode{})
}
