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
)

type storyWordInput struct {
	DisplayWord      string
	BaseWord         string
	AudioTimestampMs *int64
}

type storyWordJSON struct {
	DisplayWord      string `json:"displayWord"`
	BaseWord         string `json:"baseWord"`
	English          string `json:"english,omitempty"`    // populated from words.meaning at query time; not stored per-token
	Reading          string `json:"reading,omitempty"`    // populated from words.reading at query time; not stored per-token
	InLexicon        bool   `json:"inLexicon,omitempty"`   // true if the base word is currently in the lexicon
	WordID           int64  `json:"wordId,omitempty"`      // DB id of the lexicon entry (only set when InLexicon)
	DrillCount       int    `json:"drillCount,omitempty"`  // correct drill answers so far
	DrillTarget      int    `json:"drillTarget,omitempty"` // target correct answers to retire the word
	AudioTimestampMs *int64 `json:"audioTimestampMs"`
}

type storySentenceInput struct {
	Words            []storyWordInput
	EnglishText      *string
	IsParagraphStart bool
}

type storySentenceJSON struct {
	ID               int64           `json:"id"`
	Position         int             `json:"position"`
	Words            []storyWordJSON `json:"words"`
	EnglishText      *string         `json:"englishText"`
	IsParagraphStart bool            `json:"isParagraphStart"`
	AudioDurationMs  *int64          `json:"audioDurationMs"`
}

type storyNotedWordJSON struct {
	DisplayWord string `json:"displayWord"`
	BaseWord    string `json:"baseWord"`
	English     string `json:"english,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type storyJSON struct {
	ID         int64                `json:"id"`
	Title      string               `json:"title"`
	AudioPath  *string              `json:"audioPath"`
	HasAudio   bool                 `json:"hasAudio"`
	CreatedAt  string               `json:"createdAt"`
	NotedWords []storyNotedWordJSON `json:"notedWords"`
	Sentences  []storySentenceJSON  `json:"sentences"`
}

func insertStory(db *sql.DB, title string, audioPath *string, sentences []storySentenceInput) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	storyID, err := insertStoryTx(tx, title, audioPath, sentences)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return storyID, nil
}

func insertStoryTx(tx *sql.Tx, title string, audioPath *string, sentences []storySentenceInput) (int64, error) {
	if strings.TrimSpace(title) == "" {
		return 0, errors.New("story title is required")
	}
	if len(sentences) == 0 {
		return 0, errors.New("story must have at least one sentence")
	}

	// Collect unique base words for story_words_json.
	baseWordSet := map[string]struct{}{}
	for _, sentence := range sentences {
		for _, word := range sentence.Words {
			if word.BaseWord != "" {
				baseWordSet[word.BaseWord] = struct{}{}
			}
		}
	}
	createdAt := time.Now().UTC().Format("2006-01-02 15:04:05")

	res, err := tx.Exec(`
		INSERT INTO stories (title, audio_path, created_at)
		VALUES (?, ?, ?)
	`, title, audioPath, createdAt)
	if err != nil {
		return 0, err
	}
	storyID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for i, sentence := range sentences {
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
			INSERT INTO story_sentences (story_id, position, words_json, english_text, is_paragraph_start)
			VALUES (?, ?, ?, ?, ?)
		`, storyID, i+1, string(wordsJSON), sentence.EnglishText, paragraphStart); err != nil {
			return 0, err
		}
	}

	// Auto-add all story base words to the lexicon as in_lexicon=0 if not already present.
	baseWords := make([]string, 0, len(baseWordSet))
	for bw := range baseWordSet {
		baseWords = append(baseWords, bw)
	}
	storyWords, err := insertWordsIfAbsent(tx, baseWords)
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

	return storyID, nil
}

