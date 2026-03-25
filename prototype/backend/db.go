package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func initDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal("open db:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("ping db:", err)
	}

	// Keep a single writer connection to avoid SQLite locking issues.
	db.SetMaxOpenConns(1)

	migrate(db)
	seedDB(db)
	return db
}

func migrate(db *sql.DB) {
	// Each entry runs exactly once; user_version tracks how many have been applied.
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS words (
			id                INTEGER  PRIMARY KEY AUTOINCREMENT,
			word              TEXT     NOT NULL UNIQUE,
			reading           TEXT,
			part_of_speech    TEXT,
			meaning           TEXT,
			example_jp        TEXT,
			example_en        TEXT,
			audio_word_path    TEXT,
			audio_example_path TEXT,
			drill_count       INTEGER  NOT NULL DEFAULT 0,
			drill_target      INTEGER  NOT NULL DEFAULT 1,
			incorrect_count   INTEGER  NOT NULL DEFAULT 0,
			created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
			last_drilled_at   DATETIME,
			target_reached_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS drill_sessions (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			started_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS drill_answers (
			id          INTEGER  PRIMARY KEY AUTOINCREMENT,
			session_id  INTEGER  NOT NULL REFERENCES drill_sessions(id),
			word_id     INTEGER  NOT NULL REFERENCES words(id),
			correct     INTEGER  NOT NULL CHECK (correct IN (0, 1)),
			answered_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`ALTER TABLE words ADD COLUMN is_katakana INTEGER NOT NULL DEFAULT 0`,
	}

	var version int
	db.QueryRow("PRAGMA user_version").Scan(&version)

	for i, m := range migrations {
		if i < version {
			continue
		}
		if _, err := db.Exec(m); err != nil {
			log.Fatalf("migrate %d: %v", i+1, err)
		}
		db.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1))
	}
	log.Printf("DB migration OK (version %d)", len(migrations))
}

// resetDB drops all user tables and re-runs migrations, giving a clean slate.
func resetDB(db *sql.DB) error {
	tables, err := listTables(db)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", t)); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	db.Exec("PRAGMA user_version = 0")
	migrate(db)
	seedDB(db)
	return nil
}

// columnInfo holds metadata for a single table column.
type columnInfo struct {
	Name    string
	Type    string
	NotNull bool
	PK      bool
	Unique  bool
}

// tableInfo holds a table name, row count, and column definitions.
type tableInfo struct {
	Name    string
	Rows    int
	Columns []columnInfo
}

// listTableInfos returns all user tables with row counts and column definitions.
func listTableInfos(db *sql.DB) ([]tableInfo, error) {
	tables, err := listTables(db)
	if err != nil {
		return nil, err
	}
	infos := make([]tableInfo, 0, len(tables))
	for _, t := range tables {
		var count int
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", t)).Scan(&count)
		cols, err := listColumns(db, t)
		if err != nil {
			return nil, err
		}
		infos = append(infos, tableInfo{Name: t, Rows: count, Columns: cols})
	}
	return infos, nil
}

// listColumns returns column definitions for the given table using PRAGMA table_info.
func listColumns(db *sql.DB, table string) ([]columnInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dflt any
		rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk)
		cols = append(cols, columnInfo{
			Name:    name,
			Type:    typ,
			NotNull: notNull == 1,
			PK:      pk > 0,
		})
	}

	// Find columns covered by a single-column unique index.
	uniqueCols, err := uniqueColumnSet(db, table)
	if err != nil {
		return nil, err
	}
	for i, c := range cols {
		if uniqueCols[c.Name] {
			cols[i].Unique = true
		}
	}
	return cols, nil
}

