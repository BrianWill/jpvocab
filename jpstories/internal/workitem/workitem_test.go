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
	if got, want := strings.Join(item.Levels, ","), "native,n3,n3_abridged"; got != want {
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
	if got, want := strings.Join(item.Levels, ","), "n3,n3_abridged"; got != want {
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
		Levels:     []string{story.LevelNative, story.LevelN3Abridged},
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
							story.LevelN3Abridged: "second n3 abridged",
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
	if sentences[1].Translations[story.LevelN3Abridged] != "second n3 abridged" {
		t.Fatalf("s-002 n3_abridged = %q", sentences[1].Translations[story.LevelN3Abridged])
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

func TestExportSheetsWritesTranslatorFriendlyText(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "chunk")
	outDir := filepath.Join(dir, "agent")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestWorkItem(t, filepath.Join(sourceDir, "sample_chunk-001.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-001",
		Levels:     []string{story.LevelNative, story.LevelN3},
		Paragraphs: []WorkParagraph{{
			ID: "p-001",
			Sentences: []WorkSentence{{
				ID:      "s-001",
				English: "“Hello,” she said.",
				Translations: map[string]string{
					story.LevelNative: "",
					story.LevelN3:     "",
				},
			}},
		}},
	})

	result, err := ExportSheets(ExportSheetsOptions{
		SourceDir: sourceDir,
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("ExportSheets() error = %v", err)
	}
	if len(result.Files) != 1 || filepath.Base(result.Files[0]) != "sample_chunk-001.txt" {
		t.Fatalf("ExportSheets() files = %#v, want one matching txt file", result.Files)
	}
	data, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"# jpstories translation sheet v1",
		"source_file: sample_chunk-001.json",
		"## p-001 / s-001",
		"“Hello,” she said.",
		"native:",
		"n3:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sheet missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"story_id"`) {
		t.Fatalf("sheet looks like JSON:\n%s", text)
	}
}

func TestImportSheetsWritesCompletedJSONFromFilledSheet(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "chunk")
	inputDir := filepath.Join(dir, "agent-done")
	outputDir := filepath.Join(dir, "done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	writeTestWorkItem(t, filepath.Join(sourceDir, "sample_chunk-001.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-001",
		Levels:     []string{story.LevelNative, story.LevelN3},
		Paragraphs: []WorkParagraph{{
			ID: "p-001",
			Sentences: []WorkSentence{{
				ID:      "s-001",
				English: "First sentence.",
				Translations: map[string]string{
					story.LevelNative: "",
					story.LevelN3:     "",
				},
			}},
		}},
	})
	sheet := `# jpstories translation sheet v1
story_id: sample
story_title: Sample
chunk_id: chunk-001
levels: native,n3
source_file: sample_chunk-001.json

Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels.

## p-001 / s-001
english:
<<<JPSTORIES
First sentence.
JPSTORIES>>>
native:
<<<JPSTORIES
最初の文です。
JPSTORIES>>>
n3:
<<<JPSTORIES
はじめの文です。
JPSTORIES>>>
`
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(sheet), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := ImportSheets(ImportSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("ImportSheets() error = %v", err)
	}
	if len(result.Files) != 1 || filepath.Base(result.Files[0]) != "sample_chunk-001.json" {
		t.Fatalf("ImportSheets() files = %#v, want one matching JSON file", result.Files)
	}
	item := readTestWorkItem(t, result.Files[0])
	translations := item.Paragraphs[0].Sentences[0].Translations
	if translations[story.LevelNative] != "最初の文です。" {
		t.Fatalf("native = %q", translations[story.LevelNative])
	}
	if translations[story.LevelN3] != "はじめの文です。" {
		t.Fatalf("n3 = %q", translations[story.LevelN3])
	}
}

