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

type ExportSheetsOptions struct {
	SourceDir string
	OutputDir string
}

type ImportSheetsOptions struct {
	SourceDir string
	InputDir  string
	OutputDir string
	Check     bool
}

type ValidateSheetsOptions struct {
	SourceDir string
	InputDir  string
	Files     []string
}

type ValidateWorkItemsOptions struct {
	SourceDir string
	InputDir  string
	FixBOM    bool
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

type SheetExportResult struct {
	Files []string
}

type SheetImportResult struct {
	Files          []string
	FilesValidated int
	Failures       []SheetValidationFailure
}

type SheetValidationResult struct {
	FilesValidated int
	Failures       []SheetValidationFailure
	Files          []SheetFileValidation
}

type SheetFileValidation struct {
	File         string
	Status       string
	FailureCount int
}

type SheetValidationFailure struct {
	File    string
	Line    int
	Message string
}

type WorkItemValidationResult struct {
	FilesValidated int
	Translations   int
	Failures       []SheetValidationFailure
	Files          []SheetFileValidation
}

func (f SheetValidationFailure) String() string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.Message)
	}
	return fmt.Sprintf("%s: %s", f.File, f.Message)
}

type SheetImportError struct {
	Failures []SheetValidationFailure
}

func (e SheetImportError) Error() string {
	return fmt.Sprintf("agent sheet import failed: %d failure(s)", len(e.Failures))
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

func ValidateWorkItems(opts ValidateWorkItemsOptions) (WorkItemValidationResult, error) {
	if strings.TrimSpace(opts.SourceDir) == "" {
		return WorkItemValidationResult{}, fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.InputDir) == "" {
		return WorkItemValidationResult{}, fmt.Errorf("input directory is required")
	}

	sourcePaths, err := workItemPaths(opts.SourceDir)
	if err != nil {
		return WorkItemValidationResult{}, err
	}
	if len(sourcePaths) == 0 {
		return WorkItemValidationResult{}, fmt.Errorf("no source work item JSON files found in %s", opts.SourceDir)
	}
	inputPaths, err := workItemPaths(opts.InputDir)
	if err != nil {
		return WorkItemValidationResult{}, err
	}

	sourceByName := map[string]string{}
	inputByName := map[string]string{}
	for _, path := range sourcePaths {
		sourceByName[filepath.Base(path)] = path
	}
	for _, path := range inputPaths {
		inputByName[filepath.Base(path)] = path
	}

	names := make([]string, 0, len(sourceByName))
	for name := range sourceByName {
		names = append(names, name)
	}
	sort.Strings(names)

	var result WorkItemValidationResult
	for _, name := range names {
		sourcePath := sourceByName[name]
		inputPath, ok := inputByName[name]
		if !ok {
			result.Failures = append(result.Failures, SheetValidationFailure{
				File:    filepath.Join(opts.InputDir, name),
				Message: "missing completed work item",
			})
			result.Files = append(result.Files, SheetFileValidation{
				File:         filepath.Join(opts.InputDir, name),
				Status:       "missing",
				FailureCount: 1,
			})
			continue
		}
		result.FilesValidated++
		failures, translations := validateWorkItemPair(sourcePath, inputPath, opts.FixBOM)
		result.Failures = append(result.Failures, failures...)
		result.Translations += translations
		status := "ok"
		if len(failures) > 0 {
			status = "failed"
		}
		result.Files = append(result.Files, SheetFileValidation{
			File:         inputPath,
			Status:       status,
			FailureCount: len(failures),
		})
	}
	for name, inputPath := range inputByName {
		if _, ok := sourceByName[name]; !ok {
			result.Failures = append(result.Failures, SheetValidationFailure{
				File:    inputPath,
				Message: "extra completed work item with no matching source work item",
			})
			result.Files = append(result.Files, SheetFileValidation{
				File:         inputPath,
				Status:       "extra",
				FailureCount: 1,
			})
		}
	}

	sort.Slice(result.Failures, func(i, j int) bool {
		if result.Failures[i].File != result.Failures[j].File {
			return result.Failures[i].File < result.Failures[j].File
		}
		return result.Failures[i].Line < result.Failures[j].Line
	})
	sort.Slice(result.Files, func(i, j int) bool {
		return result.Files[i].File < result.Files[j].File
	})
	return result, nil
}

func ExportSheets(opts ExportSheetsOptions) (SheetExportResult, error) {
	if strings.TrimSpace(opts.SourceDir) == "" {
		return SheetExportResult{}, fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return SheetExportResult{}, fmt.Errorf("output directory is required")
	}

	paths, err := workItemPaths(opts.SourceDir)
	if err != nil {
		return SheetExportResult{}, err
	}
	if len(paths) == 0 {
		return SheetExportResult{}, fmt.Errorf("no work item JSON files found in %s", opts.SourceDir)
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return SheetExportResult{}, err
	}

	var files []string
	for _, path := range paths {
		item, err := readWorkItem(path)
		if err != nil {
			return SheetExportResult{}, fmt.Errorf("read %s: %w", path, err)
		}
		out := filepath.Join(opts.OutputDir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".txt")
		if err := writeTranslationSheet(out, item, filepath.Base(path)); err != nil {
			return SheetExportResult{}, err
		}
		files = append(files, out)
	}
	sort.Strings(files)
	return SheetExportResult{Files: files}, nil
}

func ImportSheets(opts ImportSheetsOptions) (SheetImportResult, error) {
	if strings.TrimSpace(opts.SourceDir) == "" {
		return SheetImportResult{}, fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.InputDir) == "" {
		return SheetImportResult{}, fmt.Errorf("input directory is required")
	}
	if !opts.Check && strings.TrimSpace(opts.OutputDir) == "" {
		return SheetImportResult{}, fmt.Errorf("output directory is required")
	}

	paths, err := translationSheetPaths(opts.InputDir)
	if err != nil {
		return SheetImportResult{}, err
	}
	if len(paths) == 0 {
		return SheetImportResult{}, fmt.Errorf("no translation sheet files found in %s", opts.InputDir)
	}
	if !opts.Check {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return SheetImportResult{}, err
		}
	}

	var result SheetImportResult
	for _, path := range paths {
		out, failures, err := importSheet(path, opts)
		if err != nil {
			return result, err
		}
		result.FilesValidated++
		if len(failures) > 0 {
			result.Failures = append(result.Failures, failures...)
			continue
		}
		result.Files = append(result.Files, out)
	}
	sort.Strings(result.Files)
	sort.Slice(result.Failures, func(i, j int) bool {
		if result.Failures[i].File != result.Failures[j].File {
			return result.Failures[i].File < result.Failures[j].File
		}
		return result.Failures[i].Line < result.Failures[j].Line
	})
	if len(result.Failures) > 0 {
		return result, SheetImportError{Failures: result.Failures}
	}
	return result, nil
}

