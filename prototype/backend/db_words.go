package main

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// existingWordInfo holds data for a word already in the lexicon.
type existingWordInfo struct {
	ID             int64
	Reading        string
	PartOfSpeech   string
	Meaning        string
	ExampleJP      string
	ExampleEN      string
	ImagePath      *string
	DrillCount     int // correct answer count
	DrillIncorrect int
	DrillTarget    int // when correct answers meets or exceeds this number, the word is "inactive"
}

// kanjiJSON is one entry in the /api/kanji response.
type kanjiJSON struct {
	ID        int64    `json:"id"`
	Character string   `json:"character"`
	Meanings  []string `json:"meanings"`
}

// kanjiDataEntry is one element of a word's kanji_data JSON column.
type kanjiDataEntry struct {
	ID      int64  `json:"id"`
	Reading string `json:"reading"`
}

// wordJSON is the JSON shape returned by the /api/words endpoint.
// Field names are chosen to match what lexicon.js already expects.
type wordJSON struct {
	ID              int64            `json:"id"`
	Word            string           `json:"word"`
	Reading         string           `json:"reading"`
	Type            string           `json:"type"`
	Meaning         string           `json:"meaning"`
	ExampleJp       string           `json:"exampleJp"`
	ExampleEn       string           `json:"exampleEn"`
	Correct         int              `json:"correct"`
	Incorrect       int              `json:"incorrect"`
	Target          int              `json:"target"`
	CreatedAt       string           `json:"createdAt"`
	LastDrilled     *string          `json:"lastDrilled"`
	ImagePath       *string          `json:"imagePath"`
	KanjiData       []kanjiDataEntry `json:"kanjiData"`
	HasWordAudio    bool             `json:"hasWordAudio"`
	HasSentenceAudio bool            `json:"hasSentenceAudio"`
}

// insertWord adds a single word to the lexicon. Only the word itself is
// required; all other fields are optional and default to empty / zero.
// kanjiData should be a JSON string like `[{"id":1,"reading":"はし"}]` or empty.
func insertWord(db *sql.DB, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData string, drillTarget int) error {
	if drillTarget < 1 {
		drillTarget = 1
	}
	kat := 0
	if containsKatakana(word) {
		kat = 1
	}
	if kanjiData == "" {
		kanjiData = "[]"
	}
	_, err := db.Exec(`
		INSERT INTO words (word, reading, part_of_speech, meaning, example_jp, example_en, drill_target, is_katakana, kanji_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, drillTarget, kat, kanjiData)
	return err
}

// insertWordReturningID inserts a new word and returns its database ID.
func insertWordReturningID(db *sql.DB, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData string, drillTarget int) (int64, error) {
	if drillTarget < 1 {
		drillTarget = 1
	}
	kat := 0
	if containsKatakana(word) {
		kat = 1
	}
	if kanjiData == "" {
		kanjiData = "[]"
	}
	res, err := db.Exec(`
		INSERT INTO words (word, reading, part_of_speech, meaning, example_jp, example_en, drill_target, is_katakana, kanji_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, drillTarget, kat, kanjiData)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// updateWordFill sets the AI-generated fields for an existing word by ID.
func updateWordFill(db *sql.DB, id int64, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData string) error {
	if kanjiData == "" {
		kanjiData = "[]"
	}
	_, err := db.Exec(`
		UPDATE words SET reading=?, part_of_speech=?, meaning=?, example_jp=?, example_en=?, kanji_data=?
		WHERE id=?
	`, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData, id)
	return err
}

// wordsExistInDB returns a set of which words from the given slice are already
// present in the lexicon, keyed by their normalised word value.
func wordsExistInDB(db *sql.DB, words []string) (map[string]bool, error) {
	if len(words) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(words))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(words))
	for i, w := range words {
		args[i] = w
	}
	rows, err := db.Query("SELECT word FROM words WHERE word IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	existing := make(map[string]bool, len(words))
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		existing[w] = true
	}
	return existing, rows.Err()
}

// wordsInfoInDB returns info for words already in the lexicon,
// keyed by their normalised word value.
func wordsInfoInDB(db *sql.DB, words []string) (map[string]existingWordInfo, error) {
	if len(words) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(words))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(words))
	for i, w := range words {
		args[i] = w
	}
	rows, err := db.Query(
		"SELECT id, word, reading, part_of_speech, meaning, example_jp, example_en, image_path, drill_count, incorrect_count, drill_target FROM words WHERE word IN ("+placeholders+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]existingWordInfo)
	for rows.Next() {
		var info existingWordInfo
		var word string
		if err := rows.Scan(&info.ID, &word, &info.Reading, &info.PartOfSpeech, &info.Meaning, &info.ExampleJP, &info.ExampleEN, &info.ImagePath, &info.DrillCount, &info.DrillIncorrect, &info.DrillTarget); err != nil {
			return nil, err
		}
		result[word] = info
	}
	return result, rows.Err()
}

// updateWordTarget updates only the drill_target for a word by ID.
func updateWordTarget(db *sql.DB, id int64, target int) error {
	_, err := db.Exec("UPDATE words SET drill_target=? WHERE id=?", target, id)
	return err
}

type wordImageInfo struct {
	Word      string
	ImagePath *string
}

