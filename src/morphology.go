package main

import (
	"log"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

var jpTokenizer *tokenizer.Tokenizer

// initTokenizer loads the IPA dictionary and constructs the package-level
// tokenizer. Called once at startup; fatal on failure.
func initTokenizer() {
	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		log.Fatal("init tokenizer:", err)
	}
	jpTokenizer = t
	log.Println("Tokenizer ready (IPA dict)")
}

// toBaseForm normalises a Japanese word to its dictionary base form using the
// IPA dictionary. It tokenises the input and returns the 原形 (base form) of
// the first substantive morpheme — skipping particles, symbols, and fillers.
//
// Examples:
//
//	食べた   → 食べる   (past-tense verb → dictionary form)
//	走っている → 走る     (te-iru form → dictionary form)
//	高かった  → 高い     (past-tense i-adjective → dictionary form)
//	猫      → 猫      (noun, already base form)
//
// Falls back to the original word if no base form can be determined.
func toBaseForm(word string) string {
	tokens := jpTokenizer.Tokenize(word)
	for _, tok := range tokens {
		f := tok.Features()
		if len(f) < 7 {
			continue
		}
		// IPAdic feature layout: [POS, sub1, sub2, sub3, conjType, conjForm, baseForm, reading, pronunciation]
		pos := f[0]
		switch pos {
		case "助詞", "助動詞", "記号", "フィラー", "その他":
			continue
		}
		base := f[6]
		if base != "" && base != "*" {
			return base
		}
	}
	return word
}

// extractContentWords tokenises arbitrary Japanese text and returns the unique
// base forms of content words — nouns, verbs, i-adjectives, and adverbs —
// in first-seen order. Grammatical words (particles, auxiliaries, numerals,
// suffixes, pronouns, non-independent nominals, etc.) are discarded.
func extractContentWords(text string) []string {
	tokens := jpTokenizer.Tokenize(text)
	seen := make(map[string]bool)
	var words []string
	for _, tok := range tokens {
		f := tok.Features()
		if len(f) < 7 {
			continue
		}
		// IPAdic feature layout: [POS, sub1, sub2, sub3, conjType, conjForm, baseForm, reading, pronunciation]
		pos := f[0]
		sub1 := f[1]

		include := false
		switch pos {
		case "名詞":
			// Skip non-vocabulary noun sub-types
			switch sub1 {
			case "数",         // numerals (一, 二, 三…)
				"接尾",         // noun suffixes (〜さ, 〜み…)
				"非自立",       // non-independent nominals (こと, の used as nominalizers)
				"代名詞",       // pronouns (これ, それ, あれ…)
				"接続詞的",     // conjunctive nouns
				"動詞非自立的", // non-independent verb-like nominals
				"特殊":         // special types
				// skip
			default:
				include = true
			}
		case "動詞":
			// Skip auxiliary/suffix verb uses (ている → いる, てくる → くる)
			if sub1 != "非自立" && sub1 != "接尾" {
				include = true
			}
		case "形容詞":
			if sub1 != "非自立" {
				include = true
			}
		case "副詞":
			include = true
		}

		if !include {
			continue
		}

		base := f[6]
		if base == "" || base == "*" {
			base = tok.Surface
		}
		if base == "" || base == "*" {
			continue
		}

		if !seen[base] && isJapaneseWord(base) {
			seen[base] = true
			words = append(words, base)
		}
	}
	return words
}

// isJapaneseWord reports whether s contains at least one hiragana, katakana,
// or CJK unified ideograph (kanji) character. Used to reject romaji, numbers,
// and other non-Japanese tokens that Kagome may surface as 名詞,一般.
func isJapaneseWord(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x309F) || // hiragana
			(r >= 0x30A0 && r <= 0x30FF) || // katakana
			(r >= 0x4E00 && r <= 0x9FFF) || // CJK unified ideographs
			(r >= 0x3400 && r <= 0x4DBF) { // CJK extension A
			return true
		}
	}
	return false
}
