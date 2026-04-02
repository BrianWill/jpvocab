package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
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
