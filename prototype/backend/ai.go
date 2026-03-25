package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// wordAutoFill holds AI-generated fields for a Japanese word.
type wordAutoFill struct {
	Reading      string `json:"reading"`
	PartOfSpeech string `json:"part_of_speech"`
	Meaning      string `json:"meaning"`
	ExampleJP    string `json:"example_jp"`
	ExampleEN    string `json:"example_en"`
}

const autoFillSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word or phrase, return a JSON object with exactly these fields:
- "reading": the word's reading in hiragana (use katakana only for loanwords)
- "part_of_speech": e.g. "noun", "verb", "i-adjective", "na-adjective", "adverb", "particle", etc.
- "meaning": concise English meaning (one short phrase or sentence)
- "example_jp": a short, natural example sentence in Japanese using the word
- "example_en": English translation of the example sentence
Return only a valid JSON object with no markdown, no code fences, and no extra commentary.`

// aiProviders holds which AI providers have API keys configured.
type aiProviders struct {
	AnthropicAvail bool
	OpenAIAvail    bool
}

// checkAIProviders reports which providers have API keys set in the environment.
func checkAIProviders() aiProviders {
	return aiProviders{
		AnthropicAvail: os.Getenv("ANTHROPIC_API_KEY") != "",
		OpenAIAvail:    os.Getenv("OPENAI_API_KEY") != "",
	}
}

// autoFillWord dispatches to the appropriate AI provider.
// providerModel must be in "provider/model" format, e.g. "anthropic/claude-haiku-4-5-20251001".
func autoFillWord(word, providerModel string) (*wordAutoFill, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ai_model value %q: expected provider/model", providerModel)
	}
	switch parts[0] {
	case "openai":
		return autoFillWordOpenAI(word, parts[1])
	default:
		return autoFillWordAnthropic(word, parts[1])
	}
}

// autoFillWordAnthropic calls the Anthropic Messages API.
// The API key is read from the ANTHROPIC_API_KEY environment variable.
func autoFillWordAnthropic(word, model string) (*wordAutoFill, error) {
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

	payload, err := json.Marshal(reqBody{
		Model:     model,
		MaxTokens: 512,
		System:    autoFillSystemPrompt,
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

// autoFillWordOpenAI calls the OpenAI Chat Completions API.
// The API key is read from the OPENAI_API_KEY environment variable.
func autoFillWordOpenAI(word, model string) (*wordAutoFill, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}

	payload, err := json.Marshal(reqBody{
		Model: model,
		Messages: []message{
			{Role: "system", Content: autoFillSystemPrompt},
			{Role: "user", Content: word},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
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
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var e wordAutoFill
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}
