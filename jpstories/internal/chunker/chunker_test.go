package chunker

import (
	"reflect"
	"strings"
	"testing"

	"jpstories/internal/story"
)

func TestDraftSplitsParagraphsSentencesAndChunks(t *testing.T) {
	text := "First sentence. Second sentence!\n\nThird sentence?\n\nFourth line without punctuation"

	got, err := Draft(text, Options{
		StoryID:            "sample-story",
		Title:              "Sample Story",
		SourceFile:         "stories/sample-story/sample-story.txt",
		ParagraphsPerChunk: 2,
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	if got.ID != "sample-story" {
		t.Fatalf("Story.ID = %q, want sample-story", got.ID)
	}
	if got.Title != "Sample Story" {
		t.Fatalf("Story.Title = %q, want Sample Story", got.Title)
	}
	if !reflect.DeepEqual(got.Levels, story.SupportedLevels) {
		t.Fatalf("Story.Levels = %#v, want %#v", got.Levels, story.SupportedLevels)
	}
	if len(got.Chunks) != 2 {
		t.Fatalf("len(Chunks) = %d, want 2", len(got.Chunks))
	}
	if got.Chunks[0].ID != "chunk-001" || got.Chunks[1].ID != "chunk-002" {
		t.Fatalf("chunk IDs = %q, %q", got.Chunks[0].ID, got.Chunks[1].ID)
	}
	if len(got.Chunks[0].Paragraphs) != 2 {
		t.Fatalf("first chunk paragraph count = %d, want 2", len(got.Chunks[0].Paragraphs))
	}

	firstParagraph := got.Chunks[0].Paragraphs[0]
	if firstParagraph.ID != "p-001" {
		t.Fatalf("first paragraph ID = %q, want p-001", firstParagraph.ID)
	}
	if len(firstParagraph.Sentences) != 2 {
		t.Fatalf("first paragraph sentence count = %d, want 2", len(firstParagraph.Sentences))
	}
	if firstParagraph.Sentences[0].ID != "s-001" || firstParagraph.Sentences[1].ID != "s-002" {
		t.Fatalf("sentence IDs = %q, %q", firstParagraph.Sentences[0].ID, firstParagraph.Sentences[1].ID)
	}
	if firstParagraph.Sentences[0].English != "First sentence." {
		t.Fatalf("first sentence = %q", firstParagraph.Sentences[0].English)
	}
	if firstParagraph.Sentences[0].Translations == nil {
		t.Fatal("Translations = nil, want empty map")
	}
}

func TestDraftIsStableForUnchangedInput(t *testing.T) {
	text := "Alpha. Beta.\n\nGamma."
	opts := Options{
		StoryID:            "stable",
		Title:              "Stable",
		SourceFile:         "stories/stable/stable.txt",
		ParagraphsPerChunk: 1,
	}

	a, err := Draft(text, opts)
	if err != nil {
		t.Fatalf("first Draft() error = %v", err)
	}
	b, err := Draft(text, opts)
	if err != nil {
		t.Fatalf("second Draft() error = %v", err)
	}

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Draft() results differ:\n%#v\n%#v", a, b)
	}
}

func TestDraftGroupsDefaultChunksBySourceWordTarget(t *testing.T) {
	text := strings.Join([]string{
		"one two three four.",
		"five six seven eight.",
		"nine ten eleven twelve.",
	}, "\n\n")

	got, err := Draft(text, Options{
		StoryID:       "word-target",
		Title:         "Word Target",
		SourceFile:    "stories/word-target/word-target.txt",
		WordsPerChunk: 8,
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	if len(got.Chunks) != 2 {
		t.Fatalf("len(Chunks) = %d, want 2", len(got.Chunks))
	}
	if len(got.Chunks[0].Paragraphs) != 2 {
		t.Fatalf("first chunk paragraph count = %d, want 2", len(got.Chunks[0].Paragraphs))
	}
	if len(got.Chunks[1].Paragraphs) != 1 {
		t.Fatalf("second chunk paragraph count = %d, want 1", len(got.Chunks[1].Paragraphs))
	}
}

func TestDraftParagraphCountOverridesWordTarget(t *testing.T) {
	text := strings.Join([]string{
		"one two three four.",
		"five six seven eight.",
		"nine ten eleven twelve.",
	}, "\n\n")

	got, err := Draft(text, Options{
		StoryID:            "paragraph-target",
		Title:              "Paragraph Target",
		SourceFile:         "stories/paragraph-target/paragraph-target.txt",
		ParagraphsPerChunk: 1,
		WordsPerChunk:      100,
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	if len(got.Chunks) != 3 {
		t.Fatalf("len(Chunks) = %d, want paragraph override to produce 3", len(got.Chunks))
	}
}

func TestDraftNormalizesWrappedParagraphs(t *testing.T) {
	text := "This paragraph wraps\nacross two lines. It stays one paragraph."

	got, err := Draft(text, Options{
		StoryID:    "wrapped",
		Title:      "Wrapped",
		SourceFile: "stories/wrapped/wrapped.txt",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	paragraph := got.Chunks[0].Paragraphs[0]
	want := "This paragraph wraps across two lines. It stays one paragraph."
	var gotText []string
	for _, sentence := range paragraph.Sentences {
		gotText = append(gotText, sentence.English)
	}
	if strings.Join(gotText, " ") != want {
		t.Fatalf("sentence English = %q, want %q", strings.Join(gotText, " "), want)
	}
}

func TestDraftSentenceSplittingHandlesAbbreviationsEllipsesAndSemicolons(t *testing.T) {
	text := `Mr. Smith waited... then Mrs. Green arrived; she waved. Dr. Brown left at 3.5 and returned.`

	got, err := Draft(text, Options{
		StoryID:    "boundaries",
		Title:      "Boundaries",
		SourceFile: "stories/boundaries/boundaries.txt",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	sentences := got.Chunks[0].Paragraphs[0].Sentences
	if len(sentences) != 3 {
		t.Fatalf("sentence count = %d, want 3: %#v", len(sentences), sentences)
	}
	want := []string{
		"Mr. Smith waited... then Mrs. Green arrived.",
		"She waved.",
		"Dr. Brown left at 3.5 and returned.",
	}
	for i, sentence := range sentences {
		if sentence.English != want[i] {
			t.Fatalf("sentence[%d] = %q, want %q", i, sentence.English, want[i])
		}
	}
}

func TestDraftSentenceSplittingKeepsSpacedEllipsesTogether(t *testing.T) {
	text := `“My dear boy! Arthur told me you were here, disguised. . . . I am so glad, so honored!”`

	got, err := Draft(text, Options{
		StoryID:    "spaced-ellipses",
		Title:      "Spaced Ellipses",
		SourceFile: "stories/spaced-ellipses/spaced-ellipses.txt",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	sentences := got.Chunks[0].Paragraphs[0].Sentences
	if len(sentences) != 2 {
		t.Fatalf("sentence count = %d, want 2: %#v", len(sentences), sentences)
	}
	want := []string{
		"“My dear boy!",
		"Arthur told me you were here, disguised. . . . I am so glad, so honored!”",
	}
	for i, sentence := range sentences {
		if sentence.English != want[i] {
			t.Fatalf("sentence[%d] = %q, want %q", i, sentence.English, want[i])
		}
	}
}

func TestDraftInfersMetadataFromSourceFile(t *testing.T) {
	got, err := Draft("Hello.", Options{
		SourceFile: "stories/the-small-door/The Small Door.txt",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	if got.ID != "the-small-door" {
		t.Fatalf("Story.ID = %q, want the-small-door", got.ID)
	}
	if got.Title != "The Small Door" {
		t.Fatalf("Story.Title = %q, want The Small Door", got.Title)
	}
}

func TestDraftRejectsEmptySource(t *testing.T) {
	_, err := Draft(" \n\n ", Options{
		StoryID:    "empty",
		Title:      "Empty",
		SourceFile: "stories/empty/empty.txt",
	})
	if err == nil {
		t.Fatal("Draft() error = nil, want error")
	}
}

func TestDraftJapaneseSplitsSentencesOnKutenAndStoresInNative(t *testing.T) {
	text := "駅で小さな鐘が鳴った。ホームに人影はなかった。\n\n彼女は静かに笑った。"

	got, err := Draft(text, Options{
		StoryID:        "jp-test",
		Title:          "JP Test",
		SourceFile:     "stories/jp-test/jp-test.txt",
		SourceLanguage: "ja",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	if got.SourceLanguage != "ja" {
		t.Errorf("SourceLanguage = %q, want ja", got.SourceLanguage)
	}
	if got.TargetLanguage != "en" {
		t.Errorf("TargetLanguage = %q, want en", got.TargetLanguage)
	}

	// Two paragraphs, first has 2 sentences (split on 。).
	if len(got.Chunks[0].Paragraphs) < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", len(got.Chunks[0].Paragraphs))
	}
	firstPara := got.Chunks[0].Paragraphs[0]
	if len(firstPara.Sentences) != 2 {
		t.Fatalf("first paragraph sentence count = %d, want 2", len(firstPara.Sentences))
	}
	s0 := firstPara.Sentences[0]
	if s0.English != "" {
		t.Errorf("sentence English = %q, want empty for ja source", s0.English)
	}
	if s0.Translations[story.LevelNative] != "駅で小さな鐘が鳴った。" {
		t.Errorf("sentence Translations[native] = %q", s0.Translations[story.LevelNative])
	}
}

func TestDraftJapaneseNormalizesMultiLineParagraphWithoutSpaces(t *testing.T) {
	text := "これは一行目。\nこれは二行目。"

	got, err := Draft(text, Options{
		StoryID:        "jp-wrap",
		Title:          "JP Wrap",
		SourceFile:     "stories/jp-wrap/jp-wrap.txt",
		SourceLanguage: "ja",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	// One paragraph; no space should be inserted between the two lines.
	para := got.Chunks[0].Paragraphs[0]
	if len(para.Sentences) != 2 {
		t.Fatalf("sentence count = %d, want 2", len(para.Sentences))
	}
	// Neither sentence should contain a space between kanji characters.
	for _, s := range para.Sentences {
		native := s.Translations[story.LevelNative]
		if strings.Contains(native, "目。 こ") {
			t.Errorf("unexpected space between ja lines: %q", native)
		}
	}
}

func TestDraftJapaneseClosingBracketsAreConsumedAfterSentenceEnd(t *testing.T) {
	text := "「鐘が鳴りました」と彼女は言った。次の文。"

	got, err := Draft(text, Options{
		StoryID:        "jp-brackets",
		Title:          "JP Brackets",
		SourceFile:     "stories/jp-brackets/jp-brackets.txt",
		SourceLanguage: "ja",
	})
	if err != nil {
		t.Fatalf("Draft() error = %v", err)
	}

	para := got.Chunks[0].Paragraphs[0]
	if len(para.Sentences) != 2 {
		t.Fatalf("sentence count = %d, want 2: %#v", len(para.Sentences), para.Sentences)
	}
	// First sentence should include the 。 but NOT the following 次.
	s0 := para.Sentences[0].Translations[story.LevelNative]
	if !strings.HasSuffix(s0, "。") {
		t.Errorf("first sentence = %q, expected to end with 。", s0)
	}
}
