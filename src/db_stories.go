package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type storyWordInput struct {
	DisplayWord string
	BaseWord    string
}

type storyWordJSON struct {
	DisplayWord string `json:"display"`
	BaseWord    string `json:"base,omitempty"`
}

type storySentenceInput struct {
	Words            []storyWordInput
	JPText           *string
	ENText           *string
	StartTimeMS      *int64
	OrigLang         string
	IsParagraphStart bool
}

type storySentenceJSON struct {
	ID               int64           `json:"id"`
	Position         int             `json:"position"`
	ChunkPosition    int             `json:"chunkPosition"`
	Words            []storyWordJSON `json:"words"`
	JPText           *string         `json:"jp"`
	ENText           *string         `json:"en"`
	StartTimeMS      *int64          `json:"startTimeMs,omitempty"`
	OrigLang         string          `json:"orig_lang"`
	IsParagraphStart bool            `json:"isParagraphStart"`
}

type storyChunkInput struct {
	Sentences []storySentenceInput
}

type storyNotedWordJSON struct {
	DisplayWord string `json:"displayWord"`
	BaseWord    string `json:"baseWord"`
	English     string `json:"english,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type storyJSON struct {
	ID               int64                `json:"id"`
	Title            string               `json:"title"`
	CreatedAt        string               `json:"createdAt"`
	MediaType        string               `json:"mediaType,omitempty"`
	MediaURL         string               `json:"mediaUrl,omitempty"`
	SentenceCount    int                  `json:"sentenceCount"`
	LexiconWordCount int                  `json:"lexiconWordCount"`
	NotedWords       []storyNotedWordJSON `json:"notedWords"`
	Sentences        []storySentenceJSON  `json:"sentences"`
}

type storyMediaInput struct {
	MediaType string
	MediaURL  string
}

const storyChunkMinChars = 200

func insertStory(db *sql.DB, title string, sentences []storySentenceInput, media storyMediaInput) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	storyID, err := insertStoryTx(tx, title, sentences, media)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return storyID, nil
}

func insertStoryTx(tx *sql.Tx, title string, sentences []storySentenceInput, media storyMediaInput) (int64, error) {
	if strings.TrimSpace(title) == "" {
		return 0, errors.New("story title is required")
	}
	if len(sentences) == 0 {
		return 0, errors.New("story must have at least one sentence")
	}
	for _, sentence := range sentences {
		if sentence.OrigLang != "jp" && sentence.OrigLang != "en" {
			return 0, errors.New("story sentence orig lang must be jp or en")
		}
		if strings.TrimSpace(storySentenceTextFromInput(sentence)) == "" {
			return 0, errors.New("story sentence text is required")
		}
		if sentence.OrigLang == "jp" && len(sentence.Words) == 0 {
			return 0, errors.New("japanese story sentence must have at least one word")
		}
	}
	media.MediaType = strings.TrimSpace(media.MediaType)
	media.MediaURL = strings.TrimSpace(media.MediaURL)
	switch media.MediaType {
	case "", "youtube", "local":
	default:
		return 0, errors.New("story media type must be youtube, local, or empty")
	}
	if media.MediaType == "" {
		media.MediaURL = ""
	} else if media.MediaURL == "" {
		return 0, errors.New("story media url is required")
	}

	chunks := buildStoryChunks(sentences)
	if len(chunks) == 0 {
		return 0, errors.New("story must have at least one sentence")
	}
	createdAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	sentenceCount := len(sentences)
	lexiconWordCount := storyLexiconWordCount(sentences)

	res, err := tx.Exec(`
		INSERT INTO stories (title, media_type, media_url, sentence_count, lexicon_word_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, title, media.MediaType, nullableStoryMediaURL(media.MediaURL), sentenceCount, lexiconWordCount, createdAt)
	if err != nil {
		return 0, err
	}
	storyID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertActivityEventTx(tx, activityEventStoryCreated, &storyID, title, nil); err != nil {
		return 0, err
	}

	// Insert all unique words into the lexicon once for the whole story.
	storyWords, err := insertWordsIfAbsent(tx, storySentenceBaseWords(sentences))
	if err != nil {
		return 0, err
	}
	storyWordsJSON, err := json.Marshal(storyWords)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE stories SET story_words_json = ? WHERE id = ?`, string(storyWordsJSON), storyID); err != nil {
		return 0, err
	}

	storySentencePos := 1
	for _, chunk := range chunks {
		for sentenceIdx, sentence := range chunk.Sentences {
			if sentence.OrigLang == "jp" && len(sentence.Words) == 0 {
				return 0, errors.New("japanese story sentence must have at least one word")
			}
			words := make([]storyWordJSON, len(sentence.Words))
			for j, word := range sentence.Words {
				if word.DisplayWord == "" {
					return 0, errors.New("story word display word is required")
				}
				if word.BaseWord == "" {
					return 0, errors.New("story word base word is required")
				}
				w := storyWordJSON{DisplayWord: word.DisplayWord}
				if word.BaseWord != word.DisplayWord {
					w.BaseWord = word.BaseWord
				}
				words[j] = w
			}
			wordsJSON, err := json.Marshal(words)
			if err != nil {
				return 0, err
			}
			paragraphStart := 0
			if sentence.IsParagraphStart {
				paragraphStart = 1
			}
			chunkStart := 0
			if sentenceIdx == 0 {
				chunkStart = 1
			}

			if _, err := tx.Exec(`
				INSERT INTO story_sentences (story_id, position, words_json, jp_text, en_text, start_time_ms, orig_lang, is_paragraph_start, is_chunk_start)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, storyID, storySentencePos, string(wordsJSON), sentence.JPText, sentence.ENText, sentence.StartTimeMS, sentence.OrigLang, paragraphStart, chunkStart); err != nil {
				return 0, err
			}
			storySentencePos++
		}
	}

	return storyID, nil
}

