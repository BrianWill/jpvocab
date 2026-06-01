package chunker

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"jpstories/internal/story"
)

const defaultWordsPerChunk = 220
const defaultJaCharsPerChunk = 700

var blankLineRE = regexp.MustCompile(`\r?\n\s*\r?\n+`)

type Options struct {
	StoryID            string
	Title              string
	SourceFile         string
	SourceLanguage     string
	ParagraphsPerChunk int
	WordsPerChunk      int
}

func Draft(text string, opts Options) (story.Story, error) {
	lang := opts.SourceLanguage
	if lang == "" {
		lang = "en"
	}

	paragraphs := splitParagraphsForLang(text, lang)
	if len(paragraphs) == 0 {
		return story.Story{}, errors.New("source text must include at least one paragraph")
	}

	chunkGroups := groupParagraphsForLang(paragraphs, opts, lang)

	storyID := strings.TrimSpace(opts.StoryID)
	if storyID == "" {
		storyID = inferStoryID(opts.SourceFile)
	}
	if storyID == "" {
		return story.Story{}, errors.New("story ID is required when it cannot be inferred from source file")
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = inferTitle(storyID)
	}

	targetLang := "ja"
	if lang == "ja" {
		targetLang = "en"
	}

	s := story.Story{
		ID:             storyID,
		Title:          title,
		SourceLanguage: lang,
		TargetLanguage: targetLang,
		SourceFile:     filepath.ToSlash(strings.TrimSpace(opts.SourceFile)),
		Levels:         append([]string(nil), story.SupportedLevels...),
	}

	nextParagraphID := 1
	nextSentenceID := 1
	for chunkIndex, paragraphGroup := range chunkGroups {
		chunk := story.Chunk{
			ID: fmt.Sprintf("chunk-%03d", chunkIndex+1),
		}
		for _, paragraphText := range paragraphGroup {
			paragraph := story.Paragraph{
				ID: fmt.Sprintf("p-%03d", nextParagraphID),
			}
			nextParagraphID++

			for _, sentenceText := range splitSentencesForLang(paragraphText, lang) {
				var sentence story.Sentence
				sentence.ID = fmt.Sprintf("s-%03d", nextSentenceID)
				sentence.Translations = map[string]string{}
				if lang == "ja" {
					sentence.Translations[story.LevelNative] = sentenceText
				} else {
					sentence.English = sentenceText
				}
				paragraph.Sentences = append(paragraph.Sentences, sentence)
				nextSentenceID++
			}
			chunk.Paragraphs = append(chunk.Paragraphs, paragraph)
		}
		s.Chunks = append(s.Chunks, chunk)
	}

	if err := story.Validate(s); err != nil {
		return story.Story{}, err
	}
	return s, nil
}

func groupParagraphsForLang(paragraphs []string, opts Options, lang string) [][]string {
	if opts.ParagraphsPerChunk > 0 {
		return groupParagraphsByCount(paragraphs, opts.ParagraphsPerChunk)
	}
	wordsPerChunk := opts.WordsPerChunk
	if wordsPerChunk <= 0 {
		if lang == "ja" {
			wordsPerChunk = defaultJaCharsPerChunk
		} else {
			wordsPerChunk = defaultWordsPerChunk
		}
	}
	return groupParagraphsByWordTarget(paragraphs, wordsPerChunk)
}

func groupParagraphsByCount(paragraphs []string, paragraphsPerChunk int) [][]string {
	var groups [][]string
	for start := 0; start < len(paragraphs); start += paragraphsPerChunk {
		end := start + paragraphsPerChunk
		if end > len(paragraphs) {
			end = len(paragraphs)
		}
		groups = append(groups, paragraphs[start:end])
	}
	return groups
}

