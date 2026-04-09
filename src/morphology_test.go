package main

import (
	"os"
	"slices"
	"strings"
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

// --- splitByClause ---

// splitJoin concatenates all chunks back into one string. Used to verify that
// splitting is lossless (chunks reconstruct the original text exactly).
func splitJoin(chunks []string) string {
	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString(c)
	}
	return sb.String()
}

func TestSplitByClause_ShortSentence(t *testing.T) {
	// Sentences below minClauseSplitRunes must be returned unsplit.
	input := "今日は。" // 4 runes
	got := splitByClause(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk for short sentence, got %d: %v", len(got), got)
	}
	if got[0] != input {
		t.Errorf("chunk should equal original; got %q", got[0])
	}
}

func TestSplitByClause_NoSplitPoint(t *testing.T) {
	// A long sentence with no clause boundaries should return a single chunk.
	// 「今日はとてもいい天気ですね。」 has no 読点 or 接続助詞.
	input := "今日はとてもいい天気ですね。" // 14 runes
	got := splitByClause(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk (no split points), got %d: %v", len(got), got)
	}
}

func TestSplitByClause_ReadotenSplit(t *testing.T) {
	// 「、」 always produces a split immediately after it.
	// The chunk before 「、」 must be ≥ minChunkRunes so it is not merged back.
	input := "今日は晴れです、外に出ましょう。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks at 読点, got %d: %v", len(got), got)
	}
	if got[0] != "今日は晴れです、" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "今日は晴れです、")
	}
	if got[1] != "外に出ましょう。" {
		t.Errorf("chunk[1] = %q, want %q", got[1], "外に出ましょう。")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_TeFollowedByReadoten(t *testing.T) {
	// 「て」 immediately before 「、」: the split should land at 「、」, not at 「て」,
	// so the 読点 stays with the first chunk and the second chunk starts cleanly.
	input := "昨日は雨が降って、風も強かった。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
	if got[0] != "昨日は雨が降って、" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "昨日は雨が降って、")
	}
	if got[1] != "風も強かった。" {
		t.Errorf("chunk[1] = %q, want %q", got[1], "風も強かった。")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_TeSplit_NoAuxiliary(t *testing.T) {
	// 「て」 not followed by a 非自立 verb → should split.
	// The pre-split chunk must be ≥ minChunkRunes so it is not merged back.
	// 「毎日とても寒くて外に出られない。」: 寒くて is an i-adjective conjunctive form,
	// followed by 外 (名詞) — not a 非自立 verb.
	input := "毎日とても寒くて外に出られない。"
	got := splitByClause(input)
	if len(got) < 2 {
		t.Fatalf("expected split at 寒くて, got %d chunk(s): %v", len(got), got)
	}
	if got[0] != "毎日とても寒くて" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "毎日とても寒くて")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_TeAuxiliary_NoSplit(t *testing.T) {
	// 「て」 followed by a 非自立 verb (〜ている) must NOT split.
	// 「毎日ご飯をたくさん食べている」: 食べ+て+いる (いる=動詞,非自立).
	input := "毎日ご飯をたくさん食べている。"
	got := splitByClause(input)
	if len(got) != 1 {
		t.Fatalf("expected no split for 〜ている, got %d chunks: %v", len(got), got)
	}
}

func TestSplitByClause_TeKuru_NoSplit(t *testing.T) {
	// 〜てくる compound (て + くる=動詞,非自立) must NOT split.
	// 「公園まで走ってきたから疲れました」: 走っ+て+き(くる,非自立).
	input := "公園まで走ってきたから疲れました。"
	got := splitByClause(input)
	// Should split at から (because), but NOT at て (compound 〜てくる).
	// Expected: ["公園まで走ってきたから", "疲れました。"] or similar.
	if len(got) < 2 {
		t.Fatalf("expected split at から, got %d chunk(s): %v", len(got), got)
	}
	// Verify て+くる stayed together (first chunk contains 走ってき).
	if !strings.Contains(got[0], "走ってき") {
		t.Errorf("expected 走ってき unsplit in chunk[0] = %q", got[0])
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_KaraConjunctive(t *testing.T) {
	// 「から」 meaning "because" (助詞,接続助詞) should split.
	// 「遅れたから電話してください」: た+から+電話.
	// The 〜てください at the end must NOT split (て+ください=動詞,非自立).
	input := "遅れたから電話してください。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
	if got[0] != "遅れたから" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "遅れたから")
	}
	if got[1] != "電話してください。" {
		t.Errorf("chunk[1] = %q, want %q", got[1], "電話してください。")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_KaraLocation_NoSplit(t *testing.T) {
	// 「から」 meaning "from" (助詞,格助詞,一般) must NOT split.
	// IPA tags this differently from conjunctive から.
	input := "私は東京から電車で来ました。"
	got := splitByClause(input)
	if len(got) != 1 {
		t.Fatalf("expected no split for locative から, got %d chunks: %v", len(got), got)
	}
}

func TestSplitByClause_GaConjunctive(t *testing.T) {
	// 「が」 as a concessive/contrastive conjunction (助詞,接続助詞) should split.
	// 「頑張ったが合格できなかった」: た+が+合格.
	input := "頑張ったが合格できなかった。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
	if got[0] != "頑張ったが" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "頑張ったが")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_GaSubject_NoSplit(t *testing.T) {
	// 「が」 as a subject marker (助詞,格助詞,一般) must NOT split.
	input := "この猫が好きな人はとても多い。"
	got := splitByClause(input)
	if len(got) != 1 {
		t.Fatalf("expected no split for subject-marker が, got %d chunks: %v", len(got), got)
	}
}

func TestSplitByClause_Nagara(t *testing.T) {
	// 「ながら」 (接続助詞, "while") should split.
	input := "音楽を聴きながら勉強しています。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks at ながら, got %d: %v", len(got), got)
	}
	if got[0] != "音楽を聴きながら" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "音楽を聴きながら")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_Setsuzokushi(t *testing.T) {
	// A 接続詞 (sentence-level conjunction) should cause a split BEFORE it.
	// Note: 「でも」 directly after a verb is tagged 助詞,副助詞 by IPA, not 接続詞.
	// 「しかし」 is unambiguously tagged 接続詞 regardless of position.
	// 「何度も失敗したしかし諦めない。」: しかし is 接続詞 → split before it.
	input := "何度も失敗したしかし諦めない。"
	got := splitByClause(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks at 接続詞 しかし, got %d: %v", len(got), got)
	}
	if got[0] != "何度も失敗した" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "何度も失敗した")
	}
	if got[1] != "しかし諦めない。" {
		t.Errorf("chunk[1] = %q, want %q", got[1], "しかし諦めない。")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_MultipleBreaks(t *testing.T) {
	// A sentence with multiple 読点 should produce multiple chunks.
	// All chunks must be ≥ minChunkRunes so nothing is merged away.
	// 「朝ごはんを食べて、歯を磨いて、学校に行きます。」
	// て is suppressed before each 、; the 読点 handles both splits.
	input := "朝ごはんを食べて、歯を磨いて、学校に行きます。"
	got := splitByClause(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(got), got)
	}
	if got[0] != "朝ごはんを食べて、" {
		t.Errorf("chunk[0] = %q, want %q", got[0], "朝ごはんを食べて、")
	}
	if got[1] != "歯を磨いて、" {
		t.Errorf("chunk[1] = %q, want %q", got[1], "歯を磨いて、")
	}
	if got[2] != "学校に行きます。" {
		t.Errorf("chunk[2] = %q, want %q", got[2], "学校に行きます。")
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_ShortChunkMerge(t *testing.T) {
	// Chunks below minChunkRunes must be merged into a neighbor.
	// 「雨が降ってから、外に出ます。」
	// Tokens split: after て (suppressed by following 読点), then after から、
	// This leaves 「から、」 (3 runes) which must merge with its neighbor.
	input := "雨が降ってから、外に出ます。"
	got := splitByClause(input)
	for _, c := range got {
		if len([]rune(c)) < minChunkRunes {
			t.Errorf("chunk %q has fewer than %d runes", c, minChunkRunes)
		}
	}
	if splitJoin(got) != input {
		t.Errorf("chunks do not reconstruct original: got %q", splitJoin(got))
	}
}

func TestSplitByClause_Lossless(t *testing.T) {
	// For every sentence, concatenating all chunks must equal the original input.
	sentences := []string{
		"昨日、友達と映画を見に行って、とても楽しかったです。",
		"電車が遅れたので、会議に少し遅刻してしまいました。",
		"新しいプロジェクトを始める前に、チーム全員で目標や役割分担を明確にしておくことがとても重要だと思います。",
		"科学技術の急速な発展により、私たちの日常生活は大きく変化しており、特に人工知能やロボット工学の分野では目覚ましい進歩が続いています。",
		"すみません、この近くに郵便局はありますか？",
	}
	for _, s := range sentences {
		got := splitByClause(s)
		if joined := splitJoin(got); joined != s {
			t.Errorf("splitByClause(%q) not lossless: got %q", s, joined)
		}
	}
}
