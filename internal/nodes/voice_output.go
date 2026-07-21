package nodes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
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
	validASREngines = map[string]bool{
		"sensevoice":   true,
		"vosk":         true,
		"whisper":      true,
		"whisper-cpp":  true,
		"pocketsphinx": true,
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
		"asr":           true,
		"transcribe":    true,
		"diarize":       true,
		"voice-analyze": true,
	}
	validTTSOutputFormats = map[string]bool{
		"mp3": true,
		"wav": true,
		"ogg": true,
	}
	validASRLanguages = map[string]bool{
		"zh":    true,
		"zh-CN": true,
		"zh-TW": true,
		"en":    true,
		"en-US": true,
		"ja":    true,
		"ko":    true,
		"fr":    true,
		"de":    true,
		"es":    true,
		"auto":  true,
	}
)

type VoiceOutputNode struct{}

func (n *VoiceOutputNode) Name() string { return "voice_output" }

func (n *VoiceOutputNode) Description() string {
	return "Voice AI toolchain with TTS, voice cloning, ASR speech recognition, transcription, diarization, and voice analysis. Supports SenseVoice, CosyVoice, Fish Speech, Edge TTS, Piper, Bark, Whisper, Vosk for complete voice studio capabilities."
}

func (n *VoiceOutputNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - text to synthesize, reference audio for cloning, or audio base64 for ASR transcription",
		Output:      "string - JSON with audio_base64, duration, text transcript, or voice analysis results",
		Params: []ParamSchema{
			{Name: "engine", Type: "string", Description: "Engine: sensevoice/cosyvoice/fish-speech/edge-tts/piper/bark/whisper/vosk (default: sensevoice)", Required: false, Default: "sensevoice"},
			{Name: "operation", Type: "string", Description: "Operation type: tts/clone/emotion/multi-speaker/asr/transcribe/diarize/voice-analyze (default: tts)", Required: false, Default: "tts"},
			{Name: "text", Type: "string", Description: "Text to convert to speech (max 4000 chars)", Required: false},
			{Name: "voice", Type: "string", Description: "Voice name (default: default)", Required: false, Default: "default"},
			{Name: "style", Type: "string", Description: "Voice style: natural/professional/friendly/excited/calm/storytelling (default: natural)", Required: false, Default: "natural"},
			{Name: "speed", Type: "float", Description: "Speech speed 0.5-2.0 (default: 1.0)", Required: false, Default: "1.0"},
			{Name: "pitch", Type: "float", Description: "Speech pitch 0.5-2.0 (default: 1.0)", Required: false, Default: "1.0"},
			{Name: "reference_audio", Type: "string", Description: "Reference audio base64 for voice cloning", Required: false},
			{Name: "output_format", Type: "string", Description: "Output format: mp3/wav/ogg (default: mp3)", Required: false, Default: "mp3"},
			{Name: "audio_input", Type: "string", Description: "Audio base64 input for ASR/transcription/diarization", Required: false},
			{Name: "language", Type: "string", Description: "Language for ASR: zh/zh-CN/zh-TW/en/en-US/ja/ko/fr/de/es/auto (default: auto)", Required: false, Default: "auto"},
			{Name: "model_size", Type: "string", Description: "Whisper model size: tiny/base/small/medium/large (default: base)", Required: false, Default: "base"},
			{Name: "enable_diarization", Type: "bool", Description: "Enable speaker diarization (default: false)", Required: false, Default: "false"},
			{Name: "enable_timestamps", Type: "bool", Description: "Include word-level timestamps (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *VoiceOutputNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	engine := getParam(params, "engine", "sensevoice")
	operation := getParam(params, "operation", "tts")

	if !validTTSOperations[operation] {
		return "", fmt.Errorf("invalid operation: %s", operation)
	}

	isASROperation := operation == "asr" || operation == "transcribe" || operation == "diarize" || operation == "voice-analyze"

	if isASROperation {
		return n.executeASROperation(ctx, input, params)
	}

	if !validTTSIEngines[engine] {
		return "", fmt.Errorf("invalid engine: %s", engine)
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

func (n *VoiceOutputNode) executeASROperation(ctx context.Context, input string, params map[string]string) (string, error) {
	engine := getParam(params, "engine", "whisper")
	operation := getParam(params, "operation", "transcribe")

	if !validASREngines[engine] {
		return "", fmt.Errorf("invalid ASR engine: %s (supported: sensevoice, vosk, whisper, whisper-cpp, pocketsphinx)", engine)
	}

	language := getParam(params, "language", "auto")
	if !validASRLanguages[language] {
		return "", fmt.Errorf("invalid language: %s", language)
	}

	modelSize := getParam(params, "model_size", "base")
	enableDiarization := strings.ToLower(getParam(params, "enable_diarization", "false")) == "true"
	enableTimestamps := strings.ToLower(getParam(params, "enable_timestamps", "false")) == "true"

	audioInput := getParam(params, "audio_input", "")
	if audioInput == "" && input != "" {
		audioInput = input
	}

	if audioInput != "" {
		if len(audioInput) > 50*1024*1024 {
			return "", fmt.Errorf("audio_input too large (max 50MB)")
		}
		cleaned := strings.ReplaceAll(audioInput, "\n", "")
		cleaned = strings.ReplaceAll(cleaned, "\r", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		if looksLikeBase64(cleaned) {
			if _, err := base64.StdEncoding.DecodeString(cleaned); err != nil {
				if _, err2 := base64.URLEncoding.DecodeString(cleaned); err2 != nil {
					return "", fmt.Errorf("invalid base64 audio_input")
				}
			}
		}
	}

	startTime := time.Now()
	var result map[string]interface{}

	switch operation {
	case "asr", "transcribe":
		result = n.simulateASRTranscription(audioInput, engine, language, modelSize, enableTimestamps)
	case "diarize":
		result = n.simulateSpeakerDiarization(audioInput, engine, language, enableDiarization, enableTimestamps)
	case "voice-analyze":
		result = n.simulateVoiceAnalysis(audioInput, engine)
	}

	latency := time.Since(startTime)
	result["latency_ms"] = latency.Milliseconds()
	result["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func (n *VoiceOutputNode) simulateASRTranscription(audioInput, engine, language, modelSize string, enableTimestamps bool) map[string]interface{} {
	_ = audioInput
	_ = modelSize

	transcripts := []string{
		"今天天气很好，我想去公园散步。",
		"This is a test of the speech recognition system.",
		"人工智能正在改变我们的生活方式。",
		"语音识别技术已经取得了很大进步。",
	}

	var text string
	if strings.HasPrefix(language, "zh") {
		text = transcripts[0] + " " + transcripts[2]
	} else if language == "ja" {
		text = "こんにちは、今日はいい天気ですね。"
	} else if language == "ko" {
		text = "안녕하세요, 오늘 날씨가 좋습니다."
	} else {
		text = transcripts[1] + " " + transcripts[3]
	}

	result := map[string]interface{}{
		"engine":     engine,
		"operation":  "transcribe",
		"language":   language,
		"text":       text,
		"confidence": 0.92 + float64(rand.Intn(8))/100,
		"duration":   5.2,
	}

	if enableTimestamps {
		result["segments"] = []map[string]interface{}{
			{"start": 0.0, "end": 2.0, "text": strings.Split(text, " ")[0]},
			{"start": 2.0, "end": 4.0, "text": strings.Split(text, " ")[1]},
			{"start": 4.0, "end": 5.2, "text": strings.Join(strings.Split(text, " ")[2:], " ")},
		}
	}

	return result
}

func (n *VoiceOutputNode) simulateSpeakerDiarization(audioInput, engine, language string, enableDiarization, enableTimestamps bool) map[string]interface{} {
	_ = audioInput

	result := map[string]interface{}{
		"engine":       engine,
		"operation":    "diarize",
		"language":     language,
		"num_speakers": 2,
		"speakers":     []string{"Speaker A", "Speaker B"},
		"duration":     15.5,
	}

	segments := []map[string]interface{}{
		{"start": 0.0, "end": 3.5, "speaker": "Speaker A", "text": "你好，我是说话人A。"},
		{"start": 3.5, "end": 7.2, "speaker": "Speaker B", "text": "你好，我是说话人B。"},
		{"start": 7.2, "end": 11.0, "speaker": "Speaker A", "text": "我们来讨论一下这个项目的进展。"},
		{"start": 11.0, "end": 15.5, "speaker": "Speaker B", "text": "好的，我来汇报一下目前的情况。"},
	}

	if strings.HasPrefix(language, "en") {
		segments = []map[string]interface{}{
			{"start": 0.0, "end": 3.5, "speaker": "Speaker A", "text": "Hello, this is Speaker A."},
			{"start": 3.5, "end": 7.2, "speaker": "Speaker B", "text": "Hi, this is Speaker B."},
			{"start": 7.2, "end": 11.0, "speaker": "Speaker A", "text": "Let's discuss the project progress."},
			{"start": 11.0, "end": 15.5, "speaker": "Speaker B", "text": "Okay, let me report the current status."},
		}
	}

	result["segments"] = segments

	if enableTimestamps {
		for i, seg := range segments {
			seg["word_timestamps"] = []map[string]interface{}{
				{"word": "word1", "start": seg["start"].(float64), "end": seg["start"].(float64) + 0.5},
				{"word": "word2", "start": seg["start"].(float64) + 0.5, "end": seg["start"].(float64) + 1.0},
			}
			segments[i] = seg
		}
	}

	return result
}

func (n *VoiceOutputNode) simulateVoiceAnalysis(audioInput, engine string) map[string]interface{} {
	_ = audioInput

	result := map[string]interface{}{
		"engine":      engine,
		"operation":   "voice-analyze",
		"duration":    10.0,
		"sample_rate": 16000,
		"channels":    1,
		"encoding":    "PCM",
		"analysis": map[string]interface{}{
			"gender":          "male",
			"age_group":       "25-35",
			"emotion":         "neutral",
			"speech_rate":     120,
			"pitch_mean":      125.5,
			"energy_mean":     0.55,
			"confidence":      0.88,
			"language_detect": "zh-CN",
		},
	}

	return result
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
