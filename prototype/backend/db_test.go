package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// testDB opens an in-memory SQLite database, runs migrations, and registers a
// cleanup to close it. Tests get a fresh, isolated schema with no seed data.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal("open:", err)
	}
	db.SetMaxOpenConns(1)
	migrate(db)
	t.Cleanup(func() { db.Close() })
	return db
}

// --- migrate ---

func TestMigrate_CreatesAllTables(t *testing.T) {
	db := testDB(t)
	tables, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"words": true, "drill_sessions": true, "drill_answers": true, "kanji": true}
	for _, table := range tables {
		delete(want, table)
	}
	if len(want) > 0 {
		t.Errorf("missing tables after migration: %v", want)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	db := testDB(t)
	// Running migrate a second time on an already-migrated DB should not error
	// or create duplicate tables.
	migrate(db)
	tables, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 4 {
		t.Errorf("expected 4 tables, got %d: %v", len(tables), tables)
	}
}

// --- insertWord ---

func TestInsertWord_Basic(t *testing.T) {
	db := testDB(t)
	err := insertWord(db, "食べる", "たべる", "verb", "to eat", "私は寿司を食べる。", "I eat sushi.", "", 3)
	if err != nil {
		t.Fatalf("insertWord: %v", err)
	}

	var reading, meaning string
	var target int
	err = db.QueryRow(`SELECT reading, meaning, drill_target FROM words WHERE word = ?`, "食べる").
		Scan(&reading, &meaning, &target)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "たべる" {
		t.Errorf("reading: got %q, want %q", reading, "たべる")
	}
	if meaning != "to eat" {
		t.Errorf("meaning: got %q, want %q", meaning, "to eat")
	}
	if target != 3 {
		t.Errorf("drill_target: got %d, want 3", target)
	}
}

func TestInsertWord_DrillTargetClampsToOne(t *testing.T) {
	db := testDB(t)
	if err := insertWord(db, "猫", "", "", "", "", "", "", 0); err != nil {
		t.Fatal(err)
	}
	var target int
	db.QueryRow(`SELECT drill_target FROM words WHERE word = ?`, "猫").Scan(&target)
	if target != 1 {
		t.Errorf("drill_target: got %d, want 1 (clamped from 0)", target)
	}
}

