package voicevox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "http://127.0.0.1:50021"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Speaker    int
	Options    AudioOptions
}

type AudioOptions struct {
	SpeedScale        float64
	PauseLengthScale  float64
	VolumeScale       float64
	PitchScale        float64
	IntonationScale   float64
	PrePhonemeLength  float64
	PostPhonemeLength float64
}

type Speaker struct {
	Name        string  `json:"name"`
	SpeakerUUID string  `json:"speaker_uuid"`
	Styles      []Style `json:"styles"`
}

type Style struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	Type string `json:"type,omitempty"`
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		Speaker: 1,
	}
}

func (c *Client) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	return c.SynthesizeWithSpeaker(ctx, text, c.Speaker)
}

func (c *Client) SynthesizeWithSpeaker(ctx context.Context, text string, speaker int) ([]byte, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, "", fmt.Errorf("text is required")
	}
	if speaker <= 0 {
		speaker = c.Speaker
	}

	query, err := c.audioQuery(ctx, text, speaker)
	if err != nil {
		return nil, "", err
	}
	query, err = applyAudioOptions(query, c.Options)
	if err != nil {
		return nil, "", err
	}
	return c.synthesis(ctx, query, speaker)
}

func (c *Client) Speakers(ctx context.Context) ([]Speaker, error) {
	endpoint, err := url.Parse(c.BaseURL + "/speakers")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("voicevox speakers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("voicevox speakers returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var speakers []Speaker
	if err := json.Unmarshal(body, &speakers); err != nil {
		return nil, fmt.Errorf("decode voicevox speakers: %w", err)
	}
	return speakers, nil
}

func (c *Client) audioQuery(ctx context.Context, text string, speaker int) ([]byte, error) {
	endpoint, err := url.Parse(c.BaseURL + "/audio_query")
	if err != nil {
		return nil, err
	}
	values := endpoint.Query()
	values.Set("text", text)
	values.Set("speaker", fmt.Sprint(speaker))
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("voicevox audio_query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("voicevox audio_query returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *Client) synthesis(ctx context.Context, query []byte, speaker int) ([]byte, string, error) {
	endpoint, err := url.Parse(c.BaseURL + "/synthesis")
	if err != nil {
		return nil, "", err
	}
	values := endpoint.Query()
	values.Set("speaker", fmt.Sprint(speaker))
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(query))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("voicevox synthesis: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("voicevox synthesis returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/wav"
	}
	return body, contentType, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func applyAudioOptions(query []byte, options AudioOptions) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(query, &payload); err != nil {
		return nil, fmt.Errorf("decode voicevox audio query: %w", err)
	}
	options.ApplyDefaults()
	payload["speedScale"] = options.SpeedScale
	payload["pauseLengthScale"] = options.PauseLengthScale
	payload["volumeScale"] = options.VolumeScale
	payload["pitchScale"] = options.PitchScale
	payload["intonationScale"] = options.IntonationScale
	payload["prePhonemeLength"] = options.PrePhonemeLength
	payload["postPhonemeLength"] = options.PostPhonemeLength

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode voicevox audio query: %w", err)
	}
	return data, nil
}

func (o *AudioOptions) ApplyDefaults() {
	if o.SpeedScale <= 0 {
		o.SpeedScale = 1
	}
	if o.PauseLengthScale <= 0 {
		o.PauseLengthScale = 1
	}
	if o.VolumeScale <= 0 {
		o.VolumeScale = 1
	}
	if o.IntonationScale <= 0 {
		o.IntonationScale = 1
	}
	if o.PrePhonemeLength <= 0 {
		o.PrePhonemeLength = 0.1
	}
	if o.PostPhonemeLength <= 0 {
		o.PostPhonemeLength = 0.1
	}
}
