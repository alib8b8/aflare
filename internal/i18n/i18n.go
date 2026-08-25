// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌‌‌​​​​‌​‌​​‌​‌‌​‌​​‌​‌‌‌‌​‌​​‌‌‌​​​​​​‌‌​‌‌‌​​​​​​​​​​​​​​​​​‌​‌​​​​‌‌‌​​‌​‌⁠
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

package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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
	instance atomic.Pointer[Translator]
	initOnce sync.Once
	initMu   sync.Mutex
)

// Init 初始化全局 Translator 实例并设置语言；lang 为空时自动探测。
// 仅首次调用会创建实例，后续调用仅切换语言。
func Init(lang string) {
	initOnce.Do(func() {
		t := &Translator{}
		t.load("en") // fallback always English
		instance.Store(t)
	})
	initMu.Lock()
	defer initMu.Unlock()
	t := instance.Load()
	if t != nil {
		t.SetLanguage(lang)
	}
}

// T 翻译指定 key 的消息并用 args 进行格式化；未初始化或未命中时返回 key 本身。
func T(key string, args ...interface{}) string {
	t := instance.Load()
	if t == nil {
		Init("")
		t = instance.Load()
	}
	if t == nil {
		return key
	}
	return t.Translate(key, args...)
}

// HasKey 判断当前语言或 fallback 中是否存在指定 key。
func HasKey(key string) bool {
	t := instance.Load()
	if t == nil {
		Init("")
		t = instance.Load()
	}
	if t == nil {
		return false
	}
	return t.HasKey(key)
}

// GetLanguage 返回当前 Translator 使用的语言代码，未初始化时返回 "en"。
func GetLanguage() string {
	t := instance.Load()
	if t == nil {
		return "en"
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lang
}

// SetLanguage 切换当前语言并加载对应 locale；lang 为空时自动探测。
// 加载失败时回退到 fallback（English）。
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

// Translate 翻译 key 并用 args 格式化；当前语言未命中时回退到 fallback，仍缺失则返回 key。
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

// HasKey 判断当前语言或 fallback 中是否包含指定 key。
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
	// 1. Check AFLARE_LANG env var
	if lang := os.Getenv("AFLARE_LANG"); lang != "" {
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

	if idx := strings.Index(lang, "-"); idx > 0 {
		lang = lang[:idx]
	}

	switch lang {
	case "en", "english":
		return "en"
	case "zh", "chinese", "中文":
		return "zh"
	default:
		return "en"
	}
}

// AvailableLanguages 返回内嵌 locale 目录下可用的语言代码列表。
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
