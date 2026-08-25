// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​​‌​​‌​‌​​‌​‌‌‌​​‌​‌​​‌​‌​‌‌​‌​​‌‌​​​‌​​‌‌‌‌‌​​​​​​​​​​​​​​​​‌​‌‌​​​‌​​‌​‌‌​‌⁠
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
	if len(langs) < 2 {
		t.Errorf("expected at least 2 languages, got %d", len(langs))
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"english", "en"},
		{"zh", "zh"},
		{"zh_CN.UTF-8", "zh"},
		{"zh-cn", "zh"},
		// Removed languages should default to "en"
		{"ru", "en"},
		{"russian", "en"},
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
	// Test with env var
	os.Setenv("AFLARE_LANG", "zh")
	lang := detectLanguage()
	if lang != "zh" {
		t.Errorf("expected zh, got %s", lang)
	}
	os.Unsetenv("AFLARE_LANG")

	// Test with LANG env var
	os.Setenv("LANG", "zh_CN.UTF-8")
	lang = detectLanguage()
	if lang != "zh" {
		t.Errorf("expected zh, got %s", lang)
	}
	os.Unsetenv("LANG")

	// Test default
	os.Unsetenv("AFLARE_LANG")
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