func importSheet(path string, opts ImportSheetsOptions) (string, []SheetValidationFailure, error) {
	sourceName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".json"
	sourcePath := filepath.Join(opts.SourceDir, sourceName)
	out := filepath.Join(opts.OutputDir, sourceName)

	source, err := readWorkItem(sourcePath)
	if err != nil {
		return out, []SheetValidationFailure{{
			File:    path,
			Message: fmt.Sprintf("read source %s: %v", sourcePath, err),
		}}, nil
	}

	sheet, failures := readStrictTranslationSheet(path)
	item, semanticFailures := workItemFromSheetDiagnostics(source, sheet, sourceName, path)
	failures = append(failures, semanticFailures...)
	if len(failures) > 0 {
		return out, failures, nil
	}
	if opts.Check {
		return out, nil, nil
	}
	if err := writeWorkItem(out, item); err != nil {
		return out, []SheetValidationFailure{{
			File:    path,
			Message: fmt.Sprintf("write completed work item %s: %v", out, err),
		}}, nil
	}
	return out, nil, nil
}

func workItemFromSheetDiagnostics(source WorkItem, sheet translationSheet, sourceFile string, path string) (WorkItem, []SheetValidationFailure) {
	var failures []SheetValidationFailure
	add := func(sentenceID, level, format string, args ...any) {
		prefix := ""
		if sentenceID != "" {
			prefix = fmt.Sprintf("sentence %s", sentenceID)
			if level != "" {
				prefix += fmt.Sprintf(" level %s", level)
			}
			prefix += ": "
		}
		failures = append(failures, SheetValidationFailure{
			File:    path,
			Message: prefix + fmt.Sprintf(format, args...),
		})
	}

	if sheet.StoryID != source.StoryID {
		add("", "", "story_id mismatch: got %q, want %q", sheet.StoryID, source.StoryID)
	}
	if sheet.StoryTitle != source.StoryTitle {
		add("", "", "story_title mismatch: got %q, want %q", sheet.StoryTitle, source.StoryTitle)
	}
	if sheet.ChunkID != source.ChunkID {
		add("", "", "chunk_id mismatch: got %q, want %q", sheet.ChunkID, source.ChunkID)
	}
	if sheet.SourceFile != "" && sheet.SourceFile != sourceFile {
		add("", "", "source_file mismatch: got %q, want %q", sheet.SourceFile, sourceFile)
	}
	if strings.Join(sheet.Levels, "\x00") != strings.Join(source.Levels, "\x00") {
		add("", "", "levels mismatch: got %q, want %q", strings.Join(sheet.Levels, ","), strings.Join(source.Levels, ","))
	}

	byID := map[string]sheetSentence{}
	for _, sentence := range sheet.Sentences {
		if sentence.SentenceID == "" {
			add("", "", "sentence id is required")
			continue
		}
		if _, exists := byID[sentence.SentenceID]; exists {
			add(sentence.SentenceID, "", "duplicate sentence")
			continue
		}
		byID[sentence.SentenceID] = sentence
	}

	result := source
	for paragraphIndex, paragraph := range source.Paragraphs {
		for sentenceIndex, sentence := range paragraph.Sentences {
			sheetSentence, ok := byID[sentence.ID]
			if !ok {
				add(sentence.ID, "", "missing from sheet")
				continue
			}
			if sheetSentence.ParagraphID != paragraph.ID {
				add(sentence.ID, "", "paragraph mismatch: got %q, want %q", sheetSentence.ParagraphID, paragraph.ID)
			}
			if sheetSentence.English != sentence.English {
				add(sentence.ID, "", "changed English text")
			}
			translations := map[string]string{}
			for level := range sentence.Translations {
				translation, ok := sheetSentence.Translations[level]
				if !ok {
					add(sentence.ID, level, "missing block")
					continue
				}
				translation = strings.TrimSpace(translation)
				if translation == "" {
					add(sentence.ID, level, "empty translation")
					continue
				}
				translations[level] = translation
			}
			for level := range sheetSentence.Translations {
				if _, ok := sentence.Translations[level]; !ok {
					add(sentence.ID, level, "extra unknown block")
				}
			}
			result.Paragraphs[paragraphIndex].Sentences[sentenceIndex].Translations = translations
			delete(byID, sentence.ID)
		}
	}
	if len(byID) > 0 {
		var extra []string
		for sentenceID := range byID {
			extra = append(extra, sentenceID)
		}
		sort.Strings(extra)
		for _, sentenceID := range extra {
			add(sentenceID, "", "unknown sentence")
		}
	}
	if len(failures) > 0 {
		return WorkItem{}, failures
	}
	return result, nil
}

