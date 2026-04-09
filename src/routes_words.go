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
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type wordImageWriter func(r *http.Request, info *wordImageInfo) (string, int, error)

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
		writeJSON(w, words)
	}
}

func apiUpdateWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
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
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
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
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
			return
		}
		if err := deleteWordByID(db, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleWordImageUpdate(db *sql.DB, w http.ResponseWriter, r *http.Request, writeImage wordImageWriter) {
	id, ok := parseRouteInt64(w, r, "id", "invalid id")
	if !ok {
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

	imagePath, status, err := writeImage(r, info)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	if err := updateWordImagePath(db, id, imagePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.ImagePath != nil && *info.ImagePath != "" && *info.ImagePath != imagePath {
		_ = os.Remove(filepath.Join("static", filepath.FromSlash(*info.ImagePath)))
	}

	writeJSON(w, map[string]string{"image_path": imagePath})
}

func persistWordAutoFill(db *sql.DB, wordID int64, filled *wordAutoFill) ([]kanjiDataEntry, error) {
	kd := make([]kanjiDataEntry, 0, len(filled.Kanji))
	for _, k := range filled.Kanji {
		kID, err := upsertKanji(db, k.Character, k.Meanings)
		if err != nil {
			continue
		}
		kd = append(kd, kanjiDataEntry{ID: kID, Reading: k.Reading})
	}
	b, _ := json.Marshal(kd)
	if err := updateWordFill(db, wordID, filled.Reading, filled.PitchAccent, filled.PartOfSpeech, filled.Meaning, filled.ExampleJP, filled.ExampleEN, string(b)); err != nil {
		return nil, err
	}
	return kd, nil
}

func apiUploadWordImage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleWordImageUpdate(db, w, r, func(r *http.Request, info *wordImageInfo) (string, int, error) {
			file, header, err := r.FormFile("image")
			if err != nil {
				return "", http.StatusBadRequest, errors.New("missing image")
			}
			defer file.Close()

			imagePath, err := saveUploadedWordImage(info.Word, header.Header.Get("Content-Type"), header.Filename, file)
			if err != nil {
				return "", http.StatusBadRequest, err
			}
			return imagePath, 0, nil
		})
	}
}

func apiDownloadWordImage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleWordImageUpdate(db, w, r, func(r *http.Request, info *wordImageInfo) (string, int, error) {
			var body struct {
				URL string `json:"url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return "", http.StatusBadRequest, errors.New("bad request")
			}
			body.URL = strings.TrimSpace(body.URL)
			if body.URL == "" {
				return "", http.StatusBadRequest, errors.New("missing url")
			}

			imagePath, err := downloadWordImage(r, info.Word, body.URL)
			if err != nil {
				return "", http.StatusBadGateway, err
			}
			return imagePath, 0, nil
		})
	}
}

func apiFindWordImage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleWordImageUpdate(db, w, r, func(r *http.Request, info *wordImageInfo) (string, int, error) {
			var body struct {
				Word        string `json:"word"`
				Meaning     string `json:"meaning"`
				AIModel     string `json:"ai_model"`
				ImageSource string `json:"image_source"` // "wikimedia" | "unsplash" | "pexels" | "pixabay"
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return "", http.StatusBadRequest, errors.New("bad request")
			}

			var (
				imageURL string
				err      error
			)
			switch body.ImageSource {
			case "unsplash", "pexels", "pixabay", "bing":
				query, qErr := suggestImageSearchQuery(db, body.Word, body.Meaning, body.AIModel)
				if qErr != nil {
					return "", http.StatusInternalServerError, qErr
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
					return "", http.StatusBadGateway, err
				}
			default: // "wikimedia"
				imageURL, err = suggestImageURL(db, body.Word, body.Meaning, body.AIModel)
				if err != nil {
					return "", http.StatusInternalServerError, err
				}
			}

			imagePath, err := downloadWordImage(r, info.Word, imageURL)
			if err != nil {
				return "", http.StatusBadGateway, err
			}
			return imagePath, 0, nil
		})
	}
}

func apiAutofillWord(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseRouteInt64(w, r, "id", "bad id")
		if !ok {
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
		filled, err := autoFillWord(db, body.Word, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		kd, err := persistWordAutoFill(db, id, filled)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"word":           body.Word,
			"reading":        filled.Reading,
			"pitch_accent":   filled.PitchAccent,
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
			writeJSON(w, []any{})
			return
		}

		wordStrings := make([]string, len(body.Words))
		for i, entry := range body.Words {
			wordStrings[i] = entry.Word
		}
		fills, err := autoFillWordsBatch(db, wordStrings, body.AIModel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type wordResult struct {
			WordID       int64            `json:"word_id"`
			Word         string           `json:"word"`
			Reading      string           `json:"reading,omitempty"`
			PitchAccent  *int             `json:"pitch_accent,omitempty"`
			PartOfSpeech string           `json:"part_of_speech,omitempty"`
			Meaning      string           `json:"meaning,omitempty"`
			ExampleJP    string           `json:"example_jp,omitempty"`
			ExampleEN    string           `json:"example_en,omitempty"`
			KanjiData    []kanjiDataEntry `json:"kanji_data,omitempty"`
			Error        string           `json:"error,omitempty"`
		}

		results := make([]wordResult, len(body.Words))
		for i, entry := range body.Words {
			filled := fills[i]
			if filled == nil {
				results[i] = wordResult{WordID: entry.ID, Word: entry.Word, Error: "AI did not return a result for this word"}
				continue
			}
			kd, err := persistWordAutoFill(db, entry.ID, filled)
			if err != nil {
				results[i] = wordResult{WordID: entry.ID, Word: entry.Word, Error: err.Error()}
				continue
			}
			results[i] = wordResult{
				WordID:       entry.ID,
				Word:         entry.Word,
				Reading:      filled.Reading,
				PitchAccent:  filled.PitchAccent,
				PartOfSpeech: filled.PartOfSpeech,
				Meaning:      filled.Meaning,
				ExampleJP:    filled.ExampleJP,
				ExampleEN:    filled.ExampleEN,
				KanjiData:    kd,
			}
		}

		writeJSON(w, results)
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

	imagePath, err := saveWordImageData(word, res.Header.Get("Content-Type"), imageURL, data)
	if err != nil {
		if strings.Contains(err.Error(), "image upload failed:") {
			return "", errors.New(strings.Replace(err.Error(), "image upload failed:", "image download failed:", 1))
		}
		return "", err
	}
	return imagePath, nil
}

func saveUploadedWordImage(word, contentType, rawName string, src multipart.File) (string, error) {
	const maxImageBytes = 10 << 20

	data, err := io.ReadAll(io.LimitReader(src, maxImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxImageBytes {
		return "", errors.New("image upload failed: file is too large")
	}

	return saveWordImageData(word, contentType, rawName, data)
}

func saveWordImageData(word, contentType, rawName string, data []byte) (string, error) {
	ext := imageExtension(contentType, rawName, data)
	if ext == "" {
		return "", errors.New("image upload failed: unsupported image format")
	}

	dir := filepath.Join("static", "images", "words")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	fileName := word + ext
	finalFSPath := filepath.Join(dir, fileName)
	if err := os.WriteFile(finalFSPath, data, 0o644); err != nil {
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

// wavDurationMs returns the audio duration in milliseconds by reading the WAV header.
// Assumes standard PCM WAV (44-byte header) as produced by VoiceVox.
func wavDurationMs(wav []byte) int64 {
	if len(wav) < 44 {
		return 0
	}
	byteRate := int64(uint32(wav[28]) | uint32(wav[29])<<8 | uint32(wav[30])<<16 | uint32(wav[31])<<24)
	dataSize := int64(uint32(wav[40]) | uint32(wav[41])<<8 | uint32(wav[42])<<16 | uint32(wav[43])<<24)
	if byteRate == 0 {
		return 0
	}
	return dataSize * 1000 / byteRate
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

// apiSynthesizeSentence synthesizes a single sentence via VoiceVox and returns OGG audio.
// The full sentence text is submitted as-is (no clause splitting at this stage).
func apiSynthesizeSentence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text            string   `json:"text"`
			Speaker         *int     `json:"speaker"`
			SpeedScale      *float64 `json:"speedScale"`
			IntonationScale *float64 `json:"intonationScale"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		text := strings.TrimSpace(body.Text)
		if text == "" {
			http.Error(w, "text is required", http.StatusBadRequest)
			return
		}
		p := defaultVoicevoxParams()
		if body.Speaker != nil {
			p.Speaker = *body.Speaker
		}
		if body.SpeedScale != nil {
			p.SpeedScale = *body.SpeedScale
		}
		if body.IntonationScale != nil {
			p.IntonationScale = *body.IntonationScale
		}
		wav, err := synthesizeVoicevox(r.Context(), text, p)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			http.Error(w, "voicevox error: "+err.Error(), http.StatusBadGateway)
			return
		}
		ogg, err := wavToOgg(r.Context(), wav)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			http.Error(w, "ffmpeg error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "audio/ogg")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(ogg) //nolint:errcheck
	}
}

func apiFfmpegAvailable() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := exec.LookPath("ffmpeg")
		writeJSON(w, map[string]bool{"available": err == nil})
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
			writeJSON(w, []any{})
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
		id, ok := parseRouteInt64(w, r, "id", "invalid id")
		if !ok {
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

		writeJSON(w, map[string]any{
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
		writeJSON(w, kanji)
	}
}
