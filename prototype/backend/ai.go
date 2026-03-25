package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// wordAutoFill holds AI-generated fields for a Japanese word.
type wordAutoFill struct {
	Reading      string `json:"reading"`
	PartOfSpeech string `json:"part_of_speech"`
	Meaning      string `json:"meaning"`
	ExampleJP    string `json:"example_jp"`
	ExampleEN    string `json:"example_en"`
}

// autoFillWord calls the Anthropic Messages API to fill in dictionary fields for a
// Japanese word. The API key is read from the ANTHROPIC_API_KEY environment variable.
func autoFillWord(word string) (*wordAutoFill, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system"`
		Messages  []message `json:"messages"`
	}

	system := `You are a Japanese dictionary assistant. Given a Japanese word or phrase, return a JSON object with exactly these fields:
- "reading": the word's reading in hiragana (use katakana only for loanwords)
- "part_of_speech": e.g. "noun", "verb", "i-adjective", "na-adjective", "adverb", "particle", etc.
- "meaning": concise English meaning (one short phrase or sentence)
- "example_jp": a short, natural example sentence in Japanese using the word
- "example_en": English translation of the example sentence
Return only a valid JSON object with no markdown, no code fences, and no extra commentary.`

	payload, err := json.Marshal(reqBody{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 512,
		System:    system,
		Messages:  []message{{Role: "user", Content: word}},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode API response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Content) == 0 || apiResp.Content[0].Text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var e wordAutoFill
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}
