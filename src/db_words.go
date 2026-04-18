package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	wordSortAdded     = "added"
	wordSortDrilled   = "drilled"
	wordSortReading   = "reading"
	wordSortType      = "type"
	wordSortCorrect   = "correct"
	wordSortIncorrect = "incorrect"
	wordSortTarget    = "target"
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
	Readings  []string `json:"readings"`
}

// kanjiDataEntry is one element of a word's kanji_data JSON column.
// Character and Meanings are not stored in the column; they are joined from the
// kanji table at query time and included in API responses so callers do not need
// a separate /api/kanji fetch.
type kanjiDataEntry struct {
	ID        int64    `json:"id"`
	Character string   `json:"character,omitempty"`
	Reading   string   `json:"reading"`
	Meanings  []string `json:"meanings,omitempty"`
	Readings  []string `json:"readings,omitempty"`
}

// wordJSON is the JSON shape returned by the /api/words endpoint.
// Field names are chosen to match what lexicon.js already expects.
type wordJSON struct {
	ID          int64            `json:"id"`
	Word        string           `json:"word"`
	Reading     string           `json:"reading"`
	PitchAccent *int             `json:"pitchAccent"`
	Type        string           `json:"type"`
	Meaning     string           `json:"meaning"`
	ExampleJp   string           `json:"exampleJp"`
	ExampleEn   string           `json:"exampleEn"`
	Correct     int              `json:"correct"`
	Incorrect   int              `json:"incorrect"`
	Target      int              `json:"target"`
	CreatedAt   string           `json:"createdAt"`
	LastDrilled *string          `json:"lastDrilled"`
	ImagePath   *string          `json:"imagePath"`
	KanjiData   []kanjiDataEntry `json:"kanjiData"`
	Tracked     int              `json:"tracked"`
}

func decodeKanjiDataEntries(kanjiData string) []kanjiDataEntry {
	var entries []kanjiDataEntry
	if strings.TrimSpace(kanjiData) != "" {
		json.Unmarshal([]byte(kanjiData), &entries) //nolint:errcheck
	}
	if entries == nil {
		entries = []kanjiDataEntry{}
	}
	return entries
}

