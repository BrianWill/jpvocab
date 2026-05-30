package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr     string         `json:"addr"`
	Stories  StoriesConfig  `json:"stories"`
	VoiceVox VoiceVoxConfig `json:"voicevox"`
}

type StoriesConfig struct {
	Dir string `json:"dir"`
}

type VoiceVoxConfig struct {
	BaseURL           string  `json:"base_url"`
	SpeakerID         int     `json:"speaker_id"`
	SpeakerName       string  `json:"speaker_name,omitempty"`
	SpeedScale        float64 `json:"speed_scale"`
	PauseLengthScale  float64 `json:"pause_length_scale"`
	VolumeScale       float64 `json:"volume_scale"`
	PitchScale        float64 `json:"pitch_scale"`
	IntonationScale   float64 `json:"intonation_scale"`
	PrePhonemeLength  float64 `json:"pre_phoneme_length"`
	PostPhonemeLength float64 `json:"post_phoneme_length"`
}

func Default() Config {
	return Config{
		Addr: "127.0.0.1:8080",
		Stories: StoriesConfig{
			Dir: "stories",
		},
		VoiceVox: VoiceVoxConfig{
			BaseURL:           "http://127.0.0.1:50021",
			SpeakerID:         1,
			SpeedScale:        1,
			PauseLengthScale:  1,
			VolumeScale:       1,
			PitchScale:        0,
			IntonationScale:   1,
			PrePhonemeLength:  0.1,
			PostPhonemeLength: 0.1,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is required")
	}
	cfg.ApplyDefaults()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) ApplyDefaults() {
	defaults := Default()
	if strings.TrimSpace(c.Addr) == "" {
		c.Addr = defaults.Addr
	}
	if strings.TrimSpace(c.Stories.Dir) == "" {
		c.Stories.Dir = defaults.Stories.Dir
	}
	if strings.TrimSpace(c.VoiceVox.BaseURL) == "" {
		c.VoiceVox.BaseURL = defaults.VoiceVox.BaseURL
	}
	if c.VoiceVox.SpeakerID <= 0 {
		c.VoiceVox.SpeakerID = defaults.VoiceVox.SpeakerID
	}
	if c.VoiceVox.SpeedScale <= 0 {
		c.VoiceVox.SpeedScale = defaults.VoiceVox.SpeedScale
	}
	if c.VoiceVox.PauseLengthScale <= 0 {
		c.VoiceVox.PauseLengthScale = defaults.VoiceVox.PauseLengthScale
	}
	if c.VoiceVox.VolumeScale <= 0 {
		c.VoiceVox.VolumeScale = defaults.VoiceVox.VolumeScale
	}
	if c.VoiceVox.IntonationScale <= 0 {
		c.VoiceVox.IntonationScale = defaults.VoiceVox.IntonationScale
	}
	if c.VoiceVox.PrePhonemeLength <= 0 {
		c.VoiceVox.PrePhonemeLength = defaults.VoiceVox.PrePhonemeLength
	}
	if c.VoiceVox.PostPhonemeLength <= 0 {
		c.VoiceVox.PostPhonemeLength = defaults.VoiceVox.PostPhonemeLength
	}
}
