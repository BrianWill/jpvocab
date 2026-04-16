package main

import (
	"database/sql"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func testRuntimeDictDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE dict_word_lookup (
			lookup_text TEXT PRIMARY KEY,
			word TEXT NOT NULL,
			reading TEXT NOT NULL,
			part_of_speech TEXT NOT NULL,
			meaning TEXT NOT NULL,
			glosses_json TEXT NOT NULL,
			kanji_json TEXT NOT NULL,
			pitch_accent INTEGER
		)`,
		`CREATE TABLE dict_kanji_lookup (
			literal TEXT PRIMARY KEY,
			meanings_json TEXT NOT NULL,
			readings_json TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("schema exec failed: %v\nsql=%s", err, stmt)
		}
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestLookupDictionaryWordInDB_ReadsFlattenedTable(t *testing.T) {
	db := testRuntimeDictDB(t)
	if _, err := db.Exec(`
		INSERT INTO dict_word_lookup (
			lookup_text, word, reading, part_of_speech, meaning, glosses_json, kanji_json, pitch_accent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "食べる", "食べる", "たべる", "ichidan-verb", "to eat", `["to eat"]`, `[{"character":"食","reading":"た","readings":["く","た","は","ショク","ジキ"],"meanings":["eat","food"]}]`, 2); err != nil {
		t.Fatal(err)
	}

	dictLookupCache = sync.Map{}
	info, err := lookupDictionaryWordInDB(db, "食べる")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected dictionary result, got nil")
	}
	if info.Reading != "たべる" || info.PartOfSpeech != "ichidan-verb" || info.Meaning != "to eat" {
		t.Fatalf("unexpected word info: %+v", info)
	}
	if info.PitchAccent == nil || *info.PitchAccent != 2 {
		t.Fatalf("pitch accent: got %v, want 2", info.PitchAccent)
	}
	if len(info.Glosses) != 1 || info.Glosses[0] != "to eat" {
		t.Fatalf("glosses: got %v", info.Glosses)
	}
	if len(info.Kanji) != 1 || info.Kanji[0].Reading != "た" {
		t.Fatalf("kanji: got %+v", info.Kanji)
	}
}

func TestLookupDictionaryWordInDB_MissingTableReturnsClearError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dictLookupCache = sync.Map{}
	_, err = lookupDictionaryWordInDB(db, "食べる")
	if !errors.Is(err, errMissingWordLookupTable) {
		t.Fatalf("expected missing lookup table error, got %v", err)
	}
}

func TestLookupDictionaryKanjiInDB_ReadsFlattenedTable(t *testing.T) {
	db := testRuntimeDictDB(t)
	if _, err := db.Exec(`
		INSERT INTO dict_kanji_lookup (literal, meanings_json, readings_json)
		VALUES (?, ?, ?)
	`, "食", `["eat","food"]`, `["く","た","は","ショク","ジキ"]`); err != nil {
		t.Fatal(err)
	}

	info, err := lookupDictionaryKanjiInDB(db, "食")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected kanji info, got nil")
	}
	if info.Character != "食" {
		t.Fatalf("character: got %q", info.Character)
	}
	if len(info.Meanings) != 2 || len(info.Readings) != 5 {
		t.Fatalf("kanji info: %+v", info)
	}
}

func TestLookupDictionaryKanjiInDB_MissingTableReturnsClearError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = lookupDictionaryKanjiInDB(db, "食")
	if !errors.Is(err, errMissingKanjiLookupTable) {
		t.Fatalf("expected missing kanji lookup table error, got %v", err)
	}
}

func TestBuildDictLookupCommandFailsClearly(t *testing.T) {
	out, err := runGoCommand(t, "run", "./cmd/build-dict-lookup")
	if err == nil {
		t.Fatal("expected build-dict-lookup command to fail")
	}
	if !strings.Contains(out, "not supported from the slim runtime jdict.db artifact") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func runGoCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}