func validateWorkItemPair(sourcePath string, inputPath string, fixBOM bool) ([]SheetValidationFailure, int) {
	add := func(failures *[]SheetValidationFailure, message string) {
		*failures = append(*failures, SheetValidationFailure{
			File:    inputPath,
			Message: message,
		})
	}
	var failures []SheetValidationFailure
	if fixBOM {
		for _, path := range []string{sourcePath, inputPath} {
			if err := removeUTF8BOM(path); err != nil {
				add(&failures, fmt.Sprintf("remove UTF-8 BOM from %s: %v", path, err))
			}
		}
		if len(failures) > 0 {
			return failures, 0
		}
	}

	source, err := readWorkItem(sourcePath)
	if err != nil {
		add(&failures, fmt.Sprintf("read source %s: %v", sourcePath, err))
		return failures, 0
	}
	done, err := readWorkItem(inputPath)
	if err != nil {
		add(&failures, fmt.Sprintf("read completed work item: %v", err))
		return failures, 0
	}

	if done.StoryID != source.StoryID {
		add(&failures, fmt.Sprintf("story_id mismatch: got %q, want %q", done.StoryID, source.StoryID))
	}
	if done.StoryTitle != source.StoryTitle {
		add(&failures, fmt.Sprintf("story_title mismatch: got %q, want %q", done.StoryTitle, source.StoryTitle))
	}
	if done.ChunkID != source.ChunkID {
		add(&failures, fmt.Sprintf("chunk_id mismatch: got %q, want %q", done.ChunkID, source.ChunkID))
	}
	if strings.Join(done.Levels, "\x00") != strings.Join(source.Levels, "\x00") {
		add(&failures, fmt.Sprintf("levels changed: got %q, want %q", strings.Join(done.Levels, ","), strings.Join(source.Levels, ",")))
	}

	levelSet := map[string]bool{}
	for _, level := range source.Levels {
		if !story.IsSupportedLevel(level) {
			add(&failures, fmt.Sprintf("unsupported level: %s", level))
		}
		if levelSet[level] {
			add(&failures, fmt.Sprintf("duplicate level: %s", level))
		}
		levelSet[level] = true
	}

	if len(done.Paragraphs) != len(source.Paragraphs) {
		add(&failures, fmt.Sprintf("paragraph count changed: got %d, want %d", len(done.Paragraphs), len(source.Paragraphs)))
	}

	translations := 0
	paragraphs := minInt(len(source.Paragraphs), len(done.Paragraphs))
	for i := 0; i < paragraphs; i++ {
		sourceParagraph := source.Paragraphs[i]
		doneParagraph := done.Paragraphs[i]
		prefix := fmt.Sprintf("paragraphs[%d]", i)
		if doneParagraph.ID != sourceParagraph.ID {
			add(&failures, fmt.Sprintf("%s.id mismatch: got %q, want %q", prefix, doneParagraph.ID, sourceParagraph.ID))
		}
		if len(doneParagraph.Sentences) != len(sourceParagraph.Sentences) {
			add(&failures, fmt.Sprintf("%s sentence count changed: got %d, want %d", prefix, len(doneParagraph.Sentences), len(sourceParagraph.Sentences)))
		}
		sentences := minInt(len(sourceParagraph.Sentences), len(doneParagraph.Sentences))
		for j := 0; j < sentences; j++ {
			sourceSentence := sourceParagraph.Sentences[j]
			doneSentence := doneParagraph.Sentences[j]
			sentencePath := fmt.Sprintf("%s.sentences[%d]", prefix, j)
			if doneSentence.ID != sourceSentence.ID {
				add(&failures, fmt.Sprintf("%s.id mismatch: got %q, want %q", sentencePath, doneSentence.ID, sourceSentence.ID))
			}
			if doneSentence.English != sourceSentence.English {
				add(&failures, fmt.Sprintf("%s.english changed", sentencePath))
			}
			sourceKeys := sortedTranslationKeys(sourceSentence.Translations)
			doneKeys := sortedTranslationKeys(doneSentence.Translations)
			if strings.Join(doneKeys, "\x00") != strings.Join(sourceKeys, "\x00") {
				add(&failures, fmt.Sprintf("%s translation level fields differ: got %q, want %q", sentencePath, strings.Join(doneKeys, ","), strings.Join(sourceKeys, ",")))
			}
			for _, level := range doneKeys {
				if !levelSet[level] {
					add(&failures, fmt.Sprintf("%s includes level not listed in levels: %s", sentencePath, level))
					continue
				}
				if strings.TrimSpace(doneSentence.Translations[level]) == "" {
					add(&failures, fmt.Sprintf("%s.%s is empty", sentencePath, level))
					continue
				}
				translations++
			}
		}
	}
	if len(failures) > 0 {
		return failures, 0
	}
	return nil, translations
}

