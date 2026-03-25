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
	want := map[string]bool{"words": true, "drill_sessions": true, "drill_answers": true}
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
	if len(tables) != 3 {
		t.Errorf("expected 3 tables, got %d: %v", len(tables), tables)
	}
}

// --- insertWord ---

func TestInsertWord_Basic(t *testing.T) {
	db := testDB(t)
	err := insertWord(db, "食べる", "たべる", "verb", "to eat", "私は寿司を食べる。", "I eat sushi.", 3)
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
	if err := insertWord(db, "猫", "", "", "", "", "", 0); err != nil {
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
	if err := insertWord(db, "犬", "", "", "", "", "", 1); err != nil {
		t.Fatal("first insert:", err)
	}
	err := insertWord(db, "犬", "", "", "", "", "", 1)
	if err == nil {
		t.Fatal("expected UNIQUE constraint error, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertWord_EmptyOptionalFields(t *testing.T) {
	db := testDB(t)
	// Insert with only the word; all other fields empty.
	if err := insertWord(db, "空", "", "", "", "", "", 1); err != nil {
		t.Fatal(err)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words WHERE word = '空'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// --- listTables ---

func TestListTables(t *testing.T) {
	db := testDB(t)
	tables, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 3 {
		t.Fatalf("expected 3 tables, got %d: %v", len(tables), tables)
	}
	// listTables returns names ordered by name
	for _, name := range tables {
		switch name {
		case "drill_answers", "drill_sessions", "words":
			// expected
		default:
			t.Errorf("unexpected table: %q", name)
		}
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
	if err := insertWord(db, "水", "みず", "noun", "water", "水を飲む。", "Drink water.", 1); err != nil {
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
		if err := insertWord(db, w, "", "", "", "", "", 1); err != nil {
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

func TestListTableInfos_RowCounts(t *testing.T) {
	db := testDB(t)
	insertWord(db, "本", "", "", "", "", "", 1)
	insertWord(db, "紙", "", "", "", "", "", 1)

	infos, err := listTableInfos(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if info.Name == "words" && info.Rows != 2 {
			t.Errorf("words: expected 2 rows, got %d", info.Rows)
		}
	}
}

// --- resetDB ---

func TestResetDB_ClearsData(t *testing.T) {
	db := testDB(t)
	// Use sentinel words unlikely to appear in seed.json.
	insertWord(db, "山", "", "", "", "", "", 1)
	insertWord(db, "川", "", "", "", "", "", 1)

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

func TestResetDB_TablesStillExist(t *testing.T) {
	db := testDB(t)
	if err := resetDB(db); err != nil {
		t.Fatal(err)
	}
	tables, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 3 {
		t.Errorf("expected 3 tables after reset, got %d: %v", len(tables), tables)
	}
}
