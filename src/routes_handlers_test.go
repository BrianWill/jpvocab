package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	if len(story.Chunks) != 1 {
		t.Fatalf("chunks: got %d, want 1", len(story.Chunks))
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
}

func TestAPIGetStory_ReturnsStoryByID(t *testing.T) {
	db := testDB(t)
	title := "Garden Story"
	id, err := insertStory(db, title, []storySentenceInput{
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}, IsParagraphStart: true},
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
	if len(story.Chunks) != 1 {
		t.Errorf("chunks: got %d, want 1", len(story.Chunks))
	}
}

func TestAPICreateStory_SplitsLongStoryIntoChunks(t *testing.T) {
	db := testDB(t)
	longSentenceA := strings.Repeat("あ", 260) + "。"
	longSentenceB := strings.Repeat("い", 260) + "。"
	longSentenceC := strings.Repeat("う", 260) + "。"
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
	if len(story.Chunks) != 2 {
		t.Fatalf("chunks: got %d, want 2", len(story.Chunks))
	}
	if len(story.Chunks[0].Sentences) != 2 {
		t.Fatalf("chunk 1 sentences: got %d, want 2", len(story.Chunks[0].Sentences))
	}
	if len(story.Chunks[1].Sentences) != 1 {
		t.Fatalf("chunk 2 sentences: got %d, want 1", len(story.Chunks[1].Sentences))
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
		{Words: []storyWordInput{{DisplayWord: "庭園", BaseWord: "庭園"}}, IsParagraphStart: true},
		{Words: []storyWordInput{{DisplayWord: "歩く", BaseWord: "歩く"}}, IsParagraphStart: false},
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