func sortedTranslationKeys(translations map[string]string) []string {
	keys := make([]string, 0, len(translations))
	for key := range translations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeUTF8BOM(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return nil
	}
	return os.WriteFile(path, data[3:], 0644)
}

func ValidateSheets(opts ValidateSheetsOptions) (SheetValidationResult, error) {
	if strings.TrimSpace(opts.SourceDir) == "" {
		return SheetValidationResult{}, fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.InputDir) == "" {
		return SheetValidationResult{}, fmt.Errorf("input directory is required")
	}

	sourcePaths, err := translationSheetPaths(opts.SourceDir)
	if err != nil {
		return SheetValidationResult{}, err
	}
	if len(sourcePaths) == 0 {
		return SheetValidationResult{}, fmt.Errorf("no source translation sheet files found in %s", opts.SourceDir)
	}
	inputPaths, err := translationSheetPaths(opts.InputDir)
	if err != nil {
		return SheetValidationResult{}, err
	}

	sourceByName := map[string]string{}
	inputByName := map[string]string{}
	for _, path := range sourcePaths {
		sourceByName[filepath.Base(path)] = path
	}
	for _, path := range inputPaths {
		inputByName[filepath.Base(path)] = path
	}

	names, selectedOnly := sheetNamesToValidate(opts.Files, sourceByName)
	var result SheetValidationResult
	for _, name := range names {
		sourcePath, sourceExists := sourceByName[name]
		if !sourceExists {
			failure := SheetValidationFailure{
				File:    filepath.Join(opts.SourceDir, name),
				Message: "assigned source sheet not found",
			}
			result.Failures = append(result.Failures, failure)
			result.Files = append(result.Files, SheetFileValidation{
				File:         filepath.Join(opts.InputDir, name),
				Status:       "missing-source",
				FailureCount: 1,
			})
			continue
		}
		inputPath, ok := inputByName[name]
		if !ok {
			failure := SheetValidationFailure{
				File:    filepath.Join(opts.InputDir, name),
				Message: "missing completed sheet",
			}
			result.Failures = append(result.Failures, failure)
			result.Files = append(result.Files, SheetFileValidation{
				File:         filepath.Join(opts.InputDir, name),
				Status:       "missing",
				FailureCount: 1,
			})
			continue
		}
		result.FilesValidated++
		failures := validateSheetPair(sourcePath, inputPath)
		result.Failures = append(result.Failures, failures...)
		status := "ok"
		if len(failures) > 0 {
			status = "failed"
		}
		result.Files = append(result.Files, SheetFileValidation{
			File:         inputPath,
			Status:       status,
			FailureCount: len(failures),
		})
	}
	if !selectedOnly {
		for name, inputPath := range inputByName {
			if _, ok := sourceByName[name]; !ok {
				result.Failures = append(result.Failures, SheetValidationFailure{
					File:    inputPath,
					Message: "extra completed sheet with no matching source sheet",
				})
				result.Files = append(result.Files, SheetFileValidation{
					File:         inputPath,
					Status:       "extra",
					FailureCount: 1,
				})
			}
		}
	}

	sort.Slice(result.Failures, func(i, j int) bool {
		if result.Failures[i].File != result.Failures[j].File {
			return result.Failures[i].File < result.Failures[j].File
		}
		return result.Failures[i].Line < result.Failures[j].Line
	})
	sort.Slice(result.Files, func(i, j int) bool {
		return result.Files[i].File < result.Files[j].File
	})
	return result, nil
}

