package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"jpstories/internal/appconfig"
	"jpstories/internal/story"
	"jpstories/internal/voicevox"
)

type Config struct {
	Addr            string
	StoriesDir      string
	VoiceVoxBaseURL string
	VoiceVoxSpeaker int
	VoiceVoxName    string
	VoiceVoxOptions appconfig.VoiceVoxConfig
	ConfigPath      string
}

type Server struct {
	cfg       Config
	appConfig appconfig.Config
	mu        sync.RWMutex
	voicevox  *voicevox.Client
}

func New(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.StoriesDir == "" {
		cfg.StoriesDir = "stories"
	}
	if cfg.VoiceVoxBaseURL == "" {
		cfg.VoiceVoxBaseURL = voicevox.DefaultBaseURL
	}
	if cfg.VoiceVoxSpeaker <= 0 {
		cfg.VoiceVoxSpeaker = 1
	}
	voiceCfg := appconfig.VoiceVoxConfig{
		BaseURL:           cfg.VoiceVoxBaseURL,
		SpeakerID:         cfg.VoiceVoxSpeaker,
		SpeakerName:       cfg.VoiceVoxName,
		SpeedScale:        cfg.VoiceVoxOptions.SpeedScale,
		PauseLengthScale:  cfg.VoiceVoxOptions.PauseLengthScale,
		VolumeScale:       cfg.VoiceVoxOptions.VolumeScale,
		PitchScale:        cfg.VoiceVoxOptions.PitchScale,
		IntonationScale:   cfg.VoiceVoxOptions.IntonationScale,
		PrePhonemeLength:  cfg.VoiceVoxOptions.PrePhonemeLength,
		PostPhonemeLength: cfg.VoiceVoxOptions.PostPhonemeLength,
	}
	appCfg := appconfig.Config{
		Addr: cfg.Addr,
		Stories: appconfig.StoriesConfig{
			Dir: cfg.StoriesDir,
		},
		VoiceVox: voiceCfg,
	}
	appCfg.ApplyDefaults()
	client := voicevox.New(cfg.VoiceVoxBaseURL)
	if cfg.VoiceVoxSpeaker > 0 {
		client.Speaker = cfg.VoiceVoxSpeaker
	}
	client.Options = voiceVoxAudioOptions(appCfg.VoiceVox)
	return &Server{cfg: cfg, appConfig: appCfg, voicevox: client}
}

