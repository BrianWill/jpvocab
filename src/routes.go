package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func parseRouteInt64(w http.ResponseWriter, r *http.Request, paramName, invalidMsg string) (int64, bool) {
	value, err := strconv.ParseInt(chi.URLParam(r, paramName), 10, 64)
	if err != nil {
		http.Error(w, invalidMsg, http.StatusBadRequest)
		return 0, false
	}
	return value, true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func serverInit(db *sql.DB) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	appPage := func(file, currentPage string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			renderAppPage(w, "static/"+file, appPageData{CurrentPage: currentPage})
		}
	}

	r.Get("/", appPage("welcome.html", "welcome"))
	r.Get("/welcome", appPage("welcome.html", "welcome"))
	r.Get("/activity", appPage("activity.html", "activity"))
	r.Get("/lexicon", appPage("lexicon.html", "lexicon"))
	r.Get("/drill", appPage("drill.html", "drill"))
	r.Get("/tutor", appPage("tutor.html", "tutor"))
	r.Get("/stories", appPage("stories.html", "stories"))
	r.Get("/stories/{id}", appPage("story.html", "story-detail"))
	r.Get("/token-usage", appPage("token-usage.html", "token-usage"))

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Get("/api/activity/stats", apiGetActivityStats(db))
	r.Get("/api/activity/calendar", apiGetActivityCalendar(db))
	r.Get("/api/stories", apiGetStories(db))
	r.Post("/api/stories", apiCreateStory(db))
	r.Get("/api/stories/{id}", apiGetStory(db))
	r.Delete("/api/stories/{id}", apiDeleteStory(db))
	r.Post("/api/stories/{id}/noted-words", apiAddStoryNotedWord(db))
	r.Delete("/api/stories/{id}/noted-words", apiDeleteStoryNotedWord(db))
	r.Post("/api/stories/{id}/generate-audio", apiGenerateStoryAudio(db))
	r.Post("/api/stories/{id}/generate-translation", apiGenerateStoryTranslation(db))
	r.Post("/api/stories/{id}/generate-word-info", apiGenerateStoryWordInfo(db))

	r.Get("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		p := checkAIProviders()
		s := checkImageSources()
		writeJSON(w, map[string]any{
			"ai": map[string]bool{
				"anthropic": p.AnthropicAvail,
				"openai":    p.OpenAIAvail,
				"google":    p.GoogleAvail,
				"mistral":   p.MistralAvail,
				"glm":       p.GLMAvail,
			},
			"image_sources": map[string]bool{
				"unsplash": s.UnsplashAvail,
				"pexels":   s.PexelsAvail,
				"pixabay":  s.PixabayAvail,
				"bing":     s.BingAvail,
			},
			"default_drill_target": func() int {
				if s, err := getDrillSettings(db); err == nil {
					return s.NewWordTarget
				}
				return 8
			}(),
		})
	})
	r.Get("/api/wordlists", apiGetWordLists(db))
	r.Get("/api/wordlists/{slug}/words", apiGetWordListWords(db))

	r.Get("/api/words", apiGetWords(db))
	r.Patch("/api/words/{id}", apiUpdateWord(db))
	r.Patch("/api/words/{id}/target", apiUpdateWordTarget(db))
	r.Delete("/api/words/{id}", apiDeleteWord(db))
	r.Post("/api/words/{id}/upload-image", apiUploadWordImage(db))
	r.Post("/api/words/{id}/download-image", apiDownloadWordImage(db))
	r.Post("/api/words/{id}/find-image", apiFindWordImage(db))
	r.Post("/api/words/autofill-batch", apiAutofillWordsBatch(db))
	r.Post("/api/words/{id}/autofill", apiAutofillWord(db))
	r.Post("/api/words/{id}/generate-audio", apiGenerateWordAudio(db))

	r.Get("/api/voicevox/speakers", apiVoicevoxSpeakers())
	r.Post("/api/voicevox/preview", apiVoicevoxPreview())
	r.Get("/api/ffmpeg/available", apiFfmpegAvailable())

	r.Get("/api/kanji", apiGetKanji(db))
	r.Get("/api/drill/sessions/current", apiGetCurrentDrillSession(db))
	r.Post("/api/drill/sessions", apiCreateDrillSession(db))
	r.Post("/api/drill/sessions/{id}/answers", apiRecordDrillAnswer(db))

	r.Get("/api/settings/drill", apiGetDrillSettings(db))
	r.Put("/api/settings/drill", apiPutDrillSettings(db))

	r.Get("/api/tutor/prompts", apiGetTutorPrompts(db))
	r.Post("/api/tutor/prompts", apiCreateTutorPrompt(db))
	r.Patch("/api/tutor/prompts/{id}", apiUpdateTutorPrompt(db))
	r.Delete("/api/tutor/prompts/{id}", apiDeleteTutorPrompt(db))
	r.Get("/api/tutor/system-prompt", apiGetTutorSystemPrompt(db))
	r.Get("/api/tutor/session", apiGetTutorSession())
	r.Delete("/api/tutor/session", apiClearTutorSession())
	r.Post("/api/tutor/chat", apiTutorChat(db))

	r.Get("/api/token-usage", apiGetTokenUsage(db))

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
	Input             string `json:"input"`
	Word              string `json:"word"`
	Added             bool   `json:"added"`
	Updated           bool   `json:"updated,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Reading           string `json:"reading,omitempty"`
	PartOfSpeech      string `json:"part_of_speech,omitempty"`
	Meaning           string `json:"meaning,omitempty"`
	ExampleJP         string `json:"example_jp,omitempty"`
	ExampleEN         string `json:"example_en,omitempty"`
	ImagePath         string `json:"image_path,omitempty"`
	SuggestedImageURL string `json:"suggested_image_url,omitempty"`
	// Populated only when the word already exists in the lexicon.
	WordID         int64 `json:"word_id,omitempty"`
	DrillCount     int   `json:"drill_count,omitempty"`
	DrillIncorrect int   `json:"drill_incorrect,omitempty"`
	DrillTarget    int   `json:"drill_target,omitempty"`
}

type appPageData struct {
	CurrentPage string
}

type indexData struct {
	Tables    []tableInfo
	Error     string
	Success   string
	Providers aiProviders
}

func apiGetTokenUsage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := getTokenUsageSummary(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log, err := getTokenUsageLog(db, 500)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		totalCalls, totalInput, totalOutput, err := getTokenUsageTotals(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if summary == nil {
			summary = []tokenUsageSummaryRow{}
		}
		if log == nil {
			log = []tokenUsageEntry{}
		}
		writeJSON(w, map[string]any{
			"totals": map[string]int{
				"calls":         totalCalls,
				"input_tokens":  totalInput,
				"output_tokens": totalOutput,
			},
			"summary": summary,
			"log":     log,
		})
	}
}

func apiGetActivityStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := getActivityStats(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stats)
	}
}

func apiGetActivityCalendar(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cal, err := getActivityCalendar(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cal)
	}
}

func apiCreateDrillSession(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			State drillSessionState `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := createDrillSession(db, body.State)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int64{"id": id})
	}
}

