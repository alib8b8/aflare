package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

type Translator struct {
	mu       sync.RWMutex
	lang     string
	messages map[string]string
	fallback map[string]string
}

var (
	instance    *Translator
	initOnce    sync.Once
)

func Init(lang string) {
	initOnce.Do(func() {
		instance = &Translator{}
		instance.load("en") // fallback always English
	})
	instance.SetLanguage(lang)
}

func T(key string, args ...interface{}) string {
	if instance == nil {
		Init("")
	}
	return instance.Translate(key, args...)
}

func HasKey(key string) bool {
	if instance == nil {
		Init("")
	}
	return instance.HasKey(key)
}

func GetLanguage() string {
	if instance == nil {
		return "en"
	}
	return instance.lang
}

func (t *Translator) SetLanguage(lang string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if lang == "" {
		lang = detectLanguage()
	}

	t.lang = lang
	messages, err := loadLocale(lang)
	if err != nil {
		t.messages = t.fallback
		return
	}
	t.messages = messages
}

func (t *Translator) Translate(key string, args ...interface{}) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	msg, ok := t.messages[key]
	if !ok {
		msg, ok = t.fallback[key]
		if !ok {
			return key
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

func (t *Translator) HasKey(key string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, ok := t.messages[key]
	if ok {
		return true
	}
	_, ok = t.fallback[key]
	return ok
}

func (t *Translator) load(lang string) {
	messages, err := loadLocale(lang)
	if err == nil {
		t.fallback = messages
	} else {
		t.fallback = make(map[string]string)
	}
}

func loadLocale(lang string) (map[string]string, error) {
	path := fmt.Sprintf("locales/%s.json", lang)
	data, err := localeFS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func detectLanguage() string {
	// 1. Check LLM_BOX_LANG env var
	if lang := os.Getenv("LLM_BOX_LANG"); lang != "" {
		return normalizeLang(lang)
	}

	// 2. Check LANG env var
	if lang := os.Getenv("LANG"); lang != "" {
		return normalizeLang(lang)
	}

	// 3. Check LANGUAGE env var
	if lang := os.Getenv("LANGUAGE"); lang != "" {
		parts := strings.Split(lang, ":")
		if len(parts) > 0 && parts[0] != "" {
			return normalizeLang(parts[0])
		}
	}

	return "en"
}

func normalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))

	if idx := strings.Index(lang, "."); idx > 0 {
		lang = lang[:idx]
	}

	if idx := strings.Index(lang, "_"); idx > 0 {
		lang = lang[:idx]
	}

	switch lang {
	case "zh", "chinese", "cn", "zh-cn", "zh_cn":
		return "zh"
	case "en", "english":
		return "en"
	case "ru", "russian", "rus", "русский":
		return "ru"
	case "fr", "french", "français", "francais":
		return "fr"
	case "ja", "japanese", "日本語", "nihongo":
		return "ja"
	case "ko", "korean", "한국어", "hangul":
		return "ko"
	case "es", "spanish", "español", "espanol":
		return "es"
	case "ar", "arabic", "عربي", "arab":
		return "ar"
	case "hi", "hindi", "हिन्दी":
		return "hi"
	default:
		return "en"
	}
}

func AvailableLanguages() []string {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return []string{"en"}
	}

	var langs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := entry.Name()[:len(entry.Name())-5]
			langs = append(langs, name)
		}
	}

	if len(langs) == 0 {
		langs = []string{"en"}
	}

	return langs
}