func (s *Server) ListenAndServe() error {
	httpServer := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("jpstories listening on http://%s", s.cfg.Addr)
	return httpServer.ListenAndServe()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir()))))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/sentence-audio", s.handleSentenceAudio)
	mux.HandleFunc("/api/voicevox-preview", s.handleVoiceVoxPreview)
	mux.HandleFunc("/settings", s.handleSettings)
	mux.HandleFunc("/stories/", s.handleStory)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	stories, err := story.ListDir(s.cfg.StoriesDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, struct {
		StoriesDir string
		Stories    []story.Summary
	}{
		StoriesDir: s.cfg.StoriesDir,
		Stories:    stories,
	}); err != nil {
		log.Printf("render index: %v", err)
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderSettings(w, "")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		baseURL := strings.TrimSpace(r.FormValue("voicevox_base_url"))
		speakerID, err := strconv.Atoi(strings.TrimSpace(r.FormValue("voicevox_speaker_id")))
		if err != nil || speakerID <= 0 {
			http.Error(w, "voicevox_speaker_id must be a positive integer", http.StatusBadRequest)
			return
		}
		speakerName := strings.TrimSpace(r.FormValue("voicevox_speaker_name"))
		options, err := voiceVoxConfigFromForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.saveVoiceVoxSettings(baseURL, speakerID, speakerName, options); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.renderSettings(w, "Settings saved.")
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) renderSettings(w http.ResponseWriter, message string) {
	cfg := s.currentConfig()
	client := voicevox.New(cfg.VoiceVox.BaseURL)
	client.Speaker = cfg.VoiceVox.SpeakerID

	speakers, err := client.Speakers(context.Background())
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	view := settingsView{
		Config:     cfg,
		Message:    message,
		Error:      errorMessage,
		Options:    speakerOptions(speakers, cfg.VoiceVox.SpeakerID),
		SelectedID: cfg.VoiceVox.SpeakerID,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := settingsTemplate.Execute(w, view); err != nil {
		log.Printf("render settings: %v", err)
	}
}

func (s *Server) handleVoiceVoxPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	baseURL := strings.TrimSpace(r.FormValue("voicevox_base_url"))
	if baseURL == "" {
		baseURL = voicevox.DefaultBaseURL
	}
	speakerID, err := strconv.Atoi(strings.TrimSpace(r.FormValue("voicevox_speaker_id")))
	if err != nil || speakerID <= 0 {
		http.Error(w, "voicevox_speaker_id must be a positive integer", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		text = "今日は日本語を勉強します。"
	}

	client := voicevox.New(baseURL)
	options, err := voiceVoxConfigFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	client.Options = voiceVoxAudioOptions(options)
	audio, contentType, err := client.SynthesizeWithSpeaker(r.Context(), text, speakerID)
	if err != nil {
		http.Error(w, "VoiceVox preview unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

func (s *Server) handleSentenceAudio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storyID := cleanQueryValue(r.URL.Query().Get("story"))
	level := cleanQueryValue(r.URL.Query().Get("level"))
	sentenceID := cleanQueryValue(r.URL.Query().Get("sentence"))
	partIndex, err := parseSentencePart(r.URL.Query().Get("part"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if storyID == "" || level == "" || sentenceID == "" {
		http.Error(w, "story, level, and sentence are required", http.StatusBadRequest)
		return
	}

	currentStory, _, err := story.LoadByID(s.cfg.StoriesDir, storyID)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	text, err := sentenceTranslation(currentStory, level, sentenceID, partIndex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := s.currentConfig()
	client := voicevox.New(cfg.VoiceVox.BaseURL)
	client.Speaker = cfg.VoiceVox.SpeakerID
	options := voiceVoxAudioOptions(cfg.VoiceVox)
	if rawSpeed := strings.TrimSpace(r.URL.Query().Get("speed")); rawSpeed != "" {
		speed, err := strconv.ParseFloat(rawSpeed, 64)
		if err != nil || speed < 0.1 || speed > 4 {
			http.Error(w, "speed must be between 0.1 and 4", http.StatusBadRequest)
			return
		}
		options.SpeedScale = speed
	}
	client.Options = options

	audio, contentType, err := client.Synthesize(r.Context(), text)
	if err != nil {
		http.Error(w, "VoiceVox playback unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

func (s *Server) saveVoiceVoxSettings(baseURL string, speakerID int, speakerName string, options appconfig.VoiceVoxConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if baseURL == "" {
		baseURL = voicevox.DefaultBaseURL
	}
	s.appConfig.VoiceVox.BaseURL = baseURL
	s.appConfig.VoiceVox.SpeakerID = speakerID
	s.appConfig.VoiceVox.SpeakerName = speakerName
	s.appConfig.VoiceVox.SpeedScale = options.SpeedScale
	s.appConfig.VoiceVox.PauseLengthScale = options.PauseLengthScale
	s.appConfig.VoiceVox.VolumeScale = options.VolumeScale
	s.appConfig.VoiceVox.PitchScale = options.PitchScale
	s.appConfig.VoiceVox.IntonationScale = options.IntonationScale
	s.appConfig.VoiceVox.PrePhonemeLength = options.PrePhonemeLength
	s.appConfig.VoiceVox.PostPhonemeLength = options.PostPhonemeLength
	s.appConfig.ApplyDefaults()

	s.cfg.VoiceVoxBaseURL = s.appConfig.VoiceVox.BaseURL
	s.cfg.VoiceVoxSpeaker = s.appConfig.VoiceVox.SpeakerID
	s.cfg.VoiceVoxName = s.appConfig.VoiceVox.SpeakerName
	s.voicevox = voicevox.New(s.appConfig.VoiceVox.BaseURL)
	s.voicevox.Speaker = s.appConfig.VoiceVox.SpeakerID
	s.voicevox.Options = voiceVoxAudioOptions(s.appConfig.VoiceVox)

	if strings.TrimSpace(s.cfg.ConfigPath) == "" {
		return nil
	}
	return appconfig.Save(s.cfg.ConfigPath, s.appConfig)
}

func voiceVoxConfigFromForm(r *http.Request) (appconfig.VoiceVoxConfig, error) {
	cfg := appconfig.Default().VoiceVox
	var err error
	if cfg.SpeedScale, err = parseVoiceVoxFloat(r, "voicevox_speed_scale", 0, 4); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.PauseLengthScale, err = parseVoiceVoxFloat(r, "voicevox_pause_length_scale", 0, 4); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.VolumeScale, err = parseVoiceVoxFloat(r, "voicevox_volume_scale", 0, 4); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.PitchScale, err = parseVoiceVoxFloat(r, "voicevox_pitch_scale", -1, 1); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.IntonationScale, err = parseVoiceVoxFloat(r, "voicevox_intonation_scale", 0, 4); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.PrePhonemeLength, err = parseVoiceVoxFloat(r, "voicevox_pre_phoneme_length", 0, 5); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	if cfg.PostPhonemeLength, err = parseVoiceVoxFloat(r, "voicevox_post_phoneme_length", 0, 5); err != nil {
		return appconfig.VoiceVoxConfig{}, err
	}
	return cfg, nil
}

func parseVoiceVoxFloat(r *http.Request, field string, minValue float64, maxValue float64) (float64, error) {
	raw := strings.TrimSpace(r.FormValue(field))
	if raw == "" {
		value := voiceVoxConfigValue(appconfig.Default().VoiceVox, field)
		return value, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", field)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %g and %g", field, minValue, maxValue)
	}
	return value, nil
}

func voiceVoxConfigValue(cfg appconfig.VoiceVoxConfig, field string) float64 {
	switch field {
	case "voicevox_speed_scale":
		return cfg.SpeedScale
	case "voicevox_pause_length_scale":
		return cfg.PauseLengthScale
	case "voicevox_volume_scale":
		return cfg.VolumeScale
	case "voicevox_pitch_scale":
		return cfg.PitchScale
	case "voicevox_intonation_scale":
		return cfg.IntonationScale
	case "voicevox_pre_phoneme_length":
		return cfg.PrePhonemeLength
	case "voicevox_post_phoneme_length":
		return cfg.PostPhonemeLength
	default:
		return 0
	}
}

func voiceVoxAudioOptions(cfg appconfig.VoiceVoxConfig) voicevox.AudioOptions {
	return voicevox.AudioOptions{
		SpeedScale:        cfg.SpeedScale,
		PauseLengthScale:  cfg.PauseLengthScale,
		VolumeScale:       cfg.VolumeScale,
		PitchScale:        cfg.PitchScale,
		IntonationScale:   cfg.IntonationScale,
		PrePhonemeLength:  cfg.PrePhonemeLength,
		PostPhonemeLength: cfg.PostPhonemeLength,
	}
}

func (s *Server) currentConfig() appconfig.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appConfig
}

func (s *Server) handleStory(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stories/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	currentStory, _, err := story.LoadByID(s.cfg.StoriesDir, id)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	selectedLevel := selectLevel(currentStory, r.URL.Query().Get("level"))
	view := storyView{
		Story:         currentStory,
		SelectedLevel: selectedLevel,
		Rows:          readingRows(currentStory, selectedLevel),
		VoiceSpeed:    s.currentConfig().VoiceVox.SpeedScale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := storyTemplate.Execute(w, view); err != nil {
		log.Printf("render story %s: %v", currentStory.ID, err)
	}
}

type storyView struct {
	Story         story.Story
	SelectedLevel string
	Rows          []readingRow
	VoiceSpeed    float64
}

type readingRow struct {
	ChunkID   string
	Paragraph story.Paragraph
	English   string
	Sentences []sentenceView
}

type sentenceView struct {
	ID       string
	Part     int
	English  string
	Japanese string
	Missing  bool
}

type settingsView struct {
	Config     appconfig.Config
	Message    string
	Error      string
	Options    []speakerOption
	SelectedID int
}

type speakerOption struct {
	ID       int
	Label    string
	Selected bool
}

func speakerOptions(speakers []voicevox.Speaker, selectedID int) []speakerOption {
	var options []speakerOption
	for _, speaker := range speakers {
		for _, style := range speaker.Styles {
			options = append(options, speakerOption{
				ID:       style.ID,
				Label:    speaker.Name + " / " + style.Name,
				Selected: style.ID == selectedID,
			})
		}
	}
	return options
}

func selectLevel(s story.Story, requested string) string {
	for _, level := range s.Levels {
		if level == requested {
			return level
		}
	}
	if len(s.Levels) > 0 {
		return s.Levels[0]
	}
	return story.LevelNative
}

func readingRows(s story.Story, level string) []readingRow {
	var rows []readingRow
	for _, chunk := range s.Chunks {
		for _, paragraph := range chunk.Paragraphs {
			row := readingRow{
				ChunkID:   chunk.ID,
				Paragraph: paragraph,
				English:   paragraphEnglish(paragraph),
			}
			for _, sentence := range paragraph.Sentences {
				text := strings.TrimSpace(sentence.Translations[level])
				missing := text == ""
				if missing {
					text = fmt.Sprintf("[missing %s translation]", level)
				}
				for index, japanese := range splitJapaneseSentences(text) {
					english := sentence.English
					if index > 0 {
						english = "↑"
					}
					row.Sentences = append(row.Sentences, sentenceView{
						ID:       sentence.ID,
						Part:     index,
						English:  english,
						Japanese: japanese,
						Missing:  missing,
					})
				}
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func splitJapaneseSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var parts []string
	runes := []rune(text)
	start := 0
	for index := 0; index < len(runes); index++ {
		if runes[index] != '。' {
			continue
		}
		end := index + 1
		for end < len(runes) && isJapaneseSentenceCloser(runes[end]) {
			end++
		}
		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			parts = append(parts, part)
		}
		start = end
	}
	remainder := strings.TrimSpace(string(runes[start:]))
	if remainder != "" {
		parts = append(parts, remainder)
	}
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}

func isJapaneseSentenceCloser(r rune) bool {
	switch r {
	case '」', '』', '）', '】', '〕', '〉', '》':
		return true
	default:
		return false
	}
}

func paragraphEnglish(paragraph story.Paragraph) string {
	parts := make([]string, 0, len(paragraph.Sentences))
	for _, sentence := range paragraph.Sentences {
		text := strings.TrimSpace(sentence.English)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func sentenceTranslation(s story.Story, level, sentenceID string, partIndex int) (string, error) {
	if !story.IsSupportedLevel(level) {
		return "", fmt.Errorf("unsupported level %q", level)
	}
	if !storyHasLevel(s, level) {
		return "", fmt.Errorf("story does not include level %q", level)
	}
	for _, chunk := range s.Chunks {
		for _, paragraph := range chunk.Paragraphs {
			for _, sentence := range paragraph.Sentences {
				if sentence.ID != sentenceID {
					continue
				}
				text := strings.TrimSpace(sentence.Translations[level])
				if text == "" {
					return "", fmt.Errorf("sentence %q is missing %s translation", sentenceID, level)
				}
				parts := splitJapaneseSentences(text)
				if partIndex < 0 || partIndex >= len(parts) {
					return "", fmt.Errorf("sentence %q part %d not found", sentenceID, partIndex)
				}
				return parts[partIndex], nil
			}
		}
	}
	return "", fmt.Errorf("sentence %q not found", sentenceID)
}

func parseSentencePart(value string) (int, error) {
	value = cleanQueryValue(value)
	if value == "" {
		return 0, nil
	}
	part, err := strconv.Atoi(value)
	if err != nil || part < 0 {
		return 0, fmt.Errorf("part must be a non-negative integer")
	}
	return part, nil
}

func cleanQueryValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func storyHasLevel(s story.Story, level string) bool {
	for _, configured := range s.Levels {
		if configured == level {
			return true
		}
	}
	return false
}

var indexTemplate = mustParseTemplate("index.html")
var storyTemplate = mustParseTemplate("story.html")
var settingsTemplate = mustParseTemplate("settings.html")

func mustParseTemplate(name string) *template.Template {
	return template.Must(template.ParseFiles(filepath.Join(templateDir(), name)))
}

func templateDir() string {
	return filepath.Join(packageDir(), "templates")
}

func staticDir() string {
	return filepath.Join(packageDir(), "static")
}

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("server package path unavailable")
	}
	return filepath.Dir(file)
}
func URL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}