func sheetNamesToValidate(files []string, sourceByName map[string]string) ([]string, bool) {
	if len(files) == 0 {
		names := make([]string, 0, len(sourceByName))
		for name := range sourceByName {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, false
	}

	seen := map[string]bool{}
	var names []string
	for _, file := range files {
		name := filepath.Base(strings.TrimSpace(file))
		if name == "" {
			continue
		}
		if filepath.Ext(name) == "" {
			name += ".txt"
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, true
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

const sheetFence = "<<<JPSTORIES"
const sheetFenceEnd = "JPSTORIES>>>"

type translationSheet struct {
	StoryID    string
	StoryTitle string
	ChunkID    string
	Levels     []string
	SourceFile string
	Sentences  []sheetSentence
}

type sheetSentence struct {
	ParagraphID  string
	SentenceID   string
	English      string
	Translations map[string]string
}

func writeTranslationSheet(path string, item WorkItem, sourceFile string) error {
	var b strings.Builder
	b.WriteString("# jpstories translation sheet v1\n")
	fmt.Fprintf(&b, "story_id: %s\n", item.StoryID)
	fmt.Fprintf(&b, "story_title: %s\n", item.StoryTitle)
	fmt.Fprintf(&b, "chunk_id: %s\n", item.ChunkID)
	fmt.Fprintf(&b, "levels: %s\n", strings.Join(item.Levels, ","))
	fmt.Fprintf(&b, "source_file: %s\n", sourceFile)
	b.WriteByte('\n')
	b.WriteString("Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels.\n\n")

	for _, paragraph := range item.Paragraphs {
		for _, sentence := range paragraph.Sentences {
			fmt.Fprintf(&b, "## %s / %s\n", paragraph.ID, sentence.ID)
			writeSheetField(&b, "english", sentence.English)
			for _, level := range item.Levels {
				if _, ok := sentence.Translations[level]; ok {
					writeSheetField(&b, level, "")
				}
			}
			b.WriteByte('\n')
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func writeSheetField(b *strings.Builder, name string, value string) {
	fmt.Fprintf(b, "%s:\n%s\n", name, sheetFence)
	b.WriteString(value)
	if value != "" && !strings.HasSuffix(value, "\n") {
		b.WriteByte('\n')
	}
	fmt.Fprintf(b, "%s\n", sheetFenceEnd)
}

func readTranslationSheet(path string) (translationSheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return translationSheet{}, err
	}
	return parseTranslationSheet(string(data))
}

func parseTranslationSheet(text string) (translationSheet, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# jpstories translation sheet v1" {
		return translationSheet{}, fmt.Errorf("missing translation sheet header")
	}

	var sheet translationSheet
	i := 1
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break
		}
		if !strings.Contains(line, ":") {
			i++
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "story_id":
			sheet.StoryID = value
		case "story_title":
			sheet.StoryTitle = value
		case "chunk_id":
			sheet.ChunkID = value
		case "levels":
			sheet.Levels = splitSheetLevels(value)
		case "source_file":
			sheet.SourceFile = value
		}
		i++
	}

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "## ") {
			return translationSheet{}, fmt.Errorf("expected sentence header at line %d", i+1)
		}
		header := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		paragraphID, sentenceID, ok := strings.Cut(header, "/")
		if !ok {
			return translationSheet{}, fmt.Errorf("invalid sentence header %q", line)
		}
		current := sheetSentence{
			ParagraphID:  strings.TrimSpace(paragraphID),
			SentenceID:   strings.TrimSpace(sentenceID),
			Translations: map[string]string{},
		}
		i++
		for i < len(lines) {
			line = strings.TrimSpace(lines[i])
			if line == "" {
				i++
				continue
			}
			if strings.HasPrefix(line, "## ") {
				break
			}
			if !strings.HasSuffix(line, ":") {
				return translationSheet{}, fmt.Errorf("expected field label at line %d", i+1)
			}
			field := strings.TrimSuffix(line, ":")
			value, next, err := parseSheetBlock(lines, i+1)
			if err != nil {
				return translationSheet{}, fmt.Errorf("%s %s: %w", current.ParagraphID, current.SentenceID, err)
			}
			if field == "english" {
				current.English = value
			} else {
				current.Translations[field] = value
			}
			i = next
		}
		sheet.Sentences = append(sheet.Sentences, current)
	}

	return sheet, nil
}

func splitSheetLevels(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	levels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			levels = append(levels, part)
		}
	}
	return levels
}

