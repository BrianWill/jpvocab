package main

import (
	"os"
	"slices"
	"testing"
)

func TestMain(m *testing.M) {
	initTokenizer()
	os.Exit(m.Run())
}

// --- toBaseForm ---

func TestToBaseForm(t *testing.T) {
	cases := []struct {
		desc  string
		input string
		want  string
	}{
		{
			desc:  "past-tense verb normalised to dictionary form",
			input: "食べた",
			want:  "食べる",
		},
		{
			desc:  "te-iru progressive normalised to dictionary form",
			input: "走っている",
			want:  "走る",
		},
		{
			desc:  "past-tense i-adjective normalised",
			input: "高かった",
			want:  "高い",
		},
		{
			desc:  "noun already in base form returned unchanged",
			input: "猫",
			want:  "猫",
		},
		{
			desc:  "i-adjective already in base form returned unchanged",
			input: "美しい",
			want:  "美しい",
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := toBaseForm(tc.input)
			if got != tc.want {
				t.Errorf("toBaseForm(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- extractContentWords ---


func TestExtractContentWords_BasicExtraction(t *testing.T) {
	// 猫が魚を食べる — noun + particle + noun + particle + verb
	got := extractContentWords("猫が魚を食べる")
	for _, want := range []string{"猫", "魚", "食べる"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in result %v", want, got)
		}
	}
}

func TestExtractContentWords_VerbNormalisation(t *testing.T) {
	// Past-tense form should come back as dictionary form.
	got := extractContentWords("走った")
	if !slices.Contains(got, "走る") {
		t.Errorf("expected 走る in result %v", got)
	}
	if slices.Contains(got, "走った") {
		t.Errorf("did not expect inflected form 走った in result %v", got)
	}
}

func TestExtractContentWords_NonIndependentVerbExcluded(t *testing.T) {
	// いる in 走っている is tagged 動詞,非自立 — it should be dropped.
	got := extractContentWords("走っている")
	if !slices.Contains(got, "走る") {
		t.Errorf("expected 走る in result %v", got)
	}
	if slices.Contains(got, "いる") {
		t.Errorf("did not expect auxiliary いる in result %v", got)
	}
}

func TestExtractContentWords_PronounExcluded(t *testing.T) {
	// これ is 名詞,代名詞 — should be filtered out.
	got := extractContentWords("これは猫だ")
	if slices.Contains(got, "これ") {
		t.Errorf("did not expect pronoun これ in result %v", got)
	}
	if !slices.Contains(got, "猫") {
		t.Errorf("expected noun 猫 in result %v", got)
	}
}

func TestExtractContentWords_NumeralExcluded(t *testing.T) {
	// 三 is 名詞,数 — should be filtered out.
	got := extractContentWords("三匹の猫")
	if slices.Contains(got, "三") {
		t.Errorf("did not expect numeral 三 in result %v", got)
	}
	if !slices.Contains(got, "猫") {
		t.Errorf("expected noun 猫 in result %v", got)
	}
}

func TestExtractContentWords_NaAdjectiveIncluded(t *testing.T) {
	// 静か is tagged 名詞,形容動詞語幹 in IPAdic — should be included as a noun.
	got := extractContentWords("静かな公園")
	if !slices.Contains(got, "静か") {
		t.Errorf("expected na-adjective 静か in result %v", got)
	}
	if !slices.Contains(got, "公園") {
		t.Errorf("expected noun 公園 in result %v", got)
	}
}

func TestExtractContentWords_Deduplication(t *testing.T) {
	// The same word appearing twice should produce a single entry.
	got := extractContentWords("猫と猫")
	count := 0
	for _, w := range got {
		if w == "猫" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 猫 exactly once, got %d times in %v", count, got)
	}
}

func TestExtractContentWords_ParticlesExcluded(t *testing.T) {
	// Particles (助詞) should never appear in output.
	particles := []string{"が", "を", "に", "は", "と", "で", "の"}
	got := extractContentWords("猫が犬と公園で遊ぶ")
	for _, p := range particles {
		if slices.Contains(got, p) {
			t.Errorf("did not expect particle %q in result %v", p, got)
		}
	}
}

// Multi-paragraph integration tests — natural flowing Japanese text.

func TestExtractContentWords_MultiParagraphDailyLife(t *testing.T) {
	// Two paragraphs covering seasons, commuting, and nature.
	// Tests that nouns, verbs, and i-adjectives are all extracted from
	// realistic prose and that particles are excluded throughout.
	input := "東京は大きな都市だ。毎日たくさんの人が電車に乗って通勤する。\n\n" +
		"夏は暑く、冬は寒い。春には美しい桜が咲く。"

	got := extractContentWords(input)

	mustInclude := []string{
		"都市",   // noun: general
		"人",    // noun: general
		"電車",   // noun: general
		"乗る",   // verb: base of 乗って
		"夏",    // noun: general
		"暑い",   // i-adjective: base of 暑く
		"冬",    // noun: general
		"寒い",   // i-adjective
		"美しい",  // i-adjective
		"桜",    // noun: general
		"咲く",   // verb
	}
	for _, w := range mustInclude {
		if !slices.Contains(got, w) {
			t.Errorf("expected content word %q in result %v", w, got)
		}
	}

	mustExclude := []string{"は", "の", "が", "に", "を", "と"} // particles
	for _, w := range mustExclude {
		if slices.Contains(got, w) {
			t.Errorf("did not expect particle %q in result %v", w, got)
		}
	}
}

func TestExtractContentWords_MultiParagraphLanguageLearning(t *testing.T) {
	// Two paragraphs about learning Japanese.
	// 難しい appears in both paragraphs — deduplication across paragraphs is verified.
	// Also checks that na-adjectives (大変, 重要), adverbs (特に, 必ず), and
	// cross-paragraph verbs are correctly extracted.
	input := "日本語は特に難しい言語だと言われている。漢字を覚えるのは大変な作業だ。\n\n" +
		"難しいと思っても、毎日練習すれば読む力と書く力が伸びる。諦めずに続ければ必ず上達する。"

	got := extractContentWords(input)

	mustInclude := []string{
		"日本語",  // noun: general
		"特に",   // adverb
		"難しい",  // i-adjective (appears in both paragraphs)
		"言語",   // noun: general
		"漢字",   // noun: general
		"覚える",  // verb: base of 覚える
		"大変",   // noun: na-adjective stem (名詞,形容動詞語幹)
		"作業",   // noun: general
		"練習",   // noun: sa-verb (サ変接続)
		"読む",   // verb
		"書く",   // verb
		"続ける",  // verb: base of 続ければ
		"必ず",   // adverb
		"上達",   // noun: sa-verb (サ変接続)
	}
	for _, w := range mustInclude {
		if !slices.Contains(got, w) {
			t.Errorf("expected content word %q in result %v", w, got)
		}
	}

	mustExclude := []string{
		"は", "を", "の", "が", // particles
		"いる",  // 動詞,非自立 — auxiliary in 言われている
		"こと",  // 名詞,非自立 — nominalizer
	}
	for _, w := range mustExclude {
		if slices.Contains(got, w) {
			t.Errorf("did not expect grammatical word %q in result %v", w, got)
		}
	}

	// 難しい appears in both paragraphs; deduplication must reduce it to one entry.
	count := 0
	for _, w := range got {
		if w == "難しい" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 難しい exactly once across both paragraphs, got %d times in %v", count, got)
	}
}

func TestExtractContentWords_NonJapaneseWordsExcluded(t *testing.T) {
	// English words and romaji that Kagome classifies as 名詞,一般 must be dropped.
	got := extractContentWords("Tokyo is a beautiful city. 東京は美しい都市だ。")
	nonJapanese := []string{"Tokyo", "is", "a", "beautiful", "city"}
	for _, w := range nonJapanese {
		if slices.Contains(got, w) {
			t.Errorf("did not expect non-Japanese word %q in result %v", w, got)
		}
	}
	if !slices.Contains(got, "東京") {
		t.Errorf("expected 東京 in result %v", got)
	}
	if !slices.Contains(got, "美しい") {
		t.Errorf("expected 美しい in result %v", got)
	}
}

func TestExtractContentWords_EmptyInput(t *testing.T) {
	got := extractContentWords("")
	if len(got) != 0 {
		t.Errorf("expected empty slice for empty input, got %v", got)
	}
}

func TestExtractContentWords_FirstSeenOrder(t *testing.T) {
	// Words should appear in the order they are first encountered in the text.
	got := extractContentWords("猫が魚を食べる")
	// All three expected words should be present; 猫 must come before 魚.
	catIdx, fishIdx := -1, -1
	for i, w := range got {
		if w == "猫" {
			catIdx = i
		}
		if w == "魚" {
			fishIdx = i
		}
	}
	if catIdx == -1 || fishIdx == -1 {
		t.Fatalf("expected 猫 and 魚 in result %v", got)
	}
	if catIdx > fishIdx {
		t.Errorf("expected 猫 (index %d) before 魚 (index %d)", catIdx, fishIdx)
	}
}
