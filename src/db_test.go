package main

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

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

func parseDBDateTime(t *testing.T, value string) time.Time {
	t.Helper()
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	t.Fatalf("unable to parse datetime %q", value)
	return time.Time{}
}

// --- migrate ---

func TestMigrate_CreatesAllTables(t *testing.T) {
	db := testDB(t)
	tables, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Error("migration produced no tables")
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	db := testDB(t) // already migrated once by testDB
	before, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	// Running migrate a second time must not error or create duplicate tables.
	migrate(db)
	after, err := listTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("second migrate changed table count: %d → %d\nbefore: %v\nafter:  %v",
			len(before), len(after), before, after)
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
	err = db.QueryRow(`SELECT reading, meaning, drill_target FROM words WHERE base_word = ?`, "食べる").
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
	db.QueryRow(`SELECT drill_target FROM words WHERE base_word = ?`, "猫").Scan(&target)
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

func TestInsertWordReturningID_PromotedTrackedZeroReturnsPromotedRowID(t *testing.T) {
	db := testDB(t)

	res, err := db.Exec(`
		INSERT INTO words (base_word, reading, part_of_speech, meaning, example_jp, example_en, drill_target, tracked)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, "黄色", "きいろ", "noun", "yellow", "黄色の花が咲いている。", "Yellow flowers are blooming.", 3)
	if err != nil {
		t.Fatal(err)
	}
	expectedID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	gotID, err := insertWordReturningID(db, "黄色", "", "", "", "", "", "", 8)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != expectedID {
		t.Fatalf("returned id: got %d, want %d", gotID, expectedID)
	}

	var tracked int
	var target int
	if err := db.QueryRow(`SELECT tracked, drill_target FROM words WHERE id = ?`, expectedID).Scan(&tracked, &target); err != nil {
		t.Fatal(err)
	}
	if tracked != 1 {
		t.Fatalf("tracked: got %d, want 1", tracked)
	}
	if target != 8 {
		t.Fatalf("drill_target: got %d, want 8", target)
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
		{"stories", true},
		{"story_sentences", true},
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

	// Find the "base_word" column index and check its value.
	wordIdx := -1
	for i, c := range cols {
		if c == "base_word" {
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
	// The "base_word" column is index 1 (after id).
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
	if word := colByName["base_word"]; !word.Unique {
		t.Error("base_word column should be UNIQUE")
	}
	if word := colByName["base_word"]; !word.NotNull {
		t.Error("base_word column should be NOT NULL")
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
		{"カタカナ", true},       // pure katakana
		{"食べるカメラ", true},     // mixed kanji + katakana
		{"ひらがな", false},      // hiragana only
		{"漢字", false},        // kanji only
		{"", false},          // empty
		{"ABCdef123", false}, // ASCII
	}
	for _, tc := range cases {
		got := containsKatakana(tc.input)
		if got != tc.want {
			t.Errorf("containsKatakana(%q) = %v, want %v", tc.input, got, tc.want)
		}
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
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target, target_reached_at) VALUES ('清', 5, 3, datetime('now'))`)
	// Word 2: close to target (drill_target - drill_count = 2, within the <= 4 bucket).
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target) VALUES ('近', 2, 4)`)
	// Word 3: mid range (drill_target - drill_count = 6, within the 4 < x <= 8 bucket).
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target) VALUES ('中', 1, 7)`)
	// Word 4: far from target (drill_target - drill_count = 10, > 8 bucket).
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target) VALUES ('遠', 0, 10)`)

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

func TestGetActivityStats_ExcludesUntracked(t *testing.T) {
	db := testDB(t)
	// One manually-added word (tracked=1, the default).
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target) VALUES ('空', 0, 3)`)
	// One story-sourced word (tracked=0) — must be invisible to stats.
	db.Exec(`INSERT INTO words (base_word, drill_count, drill_target, tracked) VALUES ('海', 0, 3, 0)`)

	stats, err := getActivityStats(db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.LexiconSize != 1 {
		t.Errorf("LexiconSize: got %d, want 1 (tracked=0 word must be excluded)", stats.LexiconSize)
	}
	if stats.ActiveWords != 1 {
		t.Errorf("ActiveWords: got %d, want 1", stats.ActiveWords)
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
	db.QueryRow(`SELECT id FROM words WHERE base_word = ?`, word).Scan(&id)
	return id
}

func TestRecordDrillAnswer_Correct(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "水", 5)
	sessionID, _ := createDrillSession(db, drillSessionState{})

	if err := recordDrillAnswer(db, sessionID, wordID, true, drillSessionState{}); err != nil {
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
	sessionID, _ := createDrillSession(db, drillSessionState{})

	if err := recordDrillAnswer(db, sessionID, wordID, false, drillSessionState{}); err != nil {
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
	sessionID, _ := createDrillSession(db, drillSessionState{})

	if err := recordDrillAnswer(db, sessionID, wordID, true, drillSessionState{}); err != nil {
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
	sessionID, _ := createDrillSession(db, drillSessionState{})

	// First correct answer: reaches target, sets target_reached_at.
	recordDrillAnswer(db, sessionID, wordID, true, drillSessionState{})
	var first string
	db.QueryRow(`SELECT target_reached_at FROM words WHERE id = ?`, wordID).Scan(&first)

	// Subsequent correct answer: must not overwrite target_reached_at.
	recordDrillAnswer(db, sessionID, wordID, true, drillSessionState{})
	var second string
	db.QueryRow(`SELECT target_reached_at FROM words WHERE id = ?`, wordID).Scan(&second)

	if first != second {
		t.Errorf("target_reached_at was overwritten: %q → %q", first, second)
	}
}

func TestCreateDrillSession_StoresStateAndClosesPrevious(t *testing.T) {
	db := testDB(t)
	firstID, err := createDrillSession(db, drillSessionState{Round: 1})
	if err != nil {
		t.Fatal(err)
	}
	secondState := drillSessionState{
		Round:         2,
		ActiveFilters: []string{"verbs"},
		Pool:          []wordJSON{{ID: 1, Word: "taberu"}},
	}
	secondID, err := createDrillSession(db, secondState)
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstID {
		t.Fatal("expected a new drill session ID")
	}

	var firstCompleted *string
	if err := db.QueryRow(`SELECT completed_at FROM drill_sessions WHERE id = ?`, firstID).Scan(&firstCompleted); err != nil {
		t.Fatal(err)
	}
	if firstCompleted == nil {
		t.Fatal("previous active session should be marked completed")
	}

	current, err := getCurrentDrillSession(db)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != secondID {
		t.Fatalf("current session: got %+v, want id %d", current, secondID)
	}
	if current.State.Round != 2 {
		t.Errorf("round: got %d, want 2", current.State.Round)
	}
	if len(current.State.Pool) != 1 || current.State.Pool[0].Word != "taberu" {
		t.Errorf("pool: got %+v", current.State.Pool)
	}
}

func TestCreateDrillSession_NormalisesNilSlicesInStoredState(t *testing.T) {
	db := testDB(t)
	sessionID, err := createDrillSession(db, drillSessionState{Round: 3})
	if err != nil {
		t.Fatal(err)
	}

	var stateJSON string
	if err := db.QueryRow(`SELECT state_json FROM drill_sessions WHERE id = ?`, sessionID).Scan(&stateJSON); err != nil {
		t.Fatal(err)
	}
	want := `{"poolSize":0,"roundSize":0,"round":3,"doneCount":0,"activeFilters":[],"pool":[],"redo":[],"remaining":[],"sidebarItems":[]}`
	if stateJSON != want {
		t.Errorf("state_json: got %s, want %s", stateJSON, want)
	}
}

func TestRecordDrillAnswer_UpdatesSessionStateAndCompletion(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "æµ·", 2)
	sessionID, err := createDrillSession(db, drillSessionState{Round: 1})
	if err != nil {
		t.Fatal(err)
	}

	state := drillSessionState{
		Round:     1,
		DoneCount: 1,
		SidebarItems: []drillSidebarItem{
			{Word: wordJSON{ID: wordID, Word: "umi"}, Status: "known"},
		},
		LastAnswered: &drillLastAnswered{
			Word: wordJSON{ID: wordID, Word: "umi"},
			Knew: true,
		},
	}
	if err := recordDrillAnswer(db, sessionID, wordID, true, state); err != nil {
		t.Fatal(err)
	}

	var stateJSON string
	var completedAt *string
	if err := db.QueryRow(`SELECT state_json, completed_at FROM drill_sessions WHERE id = ?`, sessionID).Scan(&stateJSON, &completedAt); err != nil {
		t.Fatal(err)
	}
	if stateJSON != "{}" {
		t.Errorf("state_json: got %s, want {}", stateJSON)
	}
	if completedAt == nil {
		t.Fatal("completed_at should be set for completed drill state")
	}

	current, err := getCurrentDrillSession(db)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("expected no active drill session, got %+v", current)
	}
}

func TestGetCurrentDrillSession_NormalisesSparseStoredJSON(t *testing.T) {
	db := testDB(t)
	res, err := db.Exec(`INSERT INTO drill_sessions (state_json) VALUES ('{"round":4}')`)
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	current, err := getCurrentDrillSession(db)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != sessionID {
		t.Fatalf("current session: got %+v, want id %d", current, sessionID)
	}
	if current.State.Round != 4 {
		t.Errorf("round: got %d, want 4", current.State.Round)
	}
	if current.State.ActiveFilters == nil || len(current.State.ActiveFilters) != 0 {
		t.Errorf("ActiveFilters: got %#v, want empty slice", current.State.ActiveFilters)
	}
	if current.State.Pool == nil || len(current.State.Pool) != 0 {
		t.Errorf("Pool: got %#v, want empty slice", current.State.Pool)
	}
	if current.State.Redo == nil || len(current.State.Redo) != 0 {
		t.Errorf("Redo: got %#v, want empty slice", current.State.Redo)
	}
	if current.State.Remaining == nil || len(current.State.Remaining) != 0 {
		t.Errorf("Remaining: got %#v, want empty slice", current.State.Remaining)
	}
	if current.State.SidebarItems == nil || len(current.State.SidebarItems) != 0 {
		t.Errorf("SidebarItems: got %#v, want empty slice", current.State.SidebarItems)
	}
}

// --- resetDB ---

func TestResetDB_ClearsData(t *testing.T) {
	db := testDB(t)
	// Use sentinel words guaranteed not to appear in the seed data.
	insertWord(db, "__test_reset_alpha__", "", "", "", "", "", "", 1)
	insertWord(db, "__test_reset_beta__", "", "", "", "", "", "", 1)

	if err := resetDB(db); err != nil {
		t.Fatal("resetDB:", err)
	}

	// After reset the DB is re-migrated (and possibly re-seeded from seed files).
	// Our manually-added words (tracked=1) must no longer be present; seed
	// story tokenisation may re-add some base words with tracked=0, which is fine.
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words WHERE base_word IN ('__test_reset_alpha__', '__test_reset_beta__') AND tracked = 1`).Scan(&count)
	if count != 0 {
		t.Errorf("test words still present after reset: got %d, want 0", count)
	}
}

// --- insertWordReturningID ---

func TestInsertWordReturningID(t *testing.T) {
	db := testDB(t)
	id, err := insertWordReturningID(db, "鳥", "とり", "noun", "bird", "", "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
	var word string
	db.QueryRow(`SELECT base_word FROM words WHERE id = ?`, id).Scan(&word)
	if word != "鳥" {
		t.Errorf("word: got %q, want 鳥", word)
	}
}

// --- updateWordFill ---

func TestUpdateWordFill(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "山", 1)

	err := updateWordFill(db, id, "やま", nil, "noun", "mountain", "山が高い。", "The mountain is tall.", `[{"id":1,"reading":"やま"}]`)
	if err != nil {
		t.Fatal(err)
	}

	var reading, meaning, kanjiData string
	db.QueryRow(`SELECT reading, meaning, kanji_data FROM words WHERE id = ?`, id).
		Scan(&reading, &meaning, &kanjiData)
	if reading != "やま" {
		t.Errorf("reading: got %q, want やま", reading)
	}
	if meaning != "mountain" {
		t.Errorf("meaning: got %q, want mountain", meaning)
	}
	if kanjiData != `[{"id":1,"reading":"やま"}]` {
		t.Errorf("kanji_data: got %q", kanjiData)
	}
}

func TestUpdateWordFill_EmptyKanjiDataDefaultsToArray(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "石", 1)

	if err := updateWordFill(db, id, "いし", nil, "noun", "stone", "", "", ""); err != nil {
		t.Fatal(err)
	}
	var kanjiData string
	db.QueryRow(`SELECT kanji_data FROM words WHERE id = ?`, id).Scan(&kanjiData)
	if kanjiData != "[]" {
		t.Errorf("kanji_data: got %q, want []", kanjiData)
	}
}

// --- wordsInfoInDB ---

func TestWordsInfoInDB_Empty(t *testing.T) {
	db := testDB(t)
	result, err := wordsInfoInDB(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestWordsInfoInDB_ReturnsCorrectFields(t *testing.T) {
	db := testDB(t)
	insertWord(db, "空", "そら", "noun", "sky", "空が青い。", "The sky is blue.", "", 4)

	result, err := wordsInfoInDB(db, []string{"空", "notexist"})
	if err != nil {
		t.Fatal(err)
	}
	info, ok := result["空"]
	if !ok {
		t.Fatal("空 not found in result")
	}
	if info.Reading != "そら" {
		t.Errorf("Reading: got %q, want そら", info.Reading)
	}
	if info.DrillTarget != 4 {
		t.Errorf("DrillTarget: got %d, want 4", info.DrillTarget)
	}
	if _, ok := result["notexist"]; ok {
		t.Error("notexist should not be in result")
	}
}

func TestPopulateLexiconFromWordListsIfTrackedEmpty_InsertsEntries(t *testing.T) {
	db := testDB(t)

	inserted, skipped, err := populateLexiconFromWordListsIfTrackedEmpty(db)
	if err != nil {
		t.Fatal(err)
	}
	if skipped {
		t.Fatal("expected populate to run")
	}
	if inserted == 0 {
		t.Fatal("expected word-list entries to be inserted")
	}

	count, err := trackedWordCount(db)
	if err != nil {
		t.Fatal(err)
	}
	if count != inserted {
		t.Fatalf("tracked count: got %d, want %d", count, inserted)
	}
}

func TestPopulateLexiconFromWordListsIfTrackedEmpty_SkipsNonEmptyTrackedLexicon(t *testing.T) {
	db := testDB(t)
	if err := insertWord(db, "既存", "きそん", "noun", "existing", "", "", "", 3); err != nil {
		t.Fatal(err)
	}

	inserted, skipped, err := populateLexiconFromWordListsIfTrackedEmpty(db)
	if err != nil {
		t.Fatal(err)
	}
	if !skipped {
		t.Fatal("expected populate to skip")
	}
	if inserted != 0 {
		t.Fatalf("inserted: got %d, want 0", inserted)
	}
}

// --- updateWordTarget ---

func TestUpdateWordTarget(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "森", 1)

	if err := updateWordTarget(db, id, 7); err != nil {
		t.Fatal(err)
	}
	var target int
	db.QueryRow(`SELECT drill_target FROM words WHERE id = ?`, id).Scan(&target)
	if target != 7 {
		t.Errorf("drill_target: got %d, want 7", target)
	}
}

// --- drill settings ---

func TestGetDrillSettings_Defaults(t *testing.T) {
	db := testDB(t)

	settings, err := getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 100 {
		t.Errorf("MaxWords: got %d, want 100", settings.MaxWords)
	}
	if settings.RoundSize != 10 {
		t.Errorf("RoundSize: got %d, want 10", settings.RoundSize)
	}
	wantTypes := []string{"katakana", "verbs", "nouns", "other"}
	if len(settings.WordTypes) != len(wantTypes) {
		t.Fatalf("WordTypes length: got %d, want %d (%v)", len(settings.WordTypes), len(wantTypes), settings.WordTypes)
	}
	for i := range wantTypes {
		if settings.WordTypes[i] != wantTypes[i] {
			t.Errorf("WordTypes[%d]: got %q, want %q", i, settings.WordTypes[i], wantTypes[i])
		}
	}
}

func TestGetDrillSettings_IgnoresInvalidStoredValues(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO user_settings (key, value) VALUES
		('drill_max_words', '"bad"'),
		('drill_round_size', '0'),
		('drill_word_types', 'not-json')`); err != nil {
		t.Fatal(err)
	}

	settings, err := getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 100 {
		t.Errorf("MaxWords: got %d, want default 100", settings.MaxWords)
	}
	if settings.RoundSize != 10 {
		t.Errorf("RoundSize: got %d, want default 10", settings.RoundSize)
	}
	wantTypes := []string{"katakana", "verbs", "nouns", "other"}
	for i := range wantTypes {
		if settings.WordTypes[i] != wantTypes[i] {
			t.Errorf("WordTypes[%d]: got %q, want %q", i, settings.WordTypes[i], wantTypes[i])
		}
	}
}