func TestImportSheetsCheckAggregatesDiagnosticsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "chunk")
	inputDir := filepath.Join(dir, "agent-done")
	outputDir := filepath.Join(dir, "done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	writeTestWorkItem(t, filepath.Join(sourceDir, "sample_chunk-001.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-001",
		Levels:     []string{story.LevelNative, story.LevelN3},
		Paragraphs: []WorkParagraph{{
			ID: "p-001",
			Sentences: []WorkSentence{{
				ID:      "s-001",
				English: "First sentence.",
				Translations: map[string]string{
					story.LevelNative: "",
					story.LevelN3:     "",
				},
			}},
		}},
	})
	writeTestWorkItem(t, filepath.Join(sourceDir, "sample_chunk-002.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-002",
		Levels:     []string{story.LevelNative},
		Paragraphs: []WorkParagraph{{
			ID: "p-001",
			Sentences: []WorkSentence{{
				ID:      "s-002",
				English: "Second sentence.",
				Translations: map[string]string{
					story.LevelNative: "",
				},
			}},
		}},
	})
	badOne := strings.Replace(testTwoLevelSheet("", "ã¯ã˜ã‚ã®æ–‡ã§ã™ã€‚"), "First sentence.", "Changed sentence.", 1)
	badOne = strings.Replace(badOne, "n3:\n<<<JPSTORIES", "unexpected:\n<<<JPSTORIES\nextra\nJPSTORIES>>>\nn3:\n<<<JPSTORIES", 1)
	badTwo := strings.Replace(testSheet("äºŒç•ªç›®ã®æ–‡ã§ã™ã€‚"), "chunk-001", "chunk-002", -1)
	badTwo = strings.Replace(badTwo, "s-001", "s-002", -1)
	badTwo = strings.Replace(badTwo, "First sentence.", "Second sentence.", 1)
	badTwo = strings.Replace(badTwo, "JPSTORIES>>>\n", "", 1)
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(badOne), 0644); err != nil {
		t.Fatalf("WriteFile(bad one) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-002.txt"), []byte(badTwo), 0644); err != nil {
		t.Fatalf("WriteFile(bad two) error = %v", err)
	}

	result, err := ImportSheets(ImportSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
		OutputDir: outputDir,
		Check:     true,
	})
	if err == nil {
		t.Fatal("ImportSheets() error = nil, want aggregate diagnostics")
	}
	got := validationMessages(result.Failures)
	for _, want := range []string{
		"sentence s-001: changed English text",
		"sentence s-001 level native: empty translation",
		"sentence s-001 level unexpected: extra unknown block",
		"missing closing fence",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("failures missing %q:\n%s", want, got)
		}
	}
	if result.FilesValidated != 2 {
		t.Fatalf("FilesValidated = %d, want 2", result.FilesValidated)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "sample_chunk-001.json")); !os.IsNotExist(statErr) {
		t.Fatalf("check mode wrote output or stat failed: %v", statErr)
	}
}

func TestImportSheetsCheckSucceedsWithoutOutputDir(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "chunk")
	inputDir := filepath.Join(dir, "agent-done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	writeTestWorkItem(t, filepath.Join(sourceDir, "sample_chunk-001.json"), WorkItem{
		StoryID:    "sample",
		StoryTitle: "Sample",
		ChunkID:    "chunk-001",
		Levels:     []string{story.LevelNative},
		Paragraphs: []WorkParagraph{{
			ID: "p-001",
			Sentences: []WorkSentence{{
				ID:      "s-001",
				English: "First sentence.",
				Translations: map[string]string{
					story.LevelNative: "",
				},
			}},
		}},
	})
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(testSheet("æœ€åˆã®æ–‡ã§ã™ã€‚")), 0644); err != nil {
		t.Fatalf("WriteFile(sheet) error = %v", err)
	}

	result, err := ImportSheets(ImportSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
		Check:     true,
	})
	if err != nil {
		t.Fatalf("ImportSheets() error = %v", err)
	}
	if result.FilesValidated != 1 || len(result.Files) != 1 {
		t.Fatalf("ImportSheets() result = %#v, want one ready file", result)
	}
}

func TestValidateSheetsAcceptsCompletedSheets(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "agent")
	inputDir := filepath.Join(dir, "agent-done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}

	source := testSheet("")
	completed := testSheet("æœ€åˆã®æ–‡ã§ã™ã€‚")
	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-001.txt"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(completed), 0644); err != nil {
		t.Fatalf("WriteFile(completed) error = %v", err)
	}

	result, err := ValidateSheets(ValidateSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
	})
	if err != nil {
		t.Fatalf("ValidateSheets() error = %v", err)
	}
	if result.FilesValidated != 1 {
		t.Fatalf("FilesValidated = %d, want 1", result.FilesValidated)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %#v, want none", result.Failures)
	}
}