func enrichKanjiDataEntries(db *sql.DB, entries []kanjiDataEntry) error {
	if len(entries) == 0 {
		return nil
	}

	missingIDs := make(map[int64]struct{})
	for _, entry := range entries {
		if entry.ID > 0 && entry.Character == "" {
			missingIDs[entry.ID] = struct{}{}
		}
	}
	if len(missingIDs) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(missingIDs))
	args := make([]any, 0, len(missingIDs))
	for id := range missingIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	rows, err := db.Query(
		`SELECT id, character, meanings, readings FROM kanji WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	kanjiByID := make(map[int64]kanjiJSON, len(missingIDs))
	for rows.Next() {
		var (
			info         kanjiJSON
			meaningsJSON string
			readingsJSON string
		)
		if err := rows.Scan(&info.ID, &info.Character, &meaningsJSON, &readingsJSON); err != nil {
			return err
		}
		json.Unmarshal([]byte(meaningsJSON), &info.Meanings) //nolint:errcheck
		json.Unmarshal([]byte(readingsJSON), &info.Readings) //nolint:errcheck
		if info.Meanings == nil {
			info.Meanings = []string{}
		}
		if info.Readings == nil {
			info.Readings = []string{}
		}
		kanjiByID[info.ID] = info
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range entries {
		if info, ok := kanjiByID[entries[i].ID]; ok {
			entries[i].Character = info.Character
			entries[i].Meanings = info.Meanings
			entries[i].Readings = info.Readings
		}
	}
	return nil
}

func enrichWordsKanjiDataEntries(db *sql.DB, words []wordJSON) error {
	if len(words) == 0 {
		return nil
	}

	missingIDs := make(map[int64]struct{})
	for _, word := range words {
		for _, entry := range word.KanjiData {
			if entry.ID > 0 && entry.Character == "" {
				missingIDs[entry.ID] = struct{}{}
			}
		}
	}
	if len(missingIDs) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(missingIDs))
	args := make([]any, 0, len(missingIDs))
	for id := range missingIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	rows, err := db.Query(
		`SELECT id, character, meanings, readings FROM kanji WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	kanjiByID := make(map[int64]kanjiJSON, len(missingIDs))
	for rows.Next() {
		var (
			info         kanjiJSON
			meaningsJSON string
			readingsJSON string
		)
		if err := rows.Scan(&info.ID, &info.Character, &meaningsJSON, &readingsJSON); err != nil {
			return err
		}
		json.Unmarshal([]byte(meaningsJSON), &info.Meanings) //nolint:errcheck
		json.Unmarshal([]byte(readingsJSON), &info.Readings) //nolint:errcheck
		if info.Meanings == nil {
			info.Meanings = []string{}
		}
		if info.Readings == nil {
			info.Readings = []string{}
		}
		kanjiByID[info.ID] = info
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range words {
		for j := range words[i].KanjiData {
			if info, ok := kanjiByID[words[i].KanjiData[j].ID]; ok {
				words[i].KanjiData[j].Character = info.Character
				words[i].KanjiData[j].Meanings = info.Meanings
				words[i].KanjiData[j].Readings = info.Readings
			}
		}
	}
	return nil
}

func fillWordInfoFromDictionary(
	word, reading, partOfSpeech, meaning, kanjiData string,
	upsertKanjiFn func(character string, meanings, readings []string) (int64, error),
) (string, *int, string, string, string, error) {
	var pitchAccent *int
	if kanjiData == "" {
		kanjiData = "[]"
	}
	if !dictIsReady() {
		return reading, pitchAccent, partOfSpeech, meaning, kanjiData, nil
	}

	info, err := lookupDictionaryWord(word)
	if err != nil || info == nil {
		return reading, pitchAccent, partOfSpeech, meaning, kanjiData, nil
	}

	if strings.TrimSpace(reading) == "" {
		reading = info.Reading
	}
	pitchAccent = info.PitchAccent
	if strings.TrimSpace(partOfSpeech) == "" {
		partOfSpeech = info.PartOfSpeech
	}
	if strings.TrimSpace(meaning) == "" {
		meaning = info.Meaning
	}
	if kanjiData != "[]" {
		return reading, pitchAccent, partOfSpeech, meaning, kanjiData, nil
	}

	kd := make([]kanjiDataEntry, 0, len(info.Kanji))
	for _, k := range info.Kanji {
		kID, err := upsertKanjiFn(k.Character, k.Meanings, k.Readings)
		if err != nil {
			return reading, pitchAccent, partOfSpeech, meaning, kanjiData, err
		}
		kd = append(kd, kanjiDataEntry{
			ID:        kID,
			Character: k.Character,
			Reading:   k.Reading,
			Meanings:  k.Meanings,
			Readings:  k.Readings,
		})
	}
	b, err := json.Marshal(kd)
	if err != nil {
		return reading, pitchAccent, partOfSpeech, meaning, kanjiData, err
	}
	return reading, pitchAccent, partOfSpeech, meaning, string(b), nil
}

// insertWord adds a single word to the lexicon. Only the word itself is
// required; all other fields are optional and default to empty / zero.
// kanjiData should be a JSON string like `[{"id":1,"reading":"はし"}]` or empty.
func insertWord(db *sql.DB, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData string, drillTarget int) error {
	word = strings.TrimSpace(word)
	if isLexiconBlacklistedWord(word) {
		return fmt.Errorf(lexiconBlacklistReason)
	}
	if drillTarget < 1 {
		drillTarget = 1
	}
	kat := 0
	if containsKatakana(word) {
		kat = 1
	}
	var err error
	var pitchAccent *int
	reading, pitchAccent, partOfSpeech, meaning, kanjiData, err = fillWordInfoFromDictionary(
		word, reading, partOfSpeech, meaning, kanjiData,
		func(character string, meanings, readings []string) (int64, error) {
			return upsertKanji(db, character, meanings, readings)
		},
	)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO words (base_word, reading, pitch_accent, part_of_speech, meaning, example_jp, example_en, drill_target, is_katakana, kanji_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, word, reading, pitchAccent, partOfSpeech, meaning, exampleJP, exampleEN, drillTarget, kat, kanjiData)
	return err
}

// insertWordReturningID inserts a new word into the lexicon and returns its ID.
// If the word already exists with tracked=0 (auto-inserted via a story), it is
// promoted: tracked is set to 1 and all provided fields are applied.
// Returns an error if the word already exists with tracked=1.
func insertWordReturningID(db *sql.DB, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData string, drillTarget int) (int64, error) {
	word = strings.TrimSpace(word)
	if isLexiconBlacklistedWord(word) {
		return 0, fmt.Errorf(lexiconBlacklistReason)
	}
	if drillTarget < 1 {
		drillTarget = 1
	}
	kat := 0
	if containsKatakana(word) {
		kat = 1
	}
	var err error
	var pitchAccent *int
	reading, pitchAccent, partOfSpeech, meaning, kanjiData, err = fillWordInfoFromDictionary(
		word, reading, partOfSpeech, meaning, kanjiData,
		func(character string, meanings, readings []string) (int64, error) {
			return upsertKanji(db, character, meanings, readings)
		},
	)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`
		INSERT INTO words (base_word, reading, pitch_accent, part_of_speech, meaning, example_jp, example_en, drill_target, is_katakana, kanji_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(base_word) DO UPDATE SET
			tracked        = 1,
			reading        = CASE WHEN COALESCE(reading, '')        = '' THEN excluded.reading        ELSE reading        END,
			pitch_accent   = CASE WHEN pitch_accent IS NULL         THEN excluded.pitch_accent   ELSE pitch_accent  END,
			part_of_speech = CASE WHEN COALESCE(part_of_speech, '') = '' THEN excluded.part_of_speech ELSE part_of_speech END,
			meaning        = CASE WHEN COALESCE(meaning, '')        = '' THEN excluded.meaning        ELSE meaning        END,
			example_jp     = CASE WHEN COALESCE(example_jp, '')     = '' THEN excluded.example_jp     ELSE example_jp     END,
			example_en     = CASE WHEN COALESCE(example_en, '')     = '' THEN excluded.example_en     ELSE example_en     END,
			drill_target   = excluded.drill_target,
			kanji_data     = CASE WHEN COALESCE(kanji_data, '[]')   = '[]' THEN excluded.kanji_data  ELSE kanji_data     END
		WHERE tracked = 0
	`, word, reading, pitchAccent, partOfSpeech, meaning, exampleJP, exampleEN, drillTarget, kat, kanjiData)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("already in lexicon")
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM words WHERE base_word = ?`, word).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// updateWordFill sets the AI-generated fields for an existing word by ID.
func updateWordFill(db *sql.DB, id int64, reading string, pitchAccent *int, partOfSpeech, meaning,
	exampleJP, exampleEN, kanjiData string) error {
	if kanjiData == "" {
		kanjiData = "[]"
	}
	_, err := db.Exec(`
		UPDATE words SET reading=?, pitch_accent=?, part_of_speech=?, meaning=?, example_jp=?, example_en=?, kanji_data=?
		WHERE id=?
	`, reading, pitchAccent, partOfSpeech, meaning, exampleJP, exampleEN, kanjiData, id)
	return err
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
		"SELECT id, base_word, reading, part_of_speech, meaning, example_jp, example_en, image_path, drill_count, incorrect_count, drill_target FROM words WHERE tracked = 1 AND base_word IN ("+placeholders+")",
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
	_, err := db.Exec("UPDATE words SET drill_target=MAX(drill_count,?) WHERE id=?", target, id)
	return err
}

type wordImageInfo struct {
	Word      string
	ImagePath *string
}

func getWordImageInfo(db *sql.DB, id int64) (*wordImageInfo, error) {
	var info wordImageInfo
	err := db.QueryRow("SELECT base_word, image_path FROM words WHERE id = ?", id).Scan(&info.Word, &info.ImagePath)
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
func upsertKanji(db *sql.DB, character string, meanings, readings []string) (int64, error) {
	meaningsJSON, err := json.Marshal(meanings)
	if err != nil {
		return 0, err
	}
	readingsJSON, err := json.Marshal(readings)
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`
		INSERT INTO kanji (character, meanings, readings) VALUES (?, ?, ?)
		ON CONFLICT(character) DO UPDATE SET meanings = excluded.meanings, readings = excluded.readings
	`, character, string(meaningsJSON), string(readingsJSON)); err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM kanji WHERE character = ?`, character).Scan(&id)
	return id, err
}

