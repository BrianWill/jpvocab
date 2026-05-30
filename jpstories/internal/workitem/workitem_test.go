package workitem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jpstories/internal/story"
)

func TestExportWritesOneGroupedWorkItemPerMissingChunk(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "story.json")
	outDir := filepath.Join(dir, "work")
	s := fixtureStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[story.LevelNative] = "already translated"
	if err := story.SaveFile(storyPath, s); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	result, err := Export(ExportOptions{
		StoryPath: storyPath,
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(result.Files))
	}
	if filepath.Base(result.Files[0]) != "sample_chunk-001.json" {
		t.Fatalf("file = %q, want grouped chunk file", result.Files[0])
	}

	item := readTestWorkItem(t, result.Files[0])
	if item.StoryID != "sample" {
		t.Fatalf("StoryID = %q, want sample", item.StoryID)
	}
	if item.ChunkID != "chunk-001" {
		t.Fatalf("ChunkID = %q, want chunk-001", item.ChunkID)
	}
	if got, want := strings.Join(item.Levels, ","), "native,n3,n2_abridged"; got != want {
		t.Fatalf("Levels = %q, want %q", got, want)
	}
	if len(item.Paragraphs) != 1 || len(item.Paragraphs[0].Sentences) != 2 {
		t.Fatalf("work item should include full chunk source context: %#v", item.Paragraphs)
	}
	sentences := item.Paragraphs[0].Sentences
	if _, ok := sentences[0].Translations[story.LevelNative]; ok {
		t.Fatal("s-001 includes already-translated native field")
	}
	if _, ok := sentences[1].Translations[story.LevelNative]; !ok {
		t.Fatal("s-002 missing untranslated native field")
	}
	if _, ok := sentences[0].Translations[story.LevelN3]; !ok {
		t.Fatal("s-001 missing untranslated n3 field")
	}

	data, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal(raw) error = %v", err)
	}
	if _, ok := raw["instructions"]; ok {
		t.Fatal("exported work item includes repeated instructions")
	}
	if _, ok := raw["translations"]; ok {
		t.Fatal("exported grouped work item includes top-level translations object")
	}
	if _, ok := raw["level"]; ok {
		t.Fatal("exported grouped work item includes old single level field")
	}
}

func TestExportSpecificLevelWritesGroupedChunkWithOnlyThatLevel(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "story.json")
	outDir := filepath.Join(dir, "work")
	if err := story.SaveFile(storyPath, fixtureStory()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	result, err := Export(ExportOptions{
		StoryPath: storyPath,
		OutputDir: outDir,
		Level:     story.LevelN3,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(result.Files))
	}
	item := readTestWorkItem(t, result.Files[0])
	if got, want := strings.Join(item.Levels, ","), story.LevelN3; got != want {
		t.Fatalf("Levels = %q, want %q", got, want)
	}
	for _, sentence := range item.Paragraphs[0].Sentences {
		if _, ok := sentence.Translations[story.LevelN3]; !ok {
			t.Fatalf("sentence %s missing n3 field", sentence.ID)
		}
		if _, ok := sentence.Translations[story.LevelNative]; ok {
			t.Fatalf("sentence %s includes native field", sentence.ID)
		}
	}
}

func TestExportAllLevelsSkipsCompleteLevels(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "story.json")
	outDir := filepath.Join(dir, "work")
	s := fixtureStory()
	for i := range s.Chunks[0].Paragraphs[0].Sentences {
		s.Chunks[0].Paragraphs[0].Sentences[i].Translations[story.LevelNative] = "native"
	}
	if err := story.SaveFile(storyPath, s); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	result, err := Export(ExportOptions{
		StoryPath: storyPath,
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(result.Files))
	}
	item := readTestWorkItem(t, result.Files[0])
	if got, want := strings.Join(item.Levels, ","), "n3,n2_abridged"; got != want {
		t.Fatalf("Levels = %q, want %q", got, want)
	}
	for _, sentence := range item.Paragraphs[0].Sentences {
		if _, ok := sentence.Translations[story.LevelNative]; ok {
			t.Fatalf("exported complete native level for %s", sentence.ID)
		}
	}
}

