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
		"she waved.",
		"Dr. Brown left at 3.5 and returned.",
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