func TestPutDrillSettings_RoundTripsAndDeletesInvalidMaxWords(t *testing.T) {
	db := testDB(t)
	if err := putDrillSettings(db, drillSettings{
		MaxWords:  25,
		RoundSize: 7,
		WordTypes: []string{"verbs", "nouns"},
	}); err != nil {
		t.Fatal(err)
	}

	settings, err := getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 25 || settings.RoundSize != 7 {
		t.Errorf("settings: got %+v, want MaxWords=25 RoundSize=7", settings)
	}
	if len(settings.WordTypes) != 2 || settings.WordTypes[0] != "verbs" || settings.WordTypes[1] != "nouns" {
		t.Errorf("WordTypes: got %v", settings.WordTypes)
	}

	if err := putDrillSettings(db, drillSettings{
		MaxWords:  0,
		RoundSize: 9,
		WordTypes: []string{"other"},
	}); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE key = 'drill_max_words'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected drill_max_words row to be deleted, got count %d", count)
	}

	settings, err = getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 100 {
		t.Errorf("MaxWords after delete: got %d, want default 100", settings.MaxWords)
	}
	if settings.RoundSize != 9 {
		t.Errorf("RoundSize after overwrite: got %d, want 9", settings.RoundSize)
	}
	if len(settings.WordTypes) != 1 || settings.WordTypes[0] != "other" {
		t.Errorf("WordTypes after overwrite: got %v", settings.WordTypes)
	}
}

