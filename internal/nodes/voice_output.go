package nodes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var (
	validTTSIEngines = map[string]bool{
		"sensevoice":  true,
		"cosyvoice":   true,
		"fish-speech": true,
		"edge-tts":    true,
		"piper":       true,
		"bark":        true,
	}
	validVoiceStyles = map[string]bool{
		"natural":      true,
		"professional": true,
		"friendly":     true,
		"excited":      true,
		"calm":         true,
		"storytelling": true,
	}
	validTTSOperations = map[string]bool{
		"tts":           true,
		"clone":         true,
		"emotion":       true,
		"multi-speaker": true,
	}
	validTTSOutputFormats = map[string]bool{
		"mp3": true,
		"wav": true,
		"ogg": true,
	}
)

type VoiceOutputNode struct{}

func (n *VoiceOutputNode) Name() string { return "voice_output" }

func (n *VoiceOutputNode) Description() string {
	return "Voice output node with TTS, voice cloning, and speech generation. Supports multiple engines including SenseVoice, CosyVoice, Fish Speech, Edge TTS, Piper, and Bark."
}

func (n *VoiceOutputNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - text to synthesize or reference audio for cloning",
		Output:      "string - JSON with audio_base64, duration, and metadata",
		Params: []ParamSchema{
			{Name: "engine", Type: "string", Description: "TTS engine: sensevoice/cosyvoice/fish-speech/edge-tts/piper/bark (default: sensevoice)", Required: false, Default: "sensevoice"},
			{Name: "operation", Type: "string", Description: "Operation type: tts/clone/emotion/multi-speaker (default: tts)", Required: false, Default: "tts"},
			{Name: "text", Type: "string", Description: "Text to convert to speech (max 4000 chars)", Required: false},
			{Name: "voice", Type: "string", Description: "Voice name (default: default)", Required: false, Default: "default"},
			{Name: "style", Type: "string", Description: "Voice style: natural/professional/friendly/excited/calm/storytelling (default: natural)", Required: false, Default: "natural"},
			{Name: "speed", Type: "float", Description: "Speech speed 0.5-2.0 (default: 1.0)", Required: false, Default: "1.0"},
			{Name: "pitch", Type: "float", Description: "Speech pitch 0.5-2.0 (default: 1.0)", Required: false, Default: "1.0"},
			{Name: "reference_audio", Type: "string", Description: "Reference audio base64 for voice cloning", Required: false},
			{Name: "output_format", Type: "string", Description: "Output format: mp3/wav/ogg (default: mp3)", Required: false, Default: "mp3"},
		},
	}
}

func (n *VoiceOutputNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	engine := getParam(params, "engine", "sensevoice")
	if !validTTSIEngines[engine] {
		return "", fmt.Errorf("invalid engine: %s", engine)
	}

	operation := getParam(params, "operation", "tts")
	if !validTTSOperations[operation] {
		return "", fmt.Errorf("invalid operation: %s", operation)
	}

	text := getParam(params, "text", "")
	if input != "" && text == "" {
		text = input
	}
	if operation != "clone" && len(text) > 4000 {
		return "", fmt.Errorf("text too long (max 4000 chars)")
	}

	voice := getParam(params, "voice", "default")

	style := getParam(params, "style", "natural")
	if !validVoiceStyles[style] {
		return "", fmt.Errorf("invalid style: %s", style)
	}

	speed := parseFloatSafe(getParam(params, "speed", "1.0"), 1.0)
	if speed < 0.5 || speed > 2.0 {
		speed = 1.0
	}

	pitch := parseFloatSafe(getParam(params, "pitch", "1.0"), 1.0)
	if pitch < 0.5 || pitch > 2.0 {
		pitch = 1.0
	}

	referenceAudio := getParam(params, "reference_audio", "")
	if operation == "clone" && referenceAudio == "" {
		referenceAudio = input
	}
	if referenceAudio != "" {
		if len(referenceAudio) > 10*1024*1024 {
			return "", fmt.Errorf("reference_audio too large (max 10MB)")
		}
		cleaned := strings.ReplaceAll(referenceAudio, "\n", "")
		cleaned = strings.ReplaceAll(cleaned, "\r", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		if looksLikeBase64(cleaned) {
			if _, err := base64.StdEncoding.DecodeString(cleaned); err != nil {
				if _, err2 := base64.URLEncoding.DecodeString(cleaned); err2 != nil {
					return "", fmt.Errorf("invalid base64 reference_audio")
				}
			}
		}
	}

	outputFormat := getParam(params, "output_format", "mp3")
	if !validTTSOutputFormats[outputFormat] {
		return "", fmt.Errorf("invalid output_format: %s", outputFormat)
	}

	startTime := time.Now()
	audioBase64, duration := simulateTTSGeneration(text, engine, operation, voice, style, speed, pitch, referenceAudio, outputFormat)
	latency := time.Since(startTime)

	result := map[string]interface{}{
		"engine":       engine,
		"operation":    operation,
		"audio_base64": audioBase64,
		"duration":     duration,
		"format":       outputFormat,
		"voice":        voice,
		"style":        style,
		"latency_ms":   latency.Milliseconds(),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func simulateTTSGeneration(text, engine, operation, voice, style string, speed, pitch float64, referenceAudio, format string) (string, float64) {
	duration := float64(len(text)) * 0.08 / speed
	if duration < 0.5 {
		duration = 0.5
	}

	sampleSize := int(duration * 16000 * 2)
	if sampleSize < 1000 {
		sampleSize = 1000
	}
	if sampleSize > 1024*1024 {
		sampleSize = 1024 * 1024
	}

	sampleBytes := make([]byte, sampleSize)
	for i := range sampleBytes {
		sampleBytes[i] = byte((i * 7) % 256)
	}

	audioBase64 := base64.StdEncoding.EncodeToString(sampleBytes)
	return audioBase64, duration
}

func init() {
	Register(&VoiceOutputNode{})
}