type storyMetaJSON struct {
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	CreatedAt        string `json:"createdAt"`
	MediaType        string `json:"mediaType,omitempty"`
	SentenceCount    int    `json:"sentenceCount"`
	LexiconWordCount int    `json:"lexiconWordCount"`
}

func listStoriesMeta(db *sql.DB) ([]storyMetaJSON, error) {
	rows, err := db.Query(`
		SELECT id, title, created_at, media_type, sentence_count, lexicon_word_count
		FROM stories
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stories []storyMetaJSON
	for rows.Next() {
		var s storyMetaJSON
		if err := rows.Scan(&s.ID, &s.Title, &s.CreatedAt, &s.MediaType, &s.SentenceCount, &s.LexiconWordCount); err != nil {
			return nil, err
		}
		stories = append(stories, s)
	}
	return stories, rows.Err()
}

func getStoryByID(db *sql.DB, id int64) (*storyJSON, error) {
	stories, err := queryStories(db, `WHERE s.id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(stories) == 0 {
		return nil, nil
	}
	return &stories[0], nil
}

func deleteStory(db *sql.DB, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM stories WHERE id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("story not found")
	}

	if _, err := tx.Exec(`DELETE FROM story_sentences WHERE story_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM stories WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func queryStories(db *sql.DB, whereClause string, args ...any) ([]storyJSON, error) {
	rows, err := db.Query(`
		SELECT s.id, s.title, s.created_at, s.media_type, s.media_url, s.sentence_count, s.lexicon_word_count, s.noted_words_json,
		       ss.id, ss.position, ss.words_json, ss.jp_text, ss.en_text, ss.start_time_ms, ss.orig_lang, ss.is_paragraph_start, ss.is_chunk_start
		FROM stories s
		LEFT JOIN story_sentences ss ON ss.story_id = s.id
	`+whereClause+`
		ORDER BY s.created_at DESC, s.id DESC, ss.position ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []storyJSON
	var current *storyJSON
	var currentID int64 = -1
	var currentChunkPos int

	for rows.Next() {
		var storyID int64
		var title string
		var createdAt string
		var mediaType string
		var mediaURL sql.NullString
		var sentenceCount int
		var lexiconWordCount int
		var notedWordsJSON sql.NullString
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var wordsJSON sql.NullString
		var jpText sql.NullString
		var enText sql.NullString
		var startTimeMS sql.NullInt64
		var origLang sql.NullString
		var isParagraphStart sql.NullInt64
		var isChunkStart sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &createdAt, &mediaType, &mediaURL, &sentenceCount, &lexiconWordCount, &notedWordsJSON,
			&sentenceID, &position, &wordsJSON, &jpText, &enText, &startTimeMS, &origLang, &isParagraphStart, &isChunkStart,
		); err != nil {
			return nil, err
		}

		if current == nil || storyID != currentID {
			story := storyJSON{
				ID:               storyID,
				Title:            title,
				CreatedAt:        createdAt,
				MediaType:        mediaType,
				MediaURL:         strings.TrimSpace(mediaURL.String),
				SentenceCount:    sentenceCount,
				LexiconWordCount: lexiconWordCount,
				NotedWords:       []storyNotedWordJSON{},
				Sentences:        []storySentenceJSON{},
			}
			if notedWordsJSON.Valid && notedWordsJSON.String != "" {
				json.Unmarshal([]byte(notedWordsJSON.String), &story.NotedWords) //nolint:errcheck
				if story.NotedWords == nil {
					story.NotedWords = []storyNotedWordJSON{}
				}
			}
			stories = append(stories, story)
			current = &stories[len(stories)-1]
			currentID = storyID
			currentChunkPos = 0
		}

		if sentenceID.Valid {
			if isChunkStart.Valid && isChunkStart.Int64 == 1 {
				currentChunkPos++
			}
			sentence := storySentenceJSON{
				ID:               sentenceID.Int64,
				Position:         int(position.Int64),
				ChunkPosition:    currentChunkPos,
				Words:            []storyWordJSON{},
				OrigLang:         "jp",
				IsParagraphStart: isParagraphStart.Valid && isParagraphStart.Int64 == 1,
			}
			if wordsJSON.Valid {
				if err := json.Unmarshal([]byte(wordsJSON.String), &sentence.Words); err != nil {
					return nil, err
				}
				if sentence.Words == nil {
					sentence.Words = []storyWordJSON{}
				}
				fillMissingBaseWords(sentence.Words)
			}
			if jpText.Valid {
				text := jpText.String
				sentence.JPText = &text
			}
			if enText.Valid {
				text := enText.String
				sentence.ENText = &text
			}
			if startTimeMS.Valid {
				value := startTimeMS.Int64
				sentence.StartTimeMS = &value
			}
			if origLang.Valid && origLang.String != "" {
				sentence.OrigLang = origLang.String
			}
			current.Sentences = append(current.Sentences, sentence)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stories == nil {
		stories = []storyJSON{}
	}

	return stories, nil
}

// querySentencesLite fetches sentences for a story without word enrichment.
// If chunkPosition > 0, only the sentences belonging to that chunk are returned.
// If chunkPosition == 0, all story sentences are returned.
// Words are populated from words_json (display/base only — no lexicon join).
func querySentencesLite(db *sql.DB, storyID int64, chunkPosition int) ([]storySentenceJSON, error) {
	var rows *sql.Rows
	var err error

	if chunkPosition > 0 {
		// Find the start position of the requested chunk (0-indexed offset into is_chunk_start rows).
		var startPos int
		if err := db.QueryRow(
			`SELECT position FROM story_sentences WHERE story_id = ? AND is_chunk_start = 1 ORDER BY position LIMIT 1 OFFSET ?`,
			storyID, chunkPosition-1,
		).Scan(&startPos); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil // chunk position out of range
			}
			return nil, err
		}

		// Find the start of the next chunk to bound this one.
		var nextStart sql.NullInt64
		db.QueryRow( //nolint:errcheck
			`SELECT position FROM story_sentences WHERE story_id = ? AND is_chunk_start = 1 ORDER BY position LIMIT 1 OFFSET ?`,
			storyID, chunkPosition,
		).Scan(&nextStart)

		if nextStart.Valid {
			rows, err = db.Query(
				`SELECT id, position, words_json, jp_text, en_text, start_time_ms, orig_lang, is_paragraph_start FROM story_sentences
				 WHERE story_id = ? AND position >= ? AND position < ? ORDER BY position`,
				storyID, startPos, nextStart.Int64,
			)
		} else {
			rows, err = db.Query(
				`SELECT id, position, words_json, jp_text, en_text, start_time_ms, orig_lang, is_paragraph_start FROM story_sentences
				 WHERE story_id = ? AND position >= ? ORDER BY position`,
				storyID, startPos,
			)
		}
	} else {
		rows, err = db.Query(
			`SELECT id, position, words_json, jp_text, en_text, start_time_ms, orig_lang, is_paragraph_start FROM story_sentences
			 WHERE story_id = ? ORDER BY position`,
			storyID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sentences []storySentenceJSON
	for rows.Next() {
		var s storySentenceJSON
		var wordsJSON sql.NullString
		var jpText sql.NullString
		var enText sql.NullString
		var startTimeMS sql.NullInt64
		var origLang sql.NullString
		var isParagraphStart sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Position, &wordsJSON, &jpText, &enText, &startTimeMS, &origLang, &isParagraphStart); err != nil {
			return nil, err
		}
		if wordsJSON.Valid {
			json.Unmarshal([]byte(wordsJSON.String), &s.Words) //nolint:errcheck
			fillMissingBaseWords(s.Words)
		}
		if s.Words == nil {
			s.Words = []storyWordJSON{}
		}
		if jpText.Valid {
			t := jpText.String
			s.JPText = &t
		}
		if enText.Valid {
			t := enText.String
			s.ENText = &t
		}
		if startTimeMS.Valid {
			value := startTimeMS.Int64
			s.StartTimeMS = &value
		}
		if origLang.Valid && origLang.String != "" {
			s.OrigLang = origLang.String
		} else {
			s.OrigLang = "jp"
		}
		s.IsParagraphStart = isParagraphStart.Valid && isParagraphStart.Int64 == 1
		sentences = append(sentences, s)
	}
	return sentences, rows.Err()
}

func setSentenceTranslationText(db *sql.DB, sentenceID int64, targetLang, text string) error {
	switch targetLang {
	case "jp":
		_, err := db.Exec(`UPDATE story_sentences SET jp_text = ? WHERE id = ?`, text, sentenceID)
		return err
	case "en":
		_, err := db.Exec(`UPDATE story_sentences SET en_text = ? WHERE id = ?`, text, sentenceID)
		return err
	default:
		return errors.New("target language must be jp or en")
	}
}

func setStoryNotedWords(db *sql.DB, storyID int64, words []storyNotedWordJSON) error {
	b, err := json.Marshal(words)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE stories SET noted_words_json = ? WHERE id = ?`, string(b), storyID)
	return err
}

func addStoryNotedWord(db *sql.DB, storyID int64, word storyNotedWordJSON) error {
	story, err := getStoryByID(db, storyID)
	if err != nil {
		return err
	}
	if story == nil {
		return errors.New("story not found")
	}

	word.BaseWord = strings.TrimSpace(word.BaseWord)
	word.DisplayWord = strings.TrimSpace(word.DisplayWord)
	if word.BaseWord == "" {
		return errors.New("base word is required")
	}
	if word.DisplayWord == "" {
		return errors.New("display word is required")
	}

	for _, existing := range story.NotedWords {
		if existing.BaseWord == word.BaseWord {
			return nil
		}
	}
	if word.CreatedAt == "" {
		var createdAt string
		if err := db.QueryRow(`SELECT datetime('now')`).Scan(&createdAt); err != nil {
			return err
		}
		word.CreatedAt = createdAt
	}
	if word.English == "" {
		var meaning string
		db.QueryRow(`SELECT COALESCE(meaning,'') FROM words WHERE base_word = ?`, word.BaseWord).Scan(&meaning) //nolint:errcheck
		word.English = meaning
	}

	story.NotedWords = append(story.NotedWords, word)
	return setStoryNotedWords(db, storyID, story.NotedWords)
}

func removeStoryNotedWord(db *sql.DB, storyID int64, baseWord string) error {
	story, err := getStoryByID(db, storyID)
	if err != nil {
		return err
	}
	if story == nil {
		return errors.New("story not found")
	}

	baseWord = strings.TrimSpace(baseWord)
	if baseWord == "" {
		return errors.New("base word is required")
	}

	filtered := make([]storyNotedWordJSON, 0, len(story.NotedWords))
	for _, word := range story.NotedWords {
		if word.BaseWord != baseWord {
			filtered = append(filtered, word)
		}
	}
	return setStoryNotedWords(db, storyID, filtered)
}

func storySentenceTextFromInput(sentence storySentenceInput) string {
	if sentence.OrigLang == "en" {
		if sentence.ENText != nil {
			return *sentence.ENText
		}
		return ""
	}
	if sentence.JPText != nil {
		return *sentence.JPText
	}
	var b strings.Builder
	for _, word := range sentence.Words {
		b.WriteString(word.DisplayWord)
	}
	return b.String()
}

func storySentenceDisplayText(s storySentenceJSON) string {
	if s.OrigLang == "en" {
		if s.ENText != nil {
			return *s.ENText
		}
	} else if s.JPText != nil {
		return *s.JPText
	}
	var parts []string
	for _, w := range s.Words {
		parts = append(parts, w.DisplayWord)
	}
	return strings.Join(parts, "")
}

func storySentenceSourceText(s storySentenceJSON) string {
	return strings.TrimSpace(storySentenceDisplayText(s))
}

func storySentenceTargetLang(origLang string) string {
	if origLang == "en" {
		return "jp"
	}
	return "en"
}

func classifySentenceLanguage(text string) string {
	hasJP := false
	hasLatin := false
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF) || (r >= 0x4E00 && r <= 0x9FFF):
			hasJP = true
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			hasLatin = true
		}
	}
	if hasJP {
		return "jp"
	}
	if hasLatin {
		return "en"
	}
	return "jp"
}

func buildStorySentenceInput(text string, isParagraphStart bool) storySentenceInput {
	lang := classifySentenceLanguage(text)
	sentence := storySentenceInput{
		OrigLang:         lang,
		IsParagraphStart: isParagraphStart,
	}
	if lang == "en" {
		sentence.ENText = &text
		sentence.Words = []storyWordInput{}
		return sentence
	}
	sentence.JPText = &text
	sentence.Words = buildStorySentenceWords(text)
	return sentence
}

func buildTimedStorySentenceInput(text string, isParagraphStart bool, startTimeMS int64) storySentenceInput {
	sentence := buildStorySentenceInput(text, isParagraphStart)
	sentence.StartTimeMS = &startTimeMS
	return sentence
}

// fillMissingBaseWords sets BaseWord = DisplayWord for any word where BaseWord
// was omitted during storage (i.e. they were equal and only "display" was stored).
func fillMissingBaseWords(words []storyWordJSON) {
	for i := range words {
		if words[i].BaseWord == "" {
			words[i].BaseWord = words[i].DisplayWord
		}
	}
}

func storyLexiconWordCount(sentences []storySentenceInput) int {
	texts := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		if sentence.OrigLang != "jp" {
			continue
		}
		texts = append(texts, storySentenceTextFromInput(sentence))
	}
	return storyLexiconWordCountFromTexts(texts)
}

func storyLexiconWordCountFromTexts(texts []string) int {
	seen := map[string]struct{}{}
	for _, text := range texts {
		for _, word := range extractContentWords(text) {
			if word == "" {
				continue
			}
			seen[word] = struct{}{}
		}
	}
	return len(seen)
}

func storySentenceBaseWords(sentences []storySentenceInput) []string {
	seen := map[string]struct{}{}
	baseWords := make([]string, 0)
	for _, sentence := range sentences {
		if sentence.OrigLang != "jp" {
			continue
		}
		for _, word := range sentence.Words {
			base := strings.TrimSpace(word.BaseWord)
			if base == "" {
				continue
			}
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			baseWords = append(baseWords, base)
		}
	}
	return baseWords
}

func buildStoryChunks(sentences []storySentenceInput) []storyChunkInput {
	chunks := make([]storyChunkInput, 0)
	current := storyChunkInput{Sentences: []storySentenceInput{}}
	currentChars := 0

	for _, sentence := range sentences {
		if len(current.Sentences) > 0 && currentChars >= storyChunkMinChars {
			chunks = append(chunks, current)
			current = storyChunkInput{Sentences: []storySentenceInput{}}
			currentChars = 0
		}
		current.Sentences = append(current.Sentences, sentence)
		currentChars += utf8.RuneCountInString(storySentenceTextFromInput(sentence))
	}
	if len(current.Sentences) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func buildStorySentenceWords(sentence string) []storyWordInput {
	tokens := jpTokenizer.Tokenize(sentence)
	words := make([]storyWordInput, 0, len(tokens))
	for _, tok := range tokens {
		surface := tok.Surface
		if strings.TrimSpace(surface) == "" {
			continue
		}

		base := surface
		f := tok.Features()
		if len(f) >= 7 {
			if candidate := f[6]; candidate != "" && candidate != "*" {
				base = candidate
			}
		}

		words = append(words, storyWordInput{
			DisplayWord: surface,
			BaseWord:    base,
		})
	}
	return words
}

var storySentenceSplitRE = regexp.MustCompile(`[^。！？!?.]+[。！？!?.]?`)

func buildStorySentencesFromText(content string) []storySentenceInput {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	paragraphs := strings.Split(normalized, "\n\n")
	sentences := make([]storySentenceInput, 0)

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		matches := storySentenceSplitRE.FindAllString(paragraph, -1)
		paragraphHasSentence := false
		for _, match := range matches {
			text := strings.TrimSpace(match)
			if text == "" {
				continue
			}
			sentence := buildStorySentenceInput(text, !paragraphHasSentence)
			if strings.TrimSpace(storySentenceTextFromInput(sentence)) == "" {
				continue
			}
			sentences = append(sentences, sentence)
			paragraphHasSentence = true
		}
	}

	return sentences
}

var subtitleTimeRE = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{3})\s*-->\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{3})`)

func parseStoryContent(content string) []storySentenceInput {
	if sentences := buildStorySentencesFromSubtitle(content); len(sentences) > 0 {
		return sentences
	}
	return buildStorySentencesFromText(content)
}

func buildStorySentencesFromSubtitle(content string) []storySentenceInput {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	idx := 0
	if len(lines) > 0 && strings.EqualFold(strings.TrimSpace(lines[0]), "WEBVTT") {
		idx = 1
	}

	sentences := make([]storySentenceInput, 0)
	firstSentence := true
	for idx < len(lines) {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			idx++
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "NOTE") {
			for idx < len(lines) && strings.TrimSpace(lines[idx]) != "" {
				idx++
			}
			continue
		}
		if !subtitleTimeRE.MatchString(line) {
			if idx+1 >= len(lines) || !subtitleTimeRE.MatchString(strings.TrimSpace(lines[idx+1])) {
				return nil
			}
			idx++
			line = strings.TrimSpace(lines[idx])
		}
		startTimeMS, ok := parseSubtitleStartTimeMS(line)
		if !ok {
			return nil
		}
		idx++

		cueLines := make([]string, 0)
		for idx < len(lines) {
			textLine := strings.TrimSpace(lines[idx])
			if textLine == "" {
				break
			}
			cueLines = append(cueLines, stripSubtitleTags(textLine))
			idx++
		}
		if len(cueLines) == 0 {
			continue
		}
		for _, cueLine := range cueLines {
			text := strings.TrimSpace(cueLine)
			if text == "" {
				continue
			}
			sentences = append(sentences, buildTimedStorySentenceInput(text, firstSentence, startTimeMS))
			firstSentence = false
		}
	}
	return sentences
}

