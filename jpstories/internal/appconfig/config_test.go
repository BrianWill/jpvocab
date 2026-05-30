package appconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:8080" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.VoiceVox.BaseURL != "http://127.0.0.1:50021" {
		t.Fatalf("VoiceVox.BaseURL = %q", cfg.VoiceVox.BaseURL)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Addr = "127.0.0.1:9000"
	cfg.Stories.Dir = "my_stories"
	cfg.VoiceVox.BaseURL = "http://example.test"
	cfg.VoiceVox.SpeakerID = 42
	cfg.VoiceVox.SpeakerName = "Speaker / Style"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != cfg {
		t.Fatalf("Load() = %#v, want %#v", got, cfg)
	}
}
