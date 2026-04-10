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
	DisplayWord      string
	BaseWord         string
	AudioTimestampMs *int64
}

type storyWordJSON struct {
	DisplayWord      string           `json:"displayWord"`
	BaseWord         string           `json:"baseWord"`
	English          string           `json:"english,omitempty"`     // populated from words.meaning at query time; not stored per-token
	Reading          string           `json:"reading,omitempty"`     // populated from words.reading at query time; not stored per-token
	KanjiData        []kanjiDataEntry `json:"kanjiData,omitempty"`   // populated from words.kanji_data at query time; not stored per-token
	PitchAccent      *int             `json:"pitchAccent,omitempty"` // populated from words.pitch_accent at query time; not stored per-token
	Tracked          bool             `json:"tracked,omitempty"`     // true if the base word is currently tracked (explicitly added by user)
	WordID           int64            `json:"wordId,omitempty"`      // DB id of the lexicon entry (only set when Tracked)
	DrillCount       int              `json:"drillCount,omitempty"`  // correct drill answers so far
	DrillTarget      int              `json:"drillTarget,omitempty"` // target correct answers to retire the word
	AudioTimestampMs *int64           `json:"audioTimestampMs"`
}

type storySentenceInput struct {
	Words            []storyWordInput
	EnglishText      *string
	IsParagraphStart bool
}

type storySentenceJSON struct {
	ID               int64           `json:"id"`
	ChunkID          int64           `json:"chunkId,omitempty"`
	ChunkPosition    int             `json:"chunkPosition,omitempty"`
	Position         int             `json:"position"`
	Words            []storyWordJSON `json:"words"`
	EnglishText      *string         `json:"englishText"`
	IsParagraphStart bool            `json:"isParagraphStart"`
}

type storyChunkInput struct {
	Sentences []storySentenceInput
}

type storyChunkJSON struct {
	ID         int64               `json:"id"`
	Position   int                 `json:"position"`
	StoryWords []string            `json:"storyWords"`
	Sentences  []storySentenceJSON `json:"sentences"`
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
	StoryWords       []string             `json:"storyWords"`
	NotedWords       []storyNotedWordJSON `json:"notedWords"`
	Chunks           []storyChunkJSON     `json:"chunks"`
	Sentences        []storySentenceJSON  `json:"sentences"`
}

