package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
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

func apiGetTutorSystemPrompt() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(tutorSystemPrompt(mode)))
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
		if len(req.Messages) == 0 {
			http.Error(w, "messages is required", http.StatusBadRequest)
			return
		}

		system := tutorSystemPrompt(req.TutorMode)
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
