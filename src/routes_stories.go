package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func newNDJSONStreamer(w http.ResponseWriter) func(v any) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, _ := w.(http.Flusher)
	return func(v any) {
		data, _ := json.Marshal(v)
		w.Write(append(data, '\n')) //nolint:errcheck
		if flusher != nil {
			flusher.Flush()
		}
	}
}

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
		stories, err := listStoriesMeta(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if stories == nil {
			stories = []storyMetaJSON{}
		}
		writeJSON(w, stories)
	}
}

func apiGetStory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
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
				if word.Tracked {
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

		writeJSON(w, story)
	}
}

func apiAddStoryNotedWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
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
		writeJSON(w, map[string]any{"notedWords": updated.NotedWords})
	}
}

func apiDeleteStoryNotedWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
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
		writeJSON(w, map[string]any{"notedWords": updated.NotedWords})
	}
}

func apiDeleteStory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
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

		id, err := insertStory(db, title, sentences)
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

		writeJSONStatus(w, http.StatusCreated, story)
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

func findStoryChunk(story *storyJSON, chunkID int64) *storyChunkJSON {
	for i := range story.Chunks {
		if story.Chunks[i].ID == chunkID {
			return &story.Chunks[i]
		}
	}
	return nil
}

// apiGenerateStoryTranslation calls an AI provider to translate all sentences in the
// story. Streams NDJSON:
//
//	{"status": "translating", "sentenceCount": N}                                   — emitted immediately
//	{"allDone": true, "inputTokens": N, "outputTokens": N, "totalTokens": N}        — on success
//	{"error": "..."}                                                                  — on failure
func apiGenerateStoryTranslation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
			return
		}

		var body struct {
			AIModel string `json:"ai_model"`
			ChunkID int64  `json:"chunk_id"`
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

		targetSentences := story.Sentences
		if body.ChunkID > 0 {
			chunk := findStoryChunk(story, body.ChunkID)
			if chunk == nil {
				http.Error(w, "story chunk not found", http.StatusBadRequest)
				return
			}
			targetSentences = chunk.Sentences
		}

		var sentenceTexts []string
		var sentenceIDs []int64
		for _, s := range targetSentences {
			text := storySentenceText(s)
			if strings.TrimSpace(text) == "" {
				continue
			}
			sentenceTexts = append(sentenceTexts, text)
			sentenceIDs = append(sentenceIDs, s.ID)
		}

		streamEvent := newNDJSONStreamer(w)
		streamEvent(map[string]any{
			"status":        "translating",
			"sentenceCount": len(sentenceTexts),
			"chunkId":       body.ChunkID,
		})

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

		result, usage, err := translateStory(db, sentenceTexts, body.AIModel)
		close(done)
		if err != nil {
			streamEvent(map[string]string{"error": err.Error()})
			return
		}

		for i, translation := range result.Sentences {
			if i >= len(sentenceIDs) || translation == "" {
				break
			}
			if err := setSentenceEnglishText(db, sentenceIDs[i], translation); err != nil {
				streamEvent(map[string]string{"error": "db error: " + err.Error()})
				return
			}
		}

		streamEvent(map[string]any{
			"allDone":      true,
			"inputTokens":  usage.InputTokens,
			"outputTokens": usage.OutputTokens,
			"totalTokens":  usage.InputTokens + usage.OutputTokens,
		})
	}
}

// apiGenerateStoryWordInfo runs autofill for all story words that have no word info yet
// (meaning, reading, part_of_speech, example_jp, example_en all empty). Streams NDJSON:
//
//	{"status": "autofilling", "wordCount": N}                                  — emitted immediately
//	{"allDone": true, "inputTokens": N, "outputTokens": N, "totalTokens": N}   — on success
//	{"error": "..."}                                                            — on failure
func apiGenerateStoryWordInfo(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid story id")
		if !ok {
			return
		}

		var body struct {
			AIModel string `json:"ai_model"`
			ChunkID int64  `json:"chunk_id"`
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

		streamEvent := newNDJSONStreamer(w)

		targetStoryWords := story.StoryWords
		if body.ChunkID > 0 {
			chunk := findStoryChunk(story, body.ChunkID)
			if chunk == nil {
				http.Error(w, "story chunk not found", http.StatusBadRequest)
				return
			}
			targetStoryWords = chunk.StoryWords
		}

		if len(targetStoryWords) == 0 {
			streamEvent(map[string]any{
				"allDone":      true,
				"inputTokens":  0,
				"outputTokens": 0,
				"totalTokens":  0,
			})
			return
		}

		// Find story words that have no word info at all yet.
		placeholders := make([]string, len(targetStoryWords))
		args := make([]any, len(targetStoryWords))
		for i, bw := range targetStoryWords {
			placeholders[i] = "?"
			args[i] = bw
		}
		rows, err := db.QueryContext(r.Context(),
			`SELECT id, base_word FROM words
			 WHERE base_word IN (`+strings.Join(placeholders, ",")+`)
			 AND COALESCE(reading,'') = ''
			 AND COALESCE(part_of_speech,'') = ''
			 AND COALESCE(meaning,'') = ''
			 AND COALESCE(example_jp,'') = ''
			 AND COALESCE(example_en,'') = ''`,
			args...,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type wordEntry struct {
			ID   int64
			Word string
		}
		var wordsToFill []wordEntry
		for rows.Next() {
			var e wordEntry
			if err := rows.Scan(&e.ID, &e.Word); err != nil {
				rows.Close()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			wordsToFill = append(wordsToFill, e)
		}
		rows.Close()

		if len(wordsToFill) == 0 {
			streamEvent(map[string]any{
				"allDone":      true,
				"inputTokens":  0,
				"outputTokens": 0,
				"totalTokens":  0,
			})
			return
		}

		streamEvent(map[string]any{
			"status":    "autofilling",
			"wordCount": len(wordsToFill),
			"chunkId":   body.ChunkID,
		})

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

		wordStrings := make([]string, len(wordsToFill))
		for i, e := range wordsToFill {
			wordStrings[i] = e.Word
		}
		fills, usage, err := autoFillWordsBatchWithUsage(db, wordStrings, body.AIModel)
		close(done)
		if err != nil {
			streamEvent(map[string]string{"error": err.Error()})
			return
		}

		for i, e := range wordsToFill {
			if fills[i] == nil {
				continue
			}
			if _, err := persistWordAutoFill(db, e.ID, fills[i]); err != nil {
				streamEvent(map[string]string{"error": "db error: " + err.Error()})
				return
			}
		}

		streamEvent(map[string]any{
			"allDone":      true,
			"inputTokens":  usage.InputTokens,
			"outputTokens": usage.OutputTokens,
			"totalTokens":  usage.InputTokens + usage.OutputTokens,
		})
	}
}