func getWordImageInfo(db *sql.DB, id int64) (*wordImageInfo, error) {
	var info wordImageInfo
	err := db.QueryRow("SELECT word, image_path FROM words WHERE id = ?", id).Scan(&info.Word, &info.ImagePath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func updateWordImagePath(db *sql.DB, id int64, imagePath string) error {
	_, err := db.Exec("UPDATE words SET image_path=? WHERE id=?", imagePath, id)
	return err
}

// upsertKanji inserts a kanji row or updates its meanings if it already exists,
// then returns the row's ID.
func upsertKanji(db *sql.DB, character string, meanings []string) (int64, error) {
	meaningsJSON, err := json.Marshal(meanings)
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`
		INSERT INTO kanji (character, meanings) VALUES (?, ?)
		ON CONFLICT(character) DO NOTHING
	`, character, string(meaningsJSON)); err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM kanji WHERE character = ?`, character).Scan(&id)
	return id, err
}

// listKanji returns all rows from the kanji table.
func listKanji(db *sql.DB) ([]kanjiJSON, error) {
	rows, err := db.Query(`SELECT id, character, meanings FROM kanji ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []kanjiJSON
	for rows.Next() {
		var k kanjiJSON
		var meaningsStr string
		if err := rows.Scan(&k.ID, &k.Character, &meaningsStr); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(meaningsStr), &k.Meanings)
		if k.Meanings == nil {
			k.Meanings = []string{}
		}
		out = append(out, k)
	}
	if out == nil {
		out = []kanjiJSON{}
	}
	return out, rows.Err()
}

// listWords returns all words from the lexicon ordered by creation date descending.
func listWords(db *sql.DB) ([]wordJSON, error) {
	rows, err := db.Query(`
		SELECT id, word, COALESCE(reading,''), COALESCE(part_of_speech,''), COALESCE(meaning,''),
		       COALESCE(example_jp,''), COALESCE(example_en,''),
		       drill_count, incorrect_count, drill_target, created_at, last_drilled_at,
		       image_path, kanji_data, has_word_audio, has_sentence_audio
		FROM words
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var words []wordJSON
	for rows.Next() {
		var w wordJSON
		var kanjiDataStr *string
		var hasWordAudio, hasSentenceAudio int
		if err := rows.Scan(
			&w.ID, &w.Word, &w.Reading, &w.Type, &w.Meaning, &w.ExampleJp, &w.ExampleEn,
			&w.Correct, &w.Incorrect, &w.Target, &w.CreatedAt, &w.LastDrilled,
			&w.ImagePath, &kanjiDataStr, &hasWordAudio, &hasSentenceAudio,
		); err != nil {
			return nil, err
		}
		w.HasWordAudio = hasWordAudio == 1
		w.HasSentenceAudio = hasSentenceAudio == 1
		if kanjiDataStr != nil {
			json.Unmarshal([]byte(*kanjiDataStr), &w.KanjiData)
		}
		if w.KanjiData == nil {
			w.KanjiData = []kanjiDataEntry{}
		}
		words = append(words, w)
	}
	return words, rows.Err()
}

// updateWord saves editable fields for an existing word by ID.
func updateWord(db *sql.DB, id int64, reading, partOfSpeech, meaning, exampleJp, exampleEn, kanjiData string, target int) error {
	if kanjiData == "" {
		kanjiData = "[]"
	}
	_, err := db.Exec(`
		UPDATE words
		SET reading=?, part_of_speech=?, meaning=?, example_jp=?, example_en=?, kanji_data=?, drill_target=?
		WHERE id=?
	`, reading, partOfSpeech, meaning, exampleJp, exampleEn, kanjiData, target, id)
	return err
}

// deleteWordByID removes a single word from the lexicon by its primary key.
func deleteWordByID(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM words WHERE id = ?", id)
	return err
}

// deleteWordsByName removes words from the lexicon by their (normalised) word value.
func deleteWordsByName(db *sql.DB, words []string) error {
	if len(words) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(words))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma
	args := make([]any, len(words))
	for i, w := range words {
		args[i] = w
	}
	_, err := db.Exec("DELETE FROM words WHERE word IN ("+placeholders+")", args...)
	return err
}

// updateWordAudioFlags sets the has_word_audio and has_sentence_audio flags for a word by ID.
func updateWordAudioFlags(db *sql.DB, id int64, hasWord, hasSentence bool) error {
	w, s := 0, 0
	if hasWord {
		w = 1
	}
	if hasSentence {
		s = 1
	}
	_, err := db.Exec("UPDATE words SET has_word_audio=?, has_sentence_audio=? WHERE id=?", w, s, id)
	return err
}

// wordAudioInfo holds the fields needed to generate audio for a word.
type wordAudioInfo struct {
	Word      string
	ExampleJP string
}

func getWordAudioInfo(db *sql.DB, id int64) (*wordAudioInfo, error) {
	var info wordAudioInfo
	err := db.QueryRow("SELECT word, COALESCE(example_jp,'') FROM words WHERE id = ?", id).
		Scan(&info.Word, &info.ExampleJP)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &info, err
}

// containsKatakana reports whether s contains any character in the main
// Katakana Unicode block (U+30A0–U+30FF).
func containsKatakana(s string) bool {
	for _, r := range s {
		if r >= 0x30A0 && r <= 0x30FF {
			return true
		}
	}
	return false
}
