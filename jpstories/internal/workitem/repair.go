package workitem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sheetHeader = "# jpstories translation sheet v1"
const sheetInstruction = "Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels."

var mojibakeMarkers = []string{"Ã", "æ", "å", "â", "ï¼", "Â"}

type RepairSheetsOptions struct {
	StoryName         string
	SourceDir         string
	InputDir          string
	Files             []string
	Check             bool
	RewriteFromSource bool
	QuarantineInvalid bool
	QuarantineDir     string
	RepairLog         string
}

type RepairSheetPairOptions struct {
	SourceSheet       string
	DoneSheet         string
	Check             bool
	RewriteFromSource bool
	QuarantineInvalid bool
	QuarantineDir     string
	RepairLog         string
}

type RepairSheetsResult struct {
	Files []RepairSheetFileResult
}

type RepairSheetFileResult struct {
	Name            string
	Status          string
	Issues          []string
	SourcePath      string
	DonePath        string
	QuarantinedPath string
}

func (r RepairSheetsResult) Counts() map[string]int {
	counts := map[string]int{
		"fixed":     0,
		"would-fix": 0,
		"ok":        0,
		"missing":   0,
		"invalid":   0,
		"extra":     0,
	}
	for _, file := range r.Files {
		counts[file.Status]++
	}
	return counts
}

func (r RepairSheetsResult) HasBlockingResults() bool {
	counts := r.Counts()
	return counts["missing"] > 0 || counts["invalid"] > 0 || counts["extra"] > 0 || counts["would-fix"] > 0
}

type repairField struct {
	Label string
	Value string
	Line  int
}

type repairSentence struct {
	ParagraphID string
	SentenceID  string
	Header      string
	Fields      []repairField
}

type repairSheet struct {
	Path           string
	Meta           map[string]string
	Levels         []string
	SourceFile     string
	Sentences      []repairSentence
	IsOutputFormat bool
}

func RepairSheets(opts RepairSheetsOptions) (RepairSheetsResult, error) {
	if strings.TrimSpace(opts.SourceDir) == "" {
		return RepairSheetsResult{}, fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.InputDir) == "" {
		return RepairSheetsResult{}, fmt.Errorf("input directory is required")
	}

	names, extras, err := repairSheetNames(opts.SourceDir, opts.InputDir, opts.Files)
	if err != nil {
		return RepairSheetsResult{}, err
	}

	check := opts.Check || opts.QuarantineInvalid
	var result RepairSheetsResult
	for _, name := range names {
		sourcePath := filepath.Join(opts.SourceDir, name)
		donePath := filepath.Join(opts.InputDir, name)
		result.Files = append(result.Files, repairSheetFile(sourcePath, donePath, check, opts.RewriteFromSource))
	}
	for _, name := range extras {
		result.Files = append(result.Files, RepairSheetFileResult{
			Name:     name,
			Status:   "extra",
			DonePath: filepath.Join(opts.InputDir, name),
		})
	}

	if opts.QuarantineInvalid && !opts.Check {
		quarantineDir := opts.QuarantineDir
		if quarantineDir == "" {
			quarantineDir = filepath.Join(filepath.Dir(opts.InputDir), "agent-done-quarantine")
		}
		quarantineInvalidRepairSheets(result.Files, quarantineDir)
	}
	if opts.RepairLog != "" {
		if err := appendRepairLog(opts.RepairLog, opts.StoryName, repairMode(opts.Check, opts.RewriteFromSource), result.Files); err != nil {
			return result, err
		}
	}
	return result, nil
}

