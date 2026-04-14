package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func withURLParam(r *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func newMultipartRequest(t *testing.T, method, path string, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField(%q): %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func parseSSEBatchResults(t *testing.T, body string) []batchWordResult {
	t.Helper()
	parts := strings.Split(body, "\n\n")
	results := make([]batchWordResult, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(part, "data: ")
		if strings.Contains(payload, `"done":true`) {
			continue
		}
		var result batchWordResult
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatalf("unmarshal SSE payload %q: %v", payload, err)
		}
		results = append(results, result)
	}
	return results
}

func setupBackupTestEnv(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("static", "images", "words"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeTestWordImageFile(t *testing.T, relPath string) {
	t.Helper()
	fullPath := filepath.Join("static", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAPIUpdateWord_InvalidID(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/words/nope", bytes.NewBufferString(`{}`))
	req = withURLParam(req, "id", "nope")
	rec := httptest.NewRecorder()

	apiUpdateWord(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIUpdateWord_RejectsNonKanaReading(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "猫", 1)
	body := `{"reading":"taberu","type":"noun","meaning":"cat","exampleJp":"","exampleEn":"","target":1,"kanjiData":[]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/words/1", bytes.NewBufferString(body))
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiUpdateWord(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIUpdateWord_Success(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "犬", 1)
	body := `{"reading":"いぬ","type":"noun","meaning":"dog","exampleJp":"犬がいる。","exampleEn":"There is a dog.","target":4,"kanjiData":[{"id":1,"reading":"いぬ"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/words/1", bytes.NewBufferString(body))
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiUpdateWord(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	var reading string
	var target int
	if err := db.QueryRow(`SELECT reading, drill_target FROM words WHERE id = ?`, wordID).Scan(&reading, &target); err != nil {
		t.Fatal(err)
	}
	if reading != "いぬ" {
		t.Errorf("reading: got %q, want %q", reading, "いぬ")
	}
	if target != 4 {
		t.Errorf("target: got %d, want 4", target)
	}
}

func TestAPIUpdateWordTarget_BadJSON(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/words/1/target", bytes.NewBufferString(`{`))
	req = withURLParam(req, "id", "1")
	rec := httptest.NewRecorder()

	apiUpdateWordTarget(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIUpdateWordTarget_Success(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "鳥", 1)
	req := httptest.NewRequest(http.MethodPatch, "/api/words/1/target", bytes.NewBufferString(`{"target":6}`))
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiUpdateWordTarget(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	var target int
	if err := db.QueryRow(`SELECT drill_target FROM words WHERE id = ?`, wordID).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if target != 6 {
		t.Errorf("target: got %d, want 6", target)
	}
}

func TestAPIUploadWordImage_MissingImage(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "upload-missing", 1)
	req := httptest.NewRequest(http.MethodPost, "/api/words/1/upload-image", bytes.NewBuffer(nil))
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiUploadWordImage(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIUploadWordImage_Success(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "upload-success", 1)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "word.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/words/1/upload-image", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiUploadWordImage(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp struct {
		ImagePath string `json:"image_path"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ImagePath != "images/words/upload-success.png" {
		t.Fatalf("image_path: got %q", resp.ImagePath)
	}
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join("static", filepath.FromSlash(resp.ImagePath)))
	})

	var imagePath string
	if err := db.QueryRow(`SELECT image_path FROM words WHERE id = ?`, wordID).Scan(&imagePath); err != nil {
		t.Fatal(err)
	}
	if imagePath != resp.ImagePath {
		t.Fatalf("db image_path: got %q, want %q", imagePath, resp.ImagePath)
	}
	if _, err := os.Stat(filepath.Join("static", filepath.FromSlash(resp.ImagePath))); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestAdminAddWordsBatch_UsesCorrectPerWordWordListInfo(t *testing.T) {
	db := testDB(t)
	req := newMultipartRequest(t, http.MethodPost, "/admin/words/batch", map[string]string{
		"words":    "黄色\n白い\n犬\nサル",
		"autofill": "off",
	})
	rec := httptest.NewRecorder()

	adminAddWordsBatch(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	results := parseSSEBatchResults(t, rec.Body.String())
	if len(results) != 4 {
		t.Fatalf("result count: got %d, want 4 body=%q", len(results), rec.Body.String())
	}

	byWord := make(map[string]batchWordResult, len(results))
	for _, result := range results {
		byWord[result.Word] = result
	}

	if got := byWord["黄色"].Reading; got != "きいろ" {
		t.Fatalf("黄色 reading: got %q, want %q", got, "きいろ")
	}
	if got := byWord["黄色"].Meaning; got != "yellow" {
		t.Fatalf("黄色 meaning: got %q, want %q", got, "yellow")
	}
	if got := byWord["白い"].Reading; got != "しろい" {
		t.Fatalf("白い reading: got %q, want %q", got, "しろい")
	}
	if got := byWord["白い"].Meaning; got != "white" {
		t.Fatalf("白い meaning: got %q, want %q", got, "white")
	}
	if got := byWord["犬"].Reading; got != "いぬ" {
		t.Fatalf("犬 reading: got %q, want %q", got, "いぬ")
	}
	if got := byWord["犬"].Meaning; got != "dog" {
		t.Fatalf("犬 meaning: got %q, want %q", got, "dog")
	}
	if got := byWord["サル"].Reading; got != "サル" {
		t.Fatalf("サル reading: got %q, want %q", got, "サル")
	}
	if got := byWord["サル"].Meaning; got != "monkey" {
		t.Fatalf("サル meaning: got %q, want %q", got, "monkey")
	}
}

func TestAdminAddWordsBatch_PromotedTrackedZeroWordKeepsItsOwnInfo(t *testing.T) {
	db := testDB(t)

	if _, err := db.Exec(`
		INSERT INTO words (base_word, reading, part_of_speech, meaning, example_jp, example_en, drill_target, tracked)
		VALUES
			('黄色', 'きいろ', 'noun', 'yellow', '黄色の花が咲いている。', 'Yellow flowers are blooming.', 3, 0),
			('サル', 'サル', 'noun', 'monkey', '木の上でサルが遊んでいる。', 'Monkeys are playing in the tree.', 3, 1)
	`); err != nil {
		t.Fatal(err)
	}

	req := newMultipartRequest(t, http.MethodPost, "/admin/words/batch", map[string]string{
		"words":    "黄色",
		"autofill": "off",
	})
	rec := httptest.NewRecorder()

	adminAddWordsBatch(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	results := parseSSEBatchResults(t, rec.Body.String())
	if len(results) != 1 {
		t.Fatalf("result count: got %d, want 1 body=%q", len(results), rec.Body.String())
	}
	result := results[0]
	if !result.Added {
		t.Fatalf("added: got %v, want true", result.Added)
	}
	if result.Word != "黄色" {
		t.Fatalf("word: got %q, want %q", result.Word, "黄色")
	}
	if result.Reading != "きいろ" {
		t.Fatalf("reading: got %q, want %q", result.Reading, "きいろ")
	}
	if result.Meaning != "yellow" {
		t.Fatalf("meaning: got %q, want %q", result.Meaning, "yellow")
	}
}

func TestAPICreateDrillSession_BadJSON(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drill/sessions", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	apiCreateDrillSession(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPICreateDrillSession_Success(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drill/sessions", bytes.NewBufferString(`{"state":{"round":2}}`))
	rec := httptest.NewRecorder()

	apiCreateDrillSession(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == 0 {
		t.Fatal("expected non-zero session id")
	}
}

func TestAPIRecordDrillAnswer_InvalidSessionID(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drill/sessions/nope/answers", bytes.NewBufferString(`{}`))
	req = withURLParam(req, "id", "nope")
	rec := httptest.NewRecorder()

	apiRecordDrillAnswer(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIRecordDrillAnswer_BadJSON(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drill/sessions/1/answers", bytes.NewBufferString(`{`))
	req = withURLParam(req, "id", "1")
	rec := httptest.NewRecorder()

	apiRecordDrillAnswer(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIRecordDrillAnswer_Success(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "海", 2)
	sessionID, err := createDrillSession(db, drillSessionState{Round: 1, Pool: []wordJSON{{ID: wordID, Word: "海"}}})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"wordId":` + int64ToString(wordID) + `,"correct":true,"state":{"round":1,"pool":[{"id":` + int64ToString(wordID) + `,"word":"海"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/drill/sessions/1/answers", bytes.NewBufferString(body))
	req = withURLParam(req, "id", int64ToString(sessionID))
	rec := httptest.NewRecorder()

	apiRecordDrillAnswer(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	var drillCount int
	if err := db.QueryRow(`SELECT drill_count FROM words WHERE id = ?`, wordID).Scan(&drillCount); err != nil {
		t.Fatal(err)
	}
	if drillCount != 1 {
		t.Errorf("drill_count: got %d, want 1", drillCount)
	}
}

func TestAPIUpdateDrillSessionState_Success(t *testing.T) {
	db := testDB(t)
	sessionID, err := createDrillSession(db, drillSessionState{Round: 1})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/drill/sessions/1/state", bytes.NewBufferString(`{"state":{"round":2,"remaining":[{"id":1,"word":"海"}],"skipAnswerReveal":false,"awaitingAdvance":true}}`))
	req = withURLParam(req, "id", int64ToString(sessionID))
	rec := httptest.NewRecorder()

	apiUpdateDrillSessionState(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	current, err := getCurrentDrillSession(db)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil {
		t.Fatal("expected current session")
	}
	if current.State.Round != 2 {
		t.Errorf("round: got %d, want 2", current.State.Round)
	}
	if current.State.SkipAnswerReveal == nil || *current.State.SkipAnswerReveal {
		t.Error("SkipAnswerReveal: got true/nil, want false")
	}
	if !current.State.AwaitingAdvance {
		t.Error("AwaitingAdvance: got false, want true")
	}
}

func TestAPIGetDrillSettings_ReturnsDefaults(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/drill", nil)
	rec := httptest.NewRecorder()

	apiGetDrillSettings(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var settings drillSettings
	if err := json.NewDecoder(rec.Body).Decode(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 100 || settings.RoundSize != 10 {
		t.Errorf("settings: got %+v", settings)
	}
	if settings.SkipAnswerReveal {
		t.Error("SkipAnswerReveal: got true, want false")
	}
	if settings.MatchingPairsMode {
		t.Error("MatchingPairsMode: got true, want false")
	}
}

func TestAPIGetWords_EmptyReturnsArray(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/words", nil)
	rec := httptest.NewRecorder()

	apiGetWords(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var words []wordJSON
	if err := json.NewDecoder(rec.Body).Decode(&words); err != nil {
		t.Fatal(err)
	}
	if words == nil {
		t.Error("expected [] not null for empty lexicon")
	}
}

func TestAPIGetWords_ReturnsInsertedWord(t *testing.T) {
	db := testDB(t)
	insertWord(db, "桜", "さくら", "noun", "cherry blossom", "", "", "", 2)
	req := httptest.NewRequest(http.MethodGet, "/api/words", nil)
	rec := httptest.NewRecorder()

	apiGetWords(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var words []wordJSON
	if err := json.NewDecoder(rec.Body).Decode(&words); err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d", len(words))
	}
	if words[0].Word != "桜" {
		t.Errorf("word: got %q, want 桜", words[0].Word)
	}
	if words[0].KanjiData == nil {
		t.Error("KanjiData should be [] not nil in JSON response")
	}
}

func TestAPIGetWords_PagedBadOffset(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/words?offset=-1&limit=50", nil)
	rec := httptest.NewRecorder()

	apiGetWords(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIGetWords_PagedReturnsItemsAndTotals(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`
		INSERT INTO words (base_word, reading, part_of_speech, meaning, drill_count, incorrect_count, drill_target, created_at, tracked)
		VALUES
			('りんご', 'りんご', 'noun', 'apple', 1, 0, 3, '2024-01-01T00:00:00Z', 1),
			('ばなな', 'ばなな', 'noun', 'banana', 3, 1, 3, '2024-02-01T00:00:00Z', 1),
			('さくらんぼ', 'さくらんぼ', 'noun', 'cherry', 0, 2, 4, '2024-03-01T00:00:00Z', 1)
	`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/words?sort=added&dir=desc&offset=0&limit=2", nil)
	rec := httptest.NewRecorder()

	apiGetWords(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	var page wordListPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 {
		t.Fatalf("total: got %d, want 3", page.Total)
	}
	if page.ActiveTotal != 2 {
		t.Fatalf("activeTotal: got %d, want 2", page.ActiveTotal)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items: got %d, want 2", len(page.Items))
	}
	if got := []string{page.Items[0].Word, page.Items[1].Word}; fmt.Sprint(got) != fmt.Sprint([]string{"さくらんぼ", "ばなな"}) {
		t.Fatalf("words: got %v", got)
	}
}

func TestAPIGetWords_PagedSortsByReadingAcrossFullSet(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`
		INSERT INTO words (base_word, reading, part_of_speech, meaning, drill_target, created_at, tracked)
		VALUES
			('猫', 'ねこ', 'noun', 'cat', 2, '2024-01-01T00:00:00Z', 1),
			('犬', 'いぬ', 'noun', 'dog', 2, '2024-02-01T00:00:00Z', 1),
			('鳥', 'とり', 'noun', 'bird', 2, '2024-03-01T00:00:00Z', 1)
	`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/words?sort=reading&dir=asc&offset=0&limit=2", nil)
	rec := httptest.NewRecorder()

	apiGetWords(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var page wordListPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if got := []string{page.Items[0].Word, page.Items[1].Word}; fmt.Sprint(got) != fmt.Sprint([]string{"犬", "鳥"}) {
		t.Fatalf("words: got %v", got)
	}
}

func TestAPIGetStories_ReturnsTitle(t *testing.T) {
	db := testDB(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stories", nil)

	title := "Garden Story"
	_, err := insertStory(db, title, []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "庭園", BaseWord: "庭園"},
			},
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	apiGetStories(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var stories []storyJSON
	if err := json.NewDecoder(rec.Body).Decode(&stories); err != nil {
		t.Fatal(err)
	}
	if len(stories) != 1 {
		t.Fatalf("expected 1 story, got %d", len(stories))
	}
	if stories[0].Title != title {
		t.Errorf("title: got %q, want %q", stories[0].Title, title)
	}
	if stories[0].SentenceCount != 1 {
		t.Errorf("sentenceCount: got %d, want 1", stories[0].SentenceCount)
	}
	if stories[0].LexiconWordCount != 1 {
		t.Errorf("lexiconWordCount: got %d, want 1", stories[0].LexiconWordCount)
	}
}

func TestAPICreateStory_BadJSON(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/stories", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	apiCreateStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPICreateStory_Success(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/stories", bytes.NewBufferString("{\"title\":\"New Story\",\"content\":\"皆さん、こんにちは。今日は庭園に行きます。\\n\\nとても静かです。\"}"))
	rec := httptest.NewRecorder()

	apiCreateStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
	var story storyJSON
	if err := json.NewDecoder(rec.Body).Decode(&story); err != nil {
		t.Fatal(err)
	}
	if story.Title != "New Story" {
		t.Fatalf("title: got %q, want %q", story.Title, "New Story")
	}
	if story.CreatedAt == "" {
		t.Fatal("expected createdAt timestamp")
	}
	parseDBDateTime(t, story.CreatedAt)
	if len(story.Sentences) != 3 {
		t.Fatalf("sentences: got %d, want 3", len(story.Sentences))
	}
	if story.SentenceCount != 3 {
		t.Fatalf("sentenceCount: got %d, want 3", story.SentenceCount)
	}
	wantLexiconWordCount := storyLexiconWordCount(buildStorySentencesFromText("皆さん、こんにちは。今日は庭園に行きます。\n\nとても静かです。"))
	if story.LexiconWordCount != wantLexiconWordCount {
		t.Fatalf("lexiconWordCount: got %d, want %d", story.LexiconWordCount, wantLexiconWordCount)
	}
	if story.Sentences[0].ChunkPosition != 1 || story.Sentences[1].ChunkPosition != 1 || story.Sentences[2].ChunkPosition != 1 {
		t.Errorf("chunkPositions: got %d, %d, %d; want 1, 1, 1",
			story.Sentences[0].ChunkPosition, story.Sentences[1].ChunkPosition, story.Sentences[2].ChunkPosition)
	}
	if !story.Sentences[0].IsParagraphStart {
		t.Fatal("sentence 1 should start a paragraph")
	}
	if story.Sentences[1].IsParagraphStart {
		t.Fatal("sentence 2 should not start a paragraph")
	}
	if !story.Sentences[2].IsParagraphStart {
		t.Fatal("sentence 3 should start a new paragraph")
	}

	var activityCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE event_type = ? AND entity_id = ? AND summary = ?`, activityEventStoryCreated, story.ID, "New Story").Scan(&activityCount); err != nil {
		t.Fatal(err)
	}
	if activityCount != 1 {
		t.Fatalf("expected 1 story activity event, got %d", activityCount)
	}
}

func TestAPICreateStory_ClassifiesMixedLanguageSentences(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPost, "/api/stories", bytes.NewBufferString(`{"title":"Mixed Story","content":"This is a test.\n\n今日はmeetingがあります。"}`))
	rec := httptest.NewRecorder()

	apiCreateStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
	var story storyJSON
	if err := json.NewDecoder(rec.Body).Decode(&story); err != nil {
		t.Fatal(err)
	}
	if len(story.Sentences) != 2 {
		t.Fatalf("sentences: got %d, want 2", len(story.Sentences))
	}
	if story.Sentences[0].OrigLang != "en" || story.Sentences[0].ENText == nil || *story.Sentences[0].ENText != "This is a test." {
		t.Fatalf("sentence 1: got %+v", story.Sentences[0])
	}
	if story.Sentences[1].OrigLang != "jp" || story.Sentences[1].JPText == nil || *story.Sentences[1].JPText != "今日はmeetingがあります。" {
		t.Fatalf("sentence 2: got %+v", story.Sentences[1])
	}
}

func TestAPIGetStory_ReturnsStoryByID(t *testing.T) {
	db := testDB(t)
	title := "Garden Story"
	id, err := insertStory(db, title, []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}, OrigLang: "jp", IsParagraphStart: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/stories/1", nil)
	req = withURLParam(req, "id", int64ToString(id))
	rec := httptest.NewRecorder()

	apiGetStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var story storyJSON
	if err := json.NewDecoder(rec.Body).Decode(&story); err != nil {
		t.Fatal(err)
	}
	if story.ID != id {
		t.Errorf("id: got %d, want %d", story.ID, id)
	}
	if story.Title != title {
		t.Errorf("title: got %q, want %q", story.Title, title)
	}
	if story.SentenceCount != 1 {
		t.Errorf("sentenceCount: got %d, want 1", story.SentenceCount)
	}
	if story.LexiconWordCount != 1 {
		t.Errorf("lexiconWordCount: got %d, want 1", story.LexiconWordCount)
	}
	if story.Sentences[0].ChunkPosition != 1 {
		t.Errorf("chunkPosition: got %d, want 1", story.Sentences[0].ChunkPosition)
	}
}

func TestAPICreateStory_SplitsLongStoryIntoChunks(t *testing.T) {
	db := testDB(t)
	longSentenceA := strings.Repeat("あ", 100) + "。"
	longSentenceB := strings.Repeat("い", 100) + "。"
	longSentenceC := strings.Repeat("う", 100) + "。"
	body := "{\"title\":\"Long Story\",\"content\":\"" + longSentenceA + longSentenceB + longSentenceC + "\"}"
	req := httptest.NewRequest(http.MethodPost, "/api/stories", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	apiCreateStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
	var story storyJSON
	if err := json.NewDecoder(rec.Body).Decode(&story); err != nil {
		t.Fatal(err)
	}
	// 3 sentences of 100+ chars each: first two fit in chunk 1, third overflows into chunk 2.
	if len(story.Sentences) != 3 {
		t.Fatalf("sentences: got %d, want 3", len(story.Sentences))
	}
	if story.Sentences[0].ChunkPosition != 1 || story.Sentences[1].ChunkPosition != 1 {
		t.Errorf("chunk 1 sentences: got chunkPositions %d, %d; want 1, 1",
			story.Sentences[0].ChunkPosition, story.Sentences[1].ChunkPosition)
	}
	if story.Sentences[2].ChunkPosition != 2 {
		t.Errorf("chunk 2 sentence: got chunkPosition %d, want 2", story.Sentences[2].ChunkPosition)
	}
}

func TestAPIDeleteStory_InvalidID(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/stories/nope", nil)
	req = withURLParam(req, "id", "nope")
	rec := httptest.NewRecorder()

	apiDeleteStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIDeleteStory_Success(t *testing.T) {
	db := testDB(t)
	id, err := insertStory(db, "Garden Story", []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}, OrigLang: "jp", IsParagraphStart: true},
		{Words: []storyWordInput{{DisplayWord: "歩く", BaseWord: "歩く"}}, OrigLang: "jp", IsParagraphStart: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/stories/1", nil)
	req = withURLParam(req, "id", int64ToString(id))
	rec := httptest.NewRecorder()

	apiDeleteStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	var storyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stories WHERE id = ?`, id).Scan(&storyCount); err != nil {
		t.Fatal(err)
	}
	if storyCount != 0 {
		t.Fatalf("story should be deleted after successful DELETE, got count %d", storyCount)
	}

	var sentenceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM story_sentences WHERE story_id = ?`, id).Scan(&sentenceCount); err != nil {
		t.Fatal(err)
	}
	if sentenceCount != 0 {
		t.Fatalf("story sentences should be deleted after successful DELETE, got count %d", sentenceCount)
	}

	var activityCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE event_type = ? AND entity_id = ?`, activityEventStoryCreated, id).Scan(&activityCount); err != nil {
		t.Fatal(err)
	}
	if activityCount != 1 {
		t.Fatalf("story activity history should remain after delete, got count %d", activityCount)
	}
}

func TestAPITutorChat_LogsUserMessageActivity(t *testing.T) {
	db := testDB(t)
	id, err := insertTutorPrompt(db, "Free Conversation", "prompt", "", "ja")
	if err != nil {
		t.Fatal(err)
	}
	origTutorChatFn := tutorChatFn
	tutorChatFn = func(_ *sql.DB, _ []message, _ string, _ string) (string, error) {
		return `{"jp":"はい","en":"Yes"}`, nil
	}
	t.Cleanup(func() { tutorChatFn = origTutorChatFn })

	body := fmt.Sprintf(`{"ai_model":"openai:gpt","tutor_mode":"%d","messages":[{"role":"user","content":"こんにちは"}]}`, id)
	req := httptest.NewRequest(http.MethodPost, "/api/tutor/chat", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	apiTutorChat(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	cal, err := getActivityCalendar(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(cal.Days[cal.Today].TutorMessages) != 1 {
		t.Fatalf("expected 1 tutor activity entry, got %d", len(cal.Days[cal.Today].TutorMessages))
	}
	if cal.Days[cal.Today].TutorMessages[0].Mode != "Free Conversation" {
		t.Fatalf("mode: got %q, want %q", cal.Days[cal.Today].TutorMessages[0].Mode, "Free Conversation")
	}
}

func TestAPITutorChat_DoesNotLogOpeningTurn(t *testing.T) {
	db := testDB(t)
	id, err := insertTutorPrompt(db, "Free Conversation", "prompt", "", "ja")
	if err != nil {
		t.Fatal(err)
	}
	origTutorChatFn := tutorChatFn
	tutorChatFn = func(_ *sql.DB, _ []message, _ string, _ string) (string, error) {
		return `{"jp":"はじめまして","en":"Nice to meet you"}`, nil
	}
	t.Cleanup(func() { tutorChatFn = origTutorChatFn })

	body := fmt.Sprintf(`{"ai_model":"openai:gpt","tutor_mode":"%d","messages":[]}`, id)
	req := httptest.NewRequest(http.MethodPost, "/api/tutor/chat", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	apiTutorChat(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE event_type = ?`, activityEventTutorUserMessage).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no tutor activity events, got %d", count)
	}
}

func TestAPIStoryNotedWords_AddAndRemove(t *testing.T) {
	db := testDB(t)
	id, err := insertStory(db, "Garden Story", []storySentenceInput{
		{
			Words: []storyWordInput{
				{DisplayWord: "庭園", BaseWord: "庭園"},
				{DisplayWord: "を", BaseWord: "を"},
				{DisplayWord: "歩く", BaseWord: "歩く"},
			},
			OrigLang:         "jp",
			IsParagraphStart: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	addReq := httptest.NewRequest(http.MethodPost, "/api/stories/1/noted-words", bytes.NewBufferString(`{"baseWord":"歩く","displayWord":"歩く"}`))
	addReq = withURLParam(addReq, "id", int64ToString(id))
	addRec := httptest.NewRecorder()
	apiAddStoryNotedWord(db).ServeHTTP(addRec, addReq)

	if addRec.Code != http.StatusOK {
		t.Fatalf("add status: got %d, want %d", addRec.Code, http.StatusOK)
	}
	var addBody struct {
		NotedWords []storyNotedWordJSON `json:"notedWords"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addBody); err != nil {
		t.Fatal(err)
	}
	if len(addBody.NotedWords) != 1 || addBody.NotedWords[0].BaseWord != "歩く" {
		t.Fatalf("unexpected add response: %+v", addBody.NotedWords)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/stories/1/noted-words", bytes.NewBufferString(`{"baseWord":"歩く"}`))
	delReq = withURLParam(delReq, "id", int64ToString(id))
	delRec := httptest.NewRecorder()
	apiDeleteStoryNotedWord(db).ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status: got %d, want %d", delRec.Code, http.StatusOK)
	}
	var delBody struct {
		NotedWords []storyNotedWordJSON `json:"notedWords"`
	}
	if err := json.NewDecoder(delRec.Body).Decode(&delBody); err != nil {
		t.Fatal(err)
	}
	if len(delBody.NotedWords) != 0 {
		t.Fatalf("expected no noted words after delete, got %+v", delBody.NotedWords)
	}
}

func TestAPIGetStory_InvalidID(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stories/nope", nil)
	req = withURLParam(req, "id", "nope")
	rec := httptest.NewRecorder()

	apiGetStory(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRenderAppPage_RendersHTMLAndSharedNav(t *testing.T) {
	rec := httptest.NewRecorder()

	renderAppPage(rec, "static/activity.html", appPageData{CurrentPage: "activity"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if strings.TrimSpace(body) == "" {
		t.Fatal("expected rendered HTML body, got empty response")
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected doctype in rendered page")
	}
	if !strings.Contains(body, `href="/welcome">語</a>`) {
		t.Error("expected shared app logo link in rendered page")
	}
	if !strings.Contains(body, `href="/stories">Stories</a>`) {
		t.Error("expected shared stories nav link in rendered page")
	}
	if !strings.Contains(body, `nav-dropdown-item nav-dropdown-item--current" href="/activity"`) {
		t.Error("expected current-page nav styling for activity page")
	}
}

func TestAPIGetWordInfo_ReturnsWordInfo(t *testing.T) {
	db := testDB(t)
	_, err := insertStory(db, "Word Info Story", []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "猫", BaseWord: "猫"}}, OrigLang: "jp", IsParagraphStart: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE words SET meaning = 'cat', reading = 'ねこ' WHERE base_word = '猫'`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/word-info?base=%E7%8C%AB", nil)
	rec := httptest.NewRecorder()
	apiGetWordInfo(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var info wordInfoResponseJSON
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info.English != "cat" {
		t.Errorf("english: got %q, want %q", info.English, "cat")
	}
	if info.Reading != "ねこ" {
		t.Errorf("reading: got %q, want %q", info.Reading, "ねこ")
	}
	if info.WordID == 0 {
		t.Error("expected non-zero wordId")
	}
}

func TestAPIGetWordInfo_EnrichesKanjiData(t *testing.T) {
	db := testDB(t)
	kanjiID, err := upsertKanji(db, "猫", []string{"cat", "feline"}, []string{"ビョウ", "ねこ"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO words (base_word, meaning, reading, kanji_data, tracked) VALUES (?, ?, ?, ?, 1)`, "猫", "cat", "ねこ", fmt.Sprintf(`[{"id":%d,"reading":"ねこ"}]`, kanjiID)); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/word-info?base=%E7%8C%AB", nil)
	rec := httptest.NewRecorder()
	apiGetWordInfo(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var info wordInfoResponseJSON
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if len(info.KanjiData) != 1 {
		t.Fatalf("expected 1 kanji entry, got %d", len(info.KanjiData))
	}
	if info.KanjiData[0].Character != "猫" {
		t.Errorf("character: got %q, want 猫", info.KanjiData[0].Character)
	}
	if len(info.KanjiData[0].Meanings) != 2 {
		t.Errorf("meanings: got %v, want two meanings", info.KanjiData[0].Meanings)
	}
	if len(info.KanjiData[0].Readings) != 2 {
		t.Errorf("readings: got %v, want two readings", info.KanjiData[0].Readings)
	}
}

func TestAPIGetWordInfoBatch(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO words (base_word, meaning, reading, tracked) VALUES ('猫', 'cat', 'ねこ', 1)`); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"bases": []string{"猫", "未知"}})
	req := httptest.NewRequest(http.MethodPost, "/api/word-info-batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	apiGetWordInfoBatch(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var result wordInfoBatchResponseJSON
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	cat := result.Words["猫"]
	if cat.English != "cat" {
		t.Errorf("english: got %q, want %q", cat.English, "cat")
	}
	if cat.WordID == 0 {
		t.Error("expected non-zero wordId for 猫")
	}
	if _, ok := result.Words["未知"]; ok {
		t.Error("expected 未知 to be absent from result (not in DB)")
	}
}

func TestAPIGetWordInfoBatch_EnrichesKanjiData(t *testing.T) {
	db := testDB(t)
	kanjiID, err := upsertKanji(db, "猫", []string{"cat", "feline"}, []string{"ビョウ", "ねこ"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO words (base_word, meaning, reading, kanji_data, tracked) VALUES (?, ?, ?, ?, 1)`, "猫", "cat", "ねこ", fmt.Sprintf(`[{"id":%d,"reading":"ねこ"}]`, kanjiID)); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"bases": []string{"猫"}})
	req := httptest.NewRequest(http.MethodPost, "/api/word-info-batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	apiGetWordInfoBatch(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var result wordInfoBatchResponseJSON
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	cat := result.Words["猫"]
	if len(cat.KanjiData) != 1 {
		t.Fatalf("expected 1 kanji entry, got %d", len(cat.KanjiData))
	}
	if cat.KanjiData[0].Character != "猫" {
		t.Errorf("character: got %q, want 猫", cat.KanjiData[0].Character)
	}
	if len(cat.KanjiData[0].Meanings) != 2 {
		t.Errorf("meanings: got %v, want two meanings", cat.KanjiData[0].Meanings)
	}
	if len(cat.KanjiData[0].Readings) != 2 {
		t.Errorf("readings: got %v, want two readings", cat.KanjiData[0].Readings)
	}
}

func TestAPIGetWordInfo_NotFound(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/word-info?base=%E7%8C%AB", nil)
	rec := httptest.NewRecorder()
	apiGetWordInfo(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var info wordInfoResponseJSON
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info.WordID != 0 {
		t.Errorf("expected zero wordId for unknown word, got %d", info.WordID)
	}
}

func TestAPIDeleteWord_InvalidID(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/words/nope", nil)
	req = withURLParam(req, "id", "nope")
	rec := httptest.NewRecorder()

	apiDeleteWord(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPIDeleteWord_Success(t *testing.T) {
	db := testDB(t)
	wordID := insertTestWord(t, db, "葉", 1)
	req := httptest.NewRequest(http.MethodDelete, "/api/words/1", nil)
	req = withURLParam(req, "id", int64ToString(wordID))
	rec := httptest.NewRecorder()

	apiDeleteWord(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM words WHERE id = ?`, wordID).Scan(&count)
	if count != 0 {
		t.Errorf("word should be deleted after successful DELETE, got count %d", count)
	}
}

func TestAPIGetKanji_EmptyReturnsArray(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/kanji", nil)
	rec := httptest.NewRecorder()

	apiGetKanji(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var kanji []kanjiJSON
	if err := json.NewDecoder(rec.Body).Decode(&kanji); err != nil {
		t.Fatal(err)
	}
	if kanji == nil {
		t.Error("expected [] not null for empty kanji table")
	}
}

func TestAPIGetKanji_ReturnsInserted(t *testing.T) {
	db := testDB(t)
	upsertKanji(db, "木", []string{"tree", "wood"}, []string{"モク", "き"})
	req := httptest.NewRequest(http.MethodGet, "/api/kanji", nil)
	rec := httptest.NewRecorder()

	apiGetKanji(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var kanji []kanjiJSON
	if err := json.NewDecoder(rec.Body).Decode(&kanji); err != nil {
		t.Fatal(err)
	}
	if len(kanji) != 1 {
		t.Fatalf("expected 1 kanji, got %d", len(kanji))
	}
	if kanji[0].Character != "木" {
		t.Errorf("character: got %q, want 木", kanji[0].Character)
	}
	if len(kanji[0].Meanings) != 2 {
		t.Errorf("meanings: got %v, want [tree wood]", kanji[0].Meanings)
	}
	if len(kanji[0].Readings) != 2 {
		t.Errorf("readings: got %v, want [モク き]", kanji[0].Readings)
	}
}

func TestAPIPutDrillSettings_RoundTrips(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/drill", bytes.NewBufferString(`{"maxWords":12,"roundSize":5,"wordTypes":["verbs","other"],"skipAnswerReveal":false,"matchingPairsMode":true}`))
	rec := httptest.NewRecorder()

	apiPutDrillSettings(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	settings, err := getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxWords != 12 || settings.RoundSize != 5 {
		t.Errorf("settings: got %+v", settings)
	}
	if settings.SkipAnswerReveal {
		t.Error("SkipAnswerReveal: got true, want false")
	}
	if !settings.MatchingPairsMode {
		t.Error("MatchingPairsMode: got false, want true")
	}
}

func TestAPIGetTokenUsage_EmptyReturnsStructure(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/token-usage", nil)
	rec := httptest.NewRecorder()

	apiGetTokenUsage(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp struct {
		Totals struct {
			Calls        int `json:"calls"`
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"totals"`
		Summary []tokenUsageSummaryRow `json:"summary"`
		Log     []tokenUsageEntry      `json:"log"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Summary == nil {
		t.Error("summary should be [] not null for empty table")
	}
	if resp.Log == nil {
		t.Error("log should be [] not null for empty table")
	}
	if resp.Totals.Calls != 0 || resp.Totals.InputTokens != 0 || resp.Totals.OutputTokens != 0 {
		t.Errorf("expected all-zero totals for empty DB, got %+v", resp.Totals)
	}
}

func TestAPICreateBackup_CreatesFiles(t *testing.T) {
	setupBackupTestEnv(t)
	db := testDB(t)

	wordID := insertTestWord(t, db, "保存", 3)
	if err := updateWordImagePath(db, wordID, "images/words/保存.png"); err != nil {
		t.Fatal(err)
	}
	writeTestWordImageFile(t, "images/words/保存.png")

	req := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	rec := httptest.NewRecorder()

	apiCreateBackup(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var manifest backupManifest
	if err := json.NewDecoder(rec.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BackupID == "" {
		t.Fatal("expected backup id")
	}
	if _, err := os.Stat(filepath.Join("backups", manifest.BackupID, "manifest.json")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join("backups", manifest.BackupID, "data.json")); err != nil {
		t.Fatalf("data missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join("backups", manifest.BackupID, "images", "words", "保存.png")); err != nil {
		t.Fatalf("image backup missing: %v", err)
	}
}

func TestAPIListBackups_ReturnsNewestFirst(t *testing.T) {
	setupBackupTestEnv(t)
	db := testDB(t)

	insertTestWord(t, db, "最初", 1)
	first, err := createBackup(db)
	if err != nil {
		t.Fatal(err)
	}
	insertTestWord(t, db, "次", 1)
	second, err := createBackup(db)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rec := httptest.NewRecorder()
	apiListBackups().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp struct {
		Backups []backupListItem `json:"backups"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Backups) != 2 {
		t.Fatalf("backup count: got %d, want 2", len(resp.Backups))
	}
	if resp.Backups[0].ID != second.BackupID || resp.Backups[1].ID != first.BackupID {
		t.Fatalf("backup order: got %+v", resp.Backups)
	}
	if resp.Backups[0].Counts["words"] < resp.Backups[1].Counts["words"] {
		t.Fatalf("expected newer backup to include at least as many words: %+v", resp.Backups)
	}
}

func TestAPIRestoreBackup_RoundTripRestoresDataAndImages(t *testing.T) {
	setupBackupTestEnv(t)
	db := testDB(t)

	wordID := insertTestWord(t, db, "戻る", 2)
	if err := updateWordImagePath(db, wordID, "images/words/戻る.png"); err != nil {
		t.Fatal(err)
	}
	writeTestWordImageFile(t, "images/words/戻る.png")

	if _, err := insertTutorPrompt(db, "Backup Prompt", "system", "hello", "en"); err != nil {
		t.Fatal(err)
	}
	if err := putDrillSettings(db, drillSettings{
		MaxWords:          15,
		RoundSize:         4,
		WordTypes:         []string{"verbs", "nouns"},
		NewWordTarget:     7,
		SkipAnswerReveal:  true,
		MatchingPairsMode: true,
	}); err != nil {
		t.Fatal(err)
	}
	storyID, err := insertStory(db, "Backup Story", []storySentenceInput{{
		Words:            []storyWordInput{{DisplayWord: "戻る", BaseWord: "戻る"}},
		JPText:           ptrString("戻る。"),
		OrigLang:         "jp",
		IsParagraphStart: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	state := drillSessionState{
		PoolSize:     1,
		RoundSize:    1,
		Round:        1,
		Pool:         []wordJSON{{ID: wordID, Word: "戻る"}},
		Remaining:    []wordJSON{},
		Redo:         []wordJSON{},
		SidebarItems: []drillSidebarItem{},
	}
	sessionID, err := createDrillSession(db, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := recordDrillAnswer(db, sessionID, wordID, true, state); err != nil {
		t.Fatal(err)
	}
	insertTokenUsage(db, "openai", "gpt-4o-mini", "backup-test", 10, 3)
	if err := insertActivityEvent(db, activityEventTutorUserMessage, nil, "Backup Event", activityEventMeta{Mode: "free"}); err != nil {
		t.Fatal(err)
	}

	manifest, err := createBackup(db)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`DELETE FROM story_sentences`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM stories`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM drill_answers`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM drill_sessions`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM words`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM tutor_prompts`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM user_settings`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM activity_events`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM token_usage`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join("static", "images", "words", "戻る.png")); err != nil {
		t.Fatal(err)
	}
	writeTestWordImageFile(t, "images/words/extra.png")

	body := bytes.NewBufferString(`{"createSafetyBackup":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/backups/"+manifest.BackupID+"/restore", body)
	req = withURLParam(req, "id", manifest.BackupID)
	rec := httptest.NewRecorder()
	apiRestoreBackup(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	var restoredWordCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM words WHERE base_word = '戻る'`).Scan(&restoredWordCount); err != nil {
		t.Fatal(err)
	}
	if restoredWordCount != 1 {
		t.Fatalf("restored word count: got %d, want 1", restoredWordCount)
	}
	var restoredStoryCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stories WHERE id = ?`, storyID).Scan(&restoredStoryCount); err != nil {
		t.Fatal(err)
	}
	if restoredStoryCount != 1 {
		t.Fatalf("restored story count: got %d, want 1", restoredStoryCount)
	}
	settings, err := getDrillSettings(db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.NewWordTarget != 7 || !settings.SkipAnswerReveal || !settings.MatchingPairsMode {
		t.Fatalf("restored settings: %+v", settings)
	}
	if _, err := os.Stat(filepath.Join("static", "images", "words", "戻る.png")); err != nil {
		t.Fatalf("restored image missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join("static", "images", "words", "extra.png")); !os.IsNotExist(err) {
		t.Fatalf("extra image should be removed on restore, err=%v", err)
	}
}

func TestAPIRestoreBackup_RejectsMissingManifest(t *testing.T) {
	setupBackupTestEnv(t)
	db := testDB(t)
	backupID := "2026-04-14_21-37-05"
	dir := filepath.Join("backups", backupID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backups/"+backupID+"/restore", bytes.NewBufferString(`{}`))
	req = withURLParam(req, "id", backupID)
	rec := httptest.NewRecorder()
	apiRestoreBackup(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAPIRestoreBackup_CreateSafetyBackupFirst(t *testing.T) {
	setupBackupTestEnv(t)
	db := testDB(t)

	insertTestWord(t, db, "元", 1)
	manifest, err := createBackup(db)
	if err != nil {
		t.Fatal(err)
	}
	insertTestWord(t, db, "現在", 1)

	req := httptest.NewRequest(http.MethodPost, "/api/backups/"+manifest.BackupID+"/restore", bytes.NewBufferString(`{"createSafetyBackup":true}`))
	req = withURLParam(req, "id", manifest.BackupID)
	rec := httptest.NewRecorder()
	apiRestoreBackup(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	backups, err := listBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("backup count after safety restore: got %d, want 2", len(backups))
	}
	var currentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM words WHERE base_word = '現在'`).Scan(&currentCount); err != nil {
		t.Fatal(err)
	}
	if currentCount != 0 {
		t.Fatalf("expected current-only word to be removed after restore, got count=%d", currentCount)
	}
}

func ptrString(value string) *string {
	return &value
}
