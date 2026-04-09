package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

// tutorSegment is a flexible map so the AI can include any string-valued fields
// beyond the standard "jp" and "en" (e.g. "correction", "note").
type tutorSegment = map[string]string

// tutorSession holds the most recent tutor conversation in memory.
// It persists across page navigations for the lifetime of the server process.
var tutorSession struct {
	AIModel   string    `json:"ai_model"`
	TutorMode string    `json:"tutor_mode"`
	Messages  []message `json:"messages"`
}

func apiGetTutorPrompts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prompts, err := listTutorPrompts(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, prompts)
	}
}

func apiCreateTutorPrompt(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Label        string `json:"label"`
			SystemPrompt string `json:"system_prompt"`
			Greeting     string `json:"greeting"`
			LangInput    string `json:"lang_input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if body.Label == "" || body.SystemPrompt == "" {
			http.Error(w, "label and system_prompt are required", http.StatusBadRequest)
			return
		}
		id, err := insertTutorPrompt(db, body.Label, body.SystemPrompt, body.Greeting, body.LangInput)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prompts, err := listTutorPrompts(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, p := range prompts {
			if p.ID == id {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(p)
				return
			}
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func apiUpdateTutorPrompt(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
			return
		}
		var body struct {
			Label        string `json:"label"`
			SystemPrompt string `json:"system_prompt"`
			Greeting     string `json:"greeting"`
			LangInput    string `json:"lang_input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if body.Label == "" || body.SystemPrompt == "" {
			http.Error(w, "label and system_prompt are required", http.StatusBadRequest)
			return
		}
		if err := updateTutorPrompt(db, id, body.Label, body.SystemPrompt, body.Greeting, body.LangInput); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prompts, err := listTutorPrompts(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, p := range prompts {
			if p.ID == id {
				writeJSON(w, p)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func apiDeleteTutorPrompt(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
			return
		}
		if err := deleteTutorPrompt(db, id); err != nil {
			if err.Error() == "cannot delete built-in prompt" {
				http.Error(w, err.Error(), http.StatusForbidden)
			} else if err.Error() == "prompt not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiGetTutorSystemPrompt(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("mode")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(tutorSystemPromptByID(db, id)))
	}
}

func apiGetTutorSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, tutorSession)
	}
}

func apiClearTutorSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tutorSession.AIModel = ""
		tutorSession.TutorMode = ""
		tutorSession.Messages = nil
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiTutorChat(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AIModel   string    `json:"ai_model"`
			TutorMode string    `json:"tutor_mode"`
			Messages  []message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.AIModel == "" {
			http.Error(w, "ai_model is required", http.StatusBadRequest)
			return
		}
		// Empty messages is valid — used to generate the bot's opening turn.

		modeID, _ := strconv.ParseInt(req.TutorMode, 10, 64)
		system := tutorSystemPromptByID(db, modeID)
		reply, err := tutorChat(db, req.Messages, system, req.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Parse the AI's single JSON object response. On failure, retry once with a format
		// reminder injected as a correction turn. The retry exchange is not stored in
		// history so future context always looks like the AI responded correctly.
		var response tutorSegment
		if jsonErr := json.Unmarshal([]byte(reply), &response); jsonErr != nil || len(response) == 0 {
			retryMsgs := append(req.Messages,
				message{Role: "assistant", Content: reply},
				message{Role: "user", Content: "Your last response was not a valid JSON object. Respond again as a single JSON object with string fields (jp, en, note, correction, etc.) — no array, no markdown, no code fences."},
			)
			if retryReply, retryErr := tutorChat(db, retryMsgs, system, req.AIModel); retryErr == nil {
				if jsonErr2 := json.Unmarshal([]byte(retryReply), &response); jsonErr2 == nil && len(response) > 0 {
					reply = retryReply
				}
			}
			if len(response) == 0 {
				response = tutorSegment{"en": reply}
			}
		}

		// Persist the full conversation (including the new reply) so it survives navigation.
		tutorSession.AIModel = req.AIModel
		tutorSession.TutorMode = req.TutorMode
		tutorSession.Messages = append(req.Messages, message{Role: "assistant", Content: reply})

		writeJSON(w, map[string]any{"response": response})
	}
}