// --- listKanji ---

func TestListKanji_EmptySliceWhenNone(t *testing.T) {
	db := testDB(t)
	kanji, err := listKanji(db)
	if err != nil {
		t.Fatal(err)
	}
	if kanji == nil {
		t.Error("listKanji should return [] not nil for empty table")
	}
}

func TestListKanji_ReturnsInserted(t *testing.T) {
	db := testDB(t)
	upsertKanji(db, "日", []string{"sun", "day"})
	upsertKanji(db, "本", []string{"origin", "book"})

	kanji, err := listKanji(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(kanji) != 2 {
		t.Fatalf("expected 2 kanji, got %d", len(kanji))
	}
	if kanji[0].Character != "日" {
		t.Errorf("first character: got %q, want 日", kanji[0].Character)
	}
	if len(kanji[0].Meanings) != 2 {
		t.Errorf("日 meanings: got %v, want [sun day]", kanji[0].Meanings)
	}
}

// --- listWords ---

func TestListWords_KanjiDataDefaultsToEmptySlice(t *testing.T) {
	db := testDB(t)
	insertWord(db, "月", "つき", "noun", "moon", "", "", "", 1)

	words, err := listWords(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d", len(words))
	}
	if words[0].KanjiData == nil {
		t.Error("KanjiData should be [] not nil")
	}
}

func TestListWords_NullableColumnsDefaultToEmpty(t *testing.T) {
	db := testDB(t)
	// Insert with only word + created_at so all nullable TEXT columns are NULL.
	db.Exec(`INSERT INTO words (base_word, created_at) VALUES ('無', datetime('now'))`)

	words, err := listWords(db)
	if err != nil {
		t.Fatalf("listWords with NULL columns: %v", err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d", len(words))
	}
	w := words[0]
	if w.Reading != "" || w.Type != "" || w.Meaning != "" || w.ExampleJp != "" || w.ExampleEn != "" {
		t.Errorf("nullable fields should default to empty string, got %+v", w)
	}
}

func TestListWords_OrderNewestFirst(t *testing.T) {
	db := testDB(t)
	insertWord(db, "古", "", "", "", "", "", "", 1)
	insertWord(db, "新", "", "", "", "", "", "", 1)
	db.Exec(`UPDATE words SET created_at = datetime('now', '-1 day') WHERE base_word = '古'`)

	words, err := listWords(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	if words[0].Word != "新" {
		t.Errorf("first word: got %q, want 新 (newest first)", words[0].Word)
	}
}

func TestListWords_EnrichesKanjiDataFromKanjiTable(t *testing.T) {
	db := testDB(t)
	kanjiID, err := upsertKanji(db, "猫", []string{"cat", "feline"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO words (base_word, reading, kanji_data, tracked) VALUES (?, ?, ?, 1)`, "猫", "ねこ", fmt.Sprintf(`[{"id":%d,"reading":"ねこ"}]`, kanjiID)); err != nil {
		t.Fatal(err)
	}

	words, err := listWords(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d", len(words))
	}
	if len(words[0].KanjiData) != 1 {
		t.Fatalf("expected 1 kanji entry, got %d", len(words[0].KanjiData))
	}
	if words[0].KanjiData[0].Character != "猫" {
		t.Errorf("character: got %q, want 猫", words[0].KanjiData[0].Character)
	}
	if len(words[0].KanjiData[0].Meanings) != 2 {
		t.Errorf("meanings: got %v, want two meanings", words[0].KanjiData[0].Meanings)
	}
}

// --- updateWord ---

func TestUpdateWord(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "雨", 3)

	if err := updateWord(db, id, "あめ", "noun", "rain", "雨が降る。", "It rains.", `[{"id":1,"reading":"あめ"}]`, 5); err != nil {
		t.Fatal(err)
	}

	var reading, pos, meaning, exJp, exEn string
	var target int
	db.QueryRow(`SELECT reading, part_of_speech, meaning, example_jp, example_en, drill_target FROM words WHERE id = ?`, id).
		Scan(&reading, &pos, &meaning, &exJp, &exEn, &target)

	if reading != "あめ" {
		t.Errorf("reading: got %q, want あめ", reading)
	}
	if pos != "noun" {
		t.Errorf("part_of_speech: got %q, want noun", pos)
	}
	if meaning != "rain" {
		t.Errorf("meaning: got %q, want rain", meaning)
	}
	if exJp != "雨が降る。" {
		t.Errorf("example_jp: got %q", exJp)
	}
	if target != 5 {
		t.Errorf("drill_target: got %d, want 5", target)
	}
}

// --- deleteWordByID / deleteWordsByName ---

func TestDeleteWordByID(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "花", 1)

	if err := deleteWordByID(db, id); err != nil {
		t.Fatal(err)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words WHERE id = ?`, id).Scan(&count)
	if count != 0 {
		t.Errorf("word should be deleted, got count %d", count)
	}
}

func TestDeleteWordsByName(t *testing.T) {
	db := testDB(t)
	insertWord(db, "春", "", "", "", "", "", "", 1)
	insertWord(db, "夏", "", "", "", "", "", "", 1)
	insertWord(db, "秋", "", "", "", "", "", "", 1)

	if err := deleteWordsByName(db, []string{"春", "夏"}); err != nil {
		t.Fatal(err)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 word remaining, got %d", count)
	}
	var remaining string
	db.QueryRow(`SELECT base_word FROM words`).Scan(&remaining)
	if remaining != "秋" {
		t.Errorf("remaining word: got %q, want 秋", remaining)
	}
}

// --- stories ---

func TestInsertStory_AndGetStoryByID(t *testing.T) {
	db := testDB(t)
	title := "足立美術館の庭園"
	jp1 := "おはよう。"
	jp2 := "今日も頑張ろう。"
	en1 := "Good morning."
	en2 := "Let's do our best today too."

	id, err := insertStory(db, title, []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "おはよう", BaseWord: "おはよう"},
				{DisplayWord: "。", BaseWord: "。"},
			},
			JPText:           &jp1,
			ENText:           &en1,
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
		{
			Words: []storyWordInput{
				{DisplayWord: "今日", BaseWord: "今日"},
				{DisplayWord: "も", BaseWord: "も"},
				{DisplayWord: "頑張ろう", BaseWord: "頑張る"},
				{DisplayWord: "。", BaseWord: "。"},
			},
			JPText:   &jp2,
			ENText:   &en2,
			OrigLang: "jp",
		},
	})
	if err != nil {
		t.Fatalf("insertStory: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero story id")
	}

	story, err := getStoryByID(db, id)
	if err != nil {
		t.Fatalf("getStoryByID: %v", err)
	}
	if story == nil {
		t.Fatal("expected story")
	}
	if story.Title != title {
		t.Errorf("title: got %q, want %q", story.Title, title)
	}
	if story.CreatedAt == "" {
		t.Fatal("expected created_at timestamp")
	}
	parseDBDateTime(t, story.CreatedAt)
	if story.SentenceCount != 2 {
		t.Fatalf("sentence count: got %d, want 2", story.SentenceCount)
	}
	if story.LexiconWordCount != 2 {
		t.Fatalf("lexicon word count: got %d, want 2", story.LexiconWordCount)
	}
	if len(story.Sentences) != 2 {
		t.Fatalf("expected 2 sentences, got %d", len(story.Sentences))
	}
	if story.Sentences[0].ChunkPosition != 1 || story.Sentences[1].ChunkPosition != 1 {
		t.Errorf("chunk positions: got %d, %d; want 1, 1", story.Sentences[0].ChunkPosition, story.Sentences[1].ChunkPosition)
	}
	if story.Sentences[0].Position != 1 || story.Sentences[1].Position != 2 {
		t.Errorf("positions: got %+v", story.Sentences)
	}
	if len(story.Sentences[0].Words) != 2 || len(story.Sentences[1].Words) != 4 {
		t.Errorf("word counts: got %+v", story.Sentences)
	}
	if story.Sentences[1].Words[2].BaseWord != "頑張る" {
		t.Errorf("base word: got %q, want 頑張る", story.Sentences[1].Words[2].BaseWord)
	}
	if story.Sentences[0].ENText == nil || *story.Sentences[0].ENText != en1 {
		t.Errorf("sentence 1 english: got %+v", story.Sentences[0].ENText)
	}
	if story.Sentences[0].JPText == nil || *story.Sentences[0].JPText != jp1 {
		t.Errorf("sentence 1 japanese: got %+v", story.Sentences[0].JPText)
	}
	if story.Sentences[0].OrigLang != "jp" {
		t.Errorf("sentence 1 orig_lang: got %q, want jp", story.Sentences[0].OrigLang)
	}
	if !story.Sentences[0].IsParagraphStart {
		t.Error("sentence 1 should be marked as paragraph start")
	}
	if story.Sentences[1].IsParagraphStart {
		t.Error("sentence 2 should not be marked as paragraph start")
	}
}

func TestInsertStory_RequiresAtLeastOneSentence(t *testing.T) {
	db := testDB(t)

	_, err := insertStory(db, "No sentences", nil)
	if err == nil {
		t.Fatal("expected error for missing sentences")
	}
	if !strings.Contains(err.Error(), "at least one sentence") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RequiresAtLeastOneWordPerSentence(t *testing.T) {
	db := testDB(t)

	text := "庭園。"
	_, err := insertStory(db, "Missing words", []storySentenceInput{{JPText: &text, OrigLang: "jp"}})
	if err == nil {
		t.Fatal("expected error for missing words")
	}
	if !strings.Contains(err.Error(), "at least one word") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RequiresTitle(t *testing.T) {
	db := testDB(t)

	_, err := insertStory(db, "", []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}, OrigLang: "jp"},
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RollsBackActivityEventOnFailure(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`
		CREATE TRIGGER fail_words_insert
		BEFORE INSERT ON words
		BEGIN
			SELECT RAISE(ABORT, 'forced word insert failure');
		END;
	`); err != nil {
		t.Fatal(err)
	}

	_, err := insertStory(db, "Broken Story", []storySentenceInput{
		{
			Words:            []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}},
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
	})
	if err == nil {
		t.Fatal("expected insertStory to fail")
	}
	if !strings.Contains(err.Error(), "forced word insert failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE event_type = ? AND summary = ?`, activityEventStoryCreated, "Broken Story").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no rolled-back story activity event, got %d", count)
	}
}

