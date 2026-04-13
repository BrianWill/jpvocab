package main

import (
	"database/sql"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"jpvocab/internal/dictlookup"

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
			kanji_json TEXT NOT NULL
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
			lookup_text, word, reading, part_of_speech, meaning, glosses_json, kanji_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "食べる", "食べる", "たべる", "ichidan-verb", "to eat", `["to eat"]`, `[{"character":"食","reading":"た","readings":["く","た","は","ショク","ジキ"],"meanings":["eat","food"]}]`); err != nil {
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
	if !errors.Is(err, dictlookup.ErrMissingWordLookupTable) {
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
	if !errors.Is(err, dictlookup.ErrMissingKanjiLookupTable) {
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

func TestCanonicalPartOfSpeechFromDict(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		want string
	}{
		{name: "godan", tags: []string{"Godan verb with `ku' ending"}, want: "godan-verb"},
		{name: "godan code", tags: []string{"v5k"}, want: "godan-verb"},
		{name: "ichidan", tags: []string{"Ichidan verb"}, want: "ichidan-verb"},
		{name: "ichidan code", tags: []string{"v1"}, want: "ichidan-verb"},
		{name: "i adjective", tags: []string{"I-adjective (keiyoushi)"}, want: "i-adjective"},
		{name: "i adjective code", tags: []string{"adj-i"}, want: "i-adjective"},
		{name: "na adjective", tags: []string{"Adjectival nouns or quasi-adjectives (keiyodoshi)"}, want: "na-adjective"},
		{name: "na adjective code", tags: []string{"adj-na"}, want: "na-adjective"},
		{name: "adverb", tags: []string{"Adverb (fukushi)"}, want: "adverb"},
		{name: "adverb code", tags: []string{"adv"}, want: "adverb"},
		{name: "noun", tags: []string{"Noun (common) (futsuumeishi)"}, want: "noun"},
		{name: "noun code", tags: []string{"n"}, want: "noun"},
		{name: "fallback other", tags: []string{"Expression (phrase, clause, etc.)"}, want: "other"},
		{name: "empty", tags: nil, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalPartOfSpeechFromDict(tc.tags); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func runGoCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}
