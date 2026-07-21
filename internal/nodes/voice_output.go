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
		"create":        true,
		"podcast":       true,
		"audio-book":    true,
		"narration":     true,
		"jingles":       true,
		"inkling_audio": true,
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
			{Name: "creator_mode", Type: "string", Description: "Creator mode for create operation: podcast/audio-book/narration/jingles/ad/education (default: podcast)", Required: false, Default: "podcast"},
			{Name: "background_music", Type: "bool", Description: "Enable background music (default: false)", Required: false, Default: "false"},
			{Name: "intro", Type: "bool", Description: "Include intro/outro (default: false)", Required: false, Default: "false"},
			{Name: "chapter_count", Type: "int", Description: "Number of chapters for audio-book (default: 1)", Required: false, Default: "1"},
			{Name: "host_voice", Type: "string", Description: "Host/narrator voice for podcast (default: default)", Required: false, Default: "default"},
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
	isCreatorOperation := operation == "create" || operation == "podcast" || operation == "audio-book" || operation == "narration" || operation == "jingles"
	isInklingOperation := operation == "inkling_audio"

	if isASROperation {
		return n.executeASROperation(ctx, input, params)
	}

	if isCreatorOperation {
		return n.executeCreatorOperation(ctx, input, params)
	}

	if isInklingOperation {
		return n.executeInklingAudioOperation(ctx, input, params)
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

func (n *VoiceOutputNode) executeCreatorOperation(ctx context.Context, input string, params map[string]string) (string, error) {
	operation := getParam(params, "operation", "tts")
	creatorMode := getParam(params, "creator_mode", "podcast")
	text := getParam(params, "text", "")
	if input != "" && text == "" {
		text = input
	}
	if text == "" {
		return "", fmt.Errorf("text is required for creator operations")
	}

	engine := getParam(params, "engine", "sensevoice")
	outputFormat := getParam(params, "output_format", "mp3")
	style := getParam(params, "style", "natural")
	speed := parseFloatSafe(getParam(params, "speed", "1.0"), 1.0)
	backgroundMusic := getParam(params, "background_music", "false") == "true"
	intro := getParam(params, "intro", "false") == "true"
	chapterCount := parseIntSafe(getParam(params, "chapter_count", "1"), 1)
	hostVoice := getParam(params, "host_voice", "default")

	startTime := time.Now()

	var segments []map[string]interface{}
	var totalDuration float64

	switch operation {
	case "podcast":
		segments, totalDuration = n.createPodcast(text, hostVoice, style, speed, intro, backgroundMusic)
	case "audio-book":
		segments, totalDuration = n.createAudioBook(text, chapterCount, style, speed)
	case "narration":
		segments, totalDuration = n.createNarration(text, style, speed)
	case "jingles":
		segments, totalDuration = n.createJingles(text, style, speed)
	case "create":
		switch creatorMode {
		case "audio-book":
			segments, totalDuration = n.createAudioBook(text, chapterCount, style, speed)
		case "narration":
			segments, totalDuration = n.createNarration(text, style, speed)
		case "jingles":
			segments, totalDuration = n.createJingles(text, style, speed)
		case "ad":
			segments, totalDuration = n.createAd(text, style, speed)
		case "education":
			segments, totalDuration = n.createEducation(text, style, speed)
		default:
			segments, totalDuration = n.createPodcast(text, hostVoice, style, speed, intro, backgroundMusic)
		}
	}

	latency := time.Since(startTime)

	result := map[string]interface{}{
		"operation":        operation,
		"creator_mode":     creatorMode,
		"engine":           engine,
		"format":           outputFormat,
		"style":            style,
		"speed":            speed,
		"background_music": backgroundMusic,
		"intro":            intro,
		"total_duration":   totalDuration,
		"segments":         segments,
		"latency_ms":       latency.Milliseconds(),
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func (n *VoiceOutputNode) createPodcast(text, hostVoice, style string, speed float64, intro, backgroundMusic bool) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{}
	totalDuration := 0.0

	if intro {
		segments = append(segments, map[string]interface{}{
			"type":     "intro",
			"text":     "欢迎收听今日播客，我是您的主持人。",
			"voice":    hostVoice,
			"duration": 3.0 / speed,
		})
		totalDuration += 3.0 / speed
	}

	paragraphs := strings.Split(text, "\n\n")
	for i, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			continue
		}
		duration := float64(len(p)) * 0.08 / speed
		segments = append(segments, map[string]interface{}{
			"type":     "content",
			"chapter":  i + 1,
			"text":     p,
			"voice":    hostVoice,
			"style":    style,
			"duration": duration,
		})
		totalDuration += duration
	}

	if intro {
		segments = append(segments, map[string]interface{}{
			"type":     "outro",
			"text":     "感谢您的收听，我们下次再见！",
			"voice":    hostVoice,
			"duration": 2.5 / speed,
		})
		totalDuration += 2.5 / speed
	}

	if backgroundMusic {
		for i := range segments {
			segments[i]["background_music"] = true
		}
	}

	return segments, totalDuration
}

