package main

import (
	"log"
	"strings"

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

// minClauseSplitRunes is the minimum rune count for a sentence to be eligible
// for clause splitting. Shorter sentences are returned as a single chunk.
const minClauseSplitRunes = 12

// minChunkRunes is the minimum rune count for any individual chunk produced by
// splitting. Chunks below this threshold are merged into an adjacent chunk to
// avoid creating fragments too short for natural TTS synthesis.
const minChunkRunes = 5

// splitByClause splits a Japanese sentence into clause-sized chunks suitable
// for low-latency TTS synthesis. Each chunk ends at a natural spoken pause
// point so it can be synthesized independently.
//
// Split rules:
//  1. After 読点 「、」 — always a split point.
//  2. After 接続助詞 (conjunctive particles: て, で, が, けど, から, ので,
//     ながら, し, たり…), unless the immediately following token is a
//     非自立 verb, which signals a compound verbal form
//     (〜ている, 〜てくる, 〜てみる, 〜てください, etc.).
//     Also suppressed when the next token is punctuation (rule 1 handles it).
//  3. Before 接続詞 (sentence-level conjunctions: しかし, でも, だから…).
//
// The sentence is returned as a single-element slice if it contains fewer than
// minClauseSplitRunes runes or if no eligible split points are found.
func splitByClause(sentence string) []string {
	if len([]rune(sentence)) < minClauseSplitRunes {
		return []string{sentence}
	}

	tokens := jpTokenizer.Tokenize(sentence)
	n := len(tokens)
	if n == 0 {
		return []string{sentence}
	}

	// Safe feature accessors for arbitrary indices.
	featOf := func(i int) []string {
		if i < 0 || i >= n {
			return nil
		}
		return tokens[i].Features()
	}
	posOf := func(f []string) string {
		if len(f) < 1 {
			return ""
		}
		return f[0]
	}
	sub1Of := func(f []string) string {
		if len(f) < 2 {
			return ""
		}
		return f[1]
	}

	// splitAfter[i] = true means a chunk boundary falls immediately after token i.
	splitAfter := make([]bool, n)

	for i := 0; i < n; i++ {
		f := featOf(i)
		p := posOf(f)
		s := sub1Of(f)

		switch {
		case p == "記号" && s == "読点":
			// 「、」 — always split after.
			splitAfter[i] = true

		case p == "助詞" && s == "接続助詞":
			nextF := featOf(i + 1)
			nextP := posOf(nextF)
			nextS := sub1Of(nextF)
			// Compound verbal forms (〜ている, 〜てくる, 〜てみる, 〜てください…):
			// the non-independent verb is the continuation, not a new clause.
			if nextP == "動詞" && nextS == "非自立" {
				break
			}
			// Punctuation immediately follows: let rule 1 handle the split there.
			if nextP == "記号" && (nextS == "読点" || nextS == "句点") {
				break
			}
			splitAfter[i] = true

		case p == "接続詞":
			// Split BEFORE a sentence-level conjunction by marking a boundary
			// after the preceding token.
			if i > 0 {
				splitAfter[i-1] = true
			}
		}
	}

	// Build chunks by collecting token surfaces up to each split boundary.
	var chunks []string
	start := 0
	for i := 0; i < n; i++ {
		if !splitAfter[i] {
			continue
		}
		var sb strings.Builder
		for j := start; j <= i; j++ {
			sb.WriteString(tokens[j].Surface)
		}
		if s := sb.String(); s != "" {
			chunks = append(chunks, s)
		}
		start = i + 1
	}
	// Trailing chunk (tokens after the last split point).
	if start < n {
		var sb strings.Builder
		for j := start; j < n; j++ {
			sb.WriteString(tokens[j].Surface)
		}
		if s := sb.String(); s != "" {
			chunks = append(chunks, s)
		}
	}

	if len(chunks) <= 1 {
		return []string{sentence}
	}

	// Merge any chunk shorter than minChunkRunes into an adjacent chunk to
	// avoid TTS fragments that are too short to sound natural.
	return mergeShortChunks(chunks, minChunkRunes)
}

// mergeShortChunks repeatedly merges any chunk shorter than minRunes into its
// neighbor (preferring the previous chunk) until all chunks meet the minimum
// or only one chunk remains.
func mergeShortChunks(chunks []string, minRunes int) []string {
	for {
		merged := false
		for i, c := range chunks {
			if len([]rune(c)) >= minRunes {
				continue
			}
			if len(chunks) == 1 {
				break
			}
			if i == 0 {
				// First chunk: merge forward into next.
				chunks[1] = c + chunks[1]
				chunks = append(chunks[:0], chunks[1:]...)
			} else {
				// Any other chunk: merge backward into previous.
				chunks[i-1] = chunks[i-1] + c
				chunks = append(chunks[:i], chunks[i+1:]...)
			}
			merged = true
			break
		}
		if !merged {
			break
		}
	}
	return chunks
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
