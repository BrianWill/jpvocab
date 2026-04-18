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

		// Remove any noted words whose base word is now tracked in the lexicon.
		var cleanedNoted []storyNotedWordJSON
		changed := false
		for _, nw := range story.NotedWords {
			var tracked int
			db.QueryRow(`SELECT tracked FROM words WHERE base_word = ?`, nw.BaseWord).Scan(&tracked) //nolint:errcheck
			if tracked == 1 {
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
			Title     string `json:"title"`
			Content   string `json:"content"`
			MediaType string `json:"mediaType"`
			MediaURL  string `json:"mediaUrl"`
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

		media, err := normalizeStoryMedia(body.MediaType, body.MediaURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		sentences := parseStoryContent(content)
		if len(sentences) == 0 {
			http.Error(w, "story must have at least one sentence", http.StatusBadRequest)
			return
		}

		id, err := insertStory(db, title, sentences, media)
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

func writeStoryJobSuccess(streamEvent func(v any), usage tokenUsage) {
	streamEvent(map[string]any{
		"allDone":      true,
		"inputTokens":  usage.InputTokens,
		"outputTokens": usage.OutputTokens,
		"totalTokens":  usage.InputTokens + usage.OutputTokens,
	})
}

func runStoryNDJSONJob(w http.ResponseWriter, initialEvent map[string]any, work func() (tokenUsage, error)) {
	streamEvent := newNDJSONStreamer(w)
	streamEvent(initialEvent)

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

	usage, err := work()
	close(done)
	if err != nil {
		streamEvent(map[string]string{"error": err.Error()})
		return
	}

	writeStoryJobSuccess(streamEvent, usage)
}

// apiGenerateStoryTranslation calls an AI provider to translate story sentences from
// each sentence's original language into the opposite language. Streams NDJSON:
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
			AIModel       string `json:"ai_model"`
			ChunkPosition int    `json:"chunk_position"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AIModel == "" {
			http.Error(w, "ai_model is required", http.StatusBadRequest)
			return
		}

		targetSentences, err := querySentencesLite(db, id, body.ChunkPosition)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if targetSentences == nil {
			http.Error(w, "story or chunk not found", http.StatusNotFound)
			return
		}

		var requests []storyTranslationRequest
		var sentenceIDs []int64
		for _, s := range targetSentences {
			text := storySentenceSourceText(s)
			if strings.TrimSpace(text) == "" {
				continue
			}
			requests = append(requests, storyTranslationRequest{
				OrigLang: s.OrigLang,
				Text:     text,
			})
			sentenceIDs = append(sentenceIDs, s.ID)
		}

		runStoryNDJSONJob(w, map[string]any{
			"status":        "translating",
			"sentenceCount": len(requests),
			"chunkPosition": body.ChunkPosition,
		}, func() (tokenUsage, error) {
			result, usage, err := translateStory(db, requests, body.AIModel)
			if err != nil {
				return tokenUsage{}, err
			}

			for i, translation := range result.Sentences {
				if i >= len(sentenceIDs) || i >= len(result.TargetLangs) || translation == "" {
					continue
				}
				if err := setSentenceTranslationText(db, sentenceIDs[i], result.TargetLangs[i], translation); err != nil {
					return tokenUsage{}, err
				}
			}

			return usage, nil
		})
	}
}
