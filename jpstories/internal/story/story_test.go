package story

import (
	"strings"
	"testing"
)

func TestValidateAcceptsValidDraftStory(t *testing.T) {
	s := validStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations = map[string]string{}

	if err := Validate(s); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateCompleteRequiresAllConfiguredTranslations(t *testing.T) {
	s := validStory()
	delete(s.Chunks[0].Paragraphs[0].Sentences[0].Translations, LevelN3)

	err := ValidateComplete(s)
	if err == nil {
		t.Fatal("ValidateComplete() error = nil, want missing translation error")
	}
	if !strings.Contains(err.Error(), "translations.n3") {
		t.Fatalf("ValidateComplete() error = %v, want n3 path", err)
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Story)
		want string
	}{
		{
			name: "story id",
			edit: func(s *Story) {
				s.ID = ""
			},
			want: "story.id",
		},
		{
			name: "source file",
			edit: func(s *Story) {
				s.SourceFile = ""
			},
			want: "story.source_file",
		},
		{
			name: "sentence english",
			edit: func(s *Story) {
				s.Chunks[0].Paragraphs[0].Sentences[0].English = ""
			},
			want: "sentences[0].english",
		},
		{
			name: "translations object",
			edit: func(s *Story) {
				s.Chunks[0].Paragraphs[0].Sentences[0].Translations = nil
			},
			want: "translations must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStory()
			tt.edit(&s)

			err := Validate(s)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Story)
		want string
	}{
		{
			name: "chunk",
			edit: func(s *Story) {
				s.Chunks = append(s.Chunks, s.Chunks[0])
			},
			want: "duplicates id \"chunk-001\"",
		},
		{
			name: "paragraph",
			edit: func(s *Story) {
				paragraph := s.Chunks[0].Paragraphs[0]
				paragraph.ID = "p-001"
				s.Chunks[0].Paragraphs = append(s.Chunks[0].Paragraphs, paragraph)
			},
			want: "duplicates id \"p-001\"",
		},
		{
			name: "sentence",
			edit: func(s *Story) {
				sentence := s.Chunks[0].Paragraphs[0].Sentences[0]
				s.Chunks[0].Paragraphs[0].Sentences = append(s.Chunks[0].Paragraphs[0].Sentences, sentence)
			},
			want: "duplicates id \"s-001\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStory()
			tt.edit(&s)

			err := Validate(s)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsUnsupportedLevels(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Story)
		want string
	}{
		{
			name: "configured level",
			edit: func(s *Story) {
				s.Levels = append(s.Levels, "n5")
			},
			want: "unsupported level \"n5\"",
		},
		{
			name: "translation level",
			edit: func(s *Story) {
				s.Chunks[0].Paragraphs[0].Sentences[0].Translations["n5"] = "bad"
			},
			want: "unsupported level \"n5\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStory()
			tt.edit(&s)

			err := Validate(s)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsOutOfOrderRecognizedIDs(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Story)
		want string
	}{
		{
			name: "chunk",
			edit: func(s *Story) {
				s.Chunks[0].ID = "chunk-002"
			},
			want: "out of order",
		},
		{
			name: "paragraph",
			edit: func(s *Story) {
				s.Chunks[0].Paragraphs[0].ID = "p-002"
			},
			want: "out of order",
		},
		{
			name: "sentence",
			edit: func(s *Story) {
				s.Chunks[0].Paragraphs[0].Sentences[0].ID = "s-002"
			},
			want: "out of order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStory()
			tt.edit(&s)

			err := Validate(s)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsEmptyTranslationWhenPresent(t *testing.T) {
	s := validStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[LevelNative] = " "

	err := Validate(s)
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if !strings.Contains(err.Error(), "must not be empty when present") {
		t.Fatalf("Validate() error = %v, want empty translation error", err)
	}
}

func validStory() Story {
	return Story{
		ID:             "sample",
		Title:          "Sample",
		SourceLanguage: "en",
		TargetLanguage: "ja",
		SourceFile:     "stories/sample/sample.txt",
		Levels:         []string{LevelNative, LevelN3, LevelN3Abridged},
		Chunks: []Chunk{
			{
				ID: "chunk-001",
				Paragraphs: []Paragraph{
					{
						ID: "p-001",
						Sentences: []Sentence{
							{
								ID:      "s-001",
								English: "A small bell rang in the station.",
								Translations: map[string]string{
									LevelNative:     "native translation",
									LevelN3:         "n3 translation",
									LevelN3Abridged: "n3 abridged translation",
								},
							},
						},
					},
				},
			},
		},
	}
}
