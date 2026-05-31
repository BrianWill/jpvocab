package story

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	LevelNative     = "native"
	LevelN3         = "n3"
	LevelN3Abridged = "n3_abridged"
)

var SupportedLevels = []string{LevelNative, LevelN3, LevelN3Abridged}

var supportedLevelSet = map[string]struct{}{
	LevelNative:     {},
	LevelN3:         {},
	LevelN3Abridged: {},
}

type Story struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	SourceLanguage string   `json:"source_language"`
	TargetLanguage string   `json:"target_language"`
	SourceFile     string   `json:"source_file"`
	Levels         []string `json:"levels"`
	Chunks         []Chunk  `json:"chunks"`
}

type Chunk struct {
	ID         string      `json:"id"`
	Paragraphs []Paragraph `json:"paragraphs"`
}

type Paragraph struct {
	ID        string     `json:"id"`
	Sentences []Sentence `json:"sentences"`
}

type Sentence struct {
	ID           string            `json:"id"`
	English      string            `json:"english"`
	Translations map[string]string `json:"translations"`
}

func Validate(s Story) error {
	return validate(s, false)
}

func ValidateComplete(s Story) error {
	return validate(s, true)
}

func IsSupportedLevel(level string) bool {
	_, ok := supportedLevelSet[level]
	return ok
}

func validate(s Story, requireTranslations bool) error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("story.id is required")
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("story.title is required")
	}
	if strings.TrimSpace(s.SourceLanguage) == "" {
		return fmt.Errorf("story.source_language is required")
	}
	if strings.TrimSpace(s.TargetLanguage) == "" {
		return fmt.Errorf("story.target_language is required")
	}
	if strings.TrimSpace(s.SourceFile) == "" {
		return fmt.Errorf("story.source_file is required")
	}
	if len(s.Levels) == 0 {
		return fmt.Errorf("story.levels must include at least one translation level")
	}
	if len(s.Chunks) == 0 {
		return fmt.Errorf("story.chunks must include at least one chunk")
	}

	if err := validateLevels(s.Levels); err != nil {
		return err
	}

	seenChunkIDs := map[string]struct{}{}
	seenParagraphIDs := map[string]struct{}{}
	seenSentenceIDs := map[string]struct{}{}
	nextParagraphOrdinal := 1
	nextSentenceOrdinal := 1

	for i, chunk := range s.Chunks {
		path := fmt.Sprintf("story.chunks[%d]", i)
		if err := validateID(path+".id", chunk.ID, seenChunkIDs); err != nil {
			return err
		}
		if err := validateOrderedID(path+".id", chunk.ID, "chunk-", i+1); err != nil {
			return err
		}
		if len(chunk.Paragraphs) == 0 {
			return fmt.Errorf("%s.paragraphs must include at least one paragraph", path)
		}

		for j, paragraph := range chunk.Paragraphs {
			paragraphPath := fmt.Sprintf("%s.paragraphs[%d]", path, j)
			if err := validateID(paragraphPath+".id", paragraph.ID, seenParagraphIDs); err != nil {
				return err
			}
			if err := validateOrderedID(paragraphPath+".id", paragraph.ID, "p-", nextParagraphOrdinal); err != nil {
				return err
			}
			nextParagraphOrdinal++
			if len(paragraph.Sentences) == 0 {
				return fmt.Errorf("%s.sentences must include at least one sentence", paragraphPath)
			}

			for k, sentence := range paragraph.Sentences {
				sentencePath := fmt.Sprintf("%s.sentences[%d]", paragraphPath, k)
				if err := validateID(sentencePath+".id", sentence.ID, seenSentenceIDs); err != nil {
					return err
				}
				if err := validateOrderedID(sentencePath+".id", sentence.ID, "s-", nextSentenceOrdinal); err != nil {
					return err
				}
				nextSentenceOrdinal++
				if strings.TrimSpace(sentence.English) == "" {
					return fmt.Errorf("%s.english is required", sentencePath)
				}
				if sentence.Translations == nil {
					return fmt.Errorf("%s.translations must be an object", sentencePath)
				}
				if err := validateTranslations(sentencePath+".translations", sentence.Translations, s.Levels, requireTranslations); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateLevels(levels []string) error {
	seen := map[string]struct{}{}
	for i, level := range levels {
		path := fmt.Sprintf("story.levels[%d]", i)
		if strings.TrimSpace(level) == "" {
			return fmt.Errorf("%s is required", path)
		}
		if !IsSupportedLevel(level) {
			return fmt.Errorf("%s has unsupported level %q", path, level)
		}
		if _, ok := seen[level]; ok {
			return fmt.Errorf("%s duplicates level %q", path, level)
		}
		seen[level] = struct{}{}
	}
	return nil
}

func validateID(path, id string, seen map[string]struct{}) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s is required", path)
	}
	if _, ok := seen[id]; ok {
		return fmt.Errorf("%s duplicates id %q", path, id)
	}
	seen[id] = struct{}{}
	return nil
}

func validateOrderedID(path, id, prefix string, want int) error {
	if !strings.HasPrefix(id, prefix) {
		return nil
	}
	got, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	if err != nil {
		return fmt.Errorf("%s has malformed ordered id %q", path, id)
	}
	if got != want {
		return fmt.Errorf("%s is out of order: got %q, want %s%03d", path, id, prefix, want)
	}
	return nil
}

func validateTranslations(path string, translations map[string]string, levels []string, requireTranslations bool) error {
	for level, text := range translations {
		if !IsSupportedLevel(level) {
			return fmt.Errorf("%s has unsupported level %q", path, level)
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s.%s must not be empty when present", path, level)
		}
	}

	if !requireTranslations {
		return nil
	}

	for _, level := range levels {
		if strings.TrimSpace(translations[level]) == "" {
			return fmt.Errorf("%s.%s is required for a complete story", path, level)
		}
	}
	return nil
}
