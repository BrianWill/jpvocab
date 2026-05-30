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
