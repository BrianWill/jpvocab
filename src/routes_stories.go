package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func findStoryWord(story *storyJSON, baseWord, displayWord string) *storyWordJSON {
	for _, sentence := range story.Sentences {
		for _, word := range sentence.Words {
			if word.BaseWord != baseWord {
				continue
			}
			if strings.TrimSpace(displayWord) == "" || word.DisplayWord == displayWord {
				w := word
				return &w
			}
		}
	}
	return nil
}

func apiGetStories(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stories, err := listStories(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if stories == nil {
			stories = []storyJSON{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stories)
	}
}

func apiGetStory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}
		story, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if story == nil {
			http.Error(w, "story not found", http.StatusNotFound)
			return
		}

		// Remove any noted words whose base word is now in the lexicon.
		lexiconSet := map[string]bool{}
		for _, sentence := range story.Sentences {
			for _, word := range sentence.Words {
				if word.InLexicon {
					lexiconSet[word.BaseWord] = true
				}
			}
		}
		var cleanedNoted []storyNotedWordJSON
		changed := false
		for _, nw := range story.NotedWords {
			if lexiconSet[nw.BaseWord] {
				changed = true
			} else {
				cleanedNoted = append(cleanedNoted, nw)
			}
		}
		if changed {
			if cleanedNoted == nil {
				cleanedNoted = []storyNotedWordJSON{}
			}
			story.NotedWords = cleanedNoted
			setStoryNotedWords(db, id, cleanedNoted) //nolint:errcheck
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(story)
	}
}

func apiAddStoryNotedWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}

		var body struct {
			BaseWord    string `json:"baseWord"`
			DisplayWord string `json:"displayWord"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		story, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if story == nil {
			http.Error(w, "story not found", http.StatusNotFound)
			return
		}

		word := findStoryWord(story, strings.TrimSpace(body.BaseWord), strings.TrimSpace(body.DisplayWord))
		if word == nil {
			http.Error(w, "word not found in story", http.StatusBadRequest)
			return
		}

		if err := addStoryNotedWord(db, id, storyNotedWordJSON{
			DisplayWord: word.DisplayWord,
			BaseWord:    word.BaseWord,
			English:     word.English,
		}); err != nil {
			if err.Error() == "story not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "required") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		updated, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"notedWords": updated.NotedWords})
	}
}

func apiDeleteStoryNotedWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}

		var body struct {
			BaseWord string `json:"baseWord"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if err := removeStoryNotedWord(db, id, body.BaseWord); err != nil {
			if err.Error() == "story not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "required") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		updated, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"notedWords": updated.NotedWords})
	}
}

func apiDeleteStory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}

		if err := deleteStory(db, id); err != nil {
			if err.Error() == "story not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func apiCreateStory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		title := strings.TrimSpace(body.Title)
		content := strings.TrimSpace(body.Content)
		if title == "" {
			http.Error(w, "story title is required", http.StatusBadRequest)
			return
		}
		if content == "" {
			http.Error(w, "story content is required", http.StatusBadRequest)
			return
		}

		sentences := buildStorySentencesFromText(content)
		if len(sentences) == 0 {
			http.Error(w, "story must have at least one sentence", http.StatusBadRequest)
			return
		}

		id, err := insertStory(db, title, nil, sentences)
		if err != nil {
			if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "at least one sentence") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		story, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(story)
	}
}

// storySentenceText reconstructs the plain text of a sentence from its word tokens.
func storySentenceText(s storySentenceJSON) string {
	parts := make([]string, len(s.Words))
	for i, w := range s.Words {
		parts[i] = w.DisplayWord
	}
	return strings.Join(parts, "")
}