// uniqueColumnSet returns a set of column names that have a single-column unique
// index on the given table, by cross-referencing PRAGMA index_list and index_info.
func uniqueColumnSet(db *sql.DB, table string) (map[string]bool, error) {
	// Collect unique index names first, then close the cursor before running
	// inner queries — necessary because SetMaxOpenConns(1) means a nested
	// query would deadlock waiting for the connection the outer cursor holds.
	idxRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%q)", table))
	if err != nil {
		return nil, err
	}
	var uniqueIndexes []string
	for idxRows.Next() {
		var seq, partial int
		var idxName, origin string
		var isUnique int
		idxRows.Scan(&seq, &idxName, &isUnique, &origin, &partial)
		if isUnique == 1 {
			uniqueIndexes = append(uniqueIndexes, idxName)
		}
	}
	idxRows.Close()

	// Only mark single-column indexes — multi-column unique constraints
	// apply to the combination, not to each column individually.
	unique := make(map[string]bool)
	for _, idxName := range uniqueIndexes {
		colRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%q)", idxName))
		if err != nil {
			return nil, err
		}
		var colNames []string
		for colRows.Next() {
			var seqno, cid int
			var colName string
			colRows.Scan(&seqno, &cid, &colName)
			colNames = append(colNames, colName)
		}
		colRows.Close()
		if len(colNames) == 1 {
			unique[colNames[0]] = true
		}
	}
	return unique, nil
}

// listTables returns user-created table names (excludes sqlite internals).
func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	return tables, nil
}

// validTableName checks that the requested table actually exists, preventing SQL
// injection through the table name parameter (which cannot be parameterised in SQLite).
func validTableName(db *sql.DB, name string) bool {
	tables, err := listTables(db)
	if err != nil {
		return false
	}
	return slices.Contains(tables, name)
}

// seedWord is the shape of a word entry in seed.json.
type seedWord struct {
	Word            string  `json:"word"`
	Reading         string  `json:"reading"`
	PartOfSpeech    string  `json:"part_of_speech"`
	Meaning         string  `json:"meaning"`
	ExampleJP       string  `json:"example_jp"`
	ExampleEN       string  `json:"example_en"`
	DrillCount      int     `json:"drill_count"`
	DrillTarget     int     `json:"drill_target"`
	IncorrectCount  int     `json:"incorrect_count"`
	CreatedAt       string  `json:"created_at"`
	LastDrilledAt   *string `json:"last_drilled_at"`
	TargetReachedAt *string `json:"target_reached_at"`
}

// seedAnswer is one drill answer within a session in seed.json.
type seedAnswer struct {
	Word       string `json:"word"`
	Correct    bool   `json:"correct"`
	AnsweredAt string `json:"answered_at"`
}

// seedSession is a drill session with its answers in seed.json.
type seedSession struct {
	StartedAt string       `json:"started_at"`
	Answers   []seedAnswer `json:"answers"`
}

// seedData is the top-level shape of seed.json.
type seedData struct {
	Words    []seedWord    `json:"words"`
	Sessions []seedSession `json:"sessions"`
}