const storyChunkMinChars = 500

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
		if len(sentence.Words) == 0 {
			return 0, errors.New("story sentence must have at least one word")
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

	storyWordSeen := map[string]struct{}{}
	storyWords := make([]string, 0)
	storySentencePos := 1
	for chunkIdx, chunk := range chunks {
		res, err := tx.Exec(`
			INSERT INTO story_chunks (story_id, position)
			VALUES (?, ?)
		`, storyID, chunkIdx+1)
		if err != nil {
			return 0, err
		}
		chunkID, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}

		chunkWords, err := insertWordsIfAbsent(tx, storySentenceBaseWords(chunk.Sentences))
		if err != nil {
			return 0, err
		}
		chunkWordsJSON, err := json.Marshal(chunkWords)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE story_chunks SET story_words_json = ? WHERE id = ?`, string(chunkWordsJSON), chunkID); err != nil {
			return 0, err
		}
		for _, word := range chunkWords {
			if _, ok := storyWordSeen[word]; ok {
				continue
			}
			storyWordSeen[word] = struct{}{}
			storyWords = append(storyWords, word)
		}

		for sentenceIdx, sentence := range chunk.Sentences {
			if len(sentence.Words) == 0 {
				return 0, errors.New("story sentence must have at least one word")
			}
			words := make([]storyWordJSON, len(sentence.Words))
			for j, word := range sentence.Words {
				if word.DisplayWord == "" {
					return 0, errors.New("story word display word is required")
				}
				if word.BaseWord == "" {
					return 0, errors.New("story word base word is required")
				}
				if word.AudioTimestampMs != nil && *word.AudioTimestampMs < 0 {
					return 0, errors.New("story word audio timestamp must be non-negative")
				}
				words[j] = storyWordJSON{
					DisplayWord:      word.DisplayWord,
					BaseWord:         word.BaseWord,
					AudioTimestampMs: word.AudioTimestampMs,
				}
			}
			wordsJSON, err := json.Marshal(words)
			if err != nil {
				return 0, err
			}
			paragraphStart := 0
			if sentence.IsParagraphStart {
				paragraphStart = 1
			}

			if _, err := tx.Exec(`
				INSERT INTO story_sentences (story_id, chunk_id, position, chunk_position, words_json, english_text, is_paragraph_start)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, storyID, chunkID, storySentencePos, sentenceIdx+1, string(wordsJSON), sentence.EnglishText, paragraphStart); err != nil {
				return 0, err
			}
			storySentencePos++
		}
	}

	storyWordsJSON, err := json.Marshal(storyWords)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE stories SET story_words_json = ? WHERE id = ?`, string(storyWordsJSON), storyID); err != nil {
		return 0, err
	}

	return storyID, nil
}

func listStories(db *sql.DB) ([]storyJSON, error) {
	return queryStories(db, "")
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
		SELECT s.id, s.title, s.created_at, s.sentence_count, s.lexicon_word_count, s.noted_words_json, s.story_words_json,
		       sc.id, sc.position, sc.story_words_json,
		       ss.id, ss.position, ss.chunk_position, ss.words_json, ss.english_text, ss.is_paragraph_start
		FROM stories s
		LEFT JOIN story_chunks sc ON sc.story_id = s.id
		LEFT JOIN story_sentences ss ON ss.chunk_id = sc.id
	`+whereClause+`
		ORDER BY s.created_at DESC, s.id DESC, sc.position ASC, ss.position ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []storyJSON
	var current *storyJSON
	var currentID int64 = -1
	var currentChunk *storyChunkJSON
	var currentChunkID int64 = -1

	for rows.Next() {
		var storyID int64
		var title string
		var createdAt string
		var sentenceCount int
		var lexiconWordCount int
		var notedWordsJSON sql.NullString
		var storyWordsJSON sql.NullString
		var chunkID sql.NullInt64
		var chunkPosition sql.NullInt64
		var chunkStoryWordsJSON sql.NullString
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var chunkSentencePosition sql.NullInt64
		var wordsJSON sql.NullString
		var englishText sql.NullString
		var isParagraphStart sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &createdAt, &sentenceCount, &lexiconWordCount, &notedWordsJSON, &storyWordsJSON,
			&chunkID, &chunkPosition, &chunkStoryWordsJSON,
			&sentenceID, &position, &chunkSentencePosition, &wordsJSON, &englishText, &isParagraphStart,
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
				StoryWords:       []string{},
				NotedWords:       []storyNotedWordJSON{},
				Chunks:           []storyChunkJSON{},
				Sentences:        []storySentenceJSON{},
			}
			if notedWordsJSON.Valid && notedWordsJSON.String != "" {
				json.Unmarshal([]byte(notedWordsJSON.String), &story.NotedWords) //nolint:errcheck
				if story.NotedWords == nil {
					story.NotedWords = []storyNotedWordJSON{}
				}
			}
			if storyWordsJSON.Valid && storyWordsJSON.String != "" {
				json.Unmarshal([]byte(storyWordsJSON.String), &story.StoryWords) //nolint:errcheck
				if story.StoryWords == nil {
					story.StoryWords = []string{}
				}
			}
			stories = append(stories, story)
			current = &stories[len(stories)-1]
			currentID = storyID
			currentChunk = nil
			currentChunkID = -1
		}

		if chunkID.Valid && (currentChunk == nil || chunkID.Int64 != currentChunkID) {
			chunk := storyChunkJSON{
				ID:         chunkID.Int64,
				Position:   int(chunkPosition.Int64),
				StoryWords: []string{},
				Sentences:  []storySentenceJSON{},
			}
			if chunkStoryWordsJSON.Valid && chunkStoryWordsJSON.String != "" {
				json.Unmarshal([]byte(chunkStoryWordsJSON.String), &chunk.StoryWords) //nolint:errcheck
				if chunk.StoryWords == nil {
					chunk.StoryWords = []string{}
				}
			}
			current.Chunks = append(current.Chunks, chunk)
			currentChunk = &current.Chunks[len(current.Chunks)-1]
			currentChunkID = chunkID.Int64
		}

		if sentenceID.Valid {
			sentence := storySentenceJSON{
				ID:               sentenceID.Int64,
				ChunkID:          chunkID.Int64,
				ChunkPosition:    int(chunkSentencePosition.Int64),
				Position:         int(position.Int64),
				Words:            []storyWordJSON{},
				IsParagraphStart: isParagraphStart.Valid && isParagraphStart.Int64 == 1,
			}
			if wordsJSON.Valid {
				if err := json.Unmarshal([]byte(wordsJSON.String), &sentence.Words); err != nil {
					return nil, err
				}
				if sentence.Words == nil {
					sentence.Words = []storyWordJSON{}
				}
			}
			if englishText.Valid {
				text := englishText.String
				sentence.EnglishText = &text
			}
			if currentChunk != nil {
				currentChunk.Sentences = append(currentChunk.Sentences, sentence)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stories == nil {
		stories = []storyJSON{}
	}

	// Build the word lookup set from the pre-computed StoryWords lists plus
	// any noted words (which may reference words not in the sentence token list).
	baseWordSet := map[string]struct{}{}
	for i := range stories {
		for _, bw := range stories[i].StoryWords {
			if bw != "" {
				baseWordSet[bw] = struct{}{}
			}
		}
		for _, chunk := range stories[i].Chunks {
			for _, bw := range chunk.StoryWords {
				if bw != "" {
					baseWordSet[bw] = struct{}{}
				}
			}
		}
		for _, nw := range stories[i].NotedWords {
			if nw.BaseWord != "" {
				baseWordSet[nw.BaseWord] = struct{}{}
			}
		}
	}

	// Populate word info (meaning, reading, drill counts, tracked) from the words table.
	if len(baseWordSet) > 0 {
		placeholders := make([]string, 0, len(baseWordSet))
		lexArgs := make([]any, 0, len(baseWordSet))
		for bw := range baseWordSet {
			placeholders = append(placeholders, "?")
			lexArgs = append(lexArgs, bw)
		}
		type wordInfo struct {
			id          int64
			drillCount  int
			drillTarget int
			meaning     string
			reading     string
			pitchAccent *int
			kanjiData   []kanjiDataEntry
			tracked     int
		}
		lexRows, err := db.Query(
			`SELECT id, base_word, drill_count, drill_target, COALESCE(meaning,''), COALESCE(reading,''), pitch_accent, COALESCE(kanji_data,'[]'), tracked
			 FROM words WHERE base_word IN (`+strings.Join(placeholders, ",")+`)`,
			lexArgs...,
		)
		if err != nil {
			return nil, err
		}
		wordInfoMap := map[string]wordInfo{}
		for lexRows.Next() {
			var w string
			var info wordInfo
			var kanjiDataStr string
			if err := lexRows.Scan(&info.id, &w, &info.drillCount, &info.drillTarget, &info.meaning, &info.reading, &info.pitchAccent, &kanjiDataStr, &info.tracked); err != nil {
				lexRows.Close()
				return nil, err
			}
			json.Unmarshal([]byte(kanjiDataStr), &info.kanjiData) //nolint:errcheck
			if info.kanjiData == nil {
				info.kanjiData = []kanjiDataEntry{}
			}
			wordInfoMap[w] = info
		}
		lexRows.Close()
		if err := lexRows.Err(); err != nil {
			return nil, err
		}

		// todo: optimize with better query?
		for i := range stories {
			for c := range stories[i].Chunks {
				for j := range stories[i].Chunks[c].Sentences {
					for k := range stories[i].Chunks[c].Sentences[j].Words {
						bw := stories[i].Chunks[c].Sentences[j].Words[k].BaseWord
						if info, ok := wordInfoMap[bw]; ok {
							stories[i].Chunks[c].Sentences[j].Words[k].Tracked = info.tracked == 1
							stories[i].Chunks[c].Sentences[j].Words[k].WordID = info.id
							stories[i].Chunks[c].Sentences[j].Words[k].DrillCount = info.drillCount
							stories[i].Chunks[c].Sentences[j].Words[k].DrillTarget = info.drillTarget
							stories[i].Chunks[c].Sentences[j].Words[k].English = info.meaning
							stories[i].Chunks[c].Sentences[j].Words[k].Reading = info.reading
							stories[i].Chunks[c].Sentences[j].Words[k].KanjiData = info.kanjiData
							stories[i].Chunks[c].Sentences[j].Words[k].PitchAccent = info.pitchAccent
						}
					}
				}
			}
			for n := range stories[i].NotedWords {
				if stories[i].NotedWords[n].English == "" {
					if info, ok := wordInfoMap[stories[i].NotedWords[n].BaseWord]; ok {
						stories[i].NotedWords[n].English = info.meaning
					}
				}
			}
		}
	}

	for i := range stories {
		stories[i].Sentences = stories[i].Sentences[:0]
		if len(stories[i].StoryWords) == 0 {
			seen := map[string]struct{}{}
			for _, chunk := range stories[i].Chunks {
				for _, bw := range chunk.StoryWords {
					if _, ok := seen[bw]; ok || bw == "" {
						continue
					}
					seen[bw] = struct{}{}
					stories[i].StoryWords = append(stories[i].StoryWords, bw)
				}
			}
		}
		for _, chunk := range stories[i].Chunks {
			stories[i].Sentences = append(stories[i].Sentences, chunk.Sentences...)
		}
	}

	return stories, nil
}

func setSentenceEnglishText(db *sql.DB, sentenceID int64, text string) error {
	_, err := db.Exec(`UPDATE story_sentences SET english_text = ? WHERE id = ?`, text, sentenceID)
	return err
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
		for _, sentence := range story.Sentences {
			for _, token := range sentence.Words {
				if token.BaseWord == word.BaseWord {
					word.English = token.English
					if word.DisplayWord == "" {
						word.DisplayWord = token.DisplayWord
					}
					break
				}
			}
			if word.English != "" {
				break
			}
		}
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
	var b strings.Builder
	for _, word := range sentence.Words {
		b.WriteString(word.DisplayWord)
	}
	return b.String()
}

func storyLexiconWordCount(sentences []storySentenceInput) int {
	texts := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		texts = append(texts, storySentenceTextFromInput(sentence))
	}
	return storyLexiconWordCountFromTexts(texts)
}

func storyLexiconWordCountJSON(sentences []storySentenceJSON) int {
	texts := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		texts = append(texts, storySentenceText(sentence))
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
		if len(sentence.Words) == 0 {
			continue
		}
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

var storySentenceSplitRE = regexp.MustCompile(`[^。！？!?]+[。！？!?]?`)

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
			words := buildStorySentenceWords(text)
			if len(words) == 0 {
				continue
			}
			sentences = append(sentences, storySentenceInput{
				Words:            words,
				IsParagraphStart: !paragraphHasSentence,
			})
			paragraphHasSentence = true
		}
	}

	return sentences
}
