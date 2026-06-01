package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jpstories/internal/appconfig"
	"jpstories/internal/chunker"
	"jpstories/internal/server"
	"jpstories/internal/sourcecleaner"
	"jpstories/internal/story"
	"jpstories/internal/workitem"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runServe(nil)
	}
	if strings.HasPrefix(args[0], "-") {
		return runServe(args)
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "chunk":
		return runChunk(args[1:])
	case "clean-source":
		return runCleanSource(args[1:])
	case "prepare-story":
		return runPrepareStory(args[1:])
	case "export-work":
		return runExportWork(args[1:])
	case "export-agent-work":
		return runExportAgentWork(args[1:])
	case "validate-agent-work":
		return runValidateAgentWork(args[1:])
	case "validate-workitems":
		return runValidateWorkItems(args[1:])
	case "repair-agent-sheets":
		return runRepairAgentSheets(args[1:])
	case "import-agent-work":
		return runImportAgentWork(args[1:])
	case "accept-story":
		return runAcceptStory(args[1:])
	case "merge-work":
		return runMergeWork(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runCleanSource(args []string) error {
	fs := flag.NewFlagSet("clean-source", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	in := fs.String("in", "", "source text file to clean")
	out := fs.String("out", "", "cleaned source text output path")
	sourceLanguage := fs.String("source-language", "en", "source language: en or ja")
	paragraphMode := fs.String("paragraph-mode", sourcecleaner.ParagraphModeDialogue, "paragraph inference mode: preserve, conservative, or dialogue")
	cleanEncoding := fs.Bool("clean-encoding", true, "repair common mojibake and ligature extraction artifacts")
	repairHyphenation := fs.Bool("repair-hyphenation", true, "join line-wrapped hyphenated words")
	force := fs.Bool("force", false, "overwrite an existing output file")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *in == "" {
			*in = layout.SourcePath
		}
		if *out == "" {
			*out = layout.CleanedPath
		}
	}
	if *in == "" {
		return fmt.Errorf("clean-source requires -story or -in")
	}
	if *out == "" {
		return fmt.Errorf("clean-source requires -story or -out")
	}
	if *sourceLanguage != "en" && *sourceLanguage != "ja" {
		return fmt.Errorf("clean-source: unsupported -source-language %q (use en or ja)", *sourceLanguage)
	}
	if !*force {
		if _, err := os.Stat(*out); err == nil {
			return fmt.Errorf("refusing to overwrite %s without -force", *out)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	result, err := cleanSourceFile(*in, sourcecleaner.Options{
		CleanEncoding:     *cleanEncoding,
		RepairHyphenation: *repairHyphenation,
		ParagraphMode:     *paragraphMode,
		SourceLanguage:    *sourceLanguage,
	})
	if err != nil {
		return err
	}

	if err := writeTextFile(*out, result.Text); err != nil {
		return err
	}

	printCleanSourceResult(*out, result)
	return nil
}

func runPrepareStory(args []string) error {
	fs := flag.NewFlagSet("prepare-story", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	in := fs.String("in", "", "source text file to clean, chunk, and export")
	cleanedOut := fs.String("cleaned-out", "", "cleaned source text output path; defaults beside input as .cleaned.txt")
	storyPath := fs.String("story-path", "", "draft story JSON output path")
	workOut := fs.String("work-out", "", "directory for exported work item JSON files")
	id := fs.String("id", "", "story ID; defaults to the story output filename without extension")
	title := fs.String("title", "", "story title; defaults to title-cased story ID")
	sourceLanguage := fs.String("source-language", "en", "source language: en or ja")
	wordsPerChunk := fs.Int("words-per-chunk", 0, "target number of source words per chunk (default: 220 for en, 700 chars for ja)")
	paragraphsPerChunk := fs.Int("paragraphs-per-chunk", 0, "optional fixed number of paragraphs per chunk")
	level := fs.String("level", "", "optional translation level to export")
	paragraphMode := fs.String("paragraph-mode", sourcecleaner.ParagraphModeDialogue, "paragraph inference mode: preserve, conservative, or dialogue")
	cleanEncoding := fs.Bool("clean-encoding", true, "repair common mojibake and ligature extraction artifacts")
	repairHyphenation := fs.Bool("repair-hyphenation", true, "join line-wrapped hyphenated words")
	force := fs.Bool("force", false, "overwrite existing cleaned source and story JSON outputs")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && *storyPath == "" {
		return fmt.Errorf("prepare-story requires -story")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *in == "" {
			*in = layout.SourcePath
		}
		if *cleanedOut == "" {
			*cleanedOut = layout.CleanedPath
		}
		if *storyPath == "" {
			*storyPath = layout.StoryPath
		}
		if *workOut == "" {
			*workOut = layout.ChunkDir
		}
	}
	if *in == "" {
		return fmt.Errorf("prepare-story requires -story or -in")
	}
	if *storyPath == "" {
		return fmt.Errorf("prepare-story requires -story")
	}
	if *workOut == "" {
		*workOut = filepath.Dir(*storyPath)
	}
	if strings.TrimSpace(*cleanedOut) == "" {
		*cleanedOut = defaultCleanedPath(*in)
	}
	if strings.TrimSpace(*id) == "" {
		*id = strings.TrimSuffix(filepath.Base(*storyPath), filepath.Ext(*storyPath))
	}
	if *sourceLanguage != "en" && *sourceLanguage != "ja" {
		return fmt.Errorf("prepare-story: unsupported -source-language %q (use en or ja)", *sourceLanguage)
	}
	if !*force {
		if err := ensureNotExists(*cleanedOut); err != nil {
			return err
		}
		if err := ensureNotExists(*storyPath); err != nil {
			return err
		}
	}

	cleaned, err := cleanSourceFile(*in, sourcecleaner.Options{
		CleanEncoding:     *cleanEncoding,
		RepairHyphenation: *repairHyphenation,
		ParagraphMode:     *paragraphMode,
		SourceLanguage:    *sourceLanguage,
	})
	if err != nil {
		return err
	}
	if err := writeTextFile(*cleanedOut, cleaned.Text); err != nil {
		return err
	}
	printCleanSourceResult(*cleanedOut, cleaned)

	if err := writeDraftStory(*cleanedOut, *storyPath, *id, *title, *paragraphsPerChunk, *wordsPerChunk, *sourceLanguage, cleaned.Text); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote draft story JSON to %s\n", *storyPath)

	result, err := workitem.Export(workitem.ExportOptions{
		StoryPath: *storyPath,
		OutputDir: *workOut,
		Level:     *level,
	})
	if err != nil {
		return err
	}
	for _, file := range result.Files {
		fmt.Fprintf(os.Stderr, "wrote work item %s\n", file)
	}
	if len(result.Files) == 0 {
		fmt.Fprintln(os.Stderr, "no missing translations found")
	}
	return nil
}

func runExportWork(args []string) error {
	fs := flag.NewFlagSet("export-work", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	storyPath := fs.String("story-path", "", "story JSON file")
	out := fs.String("out", "", "directory for exported work item JSON files")
	level := fs.String("level", "", "optional translation level to export")
	chunkID := fs.String("chunk", "", "optional chunk ID to export")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && *storyPath == "" {
		return fmt.Errorf("export-work requires -story")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *storyPath == "" {
			*storyPath = layout.StoryPath
		}
		if *out == "" {
			*out = layout.ChunkDir
		}
	}
	if *out == "" {
		*out = filepath.Dir(*storyPath)
	}

	result, err := workitem.Export(workitem.ExportOptions{
		StoryPath: *storyPath,
		OutputDir: *out,
		Level:     *level,
		ChunkID:   *chunkID,
	})
	if err != nil {
		return err
	}
	for _, file := range result.Files {
		fmt.Fprintf(os.Stderr, "wrote work item %s\n", file)
	}
	if len(result.Files) == 0 {
		fmt.Fprintln(os.Stderr, "no missing translations found")
	}
	return nil
}

func runExportAgentWork(args []string) error {
	fs := flag.NewFlagSet("export-agent-work", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	in := fs.String("in", "", "directory containing source work item JSON files")
	out := fs.String("out", "", "directory for translator-friendly text sheets")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && *in == "" {
		return fmt.Errorf("export-agent-work requires -story or -in")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *in == "" {
			*in = layout.ChunkDir
		}
		if *out == "" {
			*out = layout.AgentDir
		}
	}
	if *out == "" {
		return fmt.Errorf("export-agent-work requires -story or -out")
	}

	result, err := workitem.ExportSheets(workitem.ExportSheetsOptions{
		SourceDir: *in,
		OutputDir: *out,
	})
	if err != nil {
		return err
	}
	for _, file := range result.Files {
		fmt.Fprintf(os.Stderr, "wrote agent work sheet %s\n", file)
	}
	return nil
}

func runImportAgentWork(args []string) error {
	fs := flag.NewFlagSet("import-agent-work", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	source := fs.String("source", "", "directory containing original work item JSON files")
	in := fs.String("in", "", "directory containing completed translator-friendly text sheets")
	out := fs.String("out", "", "directory for completed work item JSON files")
	check := fs.Bool("check", false, "validate completed sheets without writing completed work item JSON")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && (*source == "" || *in == "") {
		return fmt.Errorf("import-agent-work requires -story or both -source and -in")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *source == "" {
			*source = layout.ChunkDir
		}
		if *in == "" {
			*in = layout.AgentDoneDir
		}
		if *out == "" {
			*out = layout.DoneDir
		}
	}
	if *out == "" && !*check {
		return fmt.Errorf("import-agent-work requires -story or -out")
	}

	result, err := workitem.ImportSheets(workitem.ImportSheetsOptions{
		SourceDir: *source,
		InputDir:  *in,
		OutputDir: *out,
		Check:     *check,
	})
	if len(result.Failures) > 0 {
		for _, failure := range result.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
	}
	if err != nil {
		return err
	}
	if *check {
		fmt.Fprintf(os.Stderr, "checked %d completed agent sheet(s); %d ready to import\n", result.FilesValidated, len(result.Files))
		return nil
	}
	for _, file := range result.Files {
		fmt.Fprintf(os.Stderr, "wrote completed work item %s\n", file)
	}
	return nil
}

func runAcceptStory(args []string) error {
	fs := flag.NewFlagSet("accept-story", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	fixBOM := fs.Bool("fix-bom", true, "strip UTF-8 BOM bytes from completed work item JSON files during validation")
	repairAgentSheets := fs.Bool("repair-agent-sheets", false, "repair mechanical completed-sheet issues before validation")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" {
		return fmt.Errorf("accept-story requires -story")
	}
	layout, err := resolveStoryLayout(*storiesRoot, *storyName)
	if err != nil {
		return err
	}

	if *repairAgentSheets {
		fmt.Fprintf(os.Stderr, "acceptance: repairing completed agent sheets for %s\n", *storyName)
		repairResult, err := workitem.RepairSheets(workitem.RepairSheetsOptions{
			StoryName:     *storyName,
			SourceDir:     layout.AgentDir,
			InputDir:      layout.AgentDoneDir,
			QuarantineDir: filepath.Join(layout.Dir, "agent-done-quarantine"),
			RepairLog:     filepath.Join(layout.Dir, "agent-repair-log.jsonl"),
		})
		if err != nil {
			return err
		}
		printRepairSheetsResult(repairResult)
		if repairResult.HasBlockingResults() {
			return fmt.Errorf("acceptance failed: agent sheet repair found blocking result(s)")
		}
	}

	fmt.Fprintf(os.Stderr, "acceptance: validating completed agent sheets for %s\n", *storyName)
	sheetResult, err := workitem.ValidateSheets(workitem.ValidateSheetsOptions{
		SourceDir: layout.AgentDir,
		InputDir:  layout.AgentDoneDir,
	})
	if err != nil {
		return err
	}
	printAgentValidationProgress(sheetResult)
	if len(sheetResult.Failures) > 0 {
		for _, failure := range sheetResult.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
		return fmt.Errorf("acceptance failed: agent sheet validation failed")
	}
	fmt.Fprintf(os.Stderr, "acceptance: %d completed agent sheet(s) ready\n", sheetResult.FilesValidated)

	fmt.Fprintln(os.Stderr, "acceptance: checking agent sheet import")
	checkResult, err := workitem.ImportSheets(workitem.ImportSheetsOptions{
		SourceDir: layout.ChunkDir,
		InputDir:  layout.AgentDoneDir,
		Check:     true,
	})
	if len(checkResult.Failures) > 0 {
		for _, failure := range checkResult.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
	}
	if err != nil {
		return err
	}
	if checkResult.FilesValidated != sheetResult.FilesValidated {
		return fmt.Errorf("acceptance failed: checked %d sheet(s), want %d", checkResult.FilesValidated, sheetResult.FilesValidated)
	}

	fmt.Fprintln(os.Stderr, "acceptance: importing completed agent sheets")
	importResult, err := workitem.ImportSheets(workitem.ImportSheetsOptions{
		SourceDir: layout.ChunkDir,
		InputDir:  layout.AgentDoneDir,
		OutputDir: layout.DoneDir,
	})
	if len(importResult.Failures) > 0 {
		for _, failure := range importResult.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
	}
	if err != nil {
		return err
	}
	if len(importResult.Files) != sheetResult.FilesValidated {
		return fmt.Errorf("acceptance failed: imported %d completed work item(s), want %d", len(importResult.Files), sheetResult.FilesValidated)
	}
	fmt.Fprintf(os.Stderr, "acceptance: imported %d completed work item(s)\n", len(importResult.Files))

	fmt.Fprintln(os.Stderr, "acceptance: validating completed work item JSON")
	workResult, err := workitem.ValidateWorkItems(workitem.ValidateWorkItemsOptions{
		SourceDir: layout.ChunkDir,
		InputDir:  layout.DoneDir,
		FixBOM:    *fixBOM,
	})
	if err != nil {
		return err
	}
	printWorkItemValidationProgress(workResult)
	if len(workResult.Failures) > 0 {
		for _, failure := range workResult.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
		return fmt.Errorf("acceptance failed: completed work item validation failed")
	}

	fmt.Fprintln(os.Stderr, "acceptance: merging completed work items")
	mergeResult, err := workitem.Merge(workitem.MergeOptions{
		StoryPath: layout.StoryPath,
		InputDir:  layout.DoneDir,
	})
	if err != nil {
		return err
	}
	if mergeResult.FilesMerged != workResult.FilesValidated {
		return fmt.Errorf("acceptance failed: merged %d file(s), want %d", mergeResult.FilesMerged, workResult.FilesValidated)
	}
	if mergeResult.TranslationsMerged != workResult.Translations {
		return fmt.Errorf("acceptance failed: merged %d translation(s), want %d", mergeResult.TranslationsMerged, workResult.Translations)
	}
	fmt.Fprintf(os.Stderr, "acceptance: merged %d translations from %d work item file(s)\n", mergeResult.TranslationsMerged, mergeResult.FilesMerged)

	s, err := story.LoadFile(layout.StoryPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "acceptance: %s is valid\n", layout.StoryPath)
	if err := story.ValidateComplete(s); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "acceptance: %s is complete\n", layout.StoryPath)
	fmt.Fprintf(os.Stderr, "acceptance passed for %s\n", *storyName)
	return nil
}

func runValidateAgentWork(args []string) error {
	fs := flag.NewFlagSet("validate-agent-work", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	source := fs.String("source", "", "directory containing original translator-friendly text sheets")
	in := fs.String("in", "", "directory containing completed translator-friendly text sheets")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && (*source == "" || *in == "") {
		return fmt.Errorf("validate-agent-work requires -story or both -source and -in")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *source == "" {
			*source = layout.AgentDir
		}
		if *in == "" {
			*in = layout.AgentDoneDir
		}
	}

	result, err := workitem.ValidateSheets(workitem.ValidateSheetsOptions{
		SourceDir: *source,
		InputDir:  *in,
		Files:     fs.Args(),
	})
	if err != nil {
		return err
	}
	printAgentValidationProgress(result)
	if len(result.Failures) > 0 {
		for _, failure := range result.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
		return fmt.Errorf("agent sheet validation failed: %d failure(s) across %d completed file(s)", len(result.Failures), result.FilesValidated)
	}
	fmt.Fprintf(os.Stderr, "validated %d completed agent sheet(s)\n", result.FilesValidated)
	return nil
}

func runValidateWorkItems(args []string) error {
	fs := flag.NewFlagSet("validate-workitems", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	source := fs.String("source", "", "directory containing source work item JSON files")
	in := fs.String("in", "", "directory containing completed work item JSON files")
	inputPath := fs.String("input-path", "", "source work item JSON file for single-file validation")
	outputPath := fs.String("output-path", "", "completed work item JSON file for single-file validation")
	fixBOM := fs.Bool("fix-bom", false, "strip UTF-8 BOM bytes from work item JSON files before validation")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	singleMode := *inputPath != "" || *outputPath != ""
	if singleMode {
		if *storyName != "" || *source != "" || *in != "" {
			return fmt.Errorf("validate-workitems uses either -story/-source/-in or -input-path/-output-path, not both")
		}
		if *inputPath == "" || *outputPath == "" {
			return fmt.Errorf("validate-workitems requires both -input-path and -output-path in single-file mode")
		}
		result, err := workitem.ValidateWorkItemPair(workitem.ValidateWorkItemPairOptions{
			SourcePath: *inputPath,
			InputPath:  *outputPath,
			FixBOM:     *fixBOM,
		})
		if err != nil {
			return err
		}
		printWorkItemValidationProgress(result)
		if len(result.Failures) > 0 {
			for _, failure := range result.Failures {
				fmt.Fprintln(os.Stderr, failure.String())
			}
			return fmt.Errorf("completed work item validation failed: %d failure(s)", len(result.Failures))
		}
		fmt.Fprintf(os.Stderr, "validated %d completed work item(s); %d translation(s)\n", result.FilesValidated, result.Translations)
		return nil
	}

	if *storyName == "" && (*source == "" || *in == "") {
		return fmt.Errorf("validate-workitems requires -story or both -source and -in")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *source == "" {
			*source = layout.ChunkDir
		}
		if *in == "" {
			*in = layout.DoneDir
		}
	}

	result, err := workitem.ValidateWorkItems(workitem.ValidateWorkItemsOptions{
		SourceDir: *source,
		InputDir:  *in,
		FixBOM:    *fixBOM,
	})
	if err != nil {
		return err
	}
	printWorkItemValidationProgress(result)
	if len(result.Failures) > 0 {
		for _, failure := range result.Failures {
			fmt.Fprintln(os.Stderr, failure.String())
		}
		return fmt.Errorf("completed work item validation failed: %d failure(s) across %d completed file(s)", len(result.Failures), result.FilesValidated)
	}
	fmt.Fprintf(os.Stderr, "validated %d completed work item(s); %d translation(s)\n", result.FilesValidated, result.Translations)
	return nil
}

func runRepairAgentSheets(args []string) error {
	fs := flag.NewFlagSet("repair-agent-sheets", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	source := fs.String("source", "", "directory containing original translator-friendly text sheets")
	in := fs.String("in", "", "directory containing completed translator-friendly text sheets")
	sourceSheet := fs.String("source-sheet", "", "source sheet path for single-file repair")
	doneSheet := fs.String("done-sheet", "", "completed sheet path for single-file repair")
	check := fs.Bool("check", false, "report repairs without writing files")
	rewriteFromSource := fs.Bool("rewrite-from-source", false, "rebuild completed sheet shape from the source sheet and salvage translations")
	quarantineInvalid := fs.Bool("quarantine-invalid", false, "move invalid completed sheets out of agent-done after diagnostics")
	quarantineDir := fs.String("quarantine-dir", "", "directory for quarantined invalid completed sheets")
	repairLog := fs.String("repair-log", "", "JSONL repair log path")
	var files multiFlag
	fs.Var(&files, "file", "sheet name to process in story or directory mode; can be repeated")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	files = append(files, fs.Args()...)
	singleMode := *sourceSheet != "" || *doneSheet != ""
	if singleMode {
		if *storyName != "" || *source != "" || *in != "" || len(files) > 0 {
			return fmt.Errorf("repair-agent-sheets uses either -story/-source/-in or -source-sheet/-done-sheet, not both")
		}
		result, err := workitem.RepairSheetPair(workitem.RepairSheetPairOptions{
			SourceSheet:       *sourceSheet,
			DoneSheet:         *doneSheet,
			Check:             *check,
			RewriteFromSource: *rewriteFromSource,
			QuarantineInvalid: *quarantineInvalid,
			QuarantineDir:     *quarantineDir,
			RepairLog:         *repairLog,
		})
		if err != nil {
			return err
		}
		printRepairSheetsResult(result)
		if result.HasBlockingResults() {
			return fmt.Errorf("agent sheet repair found blocking result(s)")
		}
		return nil
	}

	if *storyName == "" && (*source == "" || *in == "") {
		return fmt.Errorf("repair-agent-sheets requires -story, both -source and -in, or both -source-sheet and -done-sheet")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *source == "" {
			*source = layout.AgentDir
		}
		if *in == "" {
			*in = layout.AgentDoneDir
		}
		if *repairLog == "" {
			*repairLog = filepath.Join(layout.Dir, "agent-repair-log.jsonl")
		}
		if *quarantineDir == "" {
			*quarantineDir = filepath.Join(layout.Dir, "agent-done-quarantine")
		}
	}

	result, err := workitem.RepairSheets(workitem.RepairSheetsOptions{
		StoryName:         *storyName,
		SourceDir:         *source,
		InputDir:          *in,
		Files:             files,
		Check:             *check,
		RewriteFromSource: *rewriteFromSource,
		QuarantineInvalid: *quarantineInvalid,
		QuarantineDir:     *quarantineDir,
		RepairLog:         *repairLog,
	})
	if err != nil {
		return err
	}
	printRepairSheetsResult(result)
	if result.HasBlockingResults() {
		return fmt.Errorf("agent sheet repair found blocking result(s)")
	}
	return nil
}

type multiFlag []string

func (f *multiFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func printRepairSheetsResult(result workitem.RepairSheetsResult) {
	for _, file := range result.Files {
		if len(file.Issues) == 0 {
			fmt.Fprintf(os.Stderr, "%s: %s\n", file.Status, file.Name)
		} else {
			for _, issue := range file.Issues {
				fmt.Fprintf(os.Stderr, "%s: %s: %s\n", file.Status, file.Name, issue)
			}
		}
		if file.QuarantinedPath != "" {
			fmt.Fprintf(os.Stderr, "quarantined: %s: %s\n", file.Name, file.QuarantinedPath)
		}
	}
	counts := result.Counts()
	if counts["would-fix"] > 0 {
		fmt.Fprintf(os.Stderr, "Total: %d fixed, %d would-fix, %d ok, %d missing, %d invalid, %d extra\n",
			counts["fixed"], counts["would-fix"], counts["ok"], counts["missing"], counts["invalid"], counts["extra"])
		return
	}
	fmt.Fprintf(os.Stderr, "Total: %d fixed, %d ok, %d missing, %d invalid, %d extra\n",
		counts["fixed"], counts["ok"], counts["missing"], counts["invalid"], counts["extra"])
}

func printAgentValidationProgress(result workitem.SheetValidationResult) {
	if len(result.Files) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "agent sheet validation:")
	for _, file := range result.Files {
		if file.FailureCount == 0 {
			fmt.Fprintf(os.Stderr, "  %-14s %s\n", file.Status, file.File)
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-14s %s (%d failure(s))\n", file.Status, file.File, file.FailureCount)
	}
}

func printWorkItemValidationProgress(result workitem.WorkItemValidationResult) {
	if len(result.Files) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "completed work item validation:")
	for _, file := range result.Files {
		if file.FailureCount == 0 {
			fmt.Fprintf(os.Stderr, "  %-14s %s\n", file.Status, file.File)
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-14s %s (%d failure(s))\n", file.Status, file.File, file.FailureCount)
	}
}

func runMergeWork(args []string) error {
	fs := flag.NewFlagSet("merge-work", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	storyPath := fs.String("story-path", "", "story JSON file to update")
	in := fs.String("in", "", "directory containing completed work item JSON files")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName == "" && *storyPath == "" {
		return fmt.Errorf("merge-work requires -story")
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *storyPath == "" {
			*storyPath = layout.StoryPath
		}
		if *in == "" {
			*in = layout.DoneDir
		}
	}
	if *in == "" {
		return fmt.Errorf("merge-work requires -story or -in")
	}

	result, err := workitem.Merge(workitem.MergeOptions{
		StoryPath: *storyPath,
		InputDir:  *in,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "merged %d translations from %d work item files\n", result.TranslationsMerged, result.FilesMerged)
	return nil
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	complete := fs.Bool("complete", false, "require all configured translations to be present")
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	path := ""
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		path = layout.StoryPath
	}
	if path == "" {
		if fs.NArg() != 1 {
			return fmt.Errorf("validate requires -story or one story JSON path")
		}
		path = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return fmt.Errorf("validate accepts either -story or a story JSON path, not both")
	}

	s, err := story.LoadFile(path)
	if err != nil {
		return err
	}
	if *complete {
		if err := story.ValidateComplete(s); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "%s is valid\n", path)
	return nil
}

func runChunk(args []string) error {
	fs := flag.NewFlagSet("chunk", flag.ContinueOnError)
	storyName := fs.String("story", "", "story name under stories/")
	storiesRoot := fs.String("stories", "stories", "stories root directory")
	in := fs.String("in", "", "source text file")
	out := fs.String("out", "", "draft story JSON output path")
	id := fs.String("id", "", "story ID; defaults to a slug from the input filename")
	title := fs.String("title", "", "story title; defaults to title-cased story ID")
	sourceLanguage := fs.String("source-language", "en", "source language: en or ja")
	wordsPerChunk := fs.Int("words-per-chunk", 0, "target number of source words/chars per chunk (default: 220 for en, 700 for ja)")
	paragraphsPerChunk := fs.Int("paragraphs-per-chunk", 0, "optional fixed number of paragraphs per chunk")
	force := fs.Bool("force", false, "overwrite an existing output file")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storyName != "" {
		layout, err := resolveStoryLayout(*storiesRoot, *storyName)
		if err != nil {
			return err
		}
		if *in == "" {
			*in = layout.CleanedPath
			if _, err := os.Stat(*in); os.IsNotExist(err) {
				*in = layout.SourcePath
			} else if err != nil {
				return err
			}
		}
		if *out == "" {
			*out = layout.StoryPath
		}
		if *id == "" {
			*id = layout.Name
		}
	}
	if *in == "" {
		return fmt.Errorf("chunk requires -story or -in")
	}
	if *out == "" {
		return fmt.Errorf("chunk requires -story or -out")
	}
	if !*force {
		if err := ensureNotExists(*out); err != nil {
			return err
		}
	}

	text, err := os.ReadFile(*in)
	if err != nil {
		return err
	}

	if err := writeDraftStory(*in, *out, *id, *title, *paragraphsPerChunk, *wordsPerChunk, *sourceLanguage, string(text)); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "wrote draft story JSON to %s\n", *out)
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "config.json", "server config file")
	addr := fs.String("addr", "", "HTTP address for the local reader")
	storiesDir := fs.String("stories", "", "directory containing story directories")
	voicevoxBaseURL := fs.String("voicevox", "", "VoiceVox base URL")
	voicevoxSpeaker := fs.Int("voicevox-speaker", 0, "VoiceVox speaker ID")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	appCfg, err := appconfig.Load(*configPath)
	if err != nil {
		return err
	}
	if flagProvided(fs, "addr") {
		appCfg.Addr = *addr
	}
	if flagProvided(fs, "stories") {
		appCfg.Stories.Dir = *storiesDir
	}
	if flagProvided(fs, "voicevox") {
		appCfg.VoiceVox.BaseURL = *voicevoxBaseURL
	}
	if flagProvided(fs, "voicevox-speaker") {
		appCfg.VoiceVox.SpeakerID = *voicevoxSpeaker
	}
	appCfg.ApplyDefaults()

	srv := server.New(server.Config{
		Addr:            appCfg.Addr,
		StoriesDir:      appCfg.Stories.Dir,
		VoiceVoxBaseURL: appCfg.VoiceVox.BaseURL,
		VoiceVoxSpeaker: appCfg.VoiceVox.SpeakerID,
		VoiceVoxName:    appCfg.VoiceVox.SpeakerName,
		VoiceVoxOptions: appCfg.VoiceVox,
		ConfigPath:      *configPath,
	})
	return srv.ListenAndServe()
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func ensureNotExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite %s without -force", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cleanSourceFile(path string, opts sourcecleaner.Options) (sourcecleaner.Result, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return sourcecleaner.Result{}, err
	}
	result, err := sourcecleaner.Clean(string(text), opts)
	if err != nil {
		return sourcecleaner.Result{}, err
	}
	if strings.TrimSpace(result.Text) == "" {
		return sourcecleaner.Result{}, fmt.Errorf("source text did not contain any cleaned paragraphs")
	}
	return result, nil
}

func writeTextFile(path string, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func writeDraftStory(sourceFile string, out string, id string, title string, paragraphsPerChunk int, wordsPerChunk int, sourceLanguage string, text string) error {
	draft, err := chunker.Draft(text, chunker.Options{
		StoryID:            id,
		Title:              title,
		SourceFile:         sourceFile,
		SourceLanguage:     sourceLanguage,
		ParagraphsPerChunk: paragraphsPerChunk,
		WordsPerChunk:      wordsPerChunk,
	})
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	return os.WriteFile(out, data, 0644)
}

func printCleanSourceResult(path string, result sourcecleaner.Result) {
	fmt.Fprintf(os.Stderr, "wrote cleaned source text to %s\n", path)
	fmt.Fprintf(os.Stderr, "lines=%d blank_lines=%d paragraphs=%d encoding_replacements=%d hyphenation_repairs=%d paragraph_breaks_added=%d\n",
		result.Stats.LinesIn,
		result.Stats.BlankLinesIn,
		result.Stats.ParagraphsOut,
		result.Stats.EncodingReplacements,
		result.Stats.HyphenationRepairs,
		result.Stats.ParagraphBreaksAdded,
	)
}

func defaultCleanedPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".cleaned.txt"
	}
	return strings.TrimSuffix(path, ext) + ".cleaned" + ext
}

type storyLayout struct {
	Name         string
	Dir          string
	SourcePath   string
	CleanedPath  string
	StoryPath    string
	ChunkDir     string
	AgentDir     string
	AgentDoneDir string
	DoneDir      string
}

func resolveStoryLayout(storiesRoot, name string) (storyLayout, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return storyLayout{}, fmt.Errorf("story name is required")
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) || filepath.Ext(name) != "" {
		return storyLayout{}, fmt.Errorf("story must be a story name like foo_bar, not a path")
	}
	if strings.TrimSpace(storiesRoot) == "" {
		storiesRoot = "stories"
	}
	dir := filepath.Join(storiesRoot, name)
	return storyLayout{
		Name:         name,
		Dir:          dir,
		SourcePath:   filepath.Join(dir, name+".txt"),
		CleanedPath:  filepath.Join(dir, name+".cleaned.txt"),
		StoryPath:    filepath.Join(dir, name+".json"),
		ChunkDir:     filepath.Join(dir, "chunk"),
		AgentDir:     filepath.Join(dir, "agent"),
		AgentDoneDir: filepath.Join(dir, "agent-done"),
		DoneDir:      filepath.Join(dir, "done"),
	}, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  jpstories [serve] [-config config.json] [-addr 127.0.0.1:8080] [-stories stories] [-voicevox http://127.0.0.1:50021]
  jpstories clean-source -story my_story
  jpstories prepare-story -story my_story [-words-per-chunk 220]
  jpstories chunk -story my_story [-words-per-chunk 220]
  jpstories export-work -story my_story
  jpstories export-agent-work -story my_story
  jpstories validate-agent-work -story my_story
  jpstories validate-workitems -story my_story
  jpstories repair-agent-sheets -story my_story
  jpstories import-agent-work -story my_story [-check]
  jpstories accept-story -story my_story
  jpstories merge-work -story my_story
  jpstories validate [-complete] -story my_story`)
}