func RepairSheetPair(opts RepairSheetPairOptions) (RepairSheetsResult, error) {
	if strings.TrimSpace(opts.SourceSheet) == "" {
		return RepairSheetsResult{}, fmt.Errorf("source sheet is required")
	}
	if strings.TrimSpace(opts.DoneSheet) == "" {
		return RepairSheetsResult{}, fmt.Errorf("done sheet is required")
	}
	check := opts.Check || opts.QuarantineInvalid
	result := RepairSheetsResult{
		Files: []RepairSheetFileResult{
			repairSheetFile(opts.SourceSheet, opts.DoneSheet, check, opts.RewriteFromSource),
		},
	}
	if opts.QuarantineInvalid && !opts.Check {
		quarantineDir := opts.QuarantineDir
		if quarantineDir == "" {
			quarantineDir = filepath.Join(filepath.Dir(opts.DoneSheet), "quarantine")
		}
		quarantineInvalidRepairSheets(result.Files, quarantineDir)
	}
	if opts.RepairLog != "" {
		if err := appendRepairLog(opts.RepairLog, "", repairMode(opts.Check, opts.RewriteFromSource), result.Files); err != nil {
			return result, err
		}
	}
	return result, nil
}

func repairSheetFile(sourcePath, donePath string, check bool, rewriteFromSource bool) RepairSheetFileResult {
	name := filepath.Base(donePath)
	result := RepairSheetFileResult{
		Name:       name,
		SourcePath: sourcePath,
		DonePath:   donePath,
	}
	if !fileExists(sourcePath) {
		result.Status = "invalid"
		result.Issues = []string{fmt.Sprintf("source sheet not found: %s", sourcePath)}
		return result
	}
	if !fileExists(donePath) {
		result.Status = "missing"
		return result
	}

	changed := false
	if hasUTF8BOM(donePath) {
		changed = true
		if !check {
			if err := removeUTF8BOM(donePath); err != nil {
				result.Status = "invalid"
				result.Issues = []string{fmt.Sprintf("remove UTF-8 BOM: %v", err)}
				return result
			}
		}
	}

	text, err := readRepairText(donePath)
	if err != nil {
		result.Status = "invalid"
		result.Issues = []string{err.Error()}
		return result
	}
	sourceText, err := readRepairText(sourcePath)
	if err != nil {
		result.Status = "invalid"
		result.Issues = []string{fmt.Sprintf("read source sheet: %v", err)}
		return result
	}
	source, sourceIssues := parseRepairSheet(sourcePath, sourceText)
	done, doneIssues := parseRepairOutputSheet(donePath, text)
	if len(sourceIssues) > 0 {
		result.Status = "invalid"
		for _, issue := range sourceIssues {
			result.Issues = append(result.Issues, "source sheet invalid: "+issue)
		}
		return result
	}

	if rewriteFromSource {
		translations := map[repairSentenceKey]map[string]string{}
		if done != nil {
			translations = repairFieldValuesBySentence(done)
		}
		rewritten := renderOutputFromSource(source, translations)
		if rewritten != text {
			changed = true
			text = rewritten
			done, doneIssues = parseRepairOutputSheet(donePath, text)
			if !check {
				if err := writeRepairText(donePath, text); err != nil {
					result.Status = "invalid"
					result.Issues = []string{fmt.Sprintf("write repaired sheet: %v", err)}
					return result
				}
			}
		}
	} else {
		repaired, fixedFences := repairMissingFences(text)
		text = repaired
		changed = changed || fixedFences
		if changed && !check {
			if err := writeRepairText(donePath, text); err != nil {
				result.Status = "invalid"
				result.Issues = []string{fmt.Sprintf("write repaired sheet: %v", err)}
				return result
			}
		}
		done, doneIssues = parseRepairOutputSheet(donePath, text)
	}

	result.Issues = append(result.Issues, doneIssues...)
	if done != nil {
		result.Issues = append(result.Issues, repairSemanticIssues(source, done)...)
	}
	if len(result.Issues) > 0 {
		result.Status = "invalid"
		return result
	}
	if changed {
		if check {
			result.Status = "would-fix"
		} else {
			result.Status = "fixed"
		}
		return result
	}
	result.Status = "ok"
	return result
}

