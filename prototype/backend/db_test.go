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
	want := map[string]bool{
		"words":           true,
		"drill_sessions":  true,
		"drill_answers":   true,
		"kanji":           true,
		"user_settings":   true,
		"stories":         true,
		"story_sentences": true,
	}
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
	if len(tables) != 7 {
		t.Errorf("expected 7 tables, got %d: %v", len(tables), tables)
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
	db.QueryRow(`SELECT word FROM words WHERE id = ?`, id).Scan(&word)
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
	db.Exec(`INSERT INTO words (word, created_at) VALUES ('無', datetime('now'))`)

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
	db.Exec(`UPDATE words SET created_at = datetime('now', '-1 day') WHERE word = '古'`)

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
	db.QueryRow(`SELECT word FROM words`).Scan(&remaining)
	if remaining != "秋" {
		t.Errorf("remaining word: got %q, want 秋", remaining)
	}
}

// --- stories ---

func TestInsertStory_AndListStories(t *testing.T) {
	db := testDB(t)
	title := "足立美術館の庭園"
	audioPath := "audio/stories/morning.ogg"
	en1 := "Good morning."
	en2 := "Let's do our best today too."
	ts1 := int64(0)
	ts2 := int64(700)
	ts3 := int64(2450)

	id, err := insertStory(db, title, &audioPath, []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "おはよう", BaseWord: "おはよう", AudioTimestampMs: &ts1},
				{DisplayWord: "。", BaseWord: "。", AudioTimestampMs: &ts2},
			},
			EnglishText:      &en1,
			IsParagraphStart: true,
		},
		{
			Words: []storyWordInput{
				{DisplayWord: "今日", BaseWord: "今日", AudioTimestampMs: &ts3},
				{DisplayWord: "も", BaseWord: "も"},
				{DisplayWord: "頑張ろう", BaseWord: "頑張る"},
				{DisplayWord: "。", BaseWord: "。"},
			},
			EnglishText: &en2,
		},
	})
	if err != nil {
		t.Fatalf("insertStory: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero story id")
	}

	stories, err := listStories(db)
	if err != nil {
		t.Fatalf("listStories: %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("expected 1 story, got %d", len(stories))
	}
	story := stories[0]
	if story.Title != title {
		t.Errorf("title: got %q, want %q", story.Title, title)
	}
	if story.AudioPath == nil || *story.AudioPath != audioPath {
		t.Errorf("audio_path: got %+v", story.AudioPath)
	}
	if len(story.Sentences) != 2 {
		t.Fatalf("expected 2 sentences, got %d", len(story.Sentences))
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
	if story.Sentences[0].Words[0].AudioTimestampMs == nil || *story.Sentences[0].Words[0].AudioTimestampMs != ts1 {
		t.Errorf("word audio timestamp: got %+v", story.Sentences[0].Words[0].AudioTimestampMs)
	}
	if story.Sentences[0].EnglishText == nil || *story.Sentences[0].EnglishText != en1 {
		t.Errorf("sentence 1 english: got %+v", story.Sentences[0].EnglishText)
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

	_, err := insertStory(db, "No sentences", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing sentences")
	}
	if !strings.Contains(err.Error(), "at least one sentence") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RejectsNegativeAudioTimestamp(t *testing.T) {
	db := testDB(t)
	badTs := int64(-1)

	_, err := insertStory(db, "Bad timestamps", nil, []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "文", BaseWord: "文", AudioTimestampMs: &badTs},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for negative audio timestamp")
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RequiresAtLeastOneWordPerSentence(t *testing.T) {
	db := testDB(t)

	_, err := insertStory(db, "Missing words", nil, []storySentenceInput{{}})
	if err == nil {
		t.Fatal("expected error for missing words")
	}
	if !strings.Contains(err.Error(), "at least one word") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStory_RequiresTitle(t *testing.T) {
	db := testDB(t)

	_, err := insertStory(db, "", nil, []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}},
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("unexpected error: %v", err)
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
	db.Exec(`INSERT INTO words (word, reading, meaning, drill_count, drill_target, created_at, target_reached_at)
		VALUES ('星', 'ほし', 'star', 3, 3, '2024-01-15 10:00:00', '2024-01-20 10:00:00')`)
	var wordID int64
	db.QueryRow(`SELECT id FROM words WHERE word = '星'`).Scan(&wordID)

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

func TestGetActivityCalendar_KnewFalseWhenAnyIncorrect(t *testing.T) {
	db := testDB(t)
	db.Exec(`INSERT INTO words (word, created_at) VALUES ('風', datetime('now'))`)
	var wordID int64
	db.QueryRow(`SELECT id FROM words WHERE word = '風'`).Scan(&wordID)

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
	db.Exec(`INSERT INTO words (word, created_at) VALUES ('雲', '2024-03-01 00:00:00')`)

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

// --- updateWordAudioFlags / getWordAudioInfo ---

func TestUpdateWordAudioFlags_BothFlags(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "雨", 1)

	if err := updateWordAudioFlags(db, id, true, true); err != nil {
		t.Fatal(err)
	}
	var hasWord, hasSentence int
	db.QueryRow(`SELECT has_word_audio, has_sentence_audio FROM words WHERE id = ?`, id).
		Scan(&hasWord, &hasSentence)
	if hasWord != 1 {
		t.Errorf("has_word_audio: got %d, want 1", hasWord)
	}
	if hasSentence != 1 {
		t.Errorf("has_sentence_audio: got %d, want 1", hasSentence)
	}
}

func TestUpdateWordAudioFlags_ClearsFlags(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "風", 1)
	// Set both flags then clear them.
	updateWordAudioFlags(db, id, true, true)
	if err := updateWordAudioFlags(db, id, false, false); err != nil {
		t.Fatal(err)
	}
	var hasWord, hasSentence int
	db.QueryRow(`SELECT has_word_audio, has_sentence_audio FROM words WHERE id = ?`, id).
		Scan(&hasWord, &hasSentence)
	if hasWord != 0 || hasSentence != 0 {
		t.Errorf("flags should be 0 after clearing, got word=%d sentence=%d", hasWord, hasSentence)
	}
}

func TestGetWordAudioInfo_NotFound(t *testing.T) {
	db := testDB(t)
	word, exJp, err := getWordAudioInfo(db, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if word != "" || exJp != "" {
		t.Errorf("expected empty strings for missing word, got word=%q exJp=%q", word, exJp)
	}
}

func TestGetWordAudioInfo_ReturnsWordAndExample(t *testing.T) {
	db := testDB(t)
	insertWord(db, "猫", "ねこ", "noun", "cat", "猫がいる。", "There is a cat.", "", 1)
	var id int64
	db.QueryRow(`SELECT id FROM words WHERE word = '猫'`).Scan(&id)

	word, exJp, err := getWordAudioInfo(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if word != "猫" {
		t.Errorf("word: got %q, want 猫", word)
	}
	if exJp != "猫がいる。" {
		t.Errorf("exJp: got %q, want 猫がいる。", exJp)
	}
}

func TestGetWordAudioInfo_NullExampleDefaultsToEmpty(t *testing.T) {
	db := testDB(t)
	id := insertTestWord(t, db, "犬", 1) // insertTestWord uses empty strings for all optional fields

	_, exJp, err := getWordAudioInfo(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if exJp != "" {
		t.Errorf("exJp: got %q, want empty string when example_jp is NULL", exJp)
	}
}

func TestGetActivityCalendar_HistoryStartIsContainingSunday(t *testing.T) {
	db := testDB(t)
	// 2024-01-17 is a Wednesday; the containing Sunday is 2024-01-14.
	db.Exec(`INSERT INTO words (word, created_at) VALUES ('川', '2024-01-17 00:00:00')`)

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	if cal.HistoryStart != "2024-01-14" {
		t.Errorf("HistoryStart: got %q, want 2024-01-14 (Sunday of week containing 2024-01-17)", cal.HistoryStart)
	}
}