func TestMergeUpdatesReferencedLevelsAndPreservesOthers(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "story.json")
	inDir := filepath.Join(dir, "done")
	s := fixtureStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[story.LevelNative] = "old native"
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[story.LevelN3] = "existing n3"
	if err := story.SaveFile(storyPath, s); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if err := os.MkdirAll(inDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestWorkItem(t, filepath.Join(inDir, "done.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-001",
		Levels:     []string{story.LevelNative, story.LevelN2Abridged},
		Paragraphs: []WorkParagraph{
			{
				ID: "p-001",
				Sentences: []WorkSentence{
					{
						ID:      "s-001",
						English: "First sentence.",
						Translations: map[string]string{
							story.LevelNative: "new native",
						},
					},
					{
						ID:      "s-002",
						English: "Second sentence.",
						Translations: map[string]string{
							story.LevelNative:     "second native",
							story.LevelN2Abridged: "second n2",
						},
					},
				},
			},
		},
	})

	result, err := Merge(MergeOptions{
		StoryPath: storyPath,
		InputDir:  inDir,
	})
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if result.FilesMerged != 1 || result.TranslationsMerged != 3 {
		t.Fatalf("Merge() result = %#v, want 1 file and 3 translations", result)
	}

	updated, err := story.LoadFile(storyPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	sentences := updated.Chunks[0].Paragraphs[0].Sentences
	if sentences[0].Translations[story.LevelNative] != "new native" {
		t.Fatalf("s-001 native = %q", sentences[0].Translations[story.LevelNative])
	}
	if sentences[0].Translations[story.LevelN3] != "existing n3" {
		t.Fatalf("s-001 n3 = %q, want preserved existing n3", sentences[0].Translations[story.LevelN3])
	}
	if sentences[1].Translations[story.LevelNative] != "second native" {
		t.Fatalf("s-002 native = %q", sentences[1].Translations[story.LevelNative])
	}
	if sentences[1].Translations[story.LevelN2Abridged] != "second n2" {
		t.Fatalf("s-002 n2_abridged = %q", sentences[1].Translations[story.LevelN2Abridged])
	}
}

func TestMergeRejectsMalformedAgentOutput(t *testing.T) {
	tests := []struct {
		name string
		item WorkItem
		want string
	}{
		{
			name: "wrong story",
			item: WorkItem{
				StoryID: "other",
				ChunkID: "chunk-001",
				Levels:  []string{story.LevelNative},
				Paragraphs: []WorkParagraph{{Sentences: []WorkSentence{{
					ID:           "s-001",
					Translations: map[string]string{story.LevelNative: "translation"},
				}}}},
			},
			want: "story_id mismatch",
		},
		{
			name: "unknown sentence",
			item: WorkItem{
				StoryID: "sample",
				ChunkID: "chunk-001",
				Levels:  []string{story.LevelNative},
				Paragraphs: []WorkParagraph{{Sentences: []WorkSentence{{
					ID:           "s-999",
					Translations: map[string]string{story.LevelNative: "translation"},
				}}}},
			},
			want: "sentence \"s-999\" not found",
		},
		{
			name: "empty translation",
			item: WorkItem{
				StoryID: "sample",
				ChunkID: "chunk-001",
				Levels:  []string{story.LevelNative},
				Paragraphs: []WorkParagraph{{Sentences: []WorkSentence{{
					ID:           "s-001",
					Translations: map[string]string{story.LevelNative: " "},
				}}}},
			},
			want: "translation for sentence \"s-001\" level \"native\" is empty",
		},
		{
			name: "level not listed",
			item: WorkItem{
				StoryID: "sample",
				ChunkID: "chunk-001",
				Levels:  []string{story.LevelNative},
				Paragraphs: []WorkParagraph{{Sentences: []WorkSentence{{
					ID:           "s-001",
					Translations: map[string]string{story.LevelN3: "translation"},
				}}}},
			},
			want: "includes level \"n3\" not listed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			storyPath := filepath.Join(dir, "story.json")
			inDir := filepath.Join(dir, "done")
			if err := story.SaveFile(storyPath, fixtureStory()); err != nil {
				t.Fatalf("SaveFile() error = %v", err)
			}
			if err := os.MkdirAll(inDir, 0755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			writeTestWorkItem(t, filepath.Join(inDir, "done.json"), tt.item)

			_, err := Merge(MergeOptions{
				StoryPath: storyPath,
				InputDir:  inDir,
			})
			if err == nil {
				t.Fatal("Merge() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Merge() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMergeRejectsUnknownWorkItemFields(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "story.json")
	inDir := filepath.Join(dir, "done")
	if err := story.SaveFile(storyPath, fixtureStory()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if err := os.MkdirAll(inDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data := []byte(`{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "level": "native",
  "paragraphs": [],
  "translations": { "s-001": "new native" }
}
`)
	if err := os.WriteFile(filepath.Join(inDir, "done.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Merge(MergeOptions{
		StoryPath: storyPath,
		InputDir:  inDir,
	})
	if err == nil {
		t.Fatal("Merge() error = nil")
	}
	if !strings.Contains(err.Error(), "unknown field \"level\"") {
		t.Fatalf("Merge() error = %v, want unknown level field", err)
	}
}

func readTestWorkItem(t *testing.T, path string) WorkItem {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var item WorkItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return item
}

func writeTestWorkItem(t *testing.T, path string, item WorkItem) {
	t.Helper()
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func fixtureStory() story.Story {
	return story.Story{
		ID:             "sample",
		Title:          "Sample",
		SourceLanguage: "en",
		TargetLanguage: "ja",
		SourceFile:     "stories/sample/sample.txt",
		Levels:         []string{story.LevelNative, story.LevelN3, story.LevelN2Abridged},
		Chunks: []story.Chunk{
			{
				ID: "chunk-001",
				Paragraphs: []story.Paragraph{
					{
						ID: "p-001",
						Sentences: []story.Sentence{
							{
								ID:           "s-001",
								English:      "First sentence.",
								Translations: map[string]string{},
							},
							{
								ID:           "s-002",
								English:      "Second sentence.",
								Translations: map[string]string{},
							},
						},
					},
				},
			},
		},
	}
}