func repairSheetNames(sourceDir, inputDir string, files []string) ([]string, []string, error) {
	if len(files) > 0 {
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
		return names, nil, nil
	}

	sourceNames, err := textFileNames(sourceDir)
	if err != nil {
		return nil, nil, err
	}
	inputNames, err := textFileNames(inputDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	sourceSet := map[string]bool{}
	for _, name := range sourceNames {
		sourceSet[name] = true
	}
	var extras []string
	for _, name := range inputNames {
		if !sourceSet[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	return sourceNames, extras, nil
}

func textFileNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func parseRepairSheet(path string, text string) (*repairSheet, []string) {
	lines := normalizedRepairLines(text)
	var issues []string
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != sheetHeader {
		issues = append(issues, "missing translation sheet header")
	}

	sheet := &repairSheet{
		Path: path,
		Meta: map[string]string{},
	}
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
		if strings.Contains(line, ":") {
			key, value, _ := strings.Cut(line, ":")
			sheet.Meta[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		i++
	}
	sheet.Levels = splitSheetLevels(sheet.Meta["levels"])
	sheet.SourceFile = sheet.Meta["source_file"]
	if len(sheet.Levels) == 0 {
		issues = append(issues, "missing levels metadata")
	}

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "## ") {
			issues = append(issues, fmt.Sprintf("line %d: expected sentence header", i+1))
			i++
			continue
		}

		header := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		paragraphID, sentenceID, ok := strings.Cut(header, "/")
		if !ok {
			issues = append(issues, fmt.Sprintf("line %d: invalid sentence header %q", i+1, line))
			paragraphID = header
			sentenceID = ""
		}
		current := repairSentence{
			ParagraphID: strings.TrimSpace(paragraphID),
			SentenceID:  strings.TrimSpace(sentenceID),
			Header:      header,
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
				issues = append(issues, fmt.Sprintf("line %d: expected field label", i+1))
				i++
				continue
			}

			label := strings.TrimSpace(strings.TrimSuffix(line, ":"))
			labelLine := i + 1
			i++
			if i >= len(lines) || strings.TrimSpace(lines[i]) != sheetFence {
				issues = append(issues, fmt.Sprintf("%s: %s missing opening fence at line %d", header, label, labelLine))
				continue
			}
			i++

			var blockLines []string
			closed := false
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == sheetFenceEnd {
					closed = true
					i++
					break
				}
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") || isRepairKnownLabel(lines[i], sheet.Levels) {
					issues = append(issues, fmt.Sprintf("%s: %s missing closing fence", header, label))
					break
				}
				blockLines = append(blockLines, lines[i])
				i++
			}
			if !closed && i >= len(lines) {
				issues = append(issues, fmt.Sprintf("%s: %s missing closing fence", header, label))
			}
			current.Fields = append(current.Fields, repairField{
				Label: label,
				Value: strings.Join(blockLines, "\n"),
				Line:  labelLine,
			})
		}
		sheet.Sentences = append(sheet.Sentences, current)
	}
	return sheet, issues
}

func parseRepairOutputSheet(path string, text string) (*repairSheet, []string) {
	lines := normalizedRepairLines(text)
	var issues []string
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != outputHeader {
		issues = append(issues, "missing translation output header")
	}

	sheet := &repairSheet{
		Path:           path,
		Meta:           map[string]string{},
		IsOutputFormat: true,
	}
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
		if strings.Contains(line, ":") {
			key, value, _ := strings.Cut(line, ":")
			sheet.Meta[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		i++
	}
	sheet.Levels = splitSheetLevels(sheet.Meta["levels"])
	if len(sheet.Levels) == 0 {
		issues = append(issues, "missing levels metadata")
	}

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "## ") {
			issues = append(issues, fmt.Sprintf("line %d: expected sentence header", i+1))
			i++
			continue
		}

		sentenceID := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		current := repairSentence{
			SentenceID: sentenceID,
			Header:     sentenceID,
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
				issues = append(issues, fmt.Sprintf("line %d: expected field label", i+1))
				i++
				continue
			}

			label := strings.TrimSpace(strings.TrimSuffix(line, ":"))
			labelLine := i + 1
			i++
			if i >= len(lines) || strings.TrimSpace(lines[i]) != sheetFence {
				issues = append(issues, fmt.Sprintf("%s: %s missing opening fence at line %d", sentenceID, label, labelLine))
				continue
			}
			i++

			var blockLines []string
			closed := false
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == sheetFenceEnd {
					closed = true
					i++
					break
				}
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") || isRepairKnownLabel(lines[i], sheet.Levels) {
					issues = append(issues, fmt.Sprintf("%s: %s missing closing fence", sentenceID, label))
					break
				}
				blockLines = append(blockLines, lines[i])
				i++
			}
			if !closed && i >= len(lines) {
				issues = append(issues, fmt.Sprintf("%s: %s missing closing fence", sentenceID, label))
			}
			current.Fields = append(current.Fields, repairField{
				Label: label,
				Value: strings.Join(blockLines, "\n"),
				Line:  labelLine,
			})
		}
		sheet.Sentences = append(sheet.Sentences, current)
	}
	return sheet, issues
}

