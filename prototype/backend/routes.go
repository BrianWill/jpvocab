package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
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
		r.Post("/words", adminAddWord(db))
		r.Post("/words/batch", adminAddWordsBatch(db))
	})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), r); err != nil {
		log.Fatal(err)
	}
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

func adminAddWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin?error=bad+request", http.StatusSeeOther)
			return
		}
		word := toBaseForm(r.FormValue("word"))
		if word == "" {
			http.Redirect(w, r, "/admin?error=word+is+required", http.StatusSeeOther)
			return
		}
		target, err := strconv.Atoi(r.FormValue("drill_target"))
		if err != nil || target < 1 {
			target = 1
		}
		if err := insertWord(db,
			word,
			r.FormValue("reading"),
			r.FormValue("part_of_speech"),
			r.FormValue("meaning"),
			r.FormValue("example_jp"),
			r.FormValue("example_en"),
			target,
		); err != nil {
			http.Redirect(w, r, "/admin?error="+err.Error(), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func adminAddWordsBatch(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin?error=bad+request", http.StatusSeeOther)
			return
		}
		words := parseWordList(r.FormValue("words"))
		if len(words) == 0 {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		// Normalise each word to its base form, then re-deduplicate (two
		// different conjugations of the same verb collapse to one entry).
		seen := make(map[string]bool, len(words))
		normalised := make([]string, 0, len(words))
		for _, w := range words {
			b := toBaseForm(w)
			if !seen[b] {
				seen[b] = true
				normalised = append(normalised, b)
			}
		}
		words = normalised

		autoFill := r.FormValue("autofill") == "on"
		added := 0
		for _, word := range words {
			var reading, pos, meaning, exJP, exEN string
			if autoFill {
				e, err := autoFillWord(word, r.FormValue("ai_model"))
				if err != nil {
					http.Redirect(w, r, "/admin?error=auto-fill+error+for+「"+word+"」:+"+err.Error(), http.StatusSeeOther)
					return
				}
				reading, pos, meaning, exJP, exEN = e.Reading, e.PartOfSpeech, e.Meaning, e.ExampleJP, e.ExampleEN
			}
			err := insertWord(db, word, reading, pos, meaning, exJP, exEN, 1)
			if err != nil {
				if strings.Contains(err.Error(), "UNIQUE constraint failed") {
					continue // silently skip duplicates
				}
				http.Redirect(w, r, "/admin?error="+err.Error(), http.StatusSeeOther)
				return
			}
			added++
		}
		http.Redirect(w, r, fmt.Sprintf("/admin?added=%d", added), http.StatusSeeOther)
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
