package sourcecleaner

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	ParagraphModePreserve     = "preserve"
	ParagraphModeConservative = "conservative"
	ParagraphModeDialogue     = "dialogue"
)

type Options struct {
	CleanEncoding     bool
	RepairHyphenation bool
	ParagraphMode     string
}

type Result struct {
	Text  string
	Stats Stats
}

type Stats struct {
	LinesIn              int
	BlankLinesIn         int
	ParagraphsOut        int
	EncodingReplacements int
	HyphenationRepairs   int
	ParagraphBreaksAdded int
}

func Clean(text string, opts Options) (Result, error) {
	mode := strings.TrimSpace(opts.ParagraphMode)
	if mode == "" {
		mode = ParagraphModeDialogue
	}
	if !supportedParagraphMode(mode) {
		return Result{}, fmt.Errorf("unsupported paragraph mode %q", mode)
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	stats := Stats{}
	if opts.CleanEncoding {
		var replacements int
		text, replacements = cleanEncoding(text)
		stats.EncodingReplacements = replacements
	}

	lines := strings.Split(text, "\n")
	stats.LinesIn = len(lines)

	var paragraphs []string
	var current paragraphBuilder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			stats.BlankLinesIn++
			if current.hasText() {
				paragraphs = append(paragraphs, current.text())
				current.reset()
			}
			continue
		}

		if current.hasText() && startsNewParagraph(current.text(), line, mode) {
			paragraphs = append(paragraphs, current.text())
			current.reset()
			stats.ParagraphBreaksAdded++
		}

		repaired := current.append(line, opts.RepairHyphenation)
		if repaired {
			stats.HyphenationRepairs++
		}
	}
	if current.hasText() {
		paragraphs = append(paragraphs, current.text())
	}

	stats.ParagraphsOut = len(paragraphs)
	return Result{
		Text:  strings.Join(paragraphs, "\n\n") + "\n",
		Stats: stats,
	}, nil
}

func supportedParagraphMode(mode string) bool {
	switch mode {
	case ParagraphModePreserve, ParagraphModeConservative, ParagraphModeDialogue:
		return true
	default:
		return false
	}
}

type paragraphBuilder struct {
	parts []string
}

func (b *paragraphBuilder) hasText() bool {
	return len(b.parts) > 0
}

func (b *paragraphBuilder) text() string {
	return strings.Join(b.parts, " ")
}

func (b *paragraphBuilder) reset() {
	b.parts = nil
}

func (b *paragraphBuilder) append(line string, repairHyphenation bool) bool {
	if !repairHyphenation || len(b.parts) == 0 {
		b.parts = append(b.parts, line)
		return false
	}

	last := b.parts[len(b.parts)-1]
	if !strings.HasSuffix(last, "-") || !startsLower(line) {
		b.parts = append(b.parts, line)
		return false
	}

	b.parts[len(b.parts)-1] = strings.TrimSuffix(last, "-") + line
	return true
}

func startsNewParagraph(current string, next string, mode string) bool {
	if mode == ParagraphModePreserve {
		return false
	}
	if startsWithOpeningQuote(next) && endsSentence(current) {
		return true
	}
	if mode != ParagraphModeDialogue {
		return false
	}
	return containsDialogue(current) && !startsWithOpeningQuote(next) && startsUpper(next) && endsSentence(current)
}

func startsWithOpeningQuote(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	first, _ := firstRune(text)
	return first == '"' || first == '\'' || first == '“' || first == '‘'
}

func containsDialogue(text string) bool {
	return strings.ContainsAny(text, "\"“”‘’")
}

func startsLower(text string) bool {
	r, ok := firstRune(strings.TrimLeftFunc(text, unicode.IsSpace))
	return ok && unicode.IsLower(r)
}

func startsUpper(text string) bool {
	r, ok := firstRune(strings.TrimLeftFunc(text, unicode.IsSpace))
	return ok && unicode.IsUpper(r)
}

func firstRune(text string) (rune, bool) {
	for _, r := range text {
		return r, true
	}
	return 0, false
}

func endsSentence(text string) bool {
	text = strings.TrimSpace(text)
	for text != "" {
		r, size := lastRune(text)
		switch r {
		case '"', '\'', '”', '’', ')', ']', '}':
			text = strings.TrimSpace(text[:len(text)-size])
			continue
		case '.', '!', '?':
			return r != '.' || cleanPeriodEndsSentence(text)
		default:
			return false
		}
	}
	return false
}

func cleanPeriodEndsSentence(text string) bool {
	runes := []rune(text)
	index := len(runes) - 1
	if cleanIsPartOfEllipsis(runes, index) || cleanIsDecimalPoint(runes, index) {
		return false
	}
	token := strings.Trim(strings.ToLower(cleanWordBefore(runes, index)), "\"'()[]{}")
	return token == "" || (!cleanIsKnownAbbreviation(token) && !cleanIsInitial(token))
}

func cleanIsPartOfEllipsis(runes []rune, index int) bool {
	return (index > 0 && runes[index-1] == '.') || (index+1 < len(runes) && runes[index+1] == '.')
}

func cleanIsDecimalPoint(runes []rune, index int) bool {
	return index > 0 && index+1 < len(runes) && unicode.IsDigit(runes[index-1]) && unicode.IsDigit(runes[index+1])
}

func cleanWordBefore(runes []rune, index int) string {
	start := index
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	return string(runes[start:index])
}

func cleanIsKnownAbbreviation(token string) bool {
	switch token {
	case "mr", "mrs", "ms", "miss", "mx", "dr", "prof", "sr", "jr", "st", "mt",
		"rev", "hon", "capt", "col", "gen", "lt", "sgt", "adm", "cmdr",
		"etc", "e.g", "i.e", "vs", "fig", "no", "vol", "ch", "pp":
		return true
	default:
		return false
	}
}

func cleanIsInitial(token string) bool {
	runes := []rune(strings.Trim(token, "."))
	return len(runes) == 1 && unicode.IsLetter(runes[0])
}

func lastRune(text string) (rune, int) {
	var last rune
	var size int
	for i, r := range text {
		last = r
		size = len(text) - i
	}
	return last, size
}

func cleanEncoding(text string) (string, int) {
	replacements := []struct {
		from string
		to   string
	}{
		{"â€œ", "“"},
		{"â€", "”"},
		{"â€™", "’"},
		{"â€˜", "‘"},
		{"â€”", "—"},
		{"â€“", "–"},
		{"â€¦", "…"},
		{"ﬃ", "ffi"},
		{"ﬄ", "ffl"},
		{"ﬂ", "fl"},
		{"ﬁ", "fi"},
		{"ﬀ", "ff"},
		{"ï¬ƒ", "ffi"},
		{"ï¬„", "ffl"},
		{"ï¬‚", "fl"},
		{"ï¬", "fi"},
		{"ï¬€", "ff"},
		{"Â", ""},
	}

	count := 0
	for _, replacement := range replacements {
		seen := strings.Count(text, replacement.from)
		if seen == 0 {
			continue
		}
		count += seen
		text = strings.ReplaceAll(text, replacement.from, replacement.to)
	}
	return text, count
}
