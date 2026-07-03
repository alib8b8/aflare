package workflow

import (
	"strings"

	"github.com/alib8b8/llm-box/internal/i18n"
)

var llmKeywords = map[string][]string{
	"deepseek": {"deepseek"},
	"coze":     {"coze"},
	"glm":      {"glm", "智谱", "zhipu", "glm-4"},
	"kimi":     {"kimi", "moonshot", "月之暗面"},
	"minimax":  {"minimax", "abab"},
	"qwen":     {"qwen", "通义", "千问", "tongyi"},
	"ima":      {"ima", "ima.copilot", "ima copilot"},
	"xverse":   {"xverse", "x-verse"},
	"yi":       {"yi", "零一万物", "lingyiwanwu"},
	"baichuan": {"baichuan", "百川"},
	"internlm": {"internlm", "书生", "浦语"},
	"mistral":  {"mistral"},
	"mimo":     {"mimo", "xiaomi", "xiaomimimo"},
}

var actionKeywords = map[string][]string{
	"github":   {"github"},
	"summarize": {"summarize", "总结", "摘要", "summarise"},
	"translate": {"translate", "翻译", "translator"},
	"git":       {"git", "commit", "release", "push", "pull"},
	"log":       {"log", "monitor", "日志", "监控"},
}

func containsAny(desc string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(desc, kw) {
			return true
		}
	}
	return false
}

func containsLLMKeyword(desc, provider string) bool {
	keywords, ok := llmKeywords[provider]
	if !ok {
		return false
	}
	return containsAny(desc, keywords)
}

func containsActionKeyword(desc, action string) bool {
	keywords, ok := actionKeywords[action]
	if !ok {
		return false
	}
	return containsAny(desc, keywords)
}

var systemPrompts = map[string]map[string]string{
	"en": {
		"summarize": "You are a helpful assistant that summarizes text concisely.",
		"translate": "You are a translator. Translate the following text to English.",
	},
	"zh": {
		"summarize": "你是一个帮助性助手，能够简明扼要地总结文本。",
		"translate": "你是一个翻译器。请将以下文本翻译成中文。",
	},
	"ru": {
		"summarize": "Вы - помощник, который кратко суммирует текст.",
		"translate": "Вы - переводчик. Переведите следующий текст на русский.",
	},
	"fr": {
		"summarize": "Vous êtes un assistant utile qui résume le texte concisément.",
		"translate": "Vous êtes un traducteur. Traduisez le texte suivant en français.",
	},
	"ja": {
		"summarize": "あなたは、テキストを簡潔に要約する役立つアシスタントです。",
		"translate": "あなたは翻訳者です。以下のテキストを日本語に翻訳してください。",
	},
	"ko": {
		"summarize": "당신은 텍스트를 간결하게 요약하는 유용한 비서입니다.",
		"translate": "당신은 번역가입니다. 다음 텍스트를 한국어로 번역하세요.",
	},
	"es": {
		"summarize": "Eres un asistente útil que resume texto de forma concisa.",
		"translate": "Eres un traductor. Traduce el siguiente texto al español.",
	},
	"ar": {
		"summarize": "أنت مساعد مفيد يلخص النص بإيجاز.",
		"translate": "أنت مترجم. اترجم النص التالي إلى العربية.",
	},
	"hi": {
		"summarize": "आप एक सहायक सहायक हैं जो पाठ को संक्षेप में सारांशित करते हैं।",
		"translate": "आप एक अनुवादक हैं। निम्नलिखित पाठ को हिंदी में अनुवाद करें।",
	},
}

func getSystemPrompt(action string) string {
	lang := i18n.GetLanguage()
	prompts, ok := systemPrompts[lang]
	if !ok {
		prompts = systemPrompts["en"]
	}
	prompt, ok := prompts[action]
	if !ok {
		return ""
	}
	return prompt
}