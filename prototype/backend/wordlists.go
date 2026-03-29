package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed wordlists
var wordListsFS embed.FS

// wordList holds a loaded word list. Slug is derived from the filename and
// used as the URL key; Name and Words come from the JSON file itself.
type wordList struct {
	Slug  string   `json:"-"`
	Name  string   `json:"name"`
	Words []string `json:"words"`
}

// loadedWordLists holds every word list parsed at startup, in directory order.
var loadedWordLists []wordList

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
	log.Printf("Loaded %d word list(s)", len(loadedWordLists))
}

// apiGetWordLists returns the slug, name, total word count, and in-lexicon count for each word list.
func apiGetWordLists(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type item struct {
			Slug      string `json:"slug"`
			Name      string `json:"name"`
			Total     int    `json:"total"`
			InLexicon int    `json:"in_lexicon"`
		}
		items := make([]item, len(loadedWordLists))
		for i, wl := range loadedWordLists {
			inDB, err := wordsInfoInDB(db, wl.Words)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items[i] = item{Slug: wl.Slug, Name: wl.Name, Total: len(wl.Words), InLexicon: len(inDB)}
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

		inDB, err := wordsInfoInDB(db, wl.Words)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		available := make([]string, 0, len(wl.Words))
		for _, word := range wl.Words {
			if _, exists := inDB[word]; !exists {
				available = append(available, word)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"words":      available,
			"total":      len(wl.Words),
			"in_lexicon": len(wl.Words) - len(available),
		})
	}
}
