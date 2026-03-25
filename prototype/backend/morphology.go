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
