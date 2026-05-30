package workitem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jpstories/internal/story"
)

type ExportOptions struct {
	StoryPath string
	OutputDir string
	Level     string
	ChunkID   string
}

type MergeOptions struct {
	StoryPath string
	InputDir  string
}

type WorkItem struct {
	StoryID    string          `json:"story_id"`
	StoryTitle string          `json:"story_title"`
	ChunkID    string          `json:"chunk_id"`
	Levels     []string        `json:"levels"`
	Paragraphs []WorkParagraph `json:"paragraphs"`
}

type WorkParagraph struct {
	ID        string         `json:"id"`
	Sentences []WorkSentence `json:"sentences"`
}

type WorkSentence struct {
	ID           string
	English      string
	Translations map[string]string
}

type ExportResult struct {
	Files []string
}

type MergeResult struct {
	FilesMerged        int
	TranslationsMerged int
}

func Export(opts ExportOptions) (ExportResult, error) {
	if strings.TrimSpace(opts.StoryPath) == "" {
		return ExportResult{}, fmt.Errorf("story path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return ExportResult{}, fmt.Errorf("output directory is required")
	}

	s, err := story.LoadFile(opts.StoryPath)
	if err != nil {
		return ExportResult{}, err
	}

	levels, err := levelsToExport(s, opts.Level)
	if err != nil {
		return ExportResult{}, err
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return ExportResult{}, err
	}

	var files []string
	for _, chunk := range s.Chunks {
		if opts.ChunkID != "" && chunk.ID != opts.ChunkID {
			continue
		}
		item, ok := buildWorkItem(s, chunk, levels)
		if !ok {
			continue
		}
		path := filepath.Join(opts.OutputDir, workItemFileName(s.ID, chunk.ID))
		if err := writeWorkItem(path, item); err != nil {
			return ExportResult{}, err
		}
		files = append(files, path)
	}

	if opts.ChunkID != "" && !hasChunk(s, opts.ChunkID) {
		return ExportResult{}, fmt.Errorf("chunk %q not found", opts.ChunkID)
	}

	sort.Strings(files)
	return ExportResult{Files: files}, nil
}

func Merge(opts MergeOptions) (MergeResult, error) {
	if strings.TrimSpace(opts.StoryPath) == "" {
		return MergeResult{}, fmt.Errorf("story path is required")
	}
	if strings.TrimSpace(opts.InputDir) == "" {
		return MergeResult{}, fmt.Errorf("input directory is required")
	}

	s, err := story.LoadFile(opts.StoryPath)
	if err != nil {
		return MergeResult{}, err
	}

	paths, err := workItemPaths(opts.InputDir)
	if err != nil {
		return MergeResult{}, err
	}
	if len(paths) == 0 {
		return MergeResult{}, fmt.Errorf("no work item JSON files found in %s", opts.InputDir)
	}

	var result MergeResult
	for _, path := range paths {
		item, err := readWorkItem(path)
		if err != nil {
			return MergeResult{}, err
		}
		merged, err := mergeWorkItem(&s, item)
		if err != nil {
			return MergeResult{}, fmt.Errorf("merge %s: %w", path, err)
		}
		result.FilesMerged++
		result.TranslationsMerged += merged
	}

	if err := story.SaveFile(opts.StoryPath, s); err != nil {
		return MergeResult{}, err
	}
	return result, nil
}

func buildWorkItem(s story.Story, chunk story.Chunk, levels []string) (WorkItem, bool) {
	item := WorkItem{
		StoryID:    s.ID,
		StoryTitle: s.Title,
		ChunkID:    chunk.ID,
	}
	missingLevels := map[string]bool{}

	for _, paragraph := range chunk.Paragraphs {
		workParagraph := WorkParagraph{
			ID: paragraph.ID,
		}
		for _, sentence := range paragraph.Sentences {
			workSentence := WorkSentence{
				ID:           sentence.ID,
				English:      sentence.English,
				Translations: map[string]string{},
			}
			for _, level := range levels {
				if strings.TrimSpace(sentence.Translations[level]) == "" {
					workSentence.Translations[level] = ""
					missingLevels[level] = true
				}
			}
			workParagraph.Sentences = append(workParagraph.Sentences, workSentence)
		}
		item.Paragraphs = append(item.Paragraphs, workParagraph)
	}

	item.Levels = includedLevels(levels, missingLevels)
	return item, len(item.Levels) > 0
}

func levelsToExport(s story.Story, requested string) ([]string, error) {
	if requested != "" {
		if !story.IsSupportedLevel(requested) {
			return nil, fmt.Errorf("unsupported level %q", requested)
		}
		if !storyHasLevel(s, requested) {
			return nil, fmt.Errorf("story does not include level %q", requested)
		}
		return []string{requested}, nil
	}
	return append([]string(nil), s.Levels...), nil
}

func mergeWorkItem(s *story.Story, item WorkItem) (int, error) {
	if item.StoryID != s.ID {
		return 0, fmt.Errorf("story_id mismatch: got %q, want %q", item.StoryID, s.ID)
	}
	if strings.TrimSpace(item.ChunkID) == "" {
		return 0, fmt.Errorf("chunk_id is required")
	}
	if len(item.Levels) == 0 {
		return 0, fmt.Errorf("levels must include at least one translation level")
	}
	levelSet := map[string]bool{}
	for _, level := range item.Levels {
		if !story.IsSupportedLevel(level) {
			return 0, fmt.Errorf("unsupported level %q", level)
		}
		if !storyHasLevel(*s, level) {
			return 0, fmt.Errorf("story does not include level %q", level)
		}
		if levelSet[level] {
			return 0, fmt.Errorf("duplicate level %q", level)
		}
		levelSet[level] = true
	}

	chunk := findChunk(s, item.ChunkID)
	if chunk == nil {
		return 0, fmt.Errorf("chunk %q not found", item.ChunkID)
	}

	sentences := sentencesByID(chunk)
	merged := 0
	for _, paragraph := range item.Paragraphs {
		for _, workSentence := range paragraph.Sentences {
			sentence := sentences[workSentence.ID]
			if sentence == nil {
				return 0, fmt.Errorf("sentence %q not found in chunk %q", workSentence.ID, item.ChunkID)
			}
			for level, translation := range workSentence.Translations {
				if !levelSet[level] {
					return 0, fmt.Errorf("sentence %q includes level %q not listed in levels", workSentence.ID, level)
				}
				translation = strings.TrimSpace(translation)
				if translation == "" {
					return 0, fmt.Errorf("translation for sentence %q level %q is empty", workSentence.ID, level)
				}
				if sentence.Translations == nil {
					sentence.Translations = map[string]string{}
				}
				sentence.Translations[level] = translation
				merged++
			}
		}
	}

	if merged == 0 {
		return 0, fmt.Errorf("no translations merged")
	}
	return merged, nil
}

func writeWorkItem(path string, item WorkItem) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func readWorkItem(path string) (WorkItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkItem{}, err
	}

	var item WorkItem
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&item); err != nil {
		return WorkItem{}, fmt.Errorf("decode work item: %w", err)
	}
	return item, nil
}