func renderOutputFromSource(source *repairSheet, translations map[repairSentenceKey]map[string]string) string {
	var out []string
	out = append(out,
		outputHeader,
		fmt.Sprintf("story_id: %s", source.Meta["story_id"]),
		fmt.Sprintf("chunk_id: %s", source.Meta["chunk_id"]),
		fmt.Sprintf("levels: %s", strings.Join(source.Levels, ",")),
		"",
	)
	for _, sentence := range source.Sentences {
		out = append(out, fmt.Sprintf("## %s", sentence.SentenceID))
		// Done sentences (output format) have empty ParagraphID; match by SentenceID only.
		key := repairSentenceKey{ParagraphID: "", SentenceID: sentence.SentenceID}
		for _, label := range source.Levels {
			sourceHasField := false
			for _, f := range sentence.Fields {
				if f.Label == label {
					sourceHasField = true
					break
				}
			}
			if !sourceHasField {
				continue
			}
			value := strings.TrimSpace(translations[key][label])
			out = append(out, label+":", sheetFence)
			if value != "" {
				out = append(out, normalizedRepairLines(value)...)
			}
			out = append(out, sheetFenceEnd)
		}
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func repairMissingFences(text string) (string, bool) {
	lines := normalizedRepairLines(text)
	var levels []string
	for i, line := range lines {
		if i >= 20 {
			break
		}
		if strings.HasPrefix(line, "levels:") {
			_, value, _ := strings.Cut(line, ":")
			levels = splitSheetLevels(value)
			break
		}
	}

	fixed := false
	var out []string
	inBlock := false
	awaitingFence := false
	for _, line := range lines {
		startsNext := isRepairKnownLabel(line, levels) || strings.HasPrefix(strings.TrimSpace(line), "## ")
		if inBlock && startsNext {
			out = append(out, sheetFenceEnd)
			fixed = true
			inBlock = false
		}
		out = append(out, line)
		if !inBlock && isRepairKnownLabel(line, levels) {
			awaitingFence = true
		} else if awaitingFence && strings.TrimSpace(line) == sheetFence {
			inBlock = true
			awaitingFence = false
		} else if inBlock && strings.TrimSpace(line) == sheetFenceEnd {
			inBlock = false
		}
	}
	if inBlock {
		out = append(out, sheetFenceEnd)
		fixed = true
	}
	return strings.Join(out, "\n"), fixed
}

type repairSentenceKey struct {
	ParagraphID string
	SentenceID  string
}

func repairFieldValuesBySentence(sheet *repairSheet) map[repairSentenceKey]map[string]string {
	values := map[repairSentenceKey]map[string]string{}
	for _, sentence := range sheet.Sentences {
		key := repairSentenceKey{ParagraphID: sentence.ParagraphID, SentenceID: sentence.SentenceID}
		labels := values[key]
		if labels == nil {
			labels = map[string]string{}
			values[key] = labels
		}
		for _, field := range sentence.Fields {
			if _, exists := labels[field.Label]; !exists {
				labels[field.Label] = field.Value
			}
		}
	}
	return values
}

func renderRepairSheetFromSource(source *repairSheet, translations map[repairSentenceKey]map[string]string) string {
	var out []string
	out = append(out,
		sheetHeader,
		fmt.Sprintf("story_id: %s", source.Meta["story_id"]),
		fmt.Sprintf("story_title: %s", source.Meta["story_title"]),
		fmt.Sprintf("chunk_id: %s", source.Meta["chunk_id"]),
		fmt.Sprintf("levels: %s", strings.Join(source.Levels, ",")),
		fmt.Sprintf("source_file: %s", source.SourceFile),
		"",
		sheetInstruction,
		"",
	)
	for _, sentence := range source.Sentences {
		out = append(out, fmt.Sprintf("## %s / %s", sentence.ParagraphID, sentence.SentenceID))
		sourceFields := map[string]string{}
		for _, field := range sentence.Fields {
			sourceFields[field.Label] = field.Value
		}
		key := repairSentenceKey{ParagraphID: sentence.ParagraphID, SentenceID: sentence.SentenceID}
		// Determine source label from metadata (default to "english" for backward compat).
		sourceLabel := source.Meta["source_label"]
		if sourceLabel == "" {
			if source.Meta["source_language"] == "ja" {
				sourceLabel = "native"
			} else {
				sourceLabel = "english"
			}
		}
		labels := append([]string{sourceLabel}, source.Levels...)
		for _, label := range labels {
			if _, ok := sourceFields[label]; !ok {
				continue
			}
			value := sourceFields[label]
			if label != sourceLabel {
				value = strings.TrimSpace(translations[key][label])
			}
			out = append(out, label+":", sheetFence)
			if value != "" {
				out = append(out, normalizedRepairLines(value)...)
			}
			out = append(out, sheetFenceEnd)
		}
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func repairSemanticIssues(source *repairSheet, done *repairSheet) []string {
	var issues []string
	for _, key := range []string{"story_id", "chunk_id", "levels"} {
		if source.Meta[key] != done.Meta[key] {
			issues = append(issues, fmt.Sprintf("metadata %s changed", key))
		}
	}

	// Compare sentence IDs only; done is new format (no paragraph IDs).
	sourceSentenceIDs := make([]string, 0, len(source.Sentences))
	for _, s := range source.Sentences {
		sourceSentenceIDs = append(sourceSentenceIDs, s.SentenceID)
	}
	doneSentenceIDs := make([]string, 0, len(done.Sentences))
	for _, s := range done.Sentences {
		doneSentenceIDs = append(doneSentenceIDs, s.SentenceID)
	}
	if strings.Join(sourceSentenceIDs, "\x00") != strings.Join(doneSentenceIDs, "\x00") {
		sourceSet := stringSet(sourceSentenceIDs)
		doneSet := stringSet(doneSentenceIDs)
		missingOrExtra := false
		for _, id := range sourceSentenceIDs {
			if !doneSet[id] {
				issues = append(issues, fmt.Sprintf("%s: missing sentence", id))
				missingOrExtra = true
			}
		}
		for _, id := range doneSentenceIDs {
			if !sourceSet[id] {
				issues = append(issues, fmt.Sprintf("%s: extra sentence", id))
				missingOrExtra = true
			}
		}
		if !missingOrExtra {
			issues = append(issues, "sentence order changed")
		}
	}

	// Determine source label for this sheet.
	sourceLabel := source.Meta["source_label"]
	if sourceLabel == "" {
		if source.Meta["source_language"] == "ja" {
			sourceLabel = "native"
		} else {
			sourceLabel = "english"
		}
	}

	// Build expected translation field presence from source (by sentence ID).
	// The source-context block is excluded; only produce blocks are expected.
	sourceFieldsBySentenceID := map[string]map[string]bool{}
	for _, s := range source.Sentences {
		fields := map[string]bool{}
		for _, f := range s.Fields {
			if f.Label != sourceLabel {
				fields[f.Label] = true
			}
		}
		sourceFieldsBySentenceID[s.SentenceID] = fields
	}

	for _, sentence := range done.Sentences {
		expected := sourceFieldsBySentenceID[sentence.SentenceID]
		seen := map[string]int{}
		for _, field := range sentence.Fields {
			seen[field.Label]++
			if seen[field.Label] > 1 {
				issues = append(issues, fmt.Sprintf("%s: duplicate %s block", sentence.Header, field.Label))
			}
			if !containsString(source.Levels, field.Label) {
				issues = append(issues, fmt.Sprintf("%s: unexpected %s block", sentence.Header, field.Label))
				continue
			}
			if strings.TrimSpace(field.Value) == "" {
				issues = append(issues, fmt.Sprintf("%s: empty %s translation", sentence.Header, field.Label))
			} else if containsAny(field.Value, mojibakeMarkers) {
				issues = append(issues, fmt.Sprintf("%s: suspicious mojibake in %s translation", sentence.Header, field.Label))
			}
		}
		for label := range expected {
			if seen[label] == 0 {
				issues = append(issues, fmt.Sprintf("%s: missing %s block", sentence.Header, label))
			}
		}
	}
	return issues
}

func isRepairKnownLabel(line string, levels []string) bool {
	line = strings.TrimSpace(line)
	// These are always valid labels: "english" is either the source (en) or a produce field (ja);
	// "native" is either the source (ja) or a produce field (en).
	if line == "english:" || line == "native:" {
		return true
	}
	for _, level := range levels {
		if line == level+":" {
			return true
		}
	}
	return false
}

func asciiQuotes(text string) string {
	replacer := strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'")
	return replacer.Replace(text)
}

func replaceRepairBlockText(text string, label string, oldValue string, newValue string) string {
	for _, newline := range []string{"\r\n", "\n"} {
		oldBlock := label + ":" + newline + sheetFence + newline + oldValue + newline + sheetFenceEnd
		newBlock := label + ":" + newline + sheetFence + newline + newValue + newline + sheetFenceEnd
		if strings.Contains(text, oldBlock) {
			return strings.Replace(text, oldBlock, newBlock, 1)
		}
	}
	return text
}

func normalizedRepairLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func readRepairText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	return string(data), nil
}

func writeRepairText(path string, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func hasUTF8BOM(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func quarantineInvalidRepairSheets(files []RepairSheetFileResult, quarantineDir string) {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	for i := range files {
		if files[i].Status != "invalid" || files[i].DonePath == "" || !fileExists(files[i].DonePath) {
			continue
		}
		if err := os.MkdirAll(quarantineDir, 0755); err != nil {
			files[i].Issues = append(files[i].Issues, fmt.Sprintf("quarantine failed: %v", err))
			continue
		}
		target := filepath.Join(quarantineDir, timestamp+"_"+files[i].Name)
		for counter := 2; fileExists(target); counter++ {
			target = filepath.Join(quarantineDir, fmt.Sprintf("%s_%d_%s", timestamp, counter, files[i].Name))
		}
		if err := moveFile(files[i].DonePath, target); err != nil {
			files[i].Issues = append(files[i].Issues, fmt.Sprintf("quarantine failed: %v", err))
			continue
		}
		files[i].QuarantinedPath = target
	}
}

func moveFile(source string, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	if err := copyFile(source, target); err != nil {
		return err
	}
	return os.Remove(source)
}

func copyFile(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func appendRepairLog(path string, storyName string, mode string, files []RepairSheetFileResult) error {
	if path == "" {
		return nil
	}
	interesting := map[string]bool{
		"fixed":     true,
		"would-fix": true,
		"invalid":   true,
		"extra":     true,
		"missing":   true,
	}
	var rows []RepairSheetFileResult
	for _, file := range files {
		if interesting[file.Status] || file.QuarantinedPath != "" {
			rows = append(rows, file)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	for _, file := range rows {
		payload := map[string]any{
			"time":   now,
			"story":  storyName,
			"file":   file.Name,
			"status": file.Status,
			"mode":   mode,
			"issues": file.Issues,
		}
		if file.QuarantinedPath != "" {
			payload["quarantined_path"] = file.QuarantinedPath
		}
		if err := encoder.Encode(payload); err != nil {
			return err
		}
	}
	return nil
}

func repairMode(check bool, rewriteFromSource bool) string {
	if check && rewriteFromSource {
		return "check-rewrite-from-source"
	}
	if check {
		return "check"
	}
	if rewriteFromSource {
		return "rewrite-from-source"
	}
	return "repair"
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
