package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func apiGetWords(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		words, err := listWords(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if words == nil {
			words = []wordJSON{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(words)
	}
}

func apiUpdateWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body struct {
			Reading   string `json:"reading"`
			Type      string `json:"type"`
			Meaning   string `json:"meaning"`
			ExampleJp string `json:"exampleJp"`
			ExampleEn string `json:"exampleEn"`
			Target    int    `json:"target"`
			KanjiData []struct {
				ID      int64  `json:"id"`
				Reading string `json:"reading"`
			} `json:"kanjiData"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		body.Reading = strings.TrimSpace(body.Reading)
		for _, ch := range body.Reading {
			if !(ch >= 0x3040 && ch <= 0x309F) && !(ch >= 0x30A0 && ch <= 0x30FF) {
				http.Error(w, "reading must contain only kana (no spaces)", http.StatusBadRequest)
				return
			}
		}
		for _, k := range body.KanjiData {
			for _, ch := range strings.TrimSpace(k.Reading) {
				if !(ch >= 0x3040 && ch <= 0x309F) && !(ch >= 0x30A0 && ch <= 0x30FF) {
					http.Error(w, "kanji reading must contain only kana", http.StatusBadRequest)
					return
				}
			}
		}
		kanjiDataJSON, _ := json.Marshal(body.KanjiData)
		if err := updateWord(db, id, body.Reading, body.Type, body.Meaning, body.ExampleJp, body.ExampleEn, string(kanjiDataJSON), body.Target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiUpdateWordTarget(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body struct {
			Target int `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := updateWordTarget(db, id, body.Target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiDeleteWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := deleteWordByID(db, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiRerollMeaning() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Word    string `json:"word"`
			Current string `json:"current"`
			AIModel string `json:"ai_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		alternatives, err := rerollMeaning(body.Word, body.Current, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"alternatives": alternatives})
	}
}

func apiRerollExamples() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Word    string `json:"word"`
			AIModel string `json:"ai_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		alternatives, err := rerollExamples(body.Word, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"alternatives": alternatives})
	}
}

func apiAutofillWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		var body struct {
			Word    string `json:"word"`
			AIModel string `json:"ai_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		filled, err := autoFillWord(body.Word, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type kdEntry struct {
			ID      int64  `json:"id"`
			Reading string `json:"reading"`
		}
		kd := make([]kdEntry, 0, len(filled.Kanji))
		for _, k := range filled.Kanji {
			kID, kErr := upsertKanji(db, k.Character, k.Meanings)
			if kErr != nil {
				continue
			}
			kd = append(kd, kdEntry{ID: kID, Reading: k.Reading})
		}
		b, _ := json.Marshal(kd)
		if err := updateWordFill(db, id, filled.Reading, filled.PartOfSpeech, filled.Meaning, filled.ExampleJP, filled.ExampleEN, string(b)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"word":           body.Word,
			"reading":        filled.Reading,
			"part_of_speech": filled.PartOfSpeech,
			"meaning":        filled.Meaning,
			"example_jp":     filled.ExampleJP,
			"example_en":     filled.ExampleEN,
			"kanji_data":     kd,
		})
	}
}

func apiGetKanji(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kanji, err := listKanji(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(kanji)
	}
}
