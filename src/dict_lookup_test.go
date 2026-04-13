package main

import (
	"database/sql"
	"sync"
	"testing"

	"jpvocab/internal/dictlookup"

	_ "modernc.org/sqlite"
)

func testDictDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE jmdict_words (id TEXT PRIMARY KEY)`,
		`CREATE TABLE jmdict_kanji (word_id TEXT NOT NULL, text TEXT NOT NULL, common INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE jmdict_kana (word_id TEXT NOT NULL, text TEXT NOT NULL, common INTEGER NOT NULL DEFAULT 0, applies_to_kanji TEXT NOT NULL DEFAULT '["*"]')`,
		`CREATE TABLE jmdict_senses (id INTEGER PRIMARY KEY AUTOINCREMENT, word_id TEXT NOT NULL, pos TEXT NOT NULL DEFAULT '[]', applies_to_kanji TEXT NOT NULL DEFAULT '["*"]', applies_to_kana TEXT NOT NULL DEFAULT '["*"]')`,
		`CREATE TABLE jmdict_glosses (sense_id INTEGER NOT NULL, text TEXT NOT NULL)`,
		`CREATE TABLE kanjidic_readings (literal TEXT NOT NULL, type TEXT NOT NULL, value TEXT NOT NULL)`,
		`CREATE TABLE kanjidic_meanings (literal TEXT NOT NULL, value TEXT NOT NULL)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("schema exec failed: %v\nsql=%s", err, stmt)
		}
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func insertDictSense(t *testing.T, db *sql.DB, wordID, posJSON, appliesKanji, appliesKana string, glosses ...string) int64 {
	t.Helper()
	res, err := db.Exec(`
		INSERT INTO jmdict_senses (word_id, pos, applies_to_kanji, applies_to_kana)
		VALUES (?, ?, ?, ?)
	`, wordID, posJSON, appliesKanji, appliesKana)
	if err != nil {
		t.Fatal(err)
	}
	senseID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for _, gloss := range glosses {
		if _, err := db.Exec(`INSERT INTO jmdict_glosses (sense_id, text) VALUES (?, ?)`, senseID, gloss); err != nil {
			t.Fatal(err)
		}
	}
	return senseID
}

func TestLookupDictionaryWordInDB_KanjiWord(t *testing.T) {
	db := testDictDB(t)

	if _, err := db.Exec(`INSERT INTO jmdict_words (id) VALUES ('w1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kanji (word_id, text, common) VALUES ('w1', '食べる', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kana (word_id, text, common, applies_to_kanji) VALUES ('w1', 'たべる', 1, '["食べる"]')`); err != nil {
		t.Fatal(err)
	}
	insertDictSense(t, db, "w1", `["Ichidan verb","Transitive verb"]`, `["食べる"]`, `["たべる"]`, "to eat", "to consume")

	if _, err := db.Exec(`INSERT INTO kanjidic_readings (literal, type, value) VALUES ('食', 'ja_kun', 'た.べる')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kanjidic_meanings (literal, value) VALUES ('食', 'eat'), ('食', 'food')`); err != nil {
		t.Fatal(err)
	}

	info, err := lookupDictionaryWordInDB(db, "食べる")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected dictionary result, got nil")
	}
	if info.Reading != "たべる" {
		t.Fatalf("reading: got %q, want %q", info.Reading, "たべる")
	}
	if info.PartOfSpeech != "ichidan-verb" {
		t.Fatalf("part_of_speech: got %q, want %q", info.PartOfSpeech, "ichidan-verb")
	}
	if info.Meaning != "to eat; to consume" {
		t.Fatalf("meaning: got %q", info.Meaning)
	}
	if len(info.Kanji) != 1 {
		t.Fatalf("kanji count: got %d, want 1", len(info.Kanji))
	}
	if info.Kanji[0].Character != "食" {
		t.Fatalf("kanji character: got %q", info.Kanji[0].Character)
	}
	if info.Kanji[0].Reading != "た" {
		t.Fatalf("kanji reading: got %q, want %q", info.Kanji[0].Reading, "た")
	}
}

