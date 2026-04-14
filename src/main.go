package main

import (
	"flag"
	"log"
	"os"
)

const (
	// port is in the dynamic/private range (49152–65535) to avoid conflicts
	// with registered services.
	port   = 49200
	dbPath = "jpvocab.db"
)

func main() {
	seedDBFlag := flag.Bool("seed-db", false, "load seed data, but only when creating a brand-new database file")
	skipLargeStories := flag.Bool("skip-large-seed-stories", false, "skip large stories during DB seeding")
	populateLexiconFromWordLists := flag.Bool("populate-lexicon-from-wordlists", false, "when tracked lexicon is empty, insert all bundled word-list entries for large-list testing")
	flag.Parse()
	if os.Getenv("SKIP_LARGE_SEED_STORIES") == "true" {
		*skipLargeStories = true
	}
	if os.Getenv("POPULATE_LEXICON_FROM_WORDLISTS") == "true" {
		*populateLexiconFromWordLists = true
	}
	skipLargeSeedStories = *skipLargeStories

	initDictAsync() // decompress jdict.db.gz in background; overlaps with tokenizer + DB init

	initTokenizer()

	db := initDB(dbPath, *seedDBFlag)
	defer db.Close()

	if *populateLexiconFromWordLists {
		inserted, skipped, err := populateLexiconFromWordListsIfTrackedEmpty(db)
		switch {
		case err != nil:
			log.Printf("wordlists populate: failed: %v", err)
		case skipped:
			log.Printf("wordlists populate: skipped because tracked lexicon is not empty")
		default:
			log.Printf("wordlists populate: inserted %d word-list entries into the lexicon", inserted)
		}
	}

	log.Printf("jpvocab backend running on http://localhost:%d", port)
	serverInit(db)
}
