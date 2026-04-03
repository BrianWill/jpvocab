package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

func apiDownloadWordImage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		body.URL = strings.TrimSpace(body.URL)
		if body.URL == "" {
			http.Error(w, "missing url", http.StatusBadRequest)
			return
		}

		info, err := getWordImageInfo(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if info == nil {
			http.Error(w, "word not found", http.StatusNotFound)
			return
		}

		imagePath, err := downloadWordImage(r, info.Word, body.URL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := updateWordImagePath(db, id, imagePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if info.ImagePath != nil && *info.ImagePath != "" && *info.ImagePath != imagePath {
			_ = os.Remove(filepath.Join("static", filepath.FromSlash(*info.ImagePath)))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"image_path": imagePath})
	}
}

func apiFindWordImage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body struct {
			Word        string `json:"word"`
			Meaning     string `json:"meaning"`
			AIModel     string `json:"ai_model"`
			ImageSource string `json:"image_source"` // "wikimedia" | "unsplash" | "pexels" | "pixabay"
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		info, err := getWordImageInfo(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if info == nil {
			http.Error(w, "word not found", http.StatusNotFound)
			return
		}

		var imageURL string
		switch body.ImageSource {
		case "unsplash", "pexels", "pixabay", "bing":
			query, qErr := suggestImageSearchQuery(body.Word, body.Meaning, body.AIModel)
			if qErr != nil {
				http.Error(w, qErr.Error(), http.StatusInternalServerError)
				return
			}
			switch body.ImageSource {
			case "unsplash":
				imageURL, err = searchUnsplash(r.Context(), query)
			case "pexels":
				imageURL, err = searchPexels(r.Context(), query)
			case "pixabay":
				imageURL, err = searchPixabay(r.Context(), query)
			case "bing":
				imageURL, err = searchBing(r.Context(), query)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		default: // "wikimedia"
			imageURL, err = suggestImageURL(body.Word, body.Meaning, body.AIModel)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		imagePath, err := downloadWordImage(r, info.Word, imageURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := updateWordImagePath(db, id, imagePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if info.ImagePath != nil && *info.ImagePath != "" && *info.ImagePath != imagePath {
			_ = os.Remove(filepath.Join("static", filepath.FromSlash(*info.ImagePath)))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"image_path": imagePath})
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

func apiAutofillWordsBatch(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Words []struct {
				ID   int64  `json:"id"`
				Word string `json:"word"`
			} `json:"words"`
			AIModel string `json:"ai_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(body.Words) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		wordStrings := make([]string, len(body.Words))
		for i, entry := range body.Words {
			wordStrings[i] = entry.Word
		}
		fills, err := autoFillWordsBatch(wordStrings, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type kdEntry struct {
			ID      int64  `json:"id"`
			Reading string `json:"reading"`
		}
		type wordResult struct {
			WordID       int64     `json:"word_id"`
			Word         string    `json:"word"`
			Reading      string    `json:"reading,omitempty"`
			PartOfSpeech string    `json:"part_of_speech,omitempty"`
			Meaning      string    `json:"meaning,omitempty"`
			ExampleJP    string    `json:"example_jp,omitempty"`
			ExampleEN    string    `json:"example_en,omitempty"`
			KanjiData    []kdEntry `json:"kanji_data,omitempty"`
			Error        string    `json:"error,omitempty"`
		}

		results := make([]wordResult, len(body.Words))
		for i, entry := range body.Words {
			filled := fills[i]
			if filled == nil {
				results[i] = wordResult{WordID: entry.ID, Word: entry.Word, Error: "AI did not return a result for this word"}
				continue
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
			if err := updateWordFill(db, entry.ID, filled.Reading, filled.PartOfSpeech, filled.Meaning, filled.ExampleJP, filled.ExampleEN, string(b)); err != nil {
				results[i] = wordResult{WordID: entry.ID, Word: entry.Word, Error: err.Error()}
				continue
			}
			results[i] = wordResult{
				WordID:       entry.ID,
				Word:         entry.Word,
				Reading:      filled.Reading,
				PartOfSpeech: filled.PartOfSpeech,
				Meaning:      filled.Meaning,
				ExampleJP:    filled.ExampleJP,
				ExampleEN:    filled.ExampleEN,
				KanjiData:    kd,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func downloadWordImage(r *http.Request, word, imageURL string) (string, error) {
	const maxImageBytes = 10 << 20

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "jpvocab/1.0 (https://github.com/BrianWill/jpvocab; image download bot)")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", errors.New("image download failed: " + res.Status)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxImageBytes {
		return "", errors.New("image download failed: file is too large")
	}

	ext := imageExtension(res.Header.Get("Content-Type"), imageURL, data)
	if ext == "" {
		return "", errors.New("image download failed: unsupported image format")
	}

	dir := filepath.Join("static", "images", "words")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, "download-*"+ext)
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	fileName := word + ext
	finalFSPath := filepath.Join(dir, fileName)
	if _, err := os.Stat(finalFSPath); err == nil {
		if err := os.Remove(finalFSPath); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(tmpName, finalFSPath); err != nil {
		return "", err
	}

	return path.Join("images", "words", fileName), nil
}

func imageExtension(contentType, rawURL string, data []byte) string {
	if ext := extensionForContentType(contentType); ext != "" {
		return ext
	}
	if ext := extensionForContentType(http.DetectContentType(data)); ext != "" {
		return ext
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	switch strings.ToLower(filepath.Ext(parsed.Path)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return strings.ToLower(filepath.Ext(parsed.Path))
	default:
		return ""
	}
}

func extensionForContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	switch strings.ToLower(mediaType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ""
	}
}

const voicevoxBase = "http://localhost:50021"

type voicevoxParams struct {
	Speaker         int     `json:"speaker"`
	SpeedScale      float64 `json:"speedScale"`
	IntonationScale float64 `json:"intonationScale"`
}

func defaultVoicevoxParams() voicevoxParams {
	return voicevoxParams{Speaker: 1, SpeedScale: 1.0, IntonationScale: 1.0}
}

// wavToOgg converts WAV bytes to OGG/Opus via ffmpeg (must be in PATH).
func wavToOgg(ctx context.Context, wav []byte) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", "64k",
		"-f", "ogg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(wav)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// synthesizeVoicevox calls the local VoiceVox engine and returns the WAV bytes.
func synthesizeVoicevox(ctx context.Context, text string, p voicevoxParams) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	qURL := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d",
		voicevoxBase, url.QueryEscape(text), p.Speaker)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, qURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voicevox audio_query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voicevox audio_query: %s", resp.Status)
	}
	var q map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		return nil, fmt.Errorf("decode audio_query: %w", err)
	}
	q["speedScale"] = p.SpeedScale
	q["intonationScale"] = p.IntonationScale

	qJSON, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	sURL := fmt.Sprintf("%s/synthesis?speaker=%d", voicevoxBase, p.Speaker)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, sURL, bytes.NewReader(qJSON))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("voicevox synthesis: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voicevox synthesis: %s", resp2.Status)
	}
	return io.ReadAll(resp2.Body)
}

func synthesizeVoicevoxToFile(ctx context.Context, text string, p voicevoxParams, destPath string) error {
	wav, err := synthesizeVoicevox(ctx, text, p)
	if err != nil {
		return err
	}
	ogg, err := wavToOgg(ctx, wav)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, ogg, 0o644)
}

func apiFfmpegAvailable() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := exec.LookPath("ffmpeg")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"available": err == nil})
	}
}

func apiVoicevoxSpeakers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, voicevoxBase+"/speakers", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			// VoiceVox not running — return empty list so the UI degrades gracefully.
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	}
}

func apiVoicevoxPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text            string  `json:"text"`
			Speaker         int     `json:"speaker"`
			SpeedScale      float64 `json:"speedScale"`
			IntonationScale float64 `json:"intonationScale"`
		}
		body.SpeedScale = 1.0
		body.IntonationScale = 1.0
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.Text == "" {
			body.Text = "日本語の音声合成のサンプルです。"
		}
		wav, err := synthesizeVoicevox(r.Context(), body.Text, voicevoxParams{
			Speaker:         body.Speaker,
			SpeedScale:      body.SpeedScale,
			IntonationScale: body.IntonationScale,
		})
		if err != nil {
			http.Error(w, "voicevox error: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(wav)
	}
}

func apiGenerateWordAudio(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p := defaultVoicevoxParams()
		// Body is optional — frontend may omit it for default settings.
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

		word, exampleJP, err := getWordAudioInfo(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if word == "" {
			http.Error(w, "word not found", http.StatusNotFound)
			return
		}

		audioDir := filepath.Join("static", "audio")
		wordPath := filepath.Join(audioDir, word+".ogg")
		sentencePath := filepath.Join(audioDir, word+"_sentence.ogg")

		if err := synthesizeVoicevoxToFile(r.Context(), word, p, wordPath); err != nil {
			http.Error(w, "voicevox error: "+err.Error(), http.StatusBadGateway)
			return
		}

		hasSentence := false
		if exampleJP != "" {
			if err := synthesizeVoicevoxToFile(r.Context(), exampleJP, p, sentencePath); err == nil {
				hasSentence = true
			}
		}

		if err := updateWordAudioFlags(db, id, true, hasSentence); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"hasWordAudio":     true,
			"hasSentenceAudio": hasSentence,
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
