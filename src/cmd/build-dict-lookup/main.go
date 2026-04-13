package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"jpvocab/internal/dictlookup"

	_ "modernc.org/sqlite"
)

func main() {
	paths := candidatePaths()
	dictPath := ""
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			dictPath = p
			break
		}
	}
	if dictPath == "" {
		log.Fatalf("could not find jdict.db (checked %v)", paths)
	}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dictPath))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA temp_store = MEMORY;
	`); err != nil {
		log.Fatal(err)
	}

	log.Printf("rebuilding dict_word_lookup in %s", dictPath)
	if err := dictlookup.RebuildLookupTable(db); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		log.Fatal(err)
	}
	log.Printf("done")
}

func candidatePaths() []string {
	return []string{
		filepath.Join("dict", "jdict.db"),
		filepath.Join("..", "..", "dict", "jdict.db"),
		filepath.Join("..", "dict", "jdict.db"),
	}
}
