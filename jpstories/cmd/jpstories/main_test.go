package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCleanSourceWritesCleanedText(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	in := filepath.Join(storyDir, "sample.txt")
	out := filepath.Join(storyDir, "sample.cleaned.txt")
	if err := os.MkdirAll(storyDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(in, []byte("Hello, said Har-\nry.\nHi.\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runCleanSource([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runCleanSource() error = %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "Hello, said Harry.") {
		t.Fatalf("cleaned text missing repaired hyphenation: %q", got)
	}
}

func TestRunCleanSourceRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "raw.txt")
	out := filepath.Join(dir, "cleaned.txt")
	if err := os.WriteFile(in, []byte("Hello."), 0644); err != nil {
		t.Fatalf("WriteFile(in) error = %v", err)
	}
	if err := os.WriteFile(out, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile(out) error = %v", err)
	}

	err := runCleanSource([]string{"-in", in, "-out", out})
	if err == nil {
		t.Fatal("runCleanSource() error = nil, want overwrite error")
	}
}

func TestRunPrepareStoryCleansChunksAndExportsWorkItems(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "my_story")
	in := filepath.Join(storyDir, "my_story.txt")
	cleaned := filepath.Join(storyDir, "my_story.cleaned.txt")
	storyPath := filepath.Join(storyDir, "my_story.json")
	if err := os.MkdirAll(storyDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(in, []byte("First Har-\nry sentence.\n\"Second sentence.\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := runPrepareStory([]string{
		"-stories", storiesRoot,
		"-story", "my_story",
	})
	if err != nil {
		t.Fatalf("runPrepareStory() error = %v", err)
	}

	cleanedData, err := os.ReadFile(cleaned)
	if err != nil {
		t.Fatalf("ReadFile(cleaned) error = %v", err)
	}
	if !strings.Contains(string(cleanedData), "First Harry sentence.") {
		t.Fatalf("cleaned text missing repaired hyphenation: %q", cleanedData)
	}
	storyData, err := os.ReadFile(storyPath)
	if err != nil {
		t.Fatalf("ReadFile(story) error = %v", err)
	}
	if !strings.Contains(string(storyData), `"id": "my_story"`) {
		t.Fatalf("story did not infer ID from story path: %s", storyData)
	}

	files, err := filepath.Glob(filepath.Join(storyDir, "chunk", "my_story_*.json"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("exported work item count = %d, want 1 grouped chunk file: %#v", len(files), files)
	}
}

func TestRunExportAndImportAgentWork(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	chunkDir := filepath.Join(storyDir, "chunk")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("MkdirAll(chunk) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(chunkDir, "sample_chunk-001.json"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	if err := runExportAgentWork([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runExportAgentWork() error = %v", err)
	}
	sheetPath := filepath.Join(storyDir, "agent", "sample_chunk-001.txt")
	sheetData, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatalf("ReadFile(sheet) error = %v", err)
	}
	filled := strings.Replace(string(sheetData), "native:\n<<<JPSTORIES\nJPSTORIES>>>", "native:\n<<<JPSTORIES\n最初の文です。\nJPSTORIES>>>", 1)
	if err := os.WriteFile(filepath.Join(agentDoneDir, "sample_chunk-001.txt"), []byte(filled), 0644); err != nil {
		t.Fatalf("WriteFile(filled) error = %v", err)
	}

	if err := runImportAgentWork([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runImportAgentWork() error = %v", err)
	}
	doneData, err := os.ReadFile(filepath.Join(storyDir, "done", "sample_chunk-001.json"))
	if err != nil {
		t.Fatalf("ReadFile(done) error = %v", err)
	}
	if !strings.Contains(string(doneData), "最初の文です。") {
		t.Fatalf("done JSON missing imported translation: %s", doneData)
	}
}

