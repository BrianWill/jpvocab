package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

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

		for _, sentence := range story.Sentences {
			if err := r.Context().Err(); err != nil {
				// Client cancelled.
				return
			}

			text := storySentenceText(sentence)
			if strings.TrimSpace(text) == "" {
				continue
			}

			wav, err := synthesizeVoicevox(r.Context(), text, p)
			if err != nil {
				if r.Context().Err() != nil {
					return
				}
				http.Error(w, "voicevox error: "+err.Error(), http.StatusBadGateway)
				return
			}

			durationMs := wavDurationMs(wav)

			ogg, err := wavToOgg(r.Context(), wav)
			if err != nil {
				if r.Context().Err() != nil {
					return
				}
				http.Error(w, "ffmpeg error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			dest := filepath.Join(audioDir, fmt.Sprintf("sentence_%d.ogg", sentence.Position))
			if err := os.WriteFile(dest, ogg, 0o644); err != nil {
				http.Error(w, "write error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if err := setSentenceAudioDuration(db, sentence.ID, durationMs); err != nil {
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := setStoryHasAudio(db, id, true); err != nil {
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