// seedDB loads seed.json and inserts words and drill history if the words table is empty.
func seedDB(db *sql.DB) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM words").Scan(&count); err != nil {
		log.Println("seed: could not count words:", err)
		return
	}
	if count > 0 {
		return
	}

	data, err := os.ReadFile("seed.json")
	if err != nil {
		log.Println("seed: seed.json not found, skipping")
		return
	}

	var seed seedData
	if err := json.Unmarshal(data, &seed); err != nil {
		log.Fatal("seed: invalid seed.json:", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal("seed: begin tx:", err)
	}

	// Insert words, collecting word text → id for answer lookup.
	wordStmt, err := tx.Prepare(`
		INSERT INTO words
			(word, reading, part_of_speech, meaning, example_jp, example_en,
			 drill_count, drill_target, incorrect_count,
			 created_at, last_drilled_at, target_reached_at, is_katakana)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Fatal("seed: prepare words:", err)
	}
	defer wordStmt.Close()

	wordIDs := make(map[string]int64)
	for _, w := range seed.Words {
		kat := 0
		if containsKatakana(w.Word) {
			kat = 1
		}
		res, err := wordStmt.Exec(
			w.Word, w.Reading, w.PartOfSpeech, w.Meaning, w.ExampleJP, w.ExampleEN,
			w.DrillCount, w.DrillTarget, w.IncorrectCount,
			w.CreatedAt, w.LastDrilledAt, w.TargetReachedAt, kat,
		)
		if err != nil {
			tx.Rollback()
			log.Fatal("seed: insert word:", err)
		}
		id, _ := res.LastInsertId()
		wordIDs[w.Word] = id
	}

	// Insert sessions and their answers.
	sessionStmt, err := tx.Prepare(`INSERT INTO drill_sessions (started_at) VALUES (?)`)
	if err != nil {
		log.Fatal("seed: prepare sessions:", err)
	}
	defer sessionStmt.Close()

	answerStmt, err := tx.Prepare(`
		INSERT INTO drill_answers (session_id, word_id, correct, answered_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		log.Fatal("seed: prepare answers:", err)
	}
	defer answerStmt.Close()

	for _, s := range seed.Sessions {
		res, err := sessionStmt.Exec(s.StartedAt)
		if err != nil {
			tx.Rollback()
			log.Fatal("seed: insert session:", err)
		}
		sessionID, _ := res.LastInsertId()

		for _, a := range s.Answers {
			wordID, ok := wordIDs[a.Word]
			if !ok {
				tx.Rollback()
				log.Fatalf("seed: answer references unknown word %q", a.Word)
			}
			correct := 0
			if a.Correct {
				correct = 1
			}
			if _, err := answerStmt.Exec(sessionID, wordID, correct, a.AnsweredAt); err != nil {
				tx.Rollback()
				log.Fatal("seed: insert answer:", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal("seed: commit:", err)
	}
	log.Printf("DB seeded: %d words, %d sessions", len(seed.Words), len(seed.Sessions))
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

// insertWord adds a single word to the lexicon. Only the word itself is
// required; all other fields are optional and default to empty / zero.
func insertWord(db *sql.DB, word, reading, partOfSpeech, meaning, exampleJP, exampleEN string, drillTarget int) error {
	if drillTarget < 1 {
		drillTarget = 1
	}
	kat := 0
	if containsKatakana(word) {
		kat = 1
	}
	_, err := db.Exec(`
		INSERT INTO words (word, reading, part_of_speech, meaning, example_jp, example_en, drill_target, is_katakana)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, word, reading, partOfSpeech, meaning, exampleJP, exampleEN, drillTarget, kat)
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

// wordJSON is the JSON shape returned by the /api/words endpoint.
// Field names are chosen to match what lexicon.js already expects.
type wordJSON struct {
	ID          int64   `json:"id"`
	Word        string  `json:"word"`
	Reading     string  `json:"reading"`
	Type        string  `json:"type"`
	Meaning     string  `json:"meaning"`
	ExampleJp   string  `json:"exampleJp"`
	ExampleEn   string  `json:"exampleEn"`
	Correct     int     `json:"correct"`
	Incorrect   int     `json:"incorrect"`
	Target      int     `json:"target"`
	CreatedAt   string  `json:"createdAt"`
	LastDrilled *string `json:"lastDrilled"`
}

// listWords returns all words from the lexicon ordered by creation date descending.
func listWords(db *sql.DB) ([]wordJSON, error) {
	rows, err := db.Query(`
		SELECT id, word, reading, part_of_speech, meaning, example_jp, example_en,
		       drill_count, incorrect_count, drill_target, created_at, last_drilled_at
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
		if err := rows.Scan(
			&w.ID, &w.Word, &w.Reading, &w.Type, &w.Meaning, &w.ExampleJp, &w.ExampleEn,
			&w.Correct, &w.Incorrect, &w.Target, &w.CreatedAt, &w.LastDrilled,
		); err != nil {
			return nil, err
		}
		words = append(words, w)
	}
	return words, rows.Err()
}

// updateWord saves editable fields for an existing word by ID.
func updateWord(db *sql.DB, id int64, reading, partOfSpeech, meaning, exampleJp, exampleEn string, target int) error {
	_, err := db.Exec(`
		UPDATE words
		SET reading=?, part_of_speech=?, meaning=?, example_jp=?, example_en=?, drill_target=?
		WHERE id=?
	`, reading, partOfSpeech, meaning, exampleJp, exampleEn, target, id)
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

// createDrillSession inserts a new drill_sessions row and returns its ID.
func createDrillSession(db *sql.DB) (int64, error) {
	res, err := db.Exec(`INSERT INTO drill_sessions DEFAULT VALUES`)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// recordDrillAnswer inserts one row into drill_answers and updates the word's
// counts and timestamps. For a correct answer: drill_count++, last_drilled_at,
// and target_reached_at (first time drill_count reaches drill_target). For an
// incorrect answer: incorrect_count++, last_drilled_at.
func recordDrillAnswer(db *sql.DB, sessionID, wordID int64, correct bool) error {
	correctInt := 0
	if correct {
		correctInt = 1
	}
	if _, err := db.Exec(
		`INSERT INTO drill_answers (session_id, word_id, correct) VALUES (?, ?, ?)`,
		sessionID, wordID, correctInt,
	); err != nil {
		return err
	}

	if correct {
		_, err := db.Exec(`
			UPDATE words SET
				drill_count     = drill_count + 1,
				last_drilled_at = datetime('now'),
				target_reached_at = CASE
					WHEN target_reached_at IS NULL AND (drill_count + 1) >= drill_target
					THEN datetime('now')
					ELSE target_reached_at
				END
			WHERE id = ?
		`, wordID)
		return err
	}
	_, err := db.Exec(`
		UPDATE words SET
			incorrect_count = incorrect_count + 1,
			last_drilled_at = datetime('now')
		WHERE id = ?
	`, wordID)
	return err
}

// queryTable returns all rows of a table as string slices, newest first.
func queryTable(db *sql.DB, table string) (cols []string, rows [][]string, err error) {
	sqlRows, err := db.Query(fmt.Sprintf("SELECT * FROM %q ORDER BY rowid DESC", table))
	if err != nil {
		return nil, nil, err
	}
	defer sqlRows.Close()

	cols, err = sqlRows.Columns()
	if err != nil {
		return nil, nil, err
	}

	for sqlRows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		sqlRows.Scan(ptrs...)
		row := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		rows = append(rows, row)
	}
	return cols, rows, nil
}

// activityStats holds headline stats for the activity page.
type activityStats struct {
	LexiconSize     int `json:"lexiconSize"`
	ActiveWords     int `json:"activeWords"`
	ClearedLifetime int `json:"clearedLifetime"`
	DrillsCleared   int `json:"drillsCleared"`
	DrillsClose     int `json:"drillsClose"`
	DrillsMid       int `json:"drillsMid"`
	DrillsFar       int `json:"drillsFar"`
}

// getActivityStats returns headline statistics computed from the words table.
func getActivityStats(db *sql.DB) (activityStats, error) {
	var s activityStats
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN drill_count < drill_target THEN 1 ELSE 0 END),
			SUM(CASE WHEN target_reached_at IS NOT NULL THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count >= drill_target THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) <= 4 THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) > 4 AND (drill_target - drill_count) <= 8 THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) > 8 THEN 1 ELSE 0 END)
		FROM words
	`).Scan(&s.LexiconSize, &s.ActiveWords, &s.ClearedLifetime, &s.DrillsCleared, &s.DrillsClose, &s.DrillsMid, &s.DrillsFar)
	return s, err
}

// activityWordEntry is one word entry within a calendar day section.
type activityWordEntry struct {
	Word    string `json:"word"`
	Reading string `json:"reading"`
	Meaning string `json:"meaning"`
	Knew    *bool  `json:"knew,omitempty"` // set only for drilled entries
}

// activityDay holds the drilled/added/cleared events for a single calendar day.
type activityDay struct {
	Drilled []activityWordEntry `json:"drilled"`
	Added   []activityWordEntry `json:"added"`
	Cleared []activityWordEntry `json:"cleared"`
}

// activityCalendar is the full response for the /api/activity/calendar endpoint.
type activityCalendar struct {
	Today        string                 `json:"today"`
	HistoryStart string                 `json:"historyStart"`
	Days         map[string]activityDay `json:"days"`
}

// getActivityCalendar builds the date-keyed calendar data from drill_answers,
// words.created_at, and words.target_reached_at.
func getActivityCalendar(db *sql.DB) (activityCalendar, error) {
	days := make(map[string]activityDay)

	// Drilled entries — one entry per (word, date), marked wrong if any answer
	// that day was wrong (MIN(correct) = 0 if any incorrect answer exists).
	rows, err := db.Query(`
		SELECT w.word, COALESCE(w.reading,''), COALESCE(w.meaning,''),
		       MIN(da.correct), DATE(da.answered_at)
		FROM drill_answers da
		JOIN words w ON w.id = da.word_id
		GROUP BY w.id, DATE(da.answered_at)
		ORDER BY MIN(da.answered_at)
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows.Next() {
		var word, reading, meaning, dateStr string
		var correct int
		if err := rows.Scan(&word, &reading, &meaning, &correct, &dateStr); err != nil {
			rows.Close()
			return activityCalendar{}, err
		}
		knew := correct == 1
		d := days[dateStr]
		d.Drilled = append(d.Drilled, activityWordEntry{Word: word, Reading: reading, Meaning: meaning, Knew: &knew})
		days[dateStr] = d
	}
	if err := rows.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Added entries — one entry per word on its creation date.
	rows2, err := db.Query(`
		SELECT word, COALESCE(reading,''), COALESCE(meaning,''), DATE(created_at)
		FROM words
		ORDER BY created_at
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows2.Next() {
		var word, reading, meaning, dateStr string
		if err := rows2.Scan(&word, &reading, &meaning, &dateStr); err != nil {
			rows2.Close()
			return activityCalendar{}, err
		}
		d := days[dateStr]
		d.Added = append(d.Added, activityWordEntry{Word: word, Reading: reading, Meaning: meaning})
		days[dateStr] = d
	}
	if err := rows2.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Cleared entries — words that first reached their drill target on a given date.
	rows3, err := db.Query(`
		SELECT word, COALESCE(reading,''), COALESCE(meaning,''), DATE(target_reached_at)
		FROM words
		WHERE target_reached_at IS NOT NULL
		ORDER BY target_reached_at
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows3.Next() {
		var word, reading, meaning, dateStr string
		if err := rows3.Scan(&word, &reading, &meaning, &dateStr); err != nil {
			rows3.Close()
			return activityCalendar{}, err
		}
		d := days[dateStr]
		d.Cleared = append(d.Cleared, activityWordEntry{Word: word, Reading: reading, Meaning: meaning})
		days[dateStr] = d
	}
	if err := rows3.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Ensure every day's slices are non-nil so they encode as [] not null.
	for k, v := range days {
		if v.Drilled == nil {
			v.Drilled = []activityWordEntry{}
		}
		if v.Added == nil {
			v.Added = []activityWordEntry{}
		}
		if v.Cleared == nil {
			v.Cleared = []activityWordEntry{}
		}
		days[k] = v
	}

	today := time.Now().UTC().Format("2006-01-02")

	// historyStart is the Sunday of the week containing the earliest activity date.
	historyStart := today
	for dateStr := range days {
		if dateStr < historyStart {
			historyStart = dateStr
		}
	}
	historyStart = calendarWeekSunday(historyStart)

	return activityCalendar{Today: today, HistoryStart: historyStart, Days: days}, nil
}

// calendarWeekSunday returns the Sunday of the week that contains dateStr (YYYY-MM-DD).
func calendarWeekSunday(dateStr string) string {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	dayOfWeek := int(d.Weekday()) // 0 = Sunday
	sun := d.AddDate(0, 0, -dayOfWeek)
	return sun.Format("2006-01-02")
}