func TestStoryNotedWords_PersistOnStory(t *testing.T) {
	db := testDB(t)
	id, err := insertStory(db, "Garden Story", []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "庭園", BaseWord: "庭園"},
				{DisplayWord: "へ", BaseWord: "へ"},
				{DisplayWord: "行く", BaseWord: "行く"},
			},
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := addStoryNotedWord(db, id, storyNotedWordJSON{DisplayWord: "行く", BaseWord: "行く"}); err != nil {
		t.Fatalf("addStoryNotedWord: %v", err)
	}
	if err := addStoryNotedWord(db, id, storyNotedWordJSON{DisplayWord: "行く", BaseWord: "行く"}); err != nil {
		t.Fatalf("duplicate addStoryNotedWord: %v", err)
	}
	if err := addStoryNotedWord(db, id, storyNotedWordJSON{DisplayWord: "庭園", BaseWord: "庭園"}); err != nil {
		t.Fatalf("second addStoryNotedWord: %v", err)
	}

	story, err := getStoryByID(db, id)
	if err != nil {
		t.Fatalf("getStoryByID: %v", err)
	}
	if story == nil {
		t.Fatal("expected story")
	}
	if len(story.NotedWords) != 2 {
		t.Fatalf("expected 2 noted words, got %d", len(story.NotedWords))
	}
	if story.NotedWords[0].BaseWord != "行く" {
		t.Errorf("first noted word base: got %q, want %q", story.NotedWords[0].BaseWord, "行く")
	}
	if story.NotedWords[1].DisplayWord != "庭園" {
		t.Errorf("second noted word display: got %q, want %q", story.NotedWords[1].DisplayWord, "庭園")
	}

	if err := removeStoryNotedWord(db, id, "行く"); err != nil {
		t.Fatalf("removeStoryNotedWord: %v", err)
	}
	story, err = getStoryByID(db, id)
	if err != nil {
		t.Fatalf("getStoryByID after remove: %v", err)
	}
	if len(story.NotedWords) != 1 || story.NotedWords[0].BaseWord != "庭園" {
		t.Fatalf("unexpected noted words after remove: %+v", story.NotedWords)
	}
}

