package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func serverInit(db *sql.DB) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusFound)
	})

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Route("/admin", func(r chi.Router) {
		r.Get("/", adminIndex(db))
		r.Get("/tables/{table}", adminTable(db))
		r.Post("/reset-db", adminResetDB(db))
		r.Post("/words/batch", adminAddWordsBatch(db))
		r.Post("/words/delete", adminDeleteWords(db))
	})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), r); err != nil {
		log.Fatal(err)
	}
}

type batchWordResult struct {
	Input        string `json:"input"`
	Word         string `json:"word"`
	Added        bool   `json:"added"`
	Reason       string `json:"reason,omitempty"`
	Reading      string `json:"reading,omitempty"`
	PartOfSpeech string `json:"part_of_speech,omitempty"`
	Meaning      string `json:"meaning,omitempty"`
	ExampleJP    string `json:"example_jp,omitempty"`
	ExampleEN    string `json:"example_en,omitempty"`
}

type indexData struct {
	Tables    []tableInfo
	Error     string
	Success   string
	Providers aiProviders
}

func adminIndex(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		infos, err := listTableInfos(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var success string
		if n := r.URL.Query().Get("added"); n != "" {
			success = fmt.Sprintf("Added %s word(s).", n)
		}
		renderTemplate(w, "index", indexData{
			Tables:    infos,
			Error:     r.URL.Query().Get("error"),
			Success:   success,
			Providers: checkAIProviders(),
		})
	}
}


func adminAddWordsBatch(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form (sent by fetch + FormData) before writing response.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		rawWords := parseWordList(r.FormValue("words"))
		if len(rawWords) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		send := func(v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		autoFill := r.FormValue("autofill") == "on"
		aiModel := r.FormValue("ai_model")

		// Phase 1: normalise and record in-batch duplicates; collect unique norms.
		type entry struct {
			input      string
			norm       string
			isBatchDup bool
		}
		seen := make(map[string]bool, len(rawWords))
		entries := make([]entry, 0, len(rawWords))
		var uniqueNorms []string
		for _, raw := range rawWords {
			norm := toBaseForm(raw)
			if seen[norm] {
				entries = append(entries, entry{input: raw, norm: norm, isBatchDup: true})
			} else {
				seen[norm] = true
				entries = append(entries, entry{input: raw, norm: norm})
				uniqueNorms = append(uniqueNorms, norm)
			}
		}

		// Phase 2: single DB query to find which unique words already exist —
		// before making any AI requests.
		select {
		case <-r.Context().Done():
			return
		default:
		}
		existsInDB, err := wordsExistInDB(db, uniqueNorms)
		if err != nil {
			send(map[string]string{"error": err.Error()})
			return
		}

		// Phase 3: process in original input order, streaming each result.
		for _, e := range entries {
			select {
			case <-r.Context().Done():
				return
			default:
			}

			if e.isBatchDup {
				send(batchWordResult{Input: e.input, Word: e.norm, Added: false, Reason: "duplicate in input"})
				continue
			}
			if existsInDB[e.norm] {
				send(batchWordResult{Input: e.input, Word: e.norm, Added: false, Reason: "already in lexicon"})
				continue
			}

			var reading, pos, meaning, exJP, exEN string
			if autoFill {
				filled, err := autoFillWord(e.norm, aiModel)
				if err != nil {
					send(batchWordResult{Input: e.input, Word: e.norm, Added: false, Reason: "auto-fill error: " + err.Error()})
					continue
				}
				reading, pos, meaning, exJP, exEN = filled.Reading, filled.PartOfSpeech, filled.Meaning, filled.ExampleJP, filled.ExampleEN
			}

			if err := insertWord(db, e.norm, reading, pos, meaning, exJP, exEN, defaultDrillTarget); err != nil {
				reason := err.Error()
				if strings.Contains(reason, "UNIQUE constraint failed") {
					reason = "already in lexicon"
				}
				send(batchWordResult{Input: e.input, Word: e.norm, Added: false, Reason: reason,
					Reading: reading, PartOfSpeech: pos, Meaning: meaning, ExampleJP: exJP, ExampleEN: exEN})
				continue
			}

			send(batchWordResult{Input: e.input, Word: e.norm, Added: true,
				Reading: reading, PartOfSpeech: pos, Meaning: meaning, ExampleJP: exJP, ExampleEN: exEN})
		}

		send(map[string]bool{"done": true})
	}
}

func adminDeleteWords(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Words []string `json:"words"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := deleteWordsByName(db, req.Words); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseWordList splits a raw string on commas and whitespace, deduplicates,
// and returns non-empty tokens preserving first-seen order.
func parseWordList(raw string) []string {
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
	})
	seen := make(map[string]bool, len(tokens))
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			words = append(words, t)
		}
	}
	return words
}

func adminResetDB(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := resetDB(db); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func adminTable(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := chi.URLParam(r, "table")
		if !validTableName(db, table) {
			http.Error(w, "table not found", http.StatusNotFound)
			return
		}

		cols, rows, err := queryTable(db, table)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type data struct {
			Table string
			Cols  []string
			Rows  [][]string
		}
		renderTemplate(w, "table", data{Table: table, Cols: cols, Rows: rows})
	}
}


// renderTemplate parses templates from disk on every call so edits take effect
// without restarting the server. Run the server from the backend/ directory so
// that the relative "templates/" path resolves correctly.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	t, err := template.ParseFiles(
		"templates/base.html",
		"templates/"+name+".html",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Println("template error:", err)
	}
}