func listStories(db *sql.DB) ([]storyJSON, error) {
	rows, err := db.Query(`
		SELECT s.id, s.title, s.audio_path, s.has_audio, s.created_at,
		       ss.id, ss.position, ss.words_json, ss.english_text, ss.is_paragraph_start, ss.audio_duration_ms
		FROM stories s
		LEFT JOIN story_sentences ss ON ss.story_id = s.id
		ORDER BY s.created_at DESC, s.id DESC, ss.position ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []storyJSON
	var current *storyJSON
	var currentID int64 = -1

	for rows.Next() {
		var storyID int64
		var title string
		var audioPath sql.NullString
		var hasAudio int
		var createdAt string
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var wordsJSON sql.NullString
		var englishText sql.NullString
		var isParagraphStart sql.NullInt64
		var audioDurationMs sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &audioPath, &hasAudio, &createdAt,
			&sentenceID, &position, &wordsJSON, &englishText, &isParagraphStart, &audioDurationMs,
		); err != nil {
			return nil, err
		}

		if current == nil || storyID != currentID {
			story := storyJSON{
				ID:        storyID,
				Title:     title,
				HasAudio:  hasAudio == 1,
				CreatedAt: createdAt,
				Sentences: []storySentenceJSON{},
			}
			if audioPath.Valid {
				path := audioPath.String
				story.AudioPath = &path
			}
			stories = append(stories, story)
			current = &stories[len(stories)-1]
			currentID = storyID
		}

		if sentenceID.Valid {
			sentence := storySentenceJSON{
				ID:               sentenceID.Int64,
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
			if audioDurationMs.Valid {
				d := audioDurationMs.Int64
				sentence.AudioDurationMs = &d
			}
			current.Sentences = append(current.Sentences, sentence)
		}
	}
	if stories == nil {
		stories = []storyJSON{}
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
		SELECT s.id, s.title, s.audio_path, s.has_audio, s.created_at, s.noted_words_json, s.story_words_json,
		       ss.id, ss.position, ss.words_json, ss.english_text, ss.is_paragraph_start, ss.audio_duration_ms
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
	// storyWordsByID holds the pre-computed unique word list per story for the
	// batch word-info lookup after all rows are scanned.
	storyWordsByID := map[int64][]string{}

	for rows.Next() {
		var storyID int64
		var title string
		var audioPath sql.NullString
		var hasAudio int
		var createdAt string
		var notedWordsJSON sql.NullString
		var storyWordsJSON sql.NullString
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var wordsJSON sql.NullString
		var englishText sql.NullString
		var isParagraphStart sql.NullInt64
		var audioDurationMs sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &audioPath, &hasAudio, &createdAt, &notedWordsJSON, &storyWordsJSON,
			&sentenceID, &position, &wordsJSON, &englishText, &isParagraphStart, &audioDurationMs,
		); err != nil {
			return nil, err
		}

		if current == nil || storyID != currentID {
			story := storyJSON{
				ID:         storyID,
				Title:      title,
				HasAudio:   hasAudio == 1,
				CreatedAt:  createdAt,
				NotedWords: []storyNotedWordJSON{},
				Sentences:  []storySentenceJSON{},
			}
			if audioPath.Valid {
				path := audioPath.String
				story.AudioPath = &path
			}
			if notedWordsJSON.Valid && notedWordsJSON.String != "" {
				json.Unmarshal([]byte(notedWordsJSON.String), &story.NotedWords) //nolint:errcheck
				if story.NotedWords == nil {
					story.NotedWords = []storyNotedWordJSON{}
				}
			}
			if storyWordsJSON.Valid && storyWordsJSON.String != "" {
				var words []string
				if err := json.Unmarshal([]byte(storyWordsJSON.String), &words); err == nil {
					storyWordsByID[storyID] = words
				}
			}
			stories = append(stories, story)
			current = &stories[len(stories)-1]
			currentID = storyID
		}

		if sentenceID.Valid {
			sentence := storySentenceJSON{
				ID:               sentenceID.Int64,
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
			if audioDurationMs.Valid {
				d := audioDurationMs.Int64
				sentence.AudioDurationMs = &d
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

	// Build the word lookup set from the pre-computed story_words_json lists plus
	// any noted words (which may reference words not in the sentence token list).
	baseWordSet := map[string]struct{}{}
	for i := range stories {
		for _, bw := range storyWordsByID[stories[i].ID] {
			if bw != "" {
				baseWordSet[bw] = struct{}{}
			}
		}
		for _, nw := range stories[i].NotedWords {
			if nw.BaseWord != "" {
				baseWordSet[nw.BaseWord] = struct{}{}
			}
		}
	}

	// Populate word info (meaning, reading, drill counts, in_lexicon) from the words table.
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
			inLexicon   int
		}
		lexRows, err := db.Query(
			`SELECT id, word, drill_count, drill_target, COALESCE(meaning,''), COALESCE(reading,''), in_lexicon
			 FROM words WHERE word IN (`+strings.Join(placeholders, ",")+`)`,
			lexArgs...,
		)
		if err != nil {
			return nil, err
		}
		wordInfoMap := map[string]wordInfo{}
		for lexRows.Next() {
			var w string
			var info wordInfo
			if err := lexRows.Scan(&info.id, &w, &info.drillCount, &info.drillTarget, &info.meaning, &info.reading, &info.inLexicon); err != nil {
				lexRows.Close()
				return nil, err
			}
			wordInfoMap[w] = info
		}
		lexRows.Close()
		if err := lexRows.Err(); err != nil {
			return nil, err
		}
		for i := range stories {
			for j := range stories[i].Sentences {
				for k := range stories[i].Sentences[j].Words {
					bw := stories[i].Sentences[j].Words[k].BaseWord
					if info, ok := wordInfoMap[bw]; ok {
						stories[i].Sentences[j].Words[k].InLexicon = info.inLexicon == 1
						stories[i].Sentences[j].Words[k].WordID = info.id
						stories[i].Sentences[j].Words[k].DrillCount = info.drillCount
						stories[i].Sentences[j].Words[k].DrillTarget = info.drillTarget
						stories[i].Sentences[j].Words[k].English = info.meaning
						stories[i].Sentences[j].Words[k].Reading = info.reading
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


func setStoryHasAudio(db *sql.DB, storyID int64, hasAudio bool) error {
	v := 0
	if hasAudio {
		v = 1
	}
	_, err := db.Exec(`UPDATE stories SET has_audio = ? WHERE id = ?`, v, storyID)
	return err
}

func setSentenceAudioDuration(db *sql.DB, sentenceID int64, durationMs int64) error {
	_, err := db.Exec(`UPDATE story_sentences SET audio_duration_ms = ? WHERE id = ?`, durationMs, sentenceID)
	return err
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