func TestLookupDictionaryWordInDB_KanaWord(t *testing.T) {
	db := testDictDB(t)

	if _, err := db.Exec(`INSERT INTO jmdict_words (id) VALUES ('w1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kana (word_id, text, common, applies_to_kanji) VALUES ('w1', 'きれい', 1, '["*"]')`); err != nil {
		t.Fatal(err)
	}
	insertDictSense(t, db, "w1", `["Na-adjective (keiyodoshi)"]`, `["*"]`, `["きれい"]`, "pretty", "clean")

	info, err := lookupDictionaryWordInDB(db, "きれい")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected dictionary result, got nil")
	}
	if info.Reading != "きれい" {
		t.Fatalf("reading: got %q, want %q", info.Reading, "きれい")
	}
	if info.PartOfSpeech != "na-adjective" {
		t.Fatalf("part_of_speech: got %q, want %q", info.PartOfSpeech, "na-adjective")
	}
	if len(info.Kanji) != 0 {
		t.Fatalf("kanji count: got %d, want 0", len(info.Kanji))
	}
}

func TestLookupDictionaryWordInDB_InfersCompoundKanjiPrefixes(t *testing.T) {
	db := testDictDB(t)

	if _, err := db.Exec(`INSERT INTO jmdict_words (id) VALUES ('w1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kanji (word_id, text, common) VALUES ('w1', '日本語', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kana (word_id, text, common, applies_to_kanji) VALUES ('w1', 'にほんご', 1, '["日本語"]')`); err != nil {
		t.Fatal(err)
	}
	insertDictSense(t, db, "w1", `["Noun"]`, `["日本語"]`, `["にほんご"]`, "Japanese language")

	if _, err := db.Exec(`
		INSERT INTO kanjidic_readings (literal, type, value) VALUES
			('日', 'ja_on', 'ニチ'),
			('本', 'ja_on', 'ホン'),
			('語', 'ja_on', 'ゴ')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO kanjidic_meanings (literal, value) VALUES
			('日', 'day'),
			('本', 'book'),
			('語', 'language')
	`); err != nil {
		t.Fatal(err)
	}

	info, err := lookupDictionaryWordInDB(db, "日本語")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected dictionary result, got nil")
	}
	if len(info.Kanji) != 3 {
		t.Fatalf("kanji count: got %d, want 3", len(info.Kanji))
	}
	if info.PartOfSpeech != "noun" {
		t.Fatalf("part_of_speech: got %q, want %q", info.PartOfSpeech, "noun")
	}

	got := []string{info.Kanji[0].Reading, info.Kanji[1].Reading, info.Kanji[2].Reading}
	want := []string{"ニ", "ホン", "ゴ"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kanji readings: got %v, want %v", got, want)
		}
	}
}

func TestLookupDictionaryWordInDB_UsesFlattenedLookupTable(t *testing.T) {
	db := testDictDB(t)

	if _, err := db.Exec(`INSERT INTO jmdict_words (id) VALUES ('w1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kanji (word_id, text, common) VALUES ('w1', '食べる', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jmdict_kana (word_id, text, common, applies_to_kanji) VALUES ('w1', 'たべる', 1, '["食べる"]')`); err != nil {
		t.Fatal(err)
	}
	insertDictSense(t, db, "w1", `["Ichidan verb"]`, `["食べる"]`, `["たべる"]`, "to eat")
	if _, err := db.Exec(`INSERT INTO kanjidic_readings (literal, type, value) VALUES ('食', 'ja_kun', 'た.べる')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kanjidic_meanings (literal, value) VALUES ('食', 'eat')`); err != nil {
		t.Fatal(err)
	}

	if err := dictlookup.RebuildLookupTable(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE jmdict_glosses`); err != nil {
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
	if info.Meaning != "to eat" {
		t.Fatalf("meaning: got %q, want %q", info.Meaning, "to eat")
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