func TestInsertWord_Duplicate(t *testing.T) {
	db := testDB(t)
	if err := insertWord(db, "犬", "", "", "", "", "", "", 1); err != nil {
		t.Fatal("first insert:", err)
	}
	err := insertWord(db, "犬", "", "", "", "", "", "", 1)
	if err == nil {
		t.Fatal("expected UNIQUE constraint error, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- validTableName ---

func TestValidTableName(t *testing.T) {
	db := testDB(t)
	cases := []struct {
		name  string
		valid bool
	}{
		{"words", true},
		{"drill_sessions", true},
		{"drill_answers", true},
		{"sqlite_master", false},
		{"nonexistent", false},
		{"'; DROP TABLE words; --", false},
	}
	for _, tc := range cases {
		got := validTableName(db, tc.name)
		if got != tc.valid {
			t.Errorf("validTableName(%q) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}

// --- queryTable ---

func TestQueryTable_ReturnsInsertedRow(t *testing.T) {
	db := testDB(t)
	if err := insertWord(db, "水", "みず", "noun", "water", "水を飲む。", "Drink water.", "", 1); err != nil {
		t.Fatal(err)
	}

	cols, rows, err := queryTable(db, "words")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	// Find the "word" column index and check its value.
	wordIdx := -1
	for i, c := range cols {
		if c == "word" {
			wordIdx = i
			break
		}
	}
	if wordIdx == -1 {
		t.Fatal("'word' column not found in result")
	}
	if rows[0][wordIdx] != "水" {
		t.Errorf("word: got %q, want %q", rows[0][wordIdx], "水")
	}
}

func TestQueryTable_NewestFirst(t *testing.T) {
	db := testDB(t)
	for _, w := range []string{"一", "二", "三"} {
		if err := insertWord(db, w, "", "", "", "", "", "", 1); err != nil {
			t.Fatal(err)
		}
	}
	_, rows, err := queryTable(db, "words")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// Rows are ordered newest (highest rowid) first.
	// The "word" column is index 1 (after id).
	if rows[0][1] != "三" {
		t.Errorf("first row (newest): got %q, want 三", rows[0][1])
	}
}

// --- listTableInfos / column metadata ---

func TestListTableInfos_WordsColumnFlags(t *testing.T) {
	db := testDB(t)
	infos, err := listTableInfos(db)
	if err != nil {
		t.Fatal(err)
	}

	var wordTable *tableInfo
	for i := range infos {
		if infos[i].Name == "words" {
			wordTable = &infos[i]
			break
		}
	}
	if wordTable == nil {
		t.Fatal("words table not found in infos")
	}

	colByName := make(map[string]columnInfo)
	for _, c := range wordTable.Columns {
		colByName[c.Name] = c
	}

	if id := colByName["id"]; !id.PK {
		t.Error("id column should be PK")
	}
	if word := colByName["word"]; !word.Unique {
		t.Error("word column should be UNIQUE")
	}
	if word := colByName["word"]; !word.NotNull {
		t.Error("word column should be NOT NULL")
	}
	if dc := colByName["drill_count"]; !dc.NotNull {
		t.Error("drill_count column should be NOT NULL")
	}
}

// --- containsKatakana ---

func TestContainsKatakana(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"カタカナ", true},    // pure katakana
		{"食べるカメラ", true}, // mixed kanji + katakana
		{"ひらがな", false},   // hiragana only
		{"漢字", false},       // kanji only
		{"", false},           // empty
		{"ABCdef123", false},  // ASCII
	}
	for _, tc := range cases {
		got := containsKatakana(tc.input)
		if got != tc.want {
			t.Errorf("containsKatakana(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- wordsExistInDB ---

func TestWordsExistInDB_Empty(t *testing.T) {
	db := testDB(t)
	result, err := wordsExistInDB(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil map for empty input, got %v", result)
	}
}

func TestWordsExistInDB_Mixed(t *testing.T) {
	db := testDB(t)
	insertWord(db, "猫", "", "", "", "", "", "", 1)
	insertWord(db, "犬", "", "", "", "", "", "", 1)

	result, err := wordsExistInDB(db, []string{"猫", "魚"})
	if err != nil {
		t.Fatal(err)
	}
	if !result["猫"] {
		t.Error("猫 should be in result (present in DB)")
	}
	if result["魚"] {
		t.Error("魚 should not be in result (absent from DB)")
	}
}

// --- upsertKanji ---

func TestUpsertKanji_Insert(t *testing.T) {
	db := testDB(t)
	id, err := upsertKanji(db, "食", []string{"eat", "food"})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("expected non-zero ID after insert")
	}
}

func TestUpsertKanji_DuplicateReturnsSameID(t *testing.T) {
	db := testDB(t)
	id1, err := upsertKanji(db, "食", []string{"eat"})
	if err != nil {
		t.Fatal("first upsert:", err)
	}
	id2, err := upsertKanji(db, "食", []string{"eat", "food"})
	if err != nil {
		t.Fatal("second upsert:", err)
	}
	if id1 != id2 {
		t.Errorf("duplicate upsert returned different IDs: %d vs %d", id1, id2)
	}
}

// --- getActivityStats ---

func TestGetActivityStats_Buckets(t *testing.T) {
	db := testDB(t)
	// Insert words via raw SQL to control drill_count, which insertWord always sets to 0.
	// Word 1: already cleared (drill_count >= drill_target, target_reached_at set).
	db.Exec(`INSERT INTO words (word, drill_count, drill_target, target_reached_at) VALUES ('清', 5, 3, datetime('now'))`)
	// Word 2: close to target (drill_target - drill_count = 2, within the <= 4 bucket).
	db.Exec(`INSERT INTO words (word, drill_count, drill_target) VALUES ('近', 2, 4)`)
	// Word 3: mid range (drill_target - drill_count = 6, within the 4 < x <= 8 bucket).
	db.Exec(`INSERT INTO words (word, drill_count, drill_target) VALUES ('中', 1, 7)`)
	// Word 4: far from target (drill_target - drill_count = 10, > 8 bucket).
	db.Exec(`INSERT INTO words (word, drill_count, drill_target) VALUES ('遠', 0, 10)`)

	stats, err := getActivityStats(db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.LexiconSize != 4 {
		t.Errorf("LexiconSize: got %d, want 4", stats.LexiconSize)
	}
	if stats.ActiveWords != 3 {
		t.Errorf("ActiveWords: got %d, want 3", stats.ActiveWords)
	}
	if stats.ClearedLifetime != 1 {
		t.Errorf("ClearedLifetime: got %d, want 1", stats.ClearedLifetime)
	}
	if stats.DrillsCleared != 1 {
		t.Errorf("DrillsCleared: got %d, want 1", stats.DrillsCleared)
	}
	if stats.DrillsClose != 1 {
		t.Errorf("DrillsClose: got %d, want 1", stats.DrillsClose)
	}
	if stats.DrillsMid != 1 {
		t.Errorf("DrillsMid: got %d, want 1", stats.DrillsMid)
	}
	if stats.DrillsFar != 1 {
		t.Errorf("DrillsFar: got %d, want 1", stats.DrillsFar)
	}
}

// --- recordDrillAnswer ---

// insertTestWord is a helper that inserts a word and returns its DB id.
func insertTestWord(t *testing.T, db *sql.DB, word string, target int) int64 {
	t.Helper()
	if err := insertWord(db, word, "", "", "", "", "", "", target); err != nil {
		t.Fatal("insertTestWord:", err)
	}
	var id int64
	db.QueryRow(`SELECT id FROM words WHERE word = ?`, word).Scan(&id)
	return id
}

func TestRecordDrillAnswer_Correct(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "水", 5)
	sessionID, _ := createDrillSession(db)

	if err := recordDrillAnswer(db, sessionID, wordID, true); err != nil {
		t.Fatal(err)
	}

	var drillCount, incorrectCount int
	var lastDrilled *string
	db.QueryRow(`SELECT drill_count, incorrect_count, last_drilled_at FROM words WHERE id = ?`, wordID).
		Scan(&drillCount, &incorrectCount, &lastDrilled)

	if drillCount != 1 {
		t.Errorf("drill_count: got %d, want 1", drillCount)
	}
	if incorrectCount != 0 {
		t.Errorf("incorrect_count: got %d, want 0", incorrectCount)
	}
	if lastDrilled == nil {
		t.Error("last_drilled_at should be set after correct answer")
	}
}

func TestRecordDrillAnswer_Incorrect(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "火", 5)
	sessionID, _ := createDrillSession(db)

	if err := recordDrillAnswer(db, sessionID, wordID, false); err != nil {
		t.Fatal(err)
	}

	var drillCount, incorrectCount int
	var lastDrilled *string
	db.QueryRow(`SELECT drill_count, incorrect_count, last_drilled_at FROM words WHERE id = ?`, wordID).
		Scan(&drillCount, &incorrectCount, &lastDrilled)

	if drillCount != 0 {
		t.Errorf("drill_count: got %d, want 0 (incorrect should not increment)", drillCount)
	}
	if incorrectCount != 1 {
		t.Errorf("incorrect_count: got %d, want 1", incorrectCount)
	}
	if lastDrilled == nil {
		t.Error("last_drilled_at should be set after incorrect answer")
	}
}

func TestRecordDrillAnswer_TargetReachedOnFirstHit(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "木", 1) // target = 1; one correct answer clears it
	sessionID, _ := createDrillSession(db)

	if err := recordDrillAnswer(db, sessionID, wordID, true); err != nil {
		t.Fatal(err)
	}

	var targetReachedAt *string
	db.QueryRow(`SELECT target_reached_at FROM words WHERE id = ?`, wordID).Scan(&targetReachedAt)
	if targetReachedAt == nil {
		t.Error("target_reached_at should be set when drill_count reaches drill_target")
	}
}

func TestRecordDrillAnswer_TargetReachedNotOverwritten(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "土", 1)
	sessionID, _ := createDrillSession(db)

	// First correct answer: reaches target, sets target_reached_at.
	recordDrillAnswer(db, sessionID, wordID, true)
	var first string
	db.QueryRow(`SELECT target_reached_at FROM words WHERE id = ?`, wordID).Scan(&first)

	// Subsequent correct answer: must not overwrite target_reached_at.
	recordDrillAnswer(db, sessionID, wordID, true)
	var second string
	db.QueryRow(`SELECT target_reached_at FROM words WHERE id = ?`, wordID).Scan(&second)

	if first != second {
		t.Errorf("target_reached_at was overwritten: %q → %q", first, second)
	}
}

// --- resetDB ---

func TestResetDB_ClearsData(t *testing.T) {
	db := testDB(t)
	// Use sentinel words unlikely to appear in seed.json.
	insertWord(db, "山", "", "", "", "", "", "", 1)
	insertWord(db, "川", "", "", "", "", "", "", 1)

	if err := resetDB(db); err != nil {
		t.Fatal("resetDB:", err)
	}

	// After reset the DB is re-migrated (and possibly re-seeded from seed.json),
	// but our two test words must no longer be present.
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words WHERE word IN ('山', '川')`).Scan(&count)
	if count != 0 {
		t.Errorf("test words still present after reset: got %d, want 0", count)
	}
}

