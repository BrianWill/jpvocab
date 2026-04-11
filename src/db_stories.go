package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	SentenceCount    int                  `json:"sentenceCount"`
	LexiconWordCount int                  `json:"lexiconWordCount"`
	NotedWords       []storyNotedWordJSON `json:"notedWords"`
	Sentences        []storySentenceJSON  `json:"sentences"`
}

const storyChunkMinChars = 200

func insertStory(db *sql.DB, title string, sentences []storySentenceInput) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	storyID, err := insertStoryTx(tx, title, sentences)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return storyID, nil
}

func insertStoryTx(tx *sql.Tx, title string, sentences []storySentenceInput) (int64, error) {
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
	chunks := buildStoryChunks(sentences)
	if len(chunks) == 0 {
		return 0, errors.New("story must have at least one sentence")
	}
	createdAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	sentenceCount := len(sentences)
	lexiconWordCount := storyLexiconWordCount(sentences)

	res, err := tx.Exec(`
		INSERT INTO stories (title, sentence_count, lexicon_word_count, created_at)
		VALUES (?, ?, ?, ?)
	`, title, sentenceCount, lexiconWordCount, createdAt)
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
				INSERT INTO story_sentences (story_id, position, words_json, jp_text, en_text, orig_lang, is_paragraph_start, is_chunk_start)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, storyID, storySentencePos, string(wordsJSON), sentence.JPText, sentence.ENText, sentence.OrigLang, paragraphStart, chunkStart); err != nil {
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
	SentenceCount    int    `json:"sentenceCount"`
	LexiconWordCount int    `json:"lexiconWordCount"`
}

func listStoriesMeta(db *sql.DB) ([]storyMetaJSON, error) {
	rows, err := db.Query(`
		SELECT id, title, created_at, sentence_count, lexicon_word_count
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
		if err := rows.Scan(&s.ID, &s.Title, &s.CreatedAt, &s.SentenceCount, &s.LexiconWordCount); err != nil {
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

	audioDir := filepath.Join("static", "audio", "story_"+strconv.FormatInt(id, 10))
	if err := os.RemoveAll(audioDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func queryStories(db *sql.DB, whereClause string, args ...any) ([]storyJSON, error) {
	rows, err := db.Query(`
		SELECT s.id, s.title, s.created_at, s.sentence_count, s.lexicon_word_count, s.noted_words_json,
		       ss.id, ss.position, ss.words_json, ss.jp_text, ss.en_text, ss.orig_lang, ss.is_paragraph_start, ss.is_chunk_start
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
		var sentenceCount int
		var lexiconWordCount int
		var notedWordsJSON sql.NullString
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var wordsJSON sql.NullString
		var jpText sql.NullString
		var enText sql.NullString
		var origLang sql.NullString
		var isParagraphStart sql.NullInt64
		var isChunkStart sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &createdAt, &sentenceCount, &lexiconWordCount, &notedWordsJSON,
			&sentenceID, &position, &wordsJSON, &jpText, &enText, &origLang, &isParagraphStart, &isChunkStart,
		); err != nil {
			return nil, err
		}

		if current == nil || storyID != currentID {
			story := storyJSON{
				ID:               storyID,
				Title:            title,
				CreatedAt:        createdAt,
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
				`SELECT id, position, words_json, jp_text, en_text, orig_lang, is_paragraph_start FROM story_sentences
				 WHERE story_id = ? AND position >= ? AND position < ? ORDER BY position`,
				storyID, startPos, nextStart.Int64,
			)
		} else {
			rows, err = db.Query(
				`SELECT id, position, words_json, jp_text, en_text, orig_lang, is_paragraph_start FROM story_sentences
				 WHERE story_id = ? AND position >= ? ORDER BY position`,
				storyID, startPos,
			)
		}
	} else {
		rows, err = db.Query(
			`SELECT id, position, words_json, jp_text, en_text, orig_lang, is_paragraph_start FROM story_sentences
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
		var origLang sql.NullString
		var isParagraphStart sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Position, &wordsJSON, &jpText, &enText, &origLang, &isParagraphStart); err != nil {
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

func storySentenceTranslationText(s storySentenceJSON) string {
	if s.OrigLang == "en" {
		if s.JPText != nil {
			return *s.JPText
		}
		return ""
	}
	if s.ENText != nil {
		return *s.ENText
	}
	return ""
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

func storySentenceHasTranslation(s storySentenceJSON) bool {
	if s.OrigLang == "en" {
		return s.JPText != nil && strings.TrimSpace(*s.JPText) != ""
	}
	return s.ENText != nil && strings.TrimSpace(*s.ENText) != ""
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