func parseSheetBlock(lines []string, start int) (string, int, error) {
	if start >= len(lines) || strings.TrimSpace(lines[start]) != sheetFence {
		return "", start, fmt.Errorf("missing opening fence")
	}
	start++
	var b strings.Builder
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == sheetFenceEnd {
			return strings.TrimSpace(b.String()), i + 1, nil
		}
		b.WriteString(lines[i])
		if i+1 < len(lines) {
			b.WriteByte('\n')
		}
	}
	return "", start, fmt.Errorf("missing closing fence")
}

func validateSheetPair(sourcePath, inputPath string) []SheetValidationFailure {
	source, sourceFailures := readStrictTranslationSheet(sourcePath)
	input, inputFailures := readStrictTranslationSheet(inputPath)
	var failures []SheetValidationFailure
	failures = append(failures, sourceFailures...)
	failures = append(failures, inputFailures...)
	if len(sourceFailures) > 0 {
		return failures
	}

	if source.StoryID != input.StoryID {
		failures = append(failures, sheetFailure(inputPath, 0, "story_id mismatch: got %q, want %q", input.StoryID, source.StoryID))
	}
	if source.StoryTitle != input.StoryTitle {
		failures = append(failures, sheetFailure(inputPath, 0, "story_title mismatch: got %q, want %q", input.StoryTitle, source.StoryTitle))
	}
	if source.ChunkID != input.ChunkID {
		failures = append(failures, sheetFailure(inputPath, 0, "chunk_id mismatch: got %q, want %q", input.ChunkID, source.ChunkID))
	}
	if strings.Join(source.Levels, "\x00") != strings.Join(input.Levels, "\x00") {
		failures = append(failures, sheetFailure(inputPath, 0, "levels mismatch: got %q, want %q", strings.Join(input.Levels, ","), strings.Join(source.Levels, ",")))
	}
	if source.SourceFile != input.SourceFile {
		failures = append(failures, sheetFailure(inputPath, 0, "source_file mismatch: got %q, want %q", input.SourceFile, source.SourceFile))
	}

	if len(source.Sentences) != len(input.Sentences) {
		failures = append(failures, sheetFailure(inputPath, 0, "sentence count mismatch: got %d, want %d", len(input.Sentences), len(source.Sentences)))
	}
	for i, sourceSentence := range source.Sentences {
		if i >= len(input.Sentences) {
			continue
		}
		inputSentence := input.Sentences[i]
		where := fmt.Sprintf("sentence %d", i+1)
		if sourceSentence.ParagraphID != inputSentence.ParagraphID || sourceSentence.SentenceID != inputSentence.SentenceID {
			failures = append(failures, sheetFailure(inputPath, 0, "%s id/order mismatch: got %s / %s, want %s / %s", where, inputSentence.ParagraphID, inputSentence.SentenceID, sourceSentence.ParagraphID, sourceSentence.SentenceID))
			continue
		}
		if sourceSentence.English != inputSentence.English {
			failures = append(failures, sheetFailure(inputPath, 0, "%s english text changed", sourceSentence.SentenceID))
		}
		for _, level := range source.Levels {
			if _, expected := sourceSentence.Translations[level]; !expected {
				continue
			}
			translation, ok := inputSentence.Translations[level]
			if !ok {
				failures = append(failures, sheetFailure(inputPath, 0, "%s missing %s block", sourceSentence.SentenceID, level))
				continue
			}
			if strings.TrimSpace(translation) == "" {
				failures = append(failures, sheetFailure(inputPath, 0, "%s %s translation is empty", sourceSentence.SentenceID, level))
			}
		}
		for level := range inputSentence.Translations {
			if _, expected := sourceSentence.Translations[level]; !expected {
				failures = append(failures, sheetFailure(inputPath, 0, "%s includes unexpected %s block", sourceSentence.SentenceID, level))
			}
		}
	}
	return failures
}

