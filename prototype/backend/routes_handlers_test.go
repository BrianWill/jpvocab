package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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