func groupParagraphsByWordTarget(paragraphs []string, wordsPerChunk int) [][]string {
	var groups [][]string
	var current []string
	currentWords := 0

	for _, paragraph := range paragraphs {
		wordCount := sourceWordCount(paragraph)
		if len(current) > 0 && currentWords+wordCount > wordsPerChunk {
			groups = append(groups, current)
			current = nil
			currentWords = 0
		}

		current = append(current, paragraph)
		currentWords += wordCount
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func sourceWordCount(text string) int {
	words := len(strings.Fields(text))
	if words > 0 {
		return words
	}
	return len([]rune(strings.TrimSpace(text)))
}

func splitParagraphsForLang(text, lang string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := blankLineRE.Split(text, -1)
	paragraphs := make([]string, 0, len(parts))
	for _, part := range parts {
		var paragraph string
		if lang == "ja" {
			paragraph = normalizeParagraphJa(part)
		} else {
			paragraph = normalizeParagraph(part)
		}
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return paragraphs
}

func splitSentencesForLang(paragraph, lang string) []string {
	if lang == "ja" {
		return splitJapaneseSentences(paragraph)
	}
	return splitSentences(paragraph)
}

func splitSentences(paragraph string) []string {
	var sentences []string
	start := 0
	runes := []rune(paragraph)

	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '.', '!', '?', ';':
			if !isSentenceBoundary(runes, i) {
				continue
			}
			end := i + 1
			for end < len(runes) && isClosingPunctuation(runes[end]) {
				end++
			}
			if end < len(runes) && !unicode.IsSpace(runes[end]) {
				continue
			}

			sentence := strings.TrimSpace(string(runes[start:end]))
			if runes[i] == ';' {
				sentence = strings.TrimSpace(strings.TrimSuffix(sentence, ";")) + "."
			}
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			for end < len(runes) && unicode.IsSpace(runes[end]) {
				end++
			}
			start = end
			if runes[i] == ';' && start < len(runes) {
				runes[start] = unicode.ToUpper(runes[start])
			}
			i = end - 1
		}
	}

	remainder := strings.TrimSpace(string(runes[start:]))
	if remainder != "" {
		sentences = append(sentences, remainder)
	}
	return sentences
}

func isSentenceBoundary(runes []rune, index int) bool {
	switch runes[index] {
	case ';', '!', '?':
		return true
	case '.':
		return isPeriodSentenceBoundary(runes, index)
	default:
		return false
	}
}

func isPeriodSentenceBoundary(runes []rune, index int) bool {
	if isPartOfEllipsis(runes, index) || isDecimalPoint(runes, index) {
		return false
	}
	token := strings.Trim(strings.ToLower(wordBefore(runes, index)), "\"'()[]{}")
	if token == "" {
		return true
	}
	if isKnownAbbreviation(token) || isInitial(token) {
		return false
	}
	return true
}

func isPartOfEllipsis(runes []rune, index int) bool {
	return hasNeighboringPeriod(runes, index, -1) || hasNeighboringPeriod(runes, index, 1)
}

func hasNeighboringPeriod(runes []rune, index int, direction int) bool {
	for i := index + direction; i >= 0 && i < len(runes); i += direction {
		if runes[i] == '.' {
			return true
		}
		if !unicode.IsSpace(runes[i]) {
			return false
		}
	}
	return false
}

func isDecimalPoint(runes []rune, index int) bool {
	return index > 0 && index+1 < len(runes) && unicode.IsDigit(runes[index-1]) && unicode.IsDigit(runes[index+1])
}

func wordBefore(runes []rune, index int) string {
	start := index
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	return string(runes[start:index])
}

func isKnownAbbreviation(token string) bool {
	switch token {
	case "mr", "mrs", "ms", "miss", "mx", "dr", "prof", "sr", "jr", "st", "mt",
		"rev", "hon", "capt", "col", "gen", "lt", "sgt", "adm", "cmdr",
		"etc", "e.g", "i.e", "vs", "fig", "no", "vol", "ch", "pp":
		return true
	default:
		return false
	}
}

func isInitial(token string) bool {
	runes := []rune(strings.Trim(token, "."))
	return len(runes) == 1 && unicode.IsLetter(runes[0])
}

func normalizeParagraph(paragraph string) string {
	return strings.Join(strings.Fields(paragraph), " ")
}

func normalizeParagraphJa(paragraph string) string {
	lines := strings.Split(paragraph, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "")
}

func splitJapaneseSentences(paragraph string) []string {
	var sentences []string
	runes := []rune(paragraph)
	start := 0
	for i, r := range runes {
		if isJapaneseSentenceEnd(r) {
			end := i + 1
			// consume any closing brackets that follow the sentence-ender
			for end < len(runes) && isJapaneseSentenceCloser(runes[end]) {
				end++
			}
			s := strings.TrimSpace(string(runes[start:end]))
			if s != "" {
				sentences = append(sentences, s)
			}
			start = end
		}
	}
	if remainder := strings.TrimSpace(string(runes[start:])); remainder != "" {
		sentences = append(sentences, remainder)
	}
	return sentences
}

func isJapaneseSentenceEnd(r rune) bool {
	return r == '。' || r == '！' || r == '？'
}

func isJapaneseSentenceCloser(r rune) bool {
	switch r {
	case '」', '』', '）', '】', '〕', '〉', '》':
		return true
	default:
		return false
	}
}

func isClosingPunctuation(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '}':
		return true
	default:
		return false
	}
}

func inferStoryID(sourceFile string) string {
	base := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	return slug(base)
}

func inferTitle(storyID string) string {
	words := strings.Fields(strings.ReplaceAll(storyID, "-", " "))
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
