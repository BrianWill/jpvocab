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