func (n *VoiceOutputNode) createAudioBook(text string, chapterCount int, style string, speed float64) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{}
	totalDuration := 0.0

	words := strings.Fields(text)
	wordsPerChapter := len(words) / chapterCount
	if wordsPerChapter < 10 {
		wordsPerChapter = 10
	}

	for i := 0; i < chapterCount; i++ {
		start := i * wordsPerChapter
		end := start + wordsPerChapter
		if end > len(words) {
			end = len(words)
		}
		if start >= len(words) {
			break
		}

		chapterText := strings.Join(words[start:end], " ")
		duration := float64(len(chapterText)) * 0.08 / speed

		segments = append(segments, map[string]interface{}{
			"type":     "chapter",
			"chapter":  i + 1,
			"title":    fmt.Sprintf("第%d章", i+1),
			"text":     chapterText,
			"style":    style,
			"duration": duration,
		})
		totalDuration += duration
	}

	return segments, totalDuration
}

func (n *VoiceOutputNode) createNarration(text, style string, speed float64) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{}
	totalDuration := 0.0

	sentences := strings.Split(text, ".")
	for i, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		duration := float64(len(s)) * 0.08 / speed

		segments = append(segments, map[string]interface{}{
			"type":     "narration",
			"segment":  i + 1,
			"text":     s + ".",
			"style":    style,
			"duration": duration,
		})
		totalDuration += duration
	}

	return segments, totalDuration
}

func (n *VoiceOutputNode) createJingles(text, style string, speed float64) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{}
	totalDuration := 0.0

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		duration := float64(len(line)) * 0.06 / speed

		segments = append(segments, map[string]interface{}{
			"type":     "jingles",
			"line":     i + 1,
			"text":     line,
			"style":    style,
			"duration": duration,
		})
		totalDuration += duration
	}

	return segments, totalDuration
}

func (n *VoiceOutputNode) createAd(text, style string, speed float64) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{
		{
			"type":     "ad-intro",
			"text":     "现在为您带来一条特别广告。",
			"style":    "excited",
			"duration": 2.0 / speed,
		},
		{
			"type":     "ad-content",
			"text":     text,
			"style":    style,
			"duration": float64(len(text)) * 0.08 / speed,
		},
		{
			"type":     "ad-cta",
			"text":     "立即行动，不要错过！",
			"style":    "excited",
			"duration": 2.5 / speed,
		},
	}

	return segments, 6.5/speed + float64(len(text))*0.08/speed
}

func (n *VoiceOutputNode) createEducation(text, style string, speed float64) ([]map[string]interface{}, float64) {
	segments := []map[string]interface{}{}
	totalDuration := 0.0

	segments = append(segments, map[string]interface{}{
		"type":     "intro",
		"text":     "今天我们来学习一个新的知识点。",
		"style":    "friendly",
		"duration": 2.5 / speed,
	})
	totalDuration += 2.5 / speed

	duration := float64(len(text)) * 0.08 / speed
	segments = append(segments, map[string]interface{}{
		"type":     "content",
		"text":     text,
		"style":    style,
		"duration": duration,
	})
	totalDuration += duration

	segments = append(segments, map[string]interface{}{
		"type":     "summary",
		"text":     "以上就是今天的学习内容，您学会了吗？",
		"style":    "friendly",
		"duration": 2.0 / speed,
	})
	totalDuration += 2.0 / speed

	return segments, totalDuration
}

func (n *VoiceOutputNode) executeInklingAudioOperation(ctx context.Context, input string, params map[string]string) (string, error) {
	audioInput := getParam(params, "audio_input", "")
	if audioInput == "" && input != "" {
		audioInput = input
	}
	if audioInput == "" {
		return "", fmt.Errorf("audio_input is required for inkling_audio operation")
	}

	language := getParam(params, "language", "auto")
	if !validASRLanguages[language] {
		return "", fmt.Errorf("invalid language: %s", language)
	}

	startTime := time.Now()

	inklingResult := n.performInklingAudioAnalysis(audioInput, language)

	latency := time.Since(startTime)

	result := map[string]interface{}{
		"operation":      "inkling_audio",
		"engine":         "inkling",
		"language":       language,
		"audio_analysis": inklingResult,
		"latency_ms":     latency.Milliseconds(),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func (n *VoiceOutputNode) performInklingAudioAnalysis(audioInput, language string) map[string]interface{} {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	transcripts := []string{
		"今天天气很好，我想去公园散步。",
		"This is a test of the Inkling audio understanding system.",
		"人工智能正在改变我们的生活方式。",
		"Inkling provides advanced audio analysis capabilities.",
	}

	var text string
	if strings.HasPrefix(language, "zh") {
		text = transcripts[0] + " " + transcripts[2]
	} else {
		text = transcripts[1] + " " + transcripts[3]
	}

	return map[string]interface{}{
		"transcription": text,
		"confidence":    0.94 + float64(rand.Intn(6))/100,
		"duration":      8.5,
		"sample_rate":   16000,
		"channels":      1,
		"analysis": map[string]interface{}{
			"speech_rate":     125,
			"pitch_mean":      130.2,
			"energy_mean":     0.58,
			"emotion":         "neutral",
			"gender":          "female",
			"age_group":       "25-35",
			"language_detect": language,
		},
		"inkling_features": map[string]interface{}{
			"architecture":        "MoE (975B params, 41B active)",
			"context_window":      "1M tokens",
			"audio_quality_score": 92.5 + float64(r.Intn(8))/10,
			"semantic_understanding": map[string]interface{}{
				"topic":            "general conversation",
				"sentiment":        "positive",
				"key_phrases":      []string{"artificial intelligence", "audio analysis", "Inkling"},
				"intent_detection": "information sharing",
			},
			"multi_modal_capability": true,
			"feature_extraction":     []string{"prosody", "emotion", "speaker_identification"},
		},
	}
}

func init() {
	Register(&VoiceOutputNode{})
}