func TestBuildStorySentenceWords_TokenizesDisplayAndBaseForms(t *testing.T) {
	words := buildStorySentenceWords("庭園は庭のことですね。")
	if len(words) == 0 {
		t.Fatal("expected tokenized story words")
	}
	if words[0].DisplayWord != "庭園" || words[0].BaseWord != "庭園" {
		t.Errorf("first token: got %+v", words[0])
	}
	foundPeriod := false
	for _, word := range words {
		if word.DisplayWord == "。" {
			foundPeriod = true
		}
	}
	if !foundPeriod {
		t.Error("expected punctuation token in story words")
	}
}

func TestBuildStorySentencesFromText_ClassifiesJapaneseAndEnglish(t *testing.T) {
	sentences := buildStorySentencesFromText("今日はmeetingがあります。\n\nThis is a test.")
	if len(sentences) != 2 {
		t.Fatalf("expected 2 sentences, got %d", len(sentences))
	}
	if sentences[0].OrigLang != "jp" {
		t.Fatalf("sentence 1 orig_lang: got %q, want jp", sentences[0].OrigLang)
	}
	if sentences[0].JPText == nil || *sentences[0].JPText != "今日はmeetingがあります。" {
		t.Fatalf("sentence 1 jp text: got %+v", sentences[0].JPText)
	}
	if sentences[1].OrigLang != "en" {
		t.Fatalf("sentence 2 orig_lang: got %q, want en", sentences[1].OrigLang)
	}
	if sentences[1].ENText == nil || *sentences[1].ENText != "This is a test." {
		t.Fatalf("sentence 2 en text: got %+v", sentences[1].ENText)
	}
	if len(sentences[1].Words) != 0 {
		t.Fatalf("expected english sentence to have no japanese word tokens, got %d", len(sentences[1].Words))
	}
}