// apiGenerateStoryAudio synthesizes per-sentence OGG audio for a story using VoiceVox.
// Files are written to static/audio/story_{id}/sentence_{position}.ogg.
// The request body may supply VoiceVox settings; defaults are used otherwise.
// Cancellation is handled via the request context (frontend AbortController).
//
// The response is streamed as NDJSON. Each completed sentence emits:
//
//	{"sentencePosition": N}
//
// On success all sentences emit {"allDone": true}. On error {"error": "..."} is emitted.
// Audio is first written to *.ogg.temp files; originals are only replaced (via os.Rename)
// once all sentences complete. On cancellation the temp files are deleted.
func apiGenerateStoryAudio(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}

		p := defaultVoicevoxParams()
		var body struct {
			Speaker         *int     `json:"speaker"`
			SpeedScale      *float64 `json:"speedScale"`
			IntonationScale *float64 `json:"intonationScale"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.Speaker != nil {
				p.Speaker = *body.Speaker
			}
			if body.SpeedScale != nil {
				p.SpeedScale = *body.SpeedScale
			}
			if body.IntonationScale != nil {
				p.IntonationScale = *body.IntonationScale
			}
		}

		story, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if story == nil {
			http.Error(w, "story not found", http.StatusNotFound)
			return
		}

		audioDir := filepath.Join("static", "audio", fmt.Sprintf("story_%d", id))
		if err := os.MkdirAll(audioDir, 0o755); err != nil {
			http.Error(w, "could not create audio dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Start streaming NDJSON.
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher, _ := w.(http.Flusher)

		streamEvent := func(v any) {
			data, _ := json.Marshal(v)
			w.Write(append(data, '\n'))
			if flusher != nil {
				flusher.Flush()
			}
		}

		type sentenceDuration struct {
			id         int64
			durationMs int64
		}

		var tempFiles []string
		var durations []sentenceDuration
		cancelled := false

		for _, sentence := range story.Sentences {
			if r.Context().Err() != nil {
				cancelled = true
				break
			}

			text := storySentenceText(sentence)
			if strings.TrimSpace(text) == "" {
				// Empty sentence: report done immediately (no audio file needed).
				streamEvent(map[string]int{"sentencePosition": sentence.Position})
				continue
			}

			wav, err := synthesizeVoicevox(r.Context(), text, p)
			if err != nil {
				if r.Context().Err() != nil {
					cancelled = true
					break
				}
				streamEvent(map[string]string{"error": "voicevox error: " + err.Error()})
				for _, f := range tempFiles {
					os.Remove(f)
				}
				return
			}

			durationMs := wavDurationMs(wav)

			ogg, err := wavToOgg(r.Context(), wav)
			if err != nil {
				if r.Context().Err() != nil {
					cancelled = true
					break
				}
				streamEvent(map[string]string{"error": "ffmpeg error: " + err.Error()})
				for _, f := range tempFiles {
					os.Remove(f)
				}
				return
			}

			dest := filepath.Join(audioDir, fmt.Sprintf("sentence_%d.ogg", sentence.Position))
			tempDest := dest + ".temp"
			if err := os.WriteFile(tempDest, ogg, 0o644); err != nil {
				streamEvent(map[string]string{"error": "write error: " + err.Error()})
				for _, f := range tempFiles {
					os.Remove(f)
				}
				return
			}
			tempFiles = append(tempFiles, tempDest)
			durations = append(durations, sentenceDuration{sentence.ID, durationMs})

			streamEvent(map[string]int{"sentencePosition": sentence.Position})
		}

		if cancelled {
			for _, f := range tempFiles {
				os.Remove(f)
			}
			return
		}

		// Atomically replace originals with temp files.
		for _, tempPath := range tempFiles {
			finalPath := strings.TrimSuffix(tempPath, ".temp")
			if err := os.Rename(tempPath, finalPath); err != nil {
				streamEvent(map[string]string{"error": "rename error: " + err.Error()})
				return
			}
		}

		// Commit durations and hasAudio to the DB only after all files are in place.
		for _, d := range durations {
			if err := setSentenceAudioDuration(db, d.id, d.durationMs); err != nil {
				streamEvent(map[string]string{"error": "db error: " + err.Error()})
				return
			}
		}
		if err := setStoryHasAudio(db, id, true); err != nil {
			streamEvent(map[string]string{"error": "db error: " + err.Error()})
			return
		}

		streamEvent(map[string]bool{"allDone": true})
	}
}

// apiGenerateStoryTranslation calls an AI provider to translate all sentences in the story
// and generate brief glosses for words not already in the lexicon with a meaning.
// The response is streamed as NDJSON:
//
//	{"status": "translating", "sentenceCount": N, "wordCount": M}  — emitted immediately
//	{"allDone": true}                                               — on success
//	{"error": "..."}                                               — on failure
func apiGenerateStoryTranslation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid story id", http.StatusBadRequest)
			return
		}

		var body struct {
			AIModel string `json:"ai_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AIModel == "" {
			http.Error(w, "ai_model is required", http.StatusBadRequest)
			return
		}

		story, err := getStoryByID(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if story == nil {
			http.Error(w, "story not found", http.StatusNotFound)
			return
		}

		// Collect unique base words across all sentences.
		baseWordSet := map[string]struct{}{}
		for _, s := range story.Sentences {
			for _, w := range s.Words {
				if w.BaseWord != "" {
					baseWordSet[w.BaseWord] = struct{}{}
				}
			}
		}

		// Find which base words already have meanings in the lexicon.
		inLexicon := map[string]bool{}
		if len(baseWordSet) > 0 {
			placeholders := make([]string, 0, len(baseWordSet))
			args := make([]any, 0, len(baseWordSet))
			for bw := range baseWordSet {
				placeholders = append(placeholders, "?")
				args = append(args, bw)
			}
			rows, err := db.QueryContext(r.Context(),
				`SELECT word FROM words WHERE word IN (`+strings.Join(placeholders, ",")+`) AND meaning != ''`,
				args...,
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for rows.Next() {
				var word string
				rows.Scan(&word) //nolint:errcheck
				inLexicon[word] = true
			}
			rows.Close()
		}

		// Words to gloss: in story but not in lexicon with a meaning.
		var wordsToGloss []string
		for bw := range baseWordSet {
			if !inLexicon[bw] {
				wordsToGloss = append(wordsToGloss, bw)
			}
		}
		sort.Strings(wordsToGloss)

		// Build sentence input list (skip blank sentences).
		var sentenceTexts []string
		var sentenceIDs []int64
		for _, s := range story.Sentences {
			text := storySentenceText(s)
			if strings.TrimSpace(text) == "" {
				continue
			}
			sentenceTexts = append(sentenceTexts, text)
			sentenceIDs = append(sentenceIDs, s.ID)
		}

		// Start streaming NDJSON.
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher, _ := w.(http.Flusher)

		streamEvent := func(v any) {
			data, _ := json.Marshal(v)
			w.Write(append(data, '\n')) //nolint:errcheck
			if flusher != nil {
				flusher.Flush()
			}
		}

		streamEvent(map[string]any{
			"status":        "translating",
			"sentenceCount": len(sentenceTexts),
			"wordCount":     len(wordsToGloss),
		})

		// Send periodic heartbeats so the client knows the request is still alive.
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					streamEvent(map[string]string{"status": "working"})
				case <-done:
					return
				}
			}
		}()

		result, err := translateStory(db, sentenceTexts, wordsToGloss, body.AIModel)
		close(done)
		if err != nil {
			streamEvent(map[string]string{"error": err.Error()})
			return
		}

		// Save sentence translations by index (AI returns them in the same order).
		for i, translation := range result.Sentences {
			if i >= len(sentenceIDs) || translation == "" {
				break
			}
			if err := setSentenceEnglishText(db, sentenceIDs[i], translation); err != nil {
				streamEvent(map[string]string{"error": "db error: " + err.Error()})
				return
			}
		}

		// Save word glosses (merge into existing map).
		if len(result.Words) > 0 {
			newGlosses := make(map[string]storyWordGlossJSON, len(result.Words))
			for _, wg := range result.Words {
				if wg.Word != "" && (wg.Gloss != "" || wg.Reading != "") {
					newGlosses[wg.Word] = storyWordGlossJSON{
						English: wg.Gloss,
						Reading: wg.Reading,
					}
				}
			}
			if len(newGlosses) > 0 {
				if err := mergeStoryWordGlosses(db, id, newGlosses); err != nil {
					streamEvent(map[string]string{"error": "db error: " + err.Error()})
					return
				}
			}
		}

		streamEvent(map[string]bool{"allDone": true})
	}
}
