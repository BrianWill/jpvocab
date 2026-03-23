// voicevox-generator is a web server that generates audio files via a locally
// running VoiceVox engine (http://localhost:50021) and serves a browser UI for
// entering text, selecting a voice, and playing back the results.
//
// Setup:
//   1. Download and run VoiceVox from https://voicevox.hiroshiba.jp/
//   2. Run: go run .
//   3. Open http://localhost:8080

package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed index.html style.css app.js
var static embed.FS

const (
	voicevoxBase = "http://localhost:50021"
	outputDir    = "output"
	addr         = ":8080"
)

type GenerateRequest struct {
	Texts           []string `json:"texts"`
	Speaker         int      `json:"speaker"`
	SpeedScale      float64  `json:"speedScale"`
	PitchScale      float64  `json:"pitchScale"`
	IntonationScale float64  `json:"intonationScale"`
	VolumeScale     float64  `json:"volumeScale"`
}

type GenerateResult struct {
	Text  string `json:"text"`
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

func main() {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("failed to create output dir: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", serveIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /api/speakers", proxySpeakers)
	mux.HandleFunc("POST /api/generate", handleGenerate)
	mux.Handle("GET /audio/", http.StripPrefix("/audio/", http.FileServer(http.Dir(outputDir))))

	log.Printf("Listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, _ := static.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func proxySpeakers(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(voicevoxBase + "/speakers")
	if err != nil {
		http.Error(w, "VoiceVox unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	ts := time.Now().UnixMilli()
	var results []GenerateResult
	idx := 0

	for _, text := range req.Texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		filename := fmt.Sprintf("%d_%d.wav", ts, idx)
		idx++
		outPath := filepath.Join(outputDir, filename)

		if err := synthesize(text, req, outPath); err != nil {
			results = append(results, GenerateResult{Text: text, Error: err.Error()})
		} else {
			results = append(results, GenerateResult{Text: text, URL: "/audio/" + filename})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func synthesize(text string, req GenerateRequest, outPath string) error {
	// Step 1: POST /audio_query — get prosody/timing data for the text
	qURL := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d",
		voicevoxBase, url.QueryEscape(text), req.Speaker)

	resp, err := http.Post(qURL, "application/json", nil)
	if err != nil {
		return fmt.Errorf("audio_query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("audio_query: %s", resp.Status)
	}

	var q map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		return fmt.Errorf("decode audio_query response: %w", err)
	}

	// Patch prosody fields before synthesis
	q["speedScale"] = req.SpeedScale
	q["pitchScale"] = req.PitchScale
	q["intonationScale"] = req.IntonationScale
	q["volumeScale"] = req.VolumeScale

	qJSON, err := json.Marshal(q)
	if err != nil {
		return err
	}

	// Step 2: POST /synthesis — render audio query to WAV
	sURL := fmt.Sprintf("%s/synthesis?speaker=%d", voicevoxBase, req.Speaker)

	resp2, err := http.Post(sURL, "application/json", bytes.NewReader(qJSON))
	if err != nil {
		return fmt.Errorf("synthesis: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("synthesis: %s", resp2.Status)
	}

	wav, err := io.ReadAll(resp2.Body)
	if err != nil {
		return fmt.Errorf("read WAV: %w", err)
	}

	return os.WriteFile(outPath, wav, 0644)
}
