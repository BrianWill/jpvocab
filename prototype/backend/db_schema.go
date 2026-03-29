package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"

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
			is_katakana       INTEGER  NOT NULL DEFAULT 0,
			kanji_data        TEXT,
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
		`CREATE TABLE IF NOT EXISTS kanji (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			character TEXT    NOT NULL UNIQUE,
			meanings  TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_settings (
			key   TEXT NOT NULL PRIMARY KEY,
			value TEXT NOT NULL
		)`,
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

// seedKanjiDef is one entry in the top-level "kanji" array in seed.json.
type seedKanjiDef struct {
	Character string   `json:"character"`
	Meanings  []string `json:"meanings"`
}

// seedKanjiRef links a kanji character to its reading within a specific word.
type seedKanjiRef struct {
	Char    string `json:"char"`
	Reading string `json:"reading"`
}

// seedWord is the shape of a word entry in seed.json.
type seedWord struct {
	Word            string         `json:"word"`
	Reading         string         `json:"reading"`
	PartOfSpeech    string         `json:"part_of_speech"`
	Meaning         string         `json:"meaning"`
	ExampleJP       string         `json:"example_jp"`
	ExampleEN       string         `json:"example_en"`
	DrillCount      int            `json:"drill_count"`
	DrillTarget     int            `json:"drill_target"`
	IncorrectCount  int            `json:"incorrect_count"`
	CreatedAt       string         `json:"created_at"`
	LastDrilledAt   *string        `json:"last_drilled_at"`
	TargetReachedAt *string        `json:"target_reached_at"`
	KanjiData       []seedKanjiRef `json:"kanji_data,omitempty"`
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
	Kanji    []seedKanjiDef `json:"kanji"`
	Words    []seedWord     `json:"words"`
	Sessions []seedSession  `json:"sessions"`
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

	// Insert kanji definitions, collecting character → id for word kanji_data.
	kanjiStmt, err := tx.Prepare(`INSERT INTO kanji (character, meanings) VALUES (?, ?)`)
	if err != nil {
		log.Fatal("seed: prepare kanji:", err)
	}
	defer kanjiStmt.Close()

	kanjiCharToID := make(map[string]int64, len(seed.Kanji))
	for _, k := range seed.Kanji {
		meaningsJSON, _ := json.Marshal(k.Meanings)
		res, err := kanjiStmt.Exec(k.Character, string(meaningsJSON))
		if err != nil {
			tx.Rollback()
			log.Fatal("seed: insert kanji:", err)
		}
		id, _ := res.LastInsertId()
		kanjiCharToID[k.Character] = id
	}

	// Insert words, collecting word text → id for answer lookup.
	wordStmt, err := tx.Prepare(`
		INSERT INTO words
			(word, reading, part_of_speech, meaning, example_jp, example_en,
			 drill_count, drill_target, incorrect_count,
			 created_at, last_drilled_at, target_reached_at, is_katakana, kanji_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		// Resolve kanji character references to IDs.
		type kanjiDataEntry struct {
			ID      int64  `json:"id"`
			Reading string `json:"reading"`
		}
		entries := make([]kanjiDataEntry, 0, len(w.KanjiData))
		for _, ref := range w.KanjiData {
			id, ok := kanjiCharToID[ref.Char]
			if !ok {
				tx.Rollback()
				log.Fatalf("seed: word %q references unknown kanji %q", w.Word, ref.Char)
			}
			entries = append(entries, kanjiDataEntry{ID: id, Reading: ref.Reading})
		}
		kanjiDataJSON, _ := json.Marshal(entries)

		res, err := wordStmt.Exec(
			w.Word, w.Reading, w.PartOfSpeech, w.Meaning, w.ExampleJP, w.ExampleEN,
			w.DrillCount, w.DrillTarget, w.IncorrectCount,
			w.CreatedAt, w.LastDrilledAt, w.TargetReachedAt, kat, string(kanjiDataJSON),
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
