package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type storyWordInput struct {
	DisplayWord      string
	BaseWord         string
	AudioTimestampMs *int64
}

type storyWordJSON struct {
	DisplayWord      string `json:"displayWord"`
	BaseWord         string `json:"baseWord"`
	English          string `json:"english,omitempty"` // populated from word_glosses at query time; not stored per-token
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

	// Snapshot lexicon meanings for all base words used in this story.
	// Stored once at creation so the story is independent of future lexicon changes.
	baseWordSet := map[string]struct{}{}
	for _, sentence := range sentences {
		for _, word := range sentence.Words {
			if word.BaseWord != "" {
				baseWordSet[word.BaseWord] = struct{}{}
			}
		}
	}
	glossMap := map[string]string{}
	if len(baseWordSet) > 0 {
		placeholders := make([]string, 0, len(baseWordSet))
		lookupArgs := make([]any, 0, len(baseWordSet))
		for bw := range baseWordSet {
			placeholders = append(placeholders, "?")
			lookupArgs = append(lookupArgs, bw)
		}
		glossRows, err := tx.Query(
			`SELECT word, meaning FROM words WHERE word IN (`+strings.Join(placeholders, ",")+`) AND meaning != ''`,
			lookupArgs...,
		)
		if err != nil {
			return 0, err
		}
		for glossRows.Next() {
			var w, m string
			if err := glossRows.Scan(&w, &m); err != nil {
				glossRows.Close()
				return 0, err
			}
			glossMap[w] = m
		}
		glossRows.Close()
	}
	// Fill provisional glosses for grammar particles and punctuation not in the lexicon.
	for _, sentence := range sentences {
		for _, word := range sentence.Words {
			if _, ok := glossMap[word.BaseWord]; ok {
				continue
			}
			if g, ok := defaultStoryTokenGlosses[word.BaseWord]; ok {
				glossMap[word.BaseWord] = g
			} else if g, ok := defaultStoryTokenGlosses[word.DisplayWord]; ok {
				glossMap[word.BaseWord] = g
			}
		}
	}
	wordGlossesJSON, err := json.Marshal(glossMap)
	if err != nil {
		return 0, err
	}

	res, err := tx.Exec(`
		INSERT INTO stories (title, audio_path, word_glosses)
		VALUES (?, ?, ?)
	`, title, audioPath, string(wordGlossesJSON))
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

func queryStories(db *sql.DB, whereClause string, args ...any) ([]storyJSON, error) {
	rows, err := db.Query(`
		SELECT s.id, s.title, s.audio_path, s.has_audio, s.created_at, s.word_glosses, s.noted_words_json,
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
	var currentGlosses map[string]string

	for rows.Next() {
		var storyID int64
		var title string
		var audioPath sql.NullString
		var hasAudio int
		var createdAt string
		var wordGlossesJSON sql.NullString
		var notedWordsJSON sql.NullString
		var sentenceID sql.NullInt64
		var position sql.NullInt64
		var wordsJSON sql.NullString
		var englishText sql.NullString
		var isParagraphStart sql.NullInt64
		var audioDurationMs sql.NullInt64

		if err := rows.Scan(
			&storyID, &title, &audioPath, &hasAudio, &createdAt, &wordGlossesJSON, &notedWordsJSON,
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
			currentGlosses = nil
			if wordGlossesJSON.Valid {
				var g map[string]string
				if json.Unmarshal([]byte(wordGlossesJSON.String), &g) == nil {
					currentGlosses = g
				}
			}
			if notedWordsJSON.Valid && notedWordsJSON.String != "" {
				json.Unmarshal([]byte(notedWordsJSON.String), &story.NotedWords) //nolint:errcheck
				if story.NotedWords == nil {
					story.NotedWords = []storyNotedWordJSON{}
				}
				for i := range story.NotedWords {
					if story.NotedWords[i].English == "" {
						if gloss, ok := currentGlosses[story.NotedWords[i].BaseWord]; ok {
							story.NotedWords[i].English = gloss
						}
					}
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
				for k := range sentence.Words {
					if m, ok := currentGlosses[sentence.Words[k].BaseWord]; ok {
						sentence.Words[k].English = m
					}
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

// updateStoryWordGlosses merges newGlosses into the story's existing word_glosses map.
// Lexicon-sourced and default-particle glosses already present are preserved; only the
// keys present in newGlosses are overwritten.
func updateStoryWordGlosses(db *sql.DB, storyID int64, newGlosses map[string]string) error {
	var currentJSON sql.NullString
	if err := db.QueryRow(`SELECT word_glosses FROM stories WHERE id = ?`, storyID).Scan(&currentJSON); err != nil {
		return err
	}
	merged := map[string]string{}
	if currentJSON.Valid && currentJSON.String != "" {
		json.Unmarshal([]byte(currentJSON.String), &merged) //nolint:errcheck — stale JSON is non-fatal
	}
	for k, v := range newGlosses {
		merged[k] = v
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE stories SET word_glosses = ? WHERE id = ?`, string(b), storyID)
	return err
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

func provisionalStoryTokenGloss(surface, base string) string {
	if gloss, ok := defaultStoryTokenGlosses[base]; ok {
		return gloss
	}
	if gloss, ok := defaultStoryTokenGlosses[surface]; ok {
		return gloss
	}
	return base
}

var defaultStoryTokenGlosses = map[string]string{
	"。":  "period",
	"、":  "comma",
	"「":  "opening quote",
	"」":  "closing quote",
	"（":  "opening paren",
	"）":  "closing paren",
	"は":  "topic marker",
	"が":  "subject marker",
	"を":  "object marker",
	"に":  "at; in; to",
	"で":  "at; by; with",
	"の":  "of",
	"と":  "and; with; that",
	"も":  "also",
	"か":  "question marker",
	"から": "from",
	"へ":  "toward",
	"ね":  "right?",
	"よ":  "emphasis",
	"て":  "and; te-form",
	"た":  "past tense",
	"だ":  "is",
	"です": "is; polite copula",
	"ます": "polite ending",
	"いる": "to be; exist",
	"ある": "to be; exist",
	"する": "to do",
	"なる": "to become",
	"れる": "passive; potential helper",
	"そう": "it seems",
	"その": "that",
	"この": "this",
	"それ": "that",
	"一緒": "together",
	"約":  "about; approximately",
}
