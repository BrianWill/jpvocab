package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed wordlists
var wordListsFS embed.FS

// wordList holds a loaded word list. Slug is derived from the filename and
// used as the URL key; Name and Entries come from the JSON file itself.
type wordListEntry struct {
	Word              string `json:"word"`
	Reading           string `json:"reading"`
	PartOfSpeech      string `json:"part_of_speech"`
	Meaning           string `json:"meaning"`
	ExampleJP         string `json:"example_jp"`
	ExampleEN         string `json:"example_en"`
	SuggestedImageURL string `json:"suggested_image_url,omitempty"`
}

type wordList struct {
	Slug    string          `json:"-"`
	Name    string          `json:"name"`
	Entries []wordListEntry `json:"entries"`
}

// loadedWordLists holds every word list parsed at startup, in directory order.
var loadedWordLists []wordList
var wordListEntryByWord map[string]wordListEntry

// 'init' is special Go function name that is invoked automatically during package init before main()
func init() {
	entries, err := wordListsFS.ReadDir("wordlists")
	if err != nil {
		log.Fatal("wordlists: read dir:", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := wordListsFS.ReadFile("wordlists/" + entry.Name())
		if err != nil {
			log.Fatal("wordlists: read file:", entry.Name(), ":", err)
		}
		var wl wordList
		if err := json.Unmarshal(data, &wl); err != nil {
			log.Fatal("wordlists: parse:", entry.Name(), ":", err)
		}
		wl.Slug = strings.TrimSuffix(entry.Name(), ".json")
		loadedWordLists = append(loadedWordLists, wl)
	}
	wordListEntryByWord = make(map[string]wordListEntry)
	for _, wl := range loadedWordLists {
		for _, entry := range wl.Entries {
			entry.Word = strings.TrimSpace(entry.Word)
			if entry.Word == "" {
				log.Fatalf("wordlists: %s contains an entry with an empty word", wl.Slug)
			}
			if existing, ok := wordListEntryByWord[entry.Word]; ok {
				wordListEntryByWord[entry.Word] = mergeWordListEntry(existing, entry, entry.Word, wl.Slug)
				continue
			}
			wordListEntryByWord[entry.Word] = entry
		}
	}
	log.Printf("Loaded %d word list(s)", len(loadedWordLists))
}

// apiGetWordLists returns the slug, name, total word count, and tracked count for each word list.
func apiGetWordLists(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type item struct {
			Slug    string `json:"slug"`
			Name    string `json:"name"`
			Total   int    `json:"total"`
			Tracked int    `json:"tracked"`
		}
		items := make([]item, len(loadedWordLists))
		for i, wl := range loadedWordLists {
			inDB, err := wordsInfoInDB(db, wordListWords(wl.Entries))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items[i] = item{Slug: wl.Slug, Name: wl.Name, Total: len(wl.Entries), Tracked: len(inDB)}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

// apiGetWordListWords returns all words from the named list that are not
// already present in the lexicon. The frontend is responsible for sampling.
func apiGetWordListWords(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		var wl *wordList
		for i := range loadedWordLists {
			if loadedWordLists[i].Slug == slug {
				wl = &loadedWordLists[i]
				break
			}
		}
		if wl == nil {
			http.Error(w, "word list not found", http.StatusNotFound)
			return
		}

		inDB, err := wordsInfoInDB(db, wordListWords(wl.Entries))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		availableEntries := make([]wordListEntry, 0, len(wl.Entries))
		availableWords := make([]string, 0, len(wl.Entries))
		for _, entry := range wl.Entries {
			if _, exists := inDB[entry.Word]; !exists {
				availableEntries = append(availableEntries, entry)
				availableWords = append(availableWords, entry.Word)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"words":   availableWords,
			"entries": availableEntries,
			"total":   len(wl.Entries),
			"tracked": len(wl.Entries) - len(availableEntries),
		})
	}
}

func wordListWords(entries []wordListEntry) []string {
	words := make([]string, 0, len(entries))
	for _, entry := range entries {
		words = append(words, entry.Word)
	}
	return words
}

func wordListLookup(word string) (wordListEntry, bool) {
	entry, ok := wordListEntryByWord[word]
	return entry, ok
}

func populateLexiconFromWordListsIfTrackedEmpty(db *sql.DB) (int, bool, error) {
	trackedCount, err := trackedWordCount(db)
	if err != nil {
		return 0, false, err
	}
	if trackedCount > 0 {
		return 0, true, nil
	}

	drillCfg, err := getDrillSettings(db)
	if err != nil {
		return 0, false, err
	}
	newWordTarget := drillCfg.NewWordTarget
	if newWordTarget <= 0 {
		newWordTarget = 8
	}

	inserted := 0
	seen := make(map[string]struct{}, len(wordListEntryByWord))
	for _, wl := range loadedWordLists {
		for _, entry := range wl.Entries {
			word := strings.TrimSpace(entry.Word)
			if word == "" {
				continue
			}
			if _, exists := seen[word]; exists {
				continue
			}
			seen[word] = struct{}{}
			if _, err := insertWordReturningID(db, word, entry.Reading, entry.PartOfSpeech, entry.Meaning,
				entry.ExampleJP, entry.ExampleEN, "", newWordTarget); err != nil {
				return inserted, false, fmt.Errorf("insert %q from word list %q: %w", word, wl.Slug, err)
			}
			inserted++
		}
	}

	return inserted, false, nil
}

func mergeWordListEntry(existing, incoming wordListEntry, word, source string) wordListEntry {
	merged := existing
	mergeField := func(field string, dst *string, src string) {
		src = strings.TrimSpace(src)
		if *dst == "" {
			*dst = src
			return
		}
		if src != "" && *dst != src {
			log.Printf("wordlists: conflicting %s for %q in %s; keeping first value %q over %q", field, word, source, *dst, src)
		}
	}
	mergeField("reading", &merged.Reading, incoming.Reading)
	mergeField("part_of_speech", &merged.PartOfSpeech, incoming.PartOfSpeech)
	mergeField("meaning", &merged.Meaning, incoming.Meaning)
	mergeField("example_jp", &merged.ExampleJP, incoming.ExampleJP)
	mergeField("example_en", &merged.ExampleEN, incoming.ExampleEN)
	mergeField("suggested_image_url", &merged.SuggestedImageURL, incoming.SuggestedImageURL)
	return merged
}