func apiGetCurrentDrillSession(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := getCurrentDrillSession(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"session": session})
	}
}

func apiRecordDrillAnswer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := parseRouteInt64(w, r, "id", "invalid session id")
		if !ok {
			return
		}
		var body struct {
			WordID  int64             `json:"wordId"`
			Correct bool              `json:"correct"`
			State   drillSessionState `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := recordDrillAnswer(db, sessionID, body.WordID, body.Correct, body.State); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiGetDrillSettings(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := getDrillSettings(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}

func apiPutDrillSettings(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s drillSettings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := putDrillSettings(db, s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
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
		renderTemplate(w, "admin", indexData{
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
		rawWords := extractContentWords(r.FormValue("words"))
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

		drillCfg, _ := getDrillSettings(db)
		newWordTarget := drillCfg.NewWordTarget
		if newWordTarget <= 0 {
			newWordTarget = 8
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
		existingInfo, err := wordsInfoInDB(db, uniqueNorms)
		if err != nil {
			send(map[string]string{"error": err.Error()})
			return
		}

		// Phase 3: insert all words and stream results immediately so the UI
		// shows final added/skipped counts before any autofill begins.
		type insertedEntry struct {
			input  string
			norm   string
			wordID int64
		}
		var toFill []insertedEntry

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
			if info, exists := existingInfo[e.norm]; exists {
				imagePath := ""
				if info.ImagePath != nil {
					imagePath = *info.ImagePath
				}
				send(batchWordResult{
					Input:          e.input,
					Word:           e.norm,
					Added:          false,
					Reason:         "already in lexicon",
					Reading:        info.Reading,
					PartOfSpeech:   info.PartOfSpeech,
					Meaning:        info.Meaning,
					ExampleJP:      info.ExampleJP,
					ExampleEN:      info.ExampleEN,
					ImagePath:      imagePath,
					WordID:         info.ID,
					DrillCount:     info.DrillCount,
					DrillIncorrect: info.DrillIncorrect,
					DrillTarget:    info.DrillTarget,
				})
				continue
			}

			listEntry, hasListEntry := wordListLookup(e.norm)
			wordID, err := insertWordReturningID(db, e.norm, listEntry.Reading, listEntry.PartOfSpeech, listEntry.Meaning, listEntry.ExampleJP, listEntry.ExampleEN, "", newWordTarget)
			if err != nil {
				reason := err.Error()
				if strings.Contains(reason, "UNIQUE constraint failed") || reason == "already in lexicon" {
					reason = "already in lexicon"
				}
				send(batchWordResult{Input: e.input, Word: e.norm, Added: false, Reason: reason})
				continue
			}
			// Read back from DB so we reflect any info already present on the row
			// (e.g. autofilled by generate-word-info while tracked=0).
			var actualReading, actualPOS, actualMeaning, actualExJP, actualExEN string
			db.QueryRowContext(r.Context(),
				`SELECT COALESCE(reading,''), COALESCE(part_of_speech,''), COALESCE(meaning,''),
				        COALESCE(example_jp,''), COALESCE(example_en,'') FROM words WHERE id = ?`, wordID,
			).Scan(&actualReading, &actualPOS, &actualMeaning, &actualExJP, &actualExEN)

			result := batchWordResult{
				Input:        e.input,
				Word:         e.norm,
				Added:        true,
				Reading:      actualReading,
				PartOfSpeech: actualPOS,
				Meaning:      actualMeaning,
				ExampleJP:    actualExJP,
				ExampleEN:    actualExEN,
				WordID:       wordID,
				DrillTarget:  newWordTarget,
			}
			if hasListEntry {
				result.SuggestedImageURL = listEntry.SuggestedImageURL
			}
			send(result)
			if autoFill {
				toFill = append(toFill, insertedEntry{input: e.input, norm: e.norm, wordID: wordID})
			}
		}

		// Phase 4: autofill each inserted word now that all counts are settled.
	fillLoop:
		for _, ins := range toFill {
			select {
			case <-r.Context().Done():
				break fillLoop
			default:
			}

			filled, fillErr := autoFillWord(db, ins.norm, aiModel)
			if fillErr != nil {
				continue
			}
			if _, err := persistWordAutoFill(db, ins.wordID, filled); err != nil {
				continue
			}
			send(batchWordResult{Input: ins.input, Word: ins.norm, Updated: true,
				Reading: filled.Reading, PartOfSpeech: filled.PartOfSpeech, Meaning: filled.Meaning, ExampleJP: filled.ExampleJP, ExampleEN: filled.ExampleEN})
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

func renderAppPage(w http.ResponseWriter, pagePath string, data any) {
	t, err := template.ParseFiles(
		"templates/app_nav.html",
		pagePath,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, filepath.Base(pagePath), data); err != nil {
		log.Println("app template error:", err)
	}
}
