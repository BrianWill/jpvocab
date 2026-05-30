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
	wordsPerChunk := fs.Int("words-per-chunk", 220, "target number of source words per chunk")
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
	})
	if err != nil {
		return err
	}
	if err := writeTextFile(*cleanedOut, cleaned.Text); err != nil {
		return err
	}
	printCleanSourceResult(*cleanedOut, cleaned)

	if err := writeDraftStory(*cleanedOut, *storyPath, *id, *title, *paragraphsPerChunk, *wordsPerChunk, cleaned.Text); err != nil {
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
	in := fs.String("in", "", "English source text file")
	out := fs.String("out", "", "draft story JSON output path")
	id := fs.String("id", "", "story ID; defaults to a slug from the input filename")
	title := fs.String("title", "", "story title; defaults to title-cased story ID")
	wordsPerChunk := fs.Int("words-per-chunk", 220, "target number of source words per chunk")
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

	if err := writeDraftStory(*in, *out, *id, *title, *paragraphsPerChunk, *wordsPerChunk, string(text)); err != nil {
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

func writeDraftStory(sourceFile string, out string, id string, title string, paragraphsPerChunk int, wordsPerChunk int, text string) error {
	draft, err := chunker.Draft(text, chunker.Options{
		StoryID:            id,
		Title:              title,
		SourceFile:         sourceFile,
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
	Name        string
	Dir         string
	SourcePath  string
	CleanedPath string
	StoryPath   string
	ChunkDir    string
	DoneDir     string
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
		Name:        name,
		Dir:         dir,
		SourcePath:  filepath.Join(dir, name+".txt"),
		CleanedPath: filepath.Join(dir, name+".cleaned.txt"),
		StoryPath:   filepath.Join(dir, name+".json"),
		ChunkDir:    filepath.Join(dir, "chunk"),
		DoneDir:     filepath.Join(dir, "done"),
	}, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  jpstories [serve] [-config config.json] [-addr 127.0.0.1:8080] [-stories stories] [-voicevox http://127.0.0.1:50021]
  jpstories clean-source -story my_story
  jpstories prepare-story -story my_story [-words-per-chunk 220]
  jpstories chunk -story my_story [-words-per-chunk 220]
  jpstories export-work -story my_story
  jpstories merge-work -story my_story
  jpstories validate [-complete] -story my_story`)
}