func readStrictTranslationSheet(path string) (translationSheet, []SheetValidationFailure) {
	data, err := os.ReadFile(path)
	if err != nil {
		return translationSheet{}, []SheetValidationFailure{{File: path, Message: err.Error()}}
	}
	return parseStrictTranslationSheet(path, string(data))
}

func parseStrictTranslationSheet(path, text string) (translationSheet, []SheetValidationFailure) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	var sheet translationSheet
	var failures []SheetValidationFailure
	add := func(line int, format string, args ...any) {
		failures = append(failures, sheetFailure(path, line, format, args...))
	}

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# jpstories translation sheet v1" {
		add(1, "missing translation sheet header")
		return sheet, failures
	}

	i := 1
	seenMeta := map[string]bool{}
	requiredMeta := []string{"story_id", "story_title", "chunk_id", "levels", "source_file"}
	metaIndex := 0
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			add(i+1, "expected metadata line")
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if seenMeta[key] {
			add(i+1, "duplicate metadata field %q", key)
		}
		if metaIndex < len(requiredMeta) && key != requiredMeta[metaIndex] {
			add(i+1, "metadata field order mismatch: got %q, want %q", key, requiredMeta[metaIndex])
		}
		metaIndex++
		seenMeta[key] = true
		switch key {
		case "story_id":
			sheet.StoryID = value
		case "story_title":
			sheet.StoryTitle = value
		case "chunk_id":
			sheet.ChunkID = value
		case "levels":
			sheet.Levels = splitSheetLevels(value)
		case "source_file":
			sheet.SourceFile = value
		default:
			add(i+1, "unknown metadata field %q", key)
		}
	}
	for _, key := range requiredMeta {
		if !seenMeta[key] {
			add(0, "missing metadata field %q", key)
		}
	}

	if i < len(lines) {
		if strings.TrimSpace(lines[i]) != "Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels." {
			add(i+1, "unexpected text before first sentence")
		}
		i++
	}
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "## ") {
			add(i+1, "unexpected text outside sentence block")
			i++
			continue
		}
		headerLine := i + 1
		header := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		paragraphID, sentenceID, ok := strings.Cut(header, "/")
		if !ok {
			add(headerLine, "invalid sentence header %q", line)
			i++
			continue
		}
		current := sheetSentence{
			ParagraphID:  strings.TrimSpace(paragraphID),
			SentenceID:   strings.TrimSpace(sentenceID),
			Translations: map[string]string{},
		}
		seenFields := map[string]bool{}
		i++
		for i < len(lines) {
			line = strings.TrimSpace(lines[i])
			if line == "" {
				i++
				continue
			}
			if strings.HasPrefix(line, "## ") {
				break
			}
			fieldLine := i + 1
			if !strings.HasSuffix(line, ":") {
				add(fieldLine, "%s: expected field label", current.SentenceID)
				i++
				continue
			}
			field := strings.TrimSuffix(line, ":")
			if seenFields[field] {
				add(fieldLine, "%s: duplicate %s block", current.SentenceID, field)
			}
			seenFields[field] = true
			value, next, blockFailures := parseStrictSheetBlock(path, lines, i+1, current.SentenceID, field, sheet.Levels)
			failures = append(failures, blockFailures...)
			if field == "english" {
				current.English = value
			} else {
				current.Translations[field] = value
			}
			i = next
		}
		if !seenFields["english"] {
			add(headerLine, "%s: missing english block", current.SentenceID)
		}
		for _, level := range sheet.Levels {
			if !seenFields[level] {
				add(headerLine, "%s: missing %s block", current.SentenceID, level)
			}
		}
		for field := range seenFields {
			if field == "english" {
				continue
			}
			if !containsString(sheet.Levels, field) {
				add(headerLine, "%s: unexpected %s block", current.SentenceID, field)
			}
		}
		sheet.Sentences = append(sheet.Sentences, current)
	}

	return sheet, failures
}

