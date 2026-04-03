package main

import "testing"

// --- mergeWordListEntry ---

func TestMergeWordListEntry_EmptyFieldsFilledByIncoming(t *testing.T) {
	existing := wordListEntry{Word: "猫"}
	incoming := wordListEntry{
		Word:         "猫",
		Reading:      "ねこ",
		PartOfSpeech: "noun",
		Meaning:      "cat",
		ExampleJP:    "猫がいる。",
		ExampleEN:    "There is a cat.",
	}
	merged := mergeWordListEntry(existing, incoming, "猫", "test-list")
	if merged.Reading != "ねこ" {
		t.Errorf("Reading: got %q, want ねこ", merged.Reading)
	}
	if merged.PartOfSpeech != "noun" {
		t.Errorf("PartOfSpeech: got %q, want noun", merged.PartOfSpeech)
	}
	if merged.Meaning != "cat" {
		t.Errorf("Meaning: got %q, want cat", merged.Meaning)
	}
	if merged.ExampleJP != "猫がいる。" {
		t.Errorf("ExampleJP: got %q", merged.ExampleJP)
	}
	if merged.ExampleEN != "There is a cat." {
		t.Errorf("ExampleEN: got %q", merged.ExampleEN)
	}
}

func TestMergeWordListEntry_NonEmptyExistingFieldIsKept(t *testing.T) {
	existing := wordListEntry{Word: "犬", Reading: "いぬ", Meaning: "dog"}
	incoming := wordListEntry{Word: "犬", Reading: "イヌ", Meaning: "hound"}

	merged := mergeWordListEntry(existing, incoming, "犬", "test-list")
	if merged.Reading != "いぬ" {
		t.Errorf("Reading: got %q, want いぬ (first value kept on conflict)", merged.Reading)
	}
	if merged.Meaning != "dog" {
		t.Errorf("Meaning: got %q, want dog (first value kept on conflict)", merged.Meaning)
	}
}

func TestMergeWordListEntry_EmptyIncomingFieldDoesNotOverwrite(t *testing.T) {
	existing := wordListEntry{Word: "鳥", Reading: "とり", Meaning: "bird"}
	incoming := wordListEntry{Word: "鳥", Reading: "", Meaning: ""}

	merged := mergeWordListEntry(existing, incoming, "鳥", "test-list")
	if merged.Reading != "とり" {
		t.Errorf("Reading: got %q, want とり (empty incoming should not overwrite)", merged.Reading)
	}
	if merged.Meaning != "bird" {
		t.Errorf("Meaning: got %q, want bird (empty incoming should not overwrite)", merged.Meaning)
	}
}

func TestMergeWordListEntry_SuggestedImageURLMerged(t *testing.T) {
	existing := wordListEntry{Word: "山"}
	incoming := wordListEntry{Word: "山", SuggestedImageURL: "https://example.com/yama.jpg"}

	merged := mergeWordListEntry(existing, incoming, "山", "test-list")
	if merged.SuggestedImageURL != "https://example.com/yama.jpg" {
		t.Errorf("SuggestedImageURL: got %q", merged.SuggestedImageURL)
	}
}