func parseSubtitleStartTimeMS(line string) (int64, bool) {
	match := subtitleTimeRE.FindStringSubmatch(line)
	if len(match) != 9 {
		return 0, false
	}
	hours, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.ParseInt(match[2], 10, 64)
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseInt(match[3], 10, 64)
	if err != nil {
		return 0, false
	}
	millis, err := strconv.ParseInt(match[4], 10, 64)
	if err != nil {
		return 0, false
	}
	return (((hours*60)+minutes)*60+seconds)*1000 + millis, true
}

var subtitleTagRE = regexp.MustCompile(`<[^>]+>`)

func stripSubtitleTags(text string) string {
	text = subtitleTagRE.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return strings.TrimSpace(text)
}

func normalizeStoryMedia(mediaType, mediaURL string) (storyMediaInput, error) {
	media := storyMediaInput{
		MediaType: strings.TrimSpace(mediaType),
		MediaURL:  strings.TrimSpace(mediaURL),
	}
	if media.MediaURL == "" {
		return storyMediaInput{}, nil
	}
	if media.MediaType == "" {
		inferredType, err := inferStoryMediaType(media.MediaURL)
		if err != nil {
			return storyMediaInput{}, err
		}
		media.MediaType = inferredType
	}
	switch media.MediaType {
	case "youtube":
		normalized, err := normalizeYouTubeEmbedURL(media.MediaURL)
		if err != nil {
			return storyMediaInput{}, err
		}
		media.MediaURL = normalized
	case "local":
	default:
		return storyMediaInput{}, errors.New("invalid story media type")
	}
	return media, nil
}