func TestRunImportAgentWorkCheckDoesNotWriteDoneJSON(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	chunkDir := filepath.Join(storyDir, "chunk")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	doneDir := filepath.Join(storyDir, "done")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("MkdirAll(chunk) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	sheet := `# jpstories translation sheet v1
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
æœ€åˆã®æ–‡ã§ã™ã€‚
JPSTORIES>>>
`
	if err := os.WriteFile(filepath.Join(chunkDir, "sample_chunk-001.json"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDoneDir, "sample_chunk-001.txt"), []byte(sheet), 0644); err != nil {
		t.Fatalf("WriteFile(sheet) error = %v", err)
	}

	if err := runImportAgentWork([]string{"-stories", storiesRoot, "-story", "sample", "-check"}); err != nil {
		t.Fatalf("runImportAgentWork(-check) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(doneDir, "sample_chunk-001.json")); !os.IsNotExist(err) {
		t.Fatalf("check mode wrote done JSON or stat failed: %v", err)
	}
}

func TestRunAcceptStoryRunsEndToEndGate(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	chunkDir := filepath.Join(storyDir, "chunk")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("MkdirAll(chunk) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	storyJSON := `{
  "id": "sample",
  "title": "Sample",
  "source_language": "en",
  "target_language": "ja",
  "source_file": "stories/sample/sample.txt",
  "levels": ["native"],
  "chunks": [
    {
      "id": "chunk-001",
      "paragraphs": [
        {
          "id": "p-001",
          "sentences": [
            {
              "id": "s-001",
              "english": "First sentence.",
              "translations": {}
            }
          ]
        }
      ]
    }
  ]
}
`
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(storyDir, "sample.json"), []byte(storyJSON), 0644); err != nil {
		t.Fatalf("WriteFile(story) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "sample_chunk-001.json"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := runExportAgentWork([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runExportAgentWork() error = %v", err)
	}
	sheetPath := filepath.Join(storyDir, "agent", "sample_chunk-001.txt")
	sheetData, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatalf("ReadFile(sheet) error = %v", err)
	}
	filled := strings.Replace(string(sheetData), "native:\n<<<JPSTORIES\nJPSTORIES>>>", "native:\n<<<JPSTORIES\n最初の文です。\nJPSTORIES>>>", 1)
	if err := os.WriteFile(filepath.Join(agentDoneDir, "sample_chunk-001.txt"), []byte(filled), 0644); err != nil {
		t.Fatalf("WriteFile(filled) error = %v", err)
	}

	if err := runAcceptStory([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runAcceptStory() error = %v", err)
	}
	merged, err := os.ReadFile(filepath.Join(storyDir, "sample.json"))
	if err != nil {
		t.Fatalf("ReadFile(merged) error = %v", err)
	}
	if !strings.Contains(string(merged), "最初の文です。") {
		t.Fatalf("merged story missing accepted translation: %s", merged)
	}
}

func TestRunAcceptStoryRepairAgentSheetsFlag(t *testing.T) {
	t.Run("strict by default", func(t *testing.T) {
		storiesRoot := setupAcceptStoryWithMalformedDoneSheet(t)
		err := runAcceptStory([]string{"-stories", storiesRoot, "-story", "sample"})
		if err == nil {
			t.Fatal("runAcceptStory() error = nil, want malformed sheet failure")
		}
		if !strings.Contains(err.Error(), "agent sheet validation failed") {
			t.Fatalf("runAcceptStory() error = %v, want validation failure", err)
		}
	})

	t.Run("repairs when requested", func(t *testing.T) {
		storiesRoot := setupAcceptStoryWithMalformedDoneSheet(t)
		err := runAcceptStory([]string{
			"-stories", storiesRoot,
			"-story", "sample",
			"-repair-agent-sheets",
		})
		if err != nil {
			t.Fatalf("runAcceptStory(-repair-agent-sheets) error = %v", err)
		}
		merged, err := os.ReadFile(filepath.Join(storiesRoot, "sample", "sample.json"))
		if err != nil {
			t.Fatalf("ReadFile(merged) error = %v", err)
		}
		if !strings.Contains(string(merged), "translation") {
			t.Fatalf("merged story missing repaired translation: %s", merged)
		}
	})
}

func TestRunValidateAgentWorkCanGateOneAssignedSheet(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	agentDir := filepath.Join(storyDir, "agent")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	sheet := `# jpstories translation sheet v1
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

JPSTORIES>>>
`
	filled := strings.Replace(sheet, "native:\n<<<JPSTORIES\n\nJPSTORIES>>>", "native:\n<<<JPSTORIES\næœ€åˆã®æ–‡ã§ã™ã€‚\nJPSTORIES>>>", 1)
	if err := os.WriteFile(filepath.Join(agentDir, "sample_chunk-001.txt"), []byte(sheet), 0644); err != nil {
		t.Fatalf("WriteFile(source 1) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDoneDir, "sample_chunk-001.txt"), []byte(filled), 0644); err != nil {
		t.Fatalf("WriteFile(done 1) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "sample_chunk-002.txt"), []byte(strings.Replace(sheet, "chunk-001", "chunk-002", -1)), 0644); err != nil {
		t.Fatalf("WriteFile(source 2) error = %v", err)
	}

	err := runValidateAgentWork([]string{
		"-stories", storiesRoot,
		"-story", "sample",
		"sample_chunk-001.txt",
	})
	if err != nil {
		t.Fatalf("runValidateAgentWork() error = %v", err)
	}
}

func setupAcceptStoryWithMalformedDoneSheet(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	chunkDir := filepath.Join(storyDir, "chunk")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("MkdirAll(chunk) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	storyJSON := `{
  "id": "sample",
  "title": "Sample",
  "source_language": "en",
  "target_language": "ja",
  "source_file": "stories/sample/sample.txt",
  "levels": ["native"],
  "chunks": [
    {
      "id": "chunk-001",
      "paragraphs": [
        {
          "id": "p-001",
          "sentences": [
            {
              "id": "s-001",
              "english": "First sentence.",
              "translations": {}
            }
          ]
        }
      ]
    }
  ]
}
`
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(storyDir, "sample.json"), []byte(storyJSON), 0644); err != nil {
		t.Fatalf("WriteFile(story) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "sample_chunk-001.json"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := runExportAgentWork([]string{"-stories", storiesRoot, "-story", "sample"}); err != nil {
		t.Fatalf("runExportAgentWork() error = %v", err)
	}
	sheetPath := filepath.Join(storyDir, "agent", "sample_chunk-001.txt")
	sheetData, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatalf("ReadFile(sheet) error = %v", err)
	}
	filled := strings.Replace(string(sheetData), "native:\n<<<JPSTORIES\nJPSTORIES>>>", "native:\n<<<JPSTORIES\ntranslation\nJPSTORIES>>>", 1)
	lastFence := strings.LastIndex(filled, "JPSTORIES>>>")
	if lastFence < 0 {
		t.Fatalf("filled sheet missing closing fence:\n%s", filled)
	}
	malformed := filled[:lastFence] + filled[lastFence+len("JPSTORIES>>>"):]
	if err := os.WriteFile(filepath.Join(agentDoneDir, "sample_chunk-001.txt"), []byte(malformed), 0644); err != nil {
		t.Fatalf("WriteFile(filled) error = %v", err)
	}
	return storiesRoot
}

func TestRunValidateWorkItemsStoryMode(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	chunkDir := filepath.Join(storyDir, "chunk")
	doneDir := filepath.Join(storyDir, "done")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("MkdirAll(chunk) error = %v", err)
	}
	if err := os.MkdirAll(doneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(done) error = %v", err)
	}
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	done := strings.Replace(source, `"native": ""`, `"native": "translation"`, 1)
	if err := os.WriteFile(filepath.Join(chunkDir, "sample_chunk-001.json"), []byte(source), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doneDir, "sample_chunk-001.json"), []byte(done), 0644); err != nil {
		t.Fatalf("WriteFile(done) error = %v", err)
	}

	err := runValidateWorkItems([]string{
		"-stories", storiesRoot,
		"-story", "sample",
	})
	if err != nil {
		t.Fatalf("runValidateWorkItems() error = %v", err)
	}
}

func TestRunValidateWorkItemsSingleFileFixesBOM(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "sample_chunk-001.json")
	donePath := filepath.Join(dir, "done_sample_chunk-001.json")
	source := `{
  "story_id": "sample",
  "story_title": "Sample",
  "chunk_id": "chunk-001",
  "levels": ["native"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "First sentence.",
          "native": ""
        }
      ]
    }
  ]
}
`
	done := strings.Replace(source, `"native": ""`, `"native": "translation"`, 1)
	if err := os.WriteFile(sourcePath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(source)...), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(donePath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(done)...), 0644); err != nil {
		t.Fatalf("WriteFile(done) error = %v", err)
	}

	err := runValidateWorkItems([]string{
		"-input-path", sourcePath,
		"-output-path", donePath,
		"-fix-bom",
	})
	if err != nil {
		t.Fatalf("runValidateWorkItems() error = %v", err)
	}
	assertFileHasNoUTF8BOM(t, sourcePath)
	assertFileHasNoUTF8BOM(t, donePath)
}

func TestRunRepairAgentSheetsStoryMode(t *testing.T) {
	dir := t.TempDir()
	storiesRoot := filepath.Join(dir, "stories")
	storyDir := filepath.Join(storiesRoot, "sample")
	agentDir := filepath.Join(storyDir, "agent")
	agentDoneDir := filepath.Join(storyDir, "agent-done")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent) error = %v", err)
	}
	if err := os.MkdirAll(agentDoneDir, 0755); err != nil {
		t.Fatalf("MkdirAll(agent-done) error = %v", err)
	}
	sheet := `# jpstories translation sheet v1
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