func TestValidateSheetsReportsAllFilesAndStrictFailures(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "agent")
	inputDir := filepath.Join(dir, "agent-done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-001.txt"), []byte(testSheet("")), 0644); err != nil {
		t.Fatalf("WriteFile(source 1) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-002.txt"), []byte(strings.Replace(testSheet(""), "chunk-001", "chunk-002", -1)), 0644); err != nil {
		t.Fatalf("WriteFile(source 2) error = %v", err)
	}
	bad := strings.Replace(testSheet("æœ€åˆã®æ–‡ã§ã™ã€‚"), "First sentence.", "Changed sentence.", 1)
	bad = strings.Replace(bad, "native:\n<<<JPSTORIES", "native:\n<<<JPSTORIES\nextra\nJPSTORIES>>>\nnative:\n<<<JPSTORIES", 1)
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(bad), 0644); err != nil {
		t.Fatalf("WriteFile(bad) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "extra.txt"), []byte(testSheet("translation")), 0644); err != nil {
		t.Fatalf("WriteFile(extra) error = %v", err)
	}

	result, err := ValidateSheets(ValidateSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
	})
	if err != nil {
		t.Fatalf("ValidateSheets() error = %v", err)
	}
	got := validationMessages(result.Failures)
	for _, want := range []string{
		"duplicate native block",
		"english text changed",
		"missing completed sheet",
		"extra completed sheet",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("failures missing %q:\n%s", want, got)
		}
	}
}

func TestValidateSheetsCanGateAssignedFilesOnly(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "agent")
	inputDir := filepath.Join(dir, "agent-done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-001.txt"), []byte(testSheet("")), 0644); err != nil {
		t.Fatalf("WriteFile(source 1) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-002.txt"), []byte(strings.Replace(testSheet(""), "chunk-001", "chunk-002", -1)), 0644); err != nil {
		t.Fatalf("WriteFile(source 2) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_chunk-001.txt"), []byte(testSheet("æœ€åˆã®æ–‡ã§ã™ã€‚")), 0644); err != nil {
		t.Fatalf("WriteFile(done 1) error = %v", err)
	}

	result, err := ValidateSheets(ValidateSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
		Files:     []string{"sample_chunk-001"},
	})
	if err != nil {
		t.Fatalf("ValidateSheets() error = %v", err)
	}
	if result.FilesValidated != 1 {
		t.Fatalf("FilesValidated = %d, want 1", result.FilesValidated)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %#v, want none", result.Failures)
	}
	if len(result.Files) != 1 || result.Files[0].Status != "ok" {
		t.Fatalf("Files = %#v, want one ok file", result.Files)
	}
}

func TestValidateSheetsAssignedFileReportsMissingSource(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "agent")
	inputDir := filepath.Join(dir, "agent-done")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "sample_chunk-001.txt"), []byte(testSheet("")), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	result, err := ValidateSheets(ValidateSheetsOptions{
		SourceDir: sourceDir,
		InputDir:  inputDir,
		Files:     []string{"sample_chunk-999.txt"},
	})
	if err != nil {
		t.Fatalf("ValidateSheets() error = %v", err)
	}
	got := validationMessages(result.Failures)
	if !strings.Contains(got, "assigned source sheet not found") {
		t.Fatalf("failures = %s, want missing source", got)
	}
	if len(result.Files) != 1 || result.Files[0].Status != "missing-source" {
		t.Fatalf("Files = %#v, want missing-source", result.Files)
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

func validationMessages(failures []SheetValidationFailure) string {
	var b strings.Builder
	for _, failure := range failures {
		b.WriteString(failure.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func testSheet(native string) string {
	return `# jpstories translation sheet v1
story_id: sample
story_title: Sample
chunk_id: chunk-001
levels: native
source_file: sample_chunk-001.json

Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels.

## p-001 / s-001
english:
<<<JPSTORIES
First sentence.
JPSTORIES>>>
native:
<<<JPSTORIES
` + native + `
JPSTORIES>>>
`
}

func testTwoLevelSheet(native string, n3 string) string {
	return `# jpstories translation sheet v1
story_id: sample
story_title: Sample
chunk_id: chunk-001
levels: native,n3
source_file: sample_chunk-001.json

Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels.

## p-001 / s-001
english:
<<<JPSTORIES
First sentence.
JPSTORIES>>>
native:
<<<JPSTORIES
` + native + `
JPSTORIES>>>
n3:
<<<JPSTORIES
` + n3 + `
JPSTORIES>>>
`
}

func fixtureStory() story.Story {
	return story.Story{
		ID:             "sample",
		Title:          "Sample",
		SourceLanguage: "en",
		TargetLanguage: "ja",
		SourceFile:     "stories/sample/sample.txt",
		Levels:         []string{story.LevelNative, story.LevelN3, story.LevelN3Abridged},
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