func upsertKanjiTx(tx *sql.Tx, character string, meanings, readings []string) (int64, error) {
	meaningsJSON, err := json.Marshal(meanings)
	if err != nil {
		return 0, err
	}
	readingsJSON, err := json.Marshal(readings)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`
		INSERT INTO kanji (character, meanings, readings) VALUES (?, ?, ?)
		ON CONFLICT(character) DO UPDATE SET meanings = excluded.meanings, readings = excluded.readings
	`, character, string(meaningsJSON), string(readingsJSON)); err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRow(`SELECT id FROM kanji WHERE character = ?`, character).Scan(&id)
	return id, err
}

// listKanji returns all rows from the kanji table.
func listKanji(db *sql.DB) ([]kanjiJSON, error) {
	rows, err := db.Query(`SELECT id, character, meanings, readings FROM kanji ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []kanjiJSON
	for rows.Next() {
		var k kanjiJSON
		var meaningsStr string
		var readingsStr string
		if err := rows.Scan(&k.ID, &k.Character, &meaningsStr, &readingsStr); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(meaningsStr), &k.Meanings)
		json.Unmarshal([]byte(readingsStr), &k.Readings)
		if k.Meanings == nil {
			k.Meanings = []string{}
		}
		if k.Readings == nil {
			k.Readings = []string{}
		}
		out = append(out, k)
	}
	if out == nil {
		out = []kanjiJSON{}
	}
	return out, rows.Err()
}

type wordListPage struct {
	Items       []wordJSON `json:"items"`
	Total       int        `json:"total"`
	ActiveTotal int        `json:"activeTotal"`
}

func normalizeWordSort(sort string) string {
	switch sort {
	case wordSortAdded, wordSortDrilled, wordSortReading, wordSortType, wordSortCorrect, wordSortIncorrect, wordSortTarget:
		return sort
	default:
		return wordSortAdded
	}
}

func normalizeWordSortDir(dir string) string {
	if strings.EqualFold(dir, "asc") {
		return "asc"
	}
	return "desc"
}

func wordSortOrderClause(sort, dir string) string {
	sort = normalizeWordSort(sort)
	dir = normalizeWordSortDir(dir)

	switch sort {
	case wordSortAdded:
		if dir == "asc" {
			return "created_at ASC, id ASC"
		}
		return "created_at DESC, id DESC"
	case wordSortDrilled:
		if dir == "asc" {
			return "last_drilled_at IS NOT NULL ASC, last_drilled_at ASC, created_at DESC, id DESC"
		}
		return "last_drilled_at IS NULL ASC, last_drilled_at DESC, created_at DESC, id DESC"
	case wordSortReading:
		if dir == "asc" {
			return "COALESCE(reading,'') COLLATE NOCASE ASC, created_at DESC, id DESC"
		}
		return "COALESCE(reading,'') COLLATE NOCASE DESC, created_at DESC, id DESC"
	case wordSortType:
		if dir == "asc" {
			return "COALESCE(part_of_speech,'') ASC, last_drilled_at IS NULL ASC, last_drilled_at DESC, created_at DESC, id DESC"
		}
		return "COALESCE(part_of_speech,'') DESC, last_drilled_at IS NOT NULL ASC, last_drilled_at ASC, created_at DESC, id DESC"
	case wordSortCorrect:
		if dir == "asc" {
			return "drill_count ASC, created_at DESC, id DESC"
		}
		return "drill_count DESC, created_at DESC, id DESC"
	case wordSortIncorrect:
		if dir == "asc" {
			return "incorrect_count ASC, created_at DESC, id DESC"
		}
		return "incorrect_count DESC, created_at DESC, id DESC"
	case wordSortTarget:
		if dir == "asc" {
			return "drill_target ASC, created_at DESC, id DESC"
		}
		return "drill_target DESC, created_at DESC, id DESC"
	default:
		return "created_at DESC, id DESC"
	}
}

func scanWordRows(rows *sql.Rows) ([]wordJSON, error) {
	var words []wordJSON
	for rows.Next() {
		var w wordJSON
		var kanjiDataStr string
		if err := rows.Scan(
			&w.ID, &w.Word, &w.Reading, &w.PitchAccent, &w.Type, &w.Meaning, &w.ExampleJp, &w.ExampleEn,
			&w.Correct, &w.Incorrect, &w.Target, &w.CreatedAt, &w.LastDrilled,
			&w.ImagePath, &kanjiDataStr, &w.Tracked,
		); err != nil {
			return nil, err
		}
		w.KanjiData = decodeKanjiDataEntries(kanjiDataStr)
		words = append(words, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if words == nil {
		words = []wordJSON{}
	}
	return words, nil
}

// listWords returns all words from the lexicon ordered by creation date descending.
// Each word's KanjiData entries are enriched with Character and Meanings from the
// kanji table so callers receive complete kanji info without a separate query.
func listWords(db *sql.DB) ([]wordJSON, error) {
	rows, err := db.Query(`
		SELECT id, base_word, COALESCE(reading,''), pitch_accent, COALESCE(part_of_speech,''), COALESCE(meaning,''),
		       COALESCE(example_jp,''), COALESCE(example_en,''),
		       drill_count, incorrect_count, drill_target, created_at, last_drilled_at,
		       image_path, COALESCE(kanji_data,'[]'), tracked
		FROM words
		WHERE tracked = 1
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	words, err := scanWordRows(rows)
	if err != nil {
		return nil, err
	}
	if err := enrichWordsKanjiDataEntries(db, words); err != nil {
		return nil, err
	}
	return words, nil
}

func listWordsPage(db *sql.DB, sort, dir string, offset, limit int) (wordListPage, error) {
	page := wordListPage{
		Items: []wordJSON{},
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	if err := db.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN drill_count < drill_target THEN 1 ELSE 0 END) FROM words WHERE tracked = 1`).Scan(&page.Total, &page.ActiveTotal); err != nil {
		return page, err
	}
	if page.Total == 0 || offset >= page.Total {
		return page, nil
	}

	rows, err := db.Query(`
		SELECT id, base_word, COALESCE(reading,''), pitch_accent, COALESCE(part_of_speech,''), COALESCE(meaning,''),
		       COALESCE(example_jp,''), COALESCE(example_en,''),
		       drill_count, incorrect_count, drill_target, created_at, last_drilled_at,
		       image_path, COALESCE(kanji_data,'[]'), tracked
		FROM words
		WHERE tracked = 1
		ORDER BY `+wordSortOrderClause(sort, dir)+`
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	items, err := scanWordRows(rows)
	if err != nil {
		return page, err
	}
	if err := enrichWordsKanjiDataEntries(db, items); err != nil {
		return page, err
	}
	page.Items = items
	return page, nil
}

func trackedWordCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM words WHERE tracked = 1`).Scan(&count)
	return count, err
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
	_, err := db.Exec("DELETE FROM words WHERE base_word IN ("+placeholders+")", args...)
	return err
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

const lexiconBlacklistReason = "excluded from lexicon"

// wordInsertBlacklist is the combined set of Japanese particles, conjunctions,
// and standalone utility words that should never be added to the lexicon.
var wordInsertBlacklist = func() map[string]struct{} {
	particles := []string{
		// case / postpositional particles
		"が", "を", "に", "で", "へ", "と", "から", "より", "まで", "や", "か", "の",
		// binding / focus particles
		"は", "も", "こそ", "さえ", "しか", "だけ", "ばかり", "など", "ほど",
		"くらい", "ぐらい", "ずつ", "なんか", "なんて", "って",
		// conjunctive / subordinating particles
		"て", "で", "ながら", "たり", "ば", "し", "けど", "けれど", "けれども",
		"のに", "ので", "から", "たら", "なら", "ても", "でも",
		// sentence-ending particles
		"ね", "よ", "な", "わ", "ぞ", "ぜ", "さ",
	}
	conjunctions := []string{
		"そして", "しかし", "でも", "だから", "それで", "また", "あるいは",
		"または", "それとも", "ところが", "ところで", "さらに", "そのうえ",
		"それに", "しかも", "そのため", "そこで", "なぜなら", "つまり",
		"すなわち", "たとえば", "ようするに", "そもそも", "ただし",
		"もっとも", "なお", "ちなみに", "もしくは", "および", "ならびに",
		"かつ", "だが", "けれども", "ゆえに", "したがって", "さて", "では",
		"それでも", "なのに", "ともかく", "ともあれ",
	}
	utilityWords := []string{
		"する",
	}
	all := append(particles, conjunctions...)
	all = append(all, utilityWords...)
	set := make(map[string]struct{}, len(all))
	for _, w := range all {
		set[w] = struct{}{}
	}
	return set
}()

func isLexiconBlacklistedWord(word string) bool {
	_, blacklisted := wordInsertBlacklist[strings.TrimSpace(word)]
	return blacklisted
}

func markBlacklistedWordsUntracked(db *sql.DB) error {
	if len(wordInsertBlacklist) == 0 {
		return nil
	}
	tables, err := listTables(db)
	if err != nil {
		return err
	}
	hasWords := false
	for _, table := range tables {
		if table == "words" {
			hasWords = true
			break
		}
	}
	if !hasWords {
		return nil
	}
	placeholders := strings.Repeat("?,", len(wordInsertBlacklist))
	placeholders = strings.TrimRight(placeholders, ",")
	args := make([]any, 0, len(wordInsertBlacklist))
	for word := range wordInsertBlacklist {
		args = append(args, word)
	}
	_, err = db.Exec("UPDATE words SET tracked = 0 WHERE base_word IN ("+placeholders+")", args...)
	return err
}

// containsJapaneseLetter reports whether s contains at least one hiragana,
// katakana, or CJK unified ideograph. Used to exclude pure punctuation tokens
// (。、「」 etc.) from the lexicon during story import.
func containsJapaneseLetter(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x309F) || // hiragana
			(r >= 0x30A0 && r <= 0x30FF) || // katakana
			(r >= 0x4E00 && r <= 0x9FFF) { // CJK unified ideographs
			return true
		}
	}
	return false
}

// insertWordsIfAbsent adds each word in baseWords to the lexicon with
// tracked=0 if that word is not already present. Existing rows, regardless
// of their tracked value, are left untouched. Pure punctuation/symbol tokens
// (no kana or kanji) are silently skipped.
// Returns the subset of baseWords that passed the gate (inserted or pre-existing).
func insertWordsIfAbsent(tx *sql.Tx, baseWords []string) ([]string, error) {
	var accepted []string
	for _, word := range baseWords {
		if word = strings.TrimSpace(word); word == "" {
			continue
		}
		if !containsJapaneseLetter(word) {
			continue
		}
		if isLexiconBlacklistedWord(word) {
			continue
		}
		kat := 0
		if containsKatakana(word) {
			kat = 1
		}
		reading, pitchAccent, partOfSpeech, meaning, kanjiData, err := fillWordInfoFromDictionary(
			word, "", "", "", "",
			func(character string, meanings, readings []string) (int64, error) {
				return upsertKanjiTx(tx, character, meanings, readings)
			},
		)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO words (base_word, reading, pitch_accent, part_of_speech, meaning, is_katakana, kanji_data, tracked)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0)
		`, word, reading, pitchAccent, partOfSpeech, meaning, kat, kanjiData); err != nil {
			return nil, err
		}
		accepted = append(accepted, word)
	}
	return accepted, nil
}
