package i18n

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	Init("en")
	if GetLanguage() != "en" {
		t.Errorf("expected language en, got %s", GetLanguage())
	}
}

func TestInitEmpty(t *testing.T) {
	Init("")
	// Should default to something (detected or "en")
	lang := GetLanguage()
	if lang == "" {
		t.Error("expected non-empty language")
	}
}

func TestTranslate(t *testing.T) {
	Init("en")
	// Test a known key
	msg := T("tui.completed")
	if msg != "Completed" {
		t.Errorf("expected 'Completed', got '%s'", msg)
	}
}

func TestTranslateMissing(t *testing.T) {
	Init("en")
	msg := T("nonexistent.key")
	if msg != "nonexistent.key" {
		t.Errorf("expected key name as fallback, got '%s'", msg)
	}
}

func TestTranslateWithArgs(t *testing.T) {
	Init("en")
	// Test that format args work if any key uses them
	msg := T("tui.completed")
	if msg == "" {
		t.Error("expected non-empty translation")
	}
}

func TestHasKey(t *testing.T) {
	Init("en")
	if !HasKey("tui.completed") {
		t.Error("expected HasKey to return true for existing key")
	}
	if HasKey("nonexistent.key") {
		t.Error("expected HasKey to return false for missing key")
	}
}

func TestSetLanguage(t *testing.T) {
	Init("en")
	instance.Load().SetLanguage("en")
	if GetLanguage() != "en" {
		t.Errorf("expected language en, got %s", GetLanguage())
	}
}

func TestSetLanguageInvalid(t *testing.T) {
	Init("en")
	instance.Load().SetLanguage("invalid_lang")
	// Should fallback to English messages
	msg := T("tui.completed")
	if msg == "" {
		t.Error("expected non-empty fallback translation")
	}
}

func TestAvailableLanguages(t *testing.T) {
	langs := AvailableLanguages()
	if len(langs) != 1 {
		t.Errorf("expected 1 language, got %d", len(langs))
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"english", "en"},
		{"zh", "en"},
		{"zh_CN.UTF-8", "en"},
		{"zh-cn", "en"},
		{"ru", "en"},
		{"russian", "en"},
		// Removed languages should default to "en"
		{"fr", "en"},
		{"french", "en"},
		{"ja", "en"},
		{"japanese", "en"},
		{"ko", "en"},
		{"korean", "en"},
		{"es", "en"},
		{"spanish", "en"},
		{"ar", "en"},
		{"arabic", "en"},
		{"hi", "en"},
		{"hindi", "en"},
		{"unknown", "en"},
		{"", "en"},
	}

	for _, tt := range tests {
		result := normalizeLang(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeLang(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	// Test with env var (all fallback to en now)
	os.Setenv("LLM_BOX_LANG", "zh")
	lang := detectLanguage()
	if lang != "en" {
		t.Errorf("expected en, got %s", lang)
	}
	os.Unsetenv("LLM_BOX_LANG")

	// Test with LANG env var (all fallback to en now)
	os.Setenv("LANG", "ru_RU.UTF-8")
	lang = detectLanguage()
	if lang != "en" {
		t.Errorf("expected en, got %s", lang)
	}
	os.Unsetenv("LANG")

	// Test default
	os.Unsetenv("LLM_BOX_LANG")
	os.Unsetenv("LANG")
	os.Unsetenv("LANGUAGE")
	lang = detectLanguage()
	if lang != "en" {
		t.Errorf("expected en as default, got %s", lang)
	}
}

func TestAllLocalesHaveSameKeys(t *testing.T) {
	Init("en")
	enKeys := instance.Load().fallback
	langs := AvailableLanguages()

	for _, lang := range langs {
		if lang == "en" {
			continue
		}
		instance.Load().SetLanguage(lang)
		missingCount := 0
		for key := range enKeys {
			if !instance.Load().HasKey(key) {
				missingCount++
			}
		}
		if missingCount > 0 {
			t.Errorf("language %s is missing %d keys from English baseline", lang, missingCount)
		}
		instance.Load().SetLanguage("en")
	}
}
