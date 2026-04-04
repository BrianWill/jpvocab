package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(body, `href="/stories">Stories →</a>`) {
		t.Error("expected shared stories nav link in rendered page")
	}
	if !strings.Contains(body, `page-nav-link page-nav-link--current" href="/activity"`) {
		t.Error("expected current-page nav styling for activity page")
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
	upsertKanji(db, "木", []string{"tree", "wood"})
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
}

func TestAPIPutDrillSettings_RoundTrips(t *testing.T) {
	db := testDB(t)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/drill", bytes.NewBufferString(`{"maxWords":12,"roundSize":5,"wordTypes":["verbs","other"]}`))
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
}