JPSTORIES>>>
`
	done := strings.TrimSuffix(strings.Replace(sheet, "native:\n<<<JPSTORIES\n\nJPSTORIES>>>", "native:\n<<<JPSTORIES\ntranslation\nJPSTORIES>>>", 1), "JPSTORIES>>>\n")
	if err := os.WriteFile(filepath.Join(agentDir, "sample_chunk-001.txt"), []byte(sheet), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	donePath := filepath.Join(agentDoneDir, "sample_chunk-001.txt")
	if err := os.WriteFile(donePath, []byte(done), 0644); err != nil {
		t.Fatalf("WriteFile(done) error = %v", err)
	}

	err := runRepairAgentSheets([]string{
		"-stories", storiesRoot,
		"-story", "sample",
	})
	if err != nil {
		t.Fatalf("runRepairAgentSheets() error = %v", err)
	}
	data, err := os.ReadFile(donePath)
	if err != nil {
		t.Fatalf("ReadFile(done) error = %v", err)
	}
	if !strings.HasSuffix(string(data), "JPSTORIES>>>") {
		t.Fatalf("repair did not restore closing fence:\n%s", data)
	}
}

func assertFileHasNoUTF8BOM(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		t.Fatalf("%s still has UTF-8 BOM", path)
	}
}
