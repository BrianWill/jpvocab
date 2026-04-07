package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- autoFillExamples ---

// katakanaToHiragana converts katakana runes (U+30A1–U+30F6) to their hiragana
// equivalents by subtracting 0x60. All other runes pass through unchanged.
func katakanaToHiragana(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x30A1 && r <= 0x30F6 {
			b.WriteRune(r - 0x60)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestAutoFillExamples_KanjiReadingsConcatenateToWordReading(t *testing.T) {
	// Verifies that the few-shot examples embedded in the prompt are internally
	// consistent: the kanji readings, normalised to hiragana and concatenated,
	// must be a prefix of the word's full reading. The remainder is okurigana
	// (e.g. "べる" in 食べる after 食→た). This catches mistakes like writing
	// ニチ for 日 in 日本語 (にほんご), where ニチ+ホン+ゴ → にちほんご ≠ にほんご.
	for _, ex := range autoFillExamples {
		t.Run(ex.word, func(t *testing.T) {
			var result wordAutoFill
			if err := json.Unmarshal([]byte(ex.result), &result); err != nil {
				t.Fatalf("failed to parse example JSON: %v", err)
			}
			if len(result.Kanji) == 0 {
				return // pure-kana words have no kanji readings to check
			}
			var concat strings.Builder
			for _, k := range result.Kanji {
				concat.WriteString(katakanaToHiragana(k.Reading))
			}
			if !strings.HasPrefix(result.Reading, concat.String()) {
				t.Errorf("kanji readings concatenate to %q, not a prefix of word reading %q",
					concat.String(), result.Reading)
			}
		})
	}
}

var invalidProviderModels = []string{"badformat", "", "noslash"}

// --- autoFillWord ---

func TestAutoFillWord_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := autoFillWord(nil, "食べる", pm)
		if err == nil {
			t.Errorf("autoFillWord(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("autoFillWord(%q): unexpected error: %v", pm, err)
		}
	}
}

// --- rerollMeaning ---

func TestRerollMeaning_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := rerollMeaning(nil, "食べる", "to eat", pm)
		if err == nil {
			t.Errorf("rerollMeaning(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("rerollMeaning(%q): unexpected error: %v", pm, err)
		}
	}
}

// --- rerollExamples ---

func TestRerollExamples_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := rerollExamples(nil, "食べる", pm)
		if err == nil {
			t.Errorf("rerollExamples(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("rerollExamples(%q): unexpected error: %v", pm, err)
		}
	}
}