func TestStoryLexiconWordCount_FiltersNonLexiconTokens(t *testing.T) {
	got := storyLexiconWordCount([]storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "これ", BaseWord: "これ"},
				{DisplayWord: "は", BaseWord: "は"},
				{DisplayWord: "三", BaseWord: "三"},
				{DisplayWord: "匹", BaseWord: "匹"},
				{DisplayWord: "の", BaseWord: "の"},
				{DisplayWord: "猫", BaseWord: "猫"},
				{DisplayWord: "です", BaseWord: "です"},
				{DisplayWord: "。", BaseWord: "。"},
			},
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
		{
			Words: []storyWordInput{
				{DisplayWord: "静かな", BaseWord: "静か"},
				{DisplayWord: "公園", BaseWord: "公園"},
				{DisplayWord: "です", BaseWord: "です"},
				{DisplayWord: "。", BaseWord: "。"},
			},
			OrigLang: "jp",
		},
	})
	if got != 3 {
		t.Fatalf("storyLexiconWordCount: got %d, want 3", got)
	}
}

func TestBuildStoryChunks_SplitsAfterMinimumChars(t *testing.T) {
	longWord := strings.Repeat("あ", 100)
	sentences := []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: longWord + "。", BaseWord: longWord + "。"}}, OrigLang: "jp", IsParagraphStart: true},
		{Words: []storyWordInput{{DisplayWord: longWord + "。", BaseWord: longWord + "。"}}, OrigLang: "jp"},
		{Words: []storyWordInput{{DisplayWord: "短いです。", BaseWord: "短いです。"}}, OrigLang: "jp"},
	}

	chunks := buildStoryChunks(sentences)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0].Sentences) != 2 {
		t.Fatalf("expected first chunk to hold 2 sentences, got %d", len(chunks[0].Sentences))
	}
	if len(chunks[1].Sentences) != 1 {
		t.Fatalf("expected second chunk to hold 1 sentence, got %d", len(chunks[1].Sentences))
	}
}