func parseStrictSheetBlock(path string, lines []string, start int, sentenceID, field string, levels []string) (string, int, []SheetValidationFailure) {
	var failures []SheetValidationFailure
	add := func(line int, format string, args ...any) {
		failures = append(failures, sheetFailure(path, line, format, args...))
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != sheetFence {
		add(start+1, "%s %s: missing opening fence", sentenceID, field)
		return "", start + 1, failures
	}
	start++
	var b strings.Builder
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == sheetFenceEnd {
			return strings.TrimSpace(b.String()), i + 1, failures
		}
		if isSheetFieldLabel(line, levels) || strings.HasPrefix(line, "## ") {
			add(i+1, "%s %s: missing closing fence", sentenceID, field)
			return strings.TrimSpace(b.String()), i, failures
		}
		b.WriteString(lines[i])
		if i+1 < len(lines) {
			b.WriteByte('\n')
		}
	}
	add(len(lines), "%s %s: missing closing fence", sentenceID, field)
	return strings.TrimSpace(b.String()), len(lines), failures
}

func sheetFailure(path string, line int, format string, args ...any) SheetValidationFailure {
	return SheetValidationFailure{
		File:    path,
		Line:    line,
		Message: fmt.Sprintf(format, args...),
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isSheetFieldLabel(line string, levels []string) bool {
	if line == "english:" {
		return true
	}
	for _, level := range levels {
		if line == level+":" {
			return true
		}
	}
	return false
}

func workItemFromSheet(source WorkItem, sheet translationSheet, sourceFile string) (WorkItem, error) {
	if sheet.StoryID != source.StoryID {
		return WorkItem{}, fmt.Errorf("story_id mismatch: got %q, want %q", sheet.StoryID, source.StoryID)
	}
	if sheet.StoryTitle != source.StoryTitle {
		return WorkItem{}, fmt.Errorf("story_title mismatch: got %q, want %q", sheet.StoryTitle, source.StoryTitle)
	}
	if sheet.ChunkID != source.ChunkID {
		return WorkItem{}, fmt.Errorf("chunk_id mismatch: got %q, want %q", sheet.ChunkID, source.ChunkID)
	}
	if sheet.SourceFile != "" && sheet.SourceFile != sourceFile {
		return WorkItem{}, fmt.Errorf("source_file mismatch: got %q, want %q", sheet.SourceFile, sourceFile)
	}
	if strings.Join(sheet.Levels, "\x00") != strings.Join(source.Levels, "\x00") {
		return WorkItem{}, fmt.Errorf("levels mismatch: got %q, want %q", strings.Join(sheet.Levels, ","), strings.Join(source.Levels, ","))
	}

	byID := map[string]sheetSentence{}
	for _, sentence := range sheet.Sentences {
		if sentence.SentenceID == "" {
			return WorkItem{}, fmt.Errorf("sentence id is required")
		}
		if _, exists := byID[sentence.SentenceID]; exists {
			return WorkItem{}, fmt.Errorf("duplicate sentence %q", sentence.SentenceID)
		}
		byID[sentence.SentenceID] = sentence
	}

	result := source
	for paragraphIndex, paragraph := range source.Paragraphs {
		for sentenceIndex, sentence := range paragraph.Sentences {
			sheetSentence, ok := byID[sentence.ID]
			if !ok {
				return WorkItem{}, fmt.Errorf("sentence %q missing from sheet", sentence.ID)
			}
			if sheetSentence.ParagraphID != paragraph.ID {
				return WorkItem{}, fmt.Errorf("sentence %q paragraph mismatch: got %q, want %q", sentence.ID, sheetSentence.ParagraphID, paragraph.ID)
			}
			if sheetSentence.English != sentence.English {
				return WorkItem{}, fmt.Errorf("sentence %q english mismatch", sentence.ID)
			}
			translations := map[string]string{}
			for level := range sentence.Translations {
				translation, ok := sheetSentence.Translations[level]
				if !ok {
					return WorkItem{}, fmt.Errorf("sentence %q missing %s block", sentence.ID, level)
				}
				translation = strings.TrimSpace(translation)
				if translation == "" {
					return WorkItem{}, fmt.Errorf("translation for sentence %q level %q is empty", sentence.ID, level)
				}
				translations[level] = translation
			}
			for level := range sheetSentence.Translations {
				if _, ok := sentence.Translations[level]; !ok {
					return WorkItem{}, fmt.Errorf("sentence %q includes unexpected %s block", sentence.ID, level)
				}
			}
			result.Paragraphs[paragraphIndex].Sentences[sentenceIndex].Translations = translations
			delete(byID, sentence.ID)
		}
	}
	if len(byID) > 0 {
		var extra []string
		for sentenceID := range byID {
			extra = append(extra, sentenceID)
		}
		sort.Strings(extra)
		return WorkItem{}, fmt.Errorf("sheet includes unknown sentence %q", extra[0])
	}
	return result, nil
}

func translationSheetPaths(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".txt") {
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
