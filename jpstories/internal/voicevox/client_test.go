package voicevox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSynthesizeCallsAudioQueryThenSynthesis(t *testing.T) {
	var sawAudioQuery bool
	var sawSynthesis bool
	var synthesisQuery map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			sawAudioQuery = true
			if r.Method != http.MethodPost {
				t.Fatalf("audio_query method = %s, want POST", r.Method)
			}
			if r.URL.Query().Get("text") != "hello" {
				t.Fatalf("audio_query text = %q, want hello", r.URL.Query().Get("text"))
			}
			if r.URL.Query().Get("speaker") != "7" {
				t.Fatalf("audio_query speaker = %q, want 7", r.URL.Query().Get("speaker"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"accent_phrases": []any{}})
		case "/synthesis":
			sawSynthesis = true
			if r.Method != http.MethodPost {
				t.Fatalf("synthesis method = %s, want POST", r.Method)
			}
			if r.URL.Query().Get("speaker") != "7" {
				t.Fatalf("synthesis speaker = %q, want 7", r.URL.Query().Get("speaker"))
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("synthesis content type = %q, want application/json", ct)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(body), "accent_phrases") {
				t.Fatalf("synthesis body = %s, want audio query JSON", body)
			}
			if err := json.Unmarshal(body, &synthesisQuery); err != nil {
				t.Fatalf("decode synthesis body: %v", err)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("WAVE"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	client.Speaker = 7
	client.Options = AudioOptions{
		SpeedScale:        0.8,
		PauseLengthScale:  1.4,
		VolumeScale:       1.2,
		PitchScale:        -0.05,
		IntonationScale:   1.1,
		PrePhonemeLength:  0.2,
		PostPhonemeLength: 0.3,
	}

	audio, contentType, err := client.Synthesize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if string(audio) != "WAVE" {
		t.Fatalf("audio = %q, want WAVE", audio)
	}
	if contentType != "audio/wav" {
		t.Fatalf("contentType = %q, want audio/wav", contentType)
	}
	if !sawAudioQuery || !sawSynthesis {
		t.Fatalf("sawAudioQuery=%v sawSynthesis=%v, want both true", sawAudioQuery, sawSynthesis)
	}
	for key, want := range map[string]float64{
		"speedScale":        0.8,
		"pauseLengthScale":  1.4,
		"volumeScale":       1.2,
		"pitchScale":        -0.05,
		"intonationScale":   1.1,
		"prePhonemeLength":  0.2,
		"postPhonemeLength": 0.3,
	} {
		if got, ok := synthesisQuery[key].(float64); !ok || got != want {
			t.Fatalf("synthesis query %s = %#v, want %v", key, synthesisQuery[key], want)
		}
	}
}

func TestSpeakers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/speakers" {
			t.Fatalf("path = %s, want /speakers", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`[
		  {"name":"A","speaker_uuid":"uuid-a","styles":[{"name":"Normal","id":1},{"name":"Happy","id":2}]}
		]`))
	}))
	defer server.Close()

	client := New(server.URL)
	speakers, err := client.Speakers(context.Background())
	if err != nil {
		t.Fatalf("Speakers() error = %v", err)
	}
	if len(speakers) != 1 {
		t.Fatalf("len(speakers) = %d, want 1", len(speakers))
	}
	if speakers[0].Name != "A" || speakers[0].Styles[1].ID != 2 {
		t.Fatalf("speakers = %#v", speakers)
	}
}

func TestSynthesizeReturnsAudioQueryErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not running", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := New(server.URL)
	_, _, err := client.Synthesize(context.Background(), "hello")
	if err == nil {
		t.Fatal("Synthesize() error = nil")
	}
	if !strings.Contains(err.Error(), "audio_query returned") {
		t.Fatalf("Synthesize() error = %v, want audio_query returned", err)
	}
}