func TestGetStoryByID_NotFound(t *testing.T) {
	db := testDB(t)
	story, err := getStoryByID(db, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if story != nil {
		t.Errorf("expected nil story, got %+v", story)
	}
}

func TestDeleteWordsByName_Empty(t *testing.T) {
	db := testDB(t)
	if err := deleteWordsByName(db, nil); err != nil {
		t.Fatal("deleteWordsByName(nil) should be a no-op, got:", err)
	}
}

// --- calendarWeekSunday ---

func TestCalendarWeekSunday(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2024-01-14", "2024-01-14"}, // already a Sunday
		{"2024-01-15", "2024-01-14"}, // Monday
		{"2024-01-17", "2024-01-14"}, // Wednesday
		{"2024-01-20", "2024-01-14"}, // Saturday
		{"2024-01-21", "2024-01-21"}, // next Sunday → itself
		{"bad-date", "bad-date"},     // invalid input returned as-is
	}
	for _, tc := range cases {
		got := calendarWeekSunday(tc.input)
		if got != tc.want {
			t.Errorf("calendarWeekSunday(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- getActivityCalendar ---

func TestGetActivityCalendar_DrilledAddedCleared(t *testing.T) {
	db := testDB(t)

	// Word added on the 15th, drilled on the 18th, cleared on the 20th.
	db.Exec(`INSERT INTO words (base_word, reading, meaning, drill_count, drill_target, created_at, target_reached_at)
		VALUES ('星', 'ほし', 'star', 3, 3, '2024-01-15 10:00:00', '2024-01-20 10:00:00')`)
	var wordID int64
	db.QueryRow(`SELECT id FROM words WHERE base_word = '星'`).Scan(&wordID)

	db.Exec(`INSERT INTO drill_sessions (started_at) VALUES ('2024-01-18 10:00:00')`)
	var sessionID int64
	db.QueryRow(`SELECT MAX(id) FROM drill_sessions`).Scan(&sessionID)
	db.Exec(`INSERT INTO drill_answers (session_id, word_id, correct, answered_at) VALUES (?, ?, 1, '2024-01-18 10:00:00')`, sessionID, wordID)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}

	added := cal.Days["2024-01-15"]
	if len(added.Added) != 1 || added.Added[0].Word != "星" {
		t.Errorf("added: expected 星 on 2024-01-15, got %v", added.Added)
	}

	drilled := cal.Days["2024-01-18"]
	if len(drilled.Drilled) != 1 || drilled.Drilled[0].Word != "星" {
		t.Errorf("drilled: expected 星 on 2024-01-18, got %v", drilled.Drilled)
	}
	if drilled.Drilled[0].Knew == nil || !*drilled.Drilled[0].Knew {
		t.Error("Knew should be true for a correct answer")
	}

	cleared := cal.Days["2024-01-20"]
	if len(cleared.Cleared) != 1 || cleared.Cleared[0].Word != "星" {
		t.Errorf("cleared: expected 星 on 2024-01-20, got %v", cleared.Cleared)
	}
}

func TestGetActivityCalendar_IncludesStoryEvents(t *testing.T) {
	db := testDB(t)
	storyID := int64(7)
	db.Exec(`
		INSERT INTO activity_events (event_type, entity_id, summary, created_at)
		VALUES (?, ?, ?, ?)
	`, activityEventStoryCreated, storyID, "Rainy Day", "2024-05-02 09:00:00")

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	day, ok := cal.Days["2024-05-02"]
	if !ok {
		t.Fatal("expected calendar entry for 2024-05-02")
	}
	if len(day.Stories) != 1 {
		t.Fatalf("Stories: got %d entries, want 1", len(day.Stories))
	}
	if day.Stories[0].StoryID != storyID {
		t.Errorf("Stories[0].StoryID: got %d, want %d", day.Stories[0].StoryID, storyID)
	}
	if day.Stories[0].Title != "Rainy Day" {
		t.Errorf("Stories[0].Title: got %q, want %q", day.Stories[0].Title, "Rainy Day")
	}
}

func TestGetActivityCalendar_IncludesTutorEvents(t *testing.T) {
	db := testDB(t)
	db.Exec(`
		INSERT INTO activity_events (event_type, summary, meta_json, created_at)
		VALUES (?, '', ?, ?)
	`, activityEventTutorUserMessage, `{"mode":"Free Conversation"}`, "2024-05-03 12:00:00")

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	day, ok := cal.Days["2024-05-03"]
	if !ok {
		t.Fatal("expected calendar entry for 2024-05-03")
	}
	if len(day.TutorMessages) != 1 {
		t.Fatalf("TutorMessages: got %d entries, want 1", len(day.TutorMessages))
	}
	if day.TutorMessages[0].Mode != "Free Conversation" {
		t.Errorf("TutorMessages[0].Mode: got %q, want %q", day.TutorMessages[0].Mode, "Free Conversation")
	}
}

func TestGetActivityCalendar_KnewFalseWhenAnyIncorrect(t *testing.T) {
	db := testDB(t)
	db.Exec(`INSERT INTO words (base_word, created_at) VALUES ('風', datetime('now'))`)
	var wordID int64
	db.QueryRow(`SELECT id FROM words WHERE base_word = '風'`).Scan(&wordID)

	db.Exec(`INSERT INTO drill_sessions DEFAULT VALUES`)
	var sessionID int64
	db.QueryRow(`SELECT MAX(id) FROM drill_sessions`).Scan(&sessionID)
	// One correct, one incorrect on the same day — MIN(correct) should be 0.
	db.Exec(`INSERT INTO drill_answers (session_id, word_id, correct) VALUES (?, ?, 1)`, sessionID, wordID)
	db.Exec(`INSERT INTO drill_answers (session_id, word_id, correct) VALUES (?, ?, 0)`, sessionID, wordID)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	today := cal.Today
	if len(cal.Days[today].Drilled) == 0 {
		t.Fatal("expected a drilled entry for today")
	}
	if entry := cal.Days[today].Drilled[0]; entry.Knew == nil || *entry.Knew {
		t.Error("Knew should be false when any answer that day was incorrect")
	}
}

func TestGetActivityCalendar_NonNilSlices(t *testing.T) {
	db := testDB(t)
	// Only an Added entry on this date — Drilled and Cleared must still be [].
	db.Exec(`INSERT INTO words (base_word, created_at) VALUES ('雲', '2024-03-01 00:00:00')`)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	day := cal.Days["2024-03-01"]
	if day.Drilled == nil {
		t.Error("Drilled should be [] not nil")
	}
	if day.Cleared == nil {
		t.Error("Cleared should be [] not nil")
	}
	if day.Stories == nil {
		t.Error("Stories should be [] not nil")
	}
	if day.TutorMessages == nil {
		t.Error("TutorMessages should be [] not nil")
	}
}

// --- getWordImageInfo / updateWordImagePath ---

func TestGetWordImageInfo_NotFound(t *testing.T) {
	db := testDB(t)
	info, err := getWordImageInfo(db, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Errorf("expected nil for missing word, got %+v", info)
	}
}

func TestGetWordImageInfo_ReturnsWordAndPath(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "星", 1)

	info, err := getWordImageInfo(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Word != "星" {
		t.Errorf("Word: got %q, want 星", info.Word)
	}
	if info.ImagePath != nil {
		t.Errorf("ImagePath should be nil before any image is set, got %v", info.ImagePath)
	}
}

func TestUpdateWordImagePath_RoundTrip(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "山", 1)

	if err := updateWordImagePath(db, id, "images/山.jpg"); err != nil {
		t.Fatal(err)
	}
	info, err := getWordImageInfo(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.ImagePath == nil {
		t.Fatal("expected non-nil ImagePath after update")
	}
	if *info.ImagePath != "images/山.jpg" {
		t.Errorf("ImagePath: got %q, want images/山.jpg", *info.ImagePath)
	}
}

func TestGetActivityCalendar_HistoryStartIsContainingSunday(t *testing.T) {
	db := testDB(t)
	// 2024-01-17 is a Wednesday; the containing Sunday is 2024-01-14.
	db.Exec(`INSERT INTO words (base_word, created_at) VALUES ('川', '2024-01-17 00:00:00')`)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	if cal.HistoryStart != "2024-01-14" {
		t.Errorf("HistoryStart: got %q, want 2024-01-14 (Sunday of week containing 2024-01-17)", cal.HistoryStart)
	}
}

func TestGetActivityCalendar_ExcludesUntracked(t *testing.T) {
	db := testDB(t)
	// One manually-added word on 2024-05-01.
	db.Exec(`INSERT INTO words (base_word, created_at) VALUES ('星', '2024-05-01 00:00:00')`)
	// One story-sourced word on the same date — must not appear in Added.
	db.Exec(`INSERT INTO words (base_word, created_at, tracked) VALUES ('月', '2024-05-01 00:00:00', 0)`)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	day, ok := cal.Days["2024-05-01"]
	if !ok {
		t.Fatal("expected calendar entry for 2024-05-01")
	}
	if len(day.Added) != 1 {
		t.Errorf("Added: got %d entries, want 1 (tracked=0 word must be excluded)", len(day.Added))
	}
	if day.Added[0].Word != "星" {
		t.Errorf("Added[0].Word: got %q, want 星", day.Added[0].Word)
	}
}

// --- token usage ---

func TestInsertTokenUsage_WritesRow(t *testing.T) {
	db := testDB(t)
	insertTokenUsage(db, "openai", "gpt-4o", "autofill", 100, 200)

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM token_usage`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 token_usage row, got %d", count)
	}
}

func TestGetTokenUsageSummary_AggregatesByProviderModel(t *testing.T) {
	db := testDB(t)
	insertTokenUsage(db, "openai", "gpt-4o", "autofill", 100, 200)
	insertTokenUsage(db, "openai", "gpt-4o", "autofill", 50, 80)
	insertTokenUsage(db, "anthropic", "claude-3", "translate", 30, 40)

	rows, err := getTokenUsageSummary(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 summary rows (one per provider+model), got %d", len(rows))
	}
	// Most-used first: openai/gpt-4o has 2 calls.
	if rows[0].Provider != "openai" || rows[0].Model != "gpt-4o" {
		t.Errorf("first row: got %+v, want openai/gpt-4o", rows[0])
	}
	if rows[0].TotalCalls != 2 {
		t.Errorf("TotalCalls: got %d, want 2", rows[0].TotalCalls)
	}
	if rows[0].InputTokens != 150 {
		t.Errorf("InputTokens: got %d, want 150", rows[0].InputTokens)
	}
	if rows[0].OutputTokens != 280 {
		t.Errorf("OutputTokens: got %d, want 280", rows[0].OutputTokens)
	}
}

func TestGetTokenUsageLog_OrdersNewestFirstAndRespectsLimit(t *testing.T) {
	db := testDB(t)
	insertTokenUsage(db, "openai", "gpt-4o", "autofill", 10, 20)
	insertTokenUsage(db, "anthropic", "claude-3", "translate", 30, 40)
	insertTokenUsage(db, "google", "gemini", "batch", 50, 60)

	entries, err := getTokenUsageLog(db, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (limit=2), got %d", len(entries))
	}
	// Newest first: google/gemini was inserted last.
	if entries[0].Provider != "google" {
		t.Errorf("first entry provider: got %q, want google", entries[0].Provider)
	}
	if entries[1].Provider != "anthropic" {
		t.Errorf("second entry provider: got %q, want anthropic", entries[1].Provider)
	}
}

func TestGetTokenUsageTotals_SumsAllRows(t *testing.T) {
	db := testDB(t)
	insertTokenUsage(db, "openai", "gpt-4o", "autofill", 100, 200)
	insertTokenUsage(db, "anthropic", "claude-3", "translate", 50, 75)

	calls, input, output, err := getTokenUsageTotals(db)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls: got %d, want 2", calls)
	}
	if input != 150 {
		t.Errorf("input tokens: got %d, want 150", input)
	}
	if output != 275 {
		t.Errorf("output tokens: got %d, want 275", output)
	}
}

func TestGetTokenUsageTotals_EmptyReturnsZeros(t *testing.T) {
	db := testDB(t)
	calls, input, output, err := getTokenUsageTotals(db)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || input != 0 || output != 0 {
		t.Errorf("expected all zeros for empty table, got calls=%d input=%d output=%d", calls, input, output)
	}
}
