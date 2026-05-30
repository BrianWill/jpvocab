package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jpstories/internal/appconfig"
	"jpstories/internal/story"
)

func TestIndexEmptyStoryDirectory(t *testing.T) {
	dir := t.TempDir()
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No stories found yet.") {
		t.Fatalf("body missing empty-state message:\n%s", body)
	}
}

func TestIndexListsStories(t *testing.T) {
	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Sample Story") {
		t.Fatalf("body missing story title:\n%s", body)
	}
	if !strings.Contains(body, `href="/stories/sample"`) {
		t.Fatalf("body missing story link:\n%s", body)
	}
}

func TestStoryMissingReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stories/missing", nil))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestStoryDetailRendersTwoColumnsAndSentenceButtons(t *testing.T) {
	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stories/sample?level=n3", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Japanese",
		"English",
		`aria-current="page">n3</a>`,
		`data-sentence-id="s-001"`,
		"n3 first",
		"First sentence.",
		"Second sentence.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestStoryDetailFallsBackForUnknownLevel(t *testing.T) {
	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stories/sample?level=bad", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `aria-current="page">native</a>`) {
		t.Fatalf("body did not fall back to native tab:\n%s", body)
	}
	if !strings.Contains(body, "native first") {
		t.Fatalf("body did not render native translation:\n%s", body)
	}
}

func TestStoryDetailShowsMissingTranslationPlaceholder(t *testing.T) {
	dir := t.TempDir()
	s := fixtureStory()
	delete(s.Chunks[0].Paragraphs[0].Sentences[1].Translations, story.LevelN2Abridged)
	writeStory(t, dir, s)
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stories/sample?level=n2_abridged", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "[missing n2_abridged translation]") {
		t.Fatalf("body missing placeholder:\n%s", body)
	}
	if !strings.Contains(body, "disabled") {
		t.Fatalf("missing translation button should be disabled:\n%s", body)
	}
}

