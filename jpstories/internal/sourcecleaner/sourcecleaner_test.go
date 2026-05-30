package sourcecleaner

import (
	"strings"
	"testing"
)

func TestCleanRepairsEncodingHyphenationAndDialogueParagraphs(t *testing.T) {
	text := "â€œCorrect,â€ said Scrimgeour. â€œA Snitch is not touched by bare\nskin before it is released, not even by the maker.\nHarryâ€™s heart was beating rather fast.\nâ€œYou donâ€™t say anything,â€ said Scrim-\ngeour."

	got, err := Clean(text, Options{
		CleanEncoding:     true,
		RepairHyphenation: true,
		ParagraphMode:     ParagraphModeDialogue,
	})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}

	want := "“Correct,” said Scrimgeour. “A Snitch is not touched by bare skin before it is released, not even by the maker.\n\nHarry’s heart was beating rather fast.\n\n“You don’t say anything,” said Scrimgeour.\n"
	if got.Text != want {
		t.Fatalf("Text = %q, want %q", got.Text, want)
	}
	if got.Stats.EncodingReplacements == 0 {
		t.Fatal("EncodingReplacements = 0, want replacements")
	}
	if got.Stats.HyphenationRepairs != 1 {
		t.Fatalf("HyphenationRepairs = %d, want 1", got.Stats.HyphenationRepairs)
	}
	if got.Stats.ParagraphsOut != 3 {
		t.Fatalf("ParagraphsOut = %d, want 3", got.Stats.ParagraphsOut)
	}
}

func TestCleanRepairsRealLigatures(t *testing.T) {
	got, err := Clean("The ﬂoor had a ﬁne oﬀice sign.", Options{
		CleanEncoding:     true,
		RepairHyphenation: true,
		ParagraphMode:     ParagraphModePreserve,
	})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	want := "The floor had a fine office sign.\n"
	if got.Text != want {
		t.Fatalf("Text = %q, want %q", got.Text, want)
	}
}

func TestCleanPreserveModeKeepsOnlyBlankLineParagraphs(t *testing.T) {
	text := "First line.\nSecond line.\n\nâ€œThird line.â€\nFourth line."

	got, err := Clean(text, Options{
		CleanEncoding:     true,
		RepairHyphenation: true,
		ParagraphMode:     ParagraphModePreserve,
	})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}

	if got.Stats.ParagraphsOut != 2 {
		t.Fatalf("ParagraphsOut = %d, want 2", got.Stats.ParagraphsOut)
	}
	if strings.Count(got.Text, "\n\n") != 1 {
		t.Fatalf("Text = %q, want one paragraph break", got.Text)
	}
}

func TestCleanParagraphInferenceIgnoresAbbreviationsAndEllipses(t *testing.T) {
	text := "Mr.\n\"Smith is waiting,\" she said.\nHe paused...\n\"Then we should hurry,\" I said."

	got, err := Clean(text, Options{
		CleanEncoding:     false,
		RepairHyphenation: true,
		ParagraphMode:     ParagraphModeConservative,
	})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}

	if got.Stats.ParagraphBreaksAdded != 0 {
		t.Fatalf("ParagraphBreaksAdded = %d, want 0; text = %q", got.Stats.ParagraphBreaksAdded, got.Text)
	}
	if strings.Count(got.Text, "\n\n") != 0 {
		t.Fatalf("Text = %q, want no inferred paragraph breaks", got.Text)
	}
}

func TestCleanRejectsUnsupportedParagraphMode(t *testing.T) {
	_, err := Clean("Hello.", Options{ParagraphMode: "wild"})
	if err == nil {
		t.Fatal("Clean() error = nil, want error")
	}
}