func inferStoryMediaType(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "http://") || strings.HasPrefix(strings.ToLower(trimmed), "https://") {
		if _, err := normalizeYouTubeEmbedURL(trimmed); err != nil {
			return "", err
		}
		return "youtube", nil
	}

	return "local", nil
}

func storyMediaPathExt(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "?#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	lastSlash := strings.LastIndexAny(trimmed, `/\`)
	lastDot := strings.LastIndex(trimmed, ".")
	if lastDot <= lastSlash {
		return ""
	}
	return trimmed[lastDot:]
}

func normalizeYouTubeEmbedURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("invalid youtube url")
	}
	if parsed.Host == "" {
		return "", errors.New("invalid youtube url")
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
	var videoID string
	switch host {
	case "youtube.com", "m.youtube.com":
		switch {
		case strings.HasPrefix(parsed.Path, "/watch"):
			videoID = parsed.Query().Get("v")
		case strings.HasPrefix(parsed.Path, "/embed/"):
			videoID = strings.TrimPrefix(parsed.Path, "/embed/")
		case strings.HasPrefix(parsed.Path, "/shorts/"):
			videoID = strings.TrimPrefix(parsed.Path, "/shorts/")
		}
	case "youtu.be":
		videoID = strings.TrimPrefix(parsed.Path, "/")
	default:
		return "", errors.New("invalid youtube url")
	}
	videoID = strings.TrimSpace(strings.Split(videoID, "/")[0])
	if videoID == "" {
		return "", errors.New("invalid youtube url")
	}
	return "https://www.youtube.com/embed/" + videoID + "?enablejsapi=1", nil
}

func nullableStoryMediaURL(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