func TestStoryDetailSplitsMultiSentenceJapaneseTranslation(t *testing.T) {
	dir := t.TempDir()
	s := fixtureStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[story.LevelN3] = "一つ目です。「二つ目です。」"
	writeStory(t, dir, s)
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stories/sample?level=n3", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{"一つ目です。", "「二つ目です。」"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, `data-sentence-id="s-001"`); got != 2 {
		t.Fatalf("s-001 rendered %d times, want 2:\n%s", got, body)
	}
	for _, want := range []string{`data-sentence-part="0"`, `data-sentence-part="1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, "First sentence."); got != 1 {
		t.Fatalf("English source rendered %d times, want 1:\n%s", got, body)
	}
	if got := strings.Count(body, ">↑</p>"); got != 1 {
		t.Fatalf("repeat marker rendered %d times, want 1:\n%s", got, body)
	}
}

func TestSentenceAudioResolvesSplitSentencePart(t *testing.T) {
	var audioQueryText string
	voicevoxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			audioQueryText = r.URL.Query().Get("text")
			_ = json.NewEncoder(w).Encode(map[string]any{"query": true})
		case "/synthesis":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("WAVE"))
		default:
			t.Fatalf("unexpected VoiceVox path %s", r.URL.Path)
		}
	}))
	defer voicevoxServer.Close()

	dir := t.TempDir()
	s := fixtureStory()
	s.Chunks[0].Paragraphs[0].Sentences[0].Translations[story.LevelN3] = "一つ目です。「二つ目です。」"
	writeStory(t, dir, s)
	srv := New(Config{StoriesDir: dir, VoiceVoxBaseURL: voicevoxServer.URL, VoiceVoxSpeaker: 2})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sentence-audio?story=sample&level=n3&sentence=s-001&part=1", nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if audioQueryText != "「二つ目です。」" {
		t.Fatalf("audio query text = %q, want split sentence part", audioQueryText)
	}
}

func TestSentenceAudioResolvesSentenceAndReturnsVoiceVoxAudio(t *testing.T) {
	var audioQueryText string
	var synthesisQuery map[string]any
	voicevoxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			audioQueryText = r.URL.Query().Get("text")
			_ = json.NewEncoder(w).Encode(map[string]any{"query": true})
		case "/synthesis":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if err := json.Unmarshal(body, &synthesisQuery); err != nil {
				t.Fatalf("decode synthesis query: %v", err)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("WAVE"))
		default:
			t.Fatalf("unexpected VoiceVox path %s", r.URL.Path)
		}
	}))
	defer voicevoxServer.Close()

	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir, VoiceVoxBaseURL: voicevoxServer.URL, VoiceVoxSpeaker: 2})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sentence-audio?story=sample&level=n3&sentence=s-001&speed=1.25", nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if audioQueryText != "n3 first" {
		t.Fatalf("audio query text = %q, want n3 first", audioQueryText)
	}
	if rr.Header().Get("Content-Type") != "audio/wav" {
		t.Fatalf("Content-Type = %q, want audio/wav", rr.Header().Get("Content-Type"))
	}
	if rr.Body.String() != "WAVE" {
		t.Fatalf("body = %q, want WAVE", rr.Body.String())
	}
	if got, ok := synthesisQuery["speedScale"].(float64); !ok || got != 1.25 {
		t.Fatalf("speedScale = %#v, want 1.25", synthesisQuery["speedScale"])
	}
}

func TestSentenceAudioAcceptsQuotedQueryValues(t *testing.T) {
	voicevoxServer := settingsVoiceVoxServer(t)
	defer voicevoxServer.Close()

	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir, VoiceVoxBaseURL: voicevoxServer.URL, VoiceVoxSpeaker: 2})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, `/api/sentence-audio?story="sample"&level="n3"&sentence="s-001"`, nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if rr.Body.String() != "WAVE" {
		t.Fatalf("body = %q, want WAVE", rr.Body.String())
	}
}

func TestSentenceAudioRejectsMissingTranslation(t *testing.T) {
	dir := t.TempDir()
	s := fixtureStory()
	delete(s.Chunks[0].Paragraphs[0].Sentences[0].Translations, story.LevelN3)
	writeStory(t, dir, s)
	srv := New(Config{StoriesDir: dir})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sentence-audio?story=sample&level=n3&sentence=s-001", nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "missing n3 translation") {
		t.Fatalf("body = %q, want missing translation message", rr.Body.String())
	}
}

func TestSentenceAudioReportsVoiceVoxUnavailable(t *testing.T) {
	voicevoxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer voicevoxServer.Close()

	dir := t.TempDir()
	writeStory(t, dir, fixtureStory())
	srv := New(Config{StoriesDir: dir, VoiceVoxBaseURL: voicevoxServer.URL})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sentence-audio?story=sample&level=n3&sentence=s-001", nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rr.Body.String(), "VoiceVox playback unavailable") {
		t.Fatalf("body = %q, want VoiceVox unavailable message", rr.Body.String())
	}
}

func TestSettingsPageListsVoiceVoxSpeakers(t *testing.T) {
	voicevoxServer := settingsVoiceVoxServer(t)
	defer voicevoxServer.Close()

	srv := New(Config{
		StoriesDir:      t.TempDir(),
		VoiceVoxBaseURL: voicevoxServer.URL,
		VoiceVoxSpeaker: 2,
	})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Settings",
		"VoiceVox URL",
		"Test Speaker / Normal",
		"Test Speaker / Happy",
		`value="2"`,
		"Speed",
		"Pause length",
		"Volume",
		"Pitch",
		"Intonation",
		"Start silence",
		"End silence",
		"Preview",
		"Save",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestSettingsPostSavesConfig(t *testing.T) {
	voicevoxServer := settingsVoiceVoxServer(t)
	defer voicevoxServer.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	srv := New(Config{
		StoriesDir:      dir,
		VoiceVoxBaseURL: voicevoxServer.URL,
		VoiceVoxSpeaker: 1,
		ConfigPath:      configPath,
	})

	form := strings.NewReader("voicevox_base_url=" + url.QueryEscape(voicevoxServer.URL) + "&voicevox_speaker_id=2&voicevox_speaker_name=Test+Speaker+%2F+Happy&voicevox_speed_scale=0.85&voicevox_pause_length_scale=1.35&voicevox_volume_scale=1.20&voicevox_pitch_scale=-0.04&voicevox_intonation_scale=1.15&voicevox_pre_phoneme_length=0.20&voicevox_post_phoneme_length=0.30")
	req := httptest.NewRequest(http.MethodPost, "/settings", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	cfg, err := appconfig.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VoiceVox.BaseURL != voicevoxServer.URL {
		t.Fatalf("BaseURL = %q, want %q", cfg.VoiceVox.BaseURL, voicevoxServer.URL)
	}
	if cfg.VoiceVox.SpeakerID != 2 {
		t.Fatalf("SpeakerID = %d, want 2", cfg.VoiceVox.SpeakerID)
	}
	if cfg.VoiceVox.SpeakerName != "Test Speaker / Happy" {
		t.Fatalf("SpeakerName = %q", cfg.VoiceVox.SpeakerName)
	}
	if cfg.VoiceVox.SpeedScale != 0.85 {
		t.Fatalf("SpeedScale = %v, want 0.85", cfg.VoiceVox.SpeedScale)
	}
	if cfg.VoiceVox.PauseLengthScale != 1.35 {
		t.Fatalf("PauseLengthScale = %v, want 1.35", cfg.VoiceVox.PauseLengthScale)
	}
	if cfg.VoiceVox.VolumeScale != 1.20 {
		t.Fatalf("VolumeScale = %v, want 1.20", cfg.VoiceVox.VolumeScale)
	}
	if cfg.VoiceVox.PitchScale != -0.04 {
		t.Fatalf("PitchScale = %v, want -0.04", cfg.VoiceVox.PitchScale)
	}
	if cfg.VoiceVox.IntonationScale != 1.15 {
		t.Fatalf("IntonationScale = %v, want 1.15", cfg.VoiceVox.IntonationScale)
	}
	if cfg.VoiceVox.PrePhonemeLength != 0.20 {
		t.Fatalf("PrePhonemeLength = %v, want 0.20", cfg.VoiceVox.PrePhonemeLength)
	}
	if cfg.VoiceVox.PostPhonemeLength != 0.30 {
		t.Fatalf("PostPhonemeLength = %v, want 0.30", cfg.VoiceVox.PostPhonemeLength)
	}
	if !strings.Contains(rr.Body.String(), "Settings saved.") {
		t.Fatalf("body missing saved message:\n%s", rr.Body.String())
	}
}

func TestVoiceVoxPreviewUsesSubmittedSettings(t *testing.T) {
	var audioQueryText string
	var audioQuerySpeaker string
	var synthesisQuery map[string]any
	voicevoxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			audioQueryText = r.URL.Query().Get("text")
			audioQuerySpeaker = r.URL.Query().Get("speaker")
			_ = json.NewEncoder(w).Encode(map[string]any{"query": true})
		case "/synthesis":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if err := json.Unmarshal(body, &synthesisQuery); err != nil {
				t.Fatalf("decode synthesis query: %v", err)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("PREVIEW"))
		default:
			t.Fatalf("unexpected VoiceVox path %s", r.URL.Path)
		}
	}))
	defer voicevoxServer.Close()

	srv := New(Config{StoriesDir: t.TempDir()})
	form := strings.NewReader("voicevox_base_url=" + url.QueryEscape(voicevoxServer.URL) + "&voicevox_speaker_id=7&text=hello&voicevox_speed_scale=0.75&voicevox_pause_length_scale=1.50&voicevox_volume_scale=1.25&voicevox_pitch_scale=0.06&voicevox_intonation_scale=1.30&voicevox_pre_phoneme_length=0.11&voicevox_post_phoneme_length=0.22")
	req := httptest.NewRequest(http.MethodPost, "/api/voicevox-preview", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if audioQueryText != "hello" {
		t.Fatalf("audio query text = %q, want hello", audioQueryText)
	}
	if audioQuerySpeaker != "7" {
		t.Fatalf("audio query speaker = %q, want 7", audioQuerySpeaker)
	}
	if rr.Body.String() != "PREVIEW" {
		t.Fatalf("body = %q, want PREVIEW", rr.Body.String())
	}
	for key, want := range map[string]float64{
		"speedScale":        0.75,
		"pauseLengthScale":  1.50,
		"volumeScale":       1.25,
		"pitchScale":        0.06,
		"intonationScale":   1.30,
		"prePhonemeLength":  0.11,
		"postPhonemeLength": 0.22,
	} {
		if got, ok := synthesisQuery[key].(float64); !ok || got != want {
			t.Fatalf("synthesis query %s = %#v, want %v", key, synthesisQuery[key], want)
		}
	}
}

func settingsVoiceVoxServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/speakers":
			_, _ = w.Write([]byte(`[
			  {"name":"Test Speaker","speaker_uuid":"test","styles":[{"name":"Normal","id":1},{"name":"Happy","id":2}]}
			]`))
		case "/audio_query":
			_ = json.NewEncoder(w).Encode(map[string]any{"query": true})
		case "/synthesis":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("WAVE"))
		default:
			t.Fatalf("unexpected VoiceVox path %s", r.URL.Path)
		}
	}))
}

func writeStory(t *testing.T, dir string, s story.Story) {
	t.Helper()
	storyDir := filepath.Join(dir, s.ID)
	if err := os.MkdirAll(storyDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := story.SaveFile(filepath.Join(storyDir, s.ID+".json"), s); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
}

func fixtureStory() story.Story {
	return story.Story{
		ID:             "sample",
		Title:          "Sample Story",
		SourceLanguage: "en",
		TargetLanguage: "ja",
		SourceFile:     "stories/sample/sample.txt",
		Levels:         []string{story.LevelNative, story.LevelN3, story.LevelN2Abridged},
		Chunks: []story.Chunk{
			{
				ID: "chunk-001",
				Paragraphs: []story.Paragraph{
					{
						ID: "p-001",
						Sentences: []story.Sentence{
							{
								ID:      "s-001",
								English: "First sentence.",
								Translations: map[string]string{
									story.LevelNative:     "native first",
									story.LevelN3:         "n3 first",
									story.LevelN2Abridged: "n2 first",
								},
							},
							{
								ID:      "s-002",
								English: "Second sentence.",
								Translations: map[string]string{
									story.LevelNative:     "native second",
									story.LevelN3:         "n3 second",
									story.LevelN2Abridged: "n2 second",
								},
							},
						},
					},
				},
			},
		},
	}
}
