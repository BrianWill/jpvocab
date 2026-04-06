//go:build integration

package main

// Integration tests for AI autofill. These make real API calls and are
// excluded from normal test runs. To run them:
//
//	go test -tags integration ./...
//
// ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY, and MISTRAL_API_KEY must be set in the environment
// for the respective subtests to run; others are skipped automatically.

import (
	"slices"
	"strings"
	"testing"
)

// checkAIIntegrationFields verifies that the fields common to every autofill
// response are populated and valid.
func checkAIIntegrationFields(t *testing.T, word string, result *wordAutoFill) {
	t.Helper()
	if result.Reading == "" {
		t.Errorf("%s: reading is empty", word)
	}
	if result.Meaning == "" {
		t.Errorf("%s: meaning is empty", word)
	}
	if !slices.Contains(validPartsOfSpeech, result.PartOfSpeech) {
		t.Errorf("%s: invalid part_of_speech %q", word, result.PartOfSpeech)
	}
}

// checkKanjiReadingsConcat verifies that the kanji readings (normalised to
// hiragana) concatenate to a prefix of the word's full reading.
func checkKanjiReadingsConcat(t *testing.T, word string, result *wordAutoFill) {
	t.Helper()
	if len(result.Kanji) == 0 {
		return
	}
	var concat strings.Builder
	for _, k := range result.Kanji {
		concat.WriteString(katakanaToHiragana(k.Reading))
	}
	if !strings.HasPrefix(result.Reading, concat.String()) {
		t.Errorf("%s: kanji readings concatenate to %q, not a prefix of word reading %q",
			word, concat.String(), result.Reading)
	}
}

func TestAutoFillWord_Integration(t *testing.T) {
	providers := checkAIProviders()

	// Each case targets a specific provider/model and word. The test for
	// gpt-4o-mini + 日本語 directly exercises the prompt fix that prevents
	// にち being returned instead of に for the 日 kanji.
	cases := []struct {
		desc         string
		word         string
		providerModel string
		needsProvider string // "anthropic", "openai", "google", or "mistral"
		extraChecks  func(t *testing.T, result *wordAutoFill)
	}{
		{
			desc:          "OpenAI gpt-4o-mini: 日本語 kanji readings must concatenate to にほんご",
			word:          "日本語",
			providerModel: "openai/gpt-4o-mini",
			needsProvider: "openai",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if len(result.Kanji) != 3 {
					t.Errorf("expected 3 kanji entries for 日本語, got %d", len(result.Kanji))
				}
				if result.Reading != "にほんご" {
					t.Errorf("reading: got %q, want にほんご", result.Reading)
				}
			},
		},
		{
			desc:          "OpenAI gpt-4o-mini: 食べる basic fields",
			word:          "食べる",
			providerModel: "openai/gpt-4o-mini",
			needsProvider: "openai",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if result.PartOfSpeech != "ichidan-verb" {
					t.Errorf("part_of_speech: got %q, want ichidan-verb", result.PartOfSpeech)
				}
			},
		},
		{
			desc:          "Anthropic claude-haiku: 日本語 kanji readings must concatenate to にほんご",
			word:          "日本語",
			providerModel: "anthropic/claude-haiku-4-5-20251001",
			needsProvider: "anthropic",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if len(result.Kanji) != 3 {
					t.Errorf("expected 3 kanji entries for 日本語, got %d", len(result.Kanji))
				}
				if result.Reading != "にほんご" {
					t.Errorf("reading: got %q, want にほんご", result.Reading)
				}
			},
		},
		{
			desc:          "Anthropic claude-haiku: 食べる basic fields",
			word:          "食べる",
			providerModel: "anthropic/claude-haiku-4-5-20251001",
			needsProvider: "anthropic",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if result.PartOfSpeech != "ichidan-verb" {
					t.Errorf("part_of_speech: got %q, want ichidan-verb", result.PartOfSpeech)
				}
			},
		},
		{
			desc:          "Google gemini-2.0-flash: 日本語 kanji readings must concatenate to にほんご",
			word:          "日本語",
			providerModel: "google/gemini-2.0-flash",
			needsProvider: "google",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if len(result.Kanji) != 3 {
					t.Errorf("expected 3 kanji entries for 日本語, got %d", len(result.Kanji))
				}
				if result.Reading != "にほんご" {
					t.Errorf("reading: got %q, want にほんご", result.Reading)
				}
			},
		},
		{
			desc:          "Google gemini-2.0-flash: 食べる basic fields",
			word:          "食べる",
			providerModel: "google/gemini-2.0-flash",
			needsProvider: "google",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if result.PartOfSpeech != "ichidan-verb" {
					t.Errorf("part_of_speech: got %q, want ichidan-verb", result.PartOfSpeech)
				}
			},
		},
		{
			desc:          "Mistral mistral-small: 日本語 kanji readings must concatenate to にほんご",
			word:          "日本語",
			providerModel: "mistral/mistral-small-latest",
			needsProvider: "mistral",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if len(result.Kanji) != 3 {
					t.Errorf("expected 3 kanji entries for 日本語, got %d", len(result.Kanji))
				}
				if result.Reading != "にほんご" {
					t.Errorf("reading: got %q, want にほんご", result.Reading)
				}
			},
		},
		{
			desc:          "Mistral mistral-small: 食べる basic fields",
			word:          "食べる",
			providerModel: "mistral/mistral-small-latest",
			needsProvider: "mistral",
			extraChecks: func(t *testing.T, result *wordAutoFill) {
				if result.PartOfSpeech != "ichidan-verb" {
					t.Errorf("part_of_speech: got %q, want ichidan-verb", result.PartOfSpeech)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.needsProvider == "openai" && !providers.OpenAIAvail {
				t.Skip("OPENAI_API_KEY not set")
			}
			if tc.needsProvider == "anthropic" && !providers.AnthropicAvail {
				t.Skip("ANTHROPIC_API_KEY not set")
			}
			if tc.needsProvider == "google" && !providers.GoogleAvail {
				t.Skip("GOOGLE_API_KEY not set")
			}
			if tc.needsProvider == "mistral" && !providers.MistralAvail {
				t.Skip("MISTRAL_API_KEY not set")
			}

			result, err := autoFillWord(tc.word, tc.providerModel)
			if err != nil {
				t.Fatalf("autoFillWord error: %v", err)
			}

			checkAIIntegrationFields(t, tc.word, result)
			checkKanjiReadingsConcat(t, tc.word, result)
			if tc.extraChecks != nil {
				tc.extraChecks(t, result)
			}
		})
	}
}