func workItemPaths(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".json") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func (s WorkSentence) MarshalJSON() ([]byte, error) {
	fields := map[string]string{
		"id":      s.ID,
		"english": s.English,
	}
	for _, level := range story.SupportedLevels {
		if translation, ok := s.Translations[level]; ok {
			fields[level] = translation
		}
	}
	return json.Marshal(fields)
}

func (s *WorkSentence) UnmarshalJSON(data []byte) error {
	var fields map[string]string
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	s.ID = fields["id"]
	s.English = fields["english"]
	s.Translations = map[string]string{}
	delete(fields, "id")
	delete(fields, "english")
	for key, value := range fields {
		if !story.IsSupportedLevel(key) {
			return fmt.Errorf("unknown sentence field %q", key)
		}
		s.Translations[key] = value
	}
	return nil
}

func workItemFileName(storyID, chunkID string) string {
	return fmt.Sprintf("%s_%s.json", storyID, chunkID)
}

func includedLevels(levels []string, included map[string]bool) []string {
	var result []string
	for _, level := range levels {
		if included[level] {
			result = append(result, level)
		}
	}
	return result
}

func hasChunk(s story.Story, chunkID string) bool {
	return findChunk(&s, chunkID) != nil
}

func findChunk(s *story.Story, chunkID string) *story.Chunk {
	for i := range s.Chunks {
		if s.Chunks[i].ID == chunkID {
			return &s.Chunks[i]
		}
	}
	return nil
}

func sentencesByID(chunk *story.Chunk) map[string]*story.Sentence {
	sentences := map[string]*story.Sentence{}
	for i := range chunk.Paragraphs {
		for j := range chunk.Paragraphs[i].Sentences {
			sentence := &chunk.Paragraphs[i].Sentences[j]
			sentences[sentence.ID] = sentence
		}
	}
	return sentences
}

func storyHasLevel(s story.Story, level string) bool {
	for _, configured := range s.Levels {
		if configured == level {
			return true
		}
	}
	return false
}
