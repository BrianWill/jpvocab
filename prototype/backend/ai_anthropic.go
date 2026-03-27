package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// callAnthropic sends a request to the Anthropic Messages API and returns the text of the first
// content block. The API key is read from ANTHROPIC_API_KEY.
func callAnthropic(model, system string, messages []message, maxTokens int) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
	}

	type reqBody struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system"`
		Messages  []message `json:"messages"`
	}
	payload, err := json.Marshal(reqBody{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("decode API response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Content) == 0 || apiResp.Content[0].Text == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return apiResp.Content[0].Text, nil
}

func autoFillWordAnthropic(word, model string) (*wordAutoFill, error) {
	messages := make([]message, 0, len(autoFillExamples)*2+2)
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})
	messages = append(messages, message{Role: "assistant", Content: "{"})

	text, err := callAnthropic(model, autoFillSystemPrompt, messages, 512)
	if err != nil {
		return nil, err
	}
	var e wordAutoFill
	if err := json.Unmarshal([]byte("{"+text), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}

func rerollMeaningAnthropic(word, currentMeaning, model string) ([]string, error) {
	messages := []message{
		{Role: "user", Content: marshalUserMsg(map[string]string{"word": word, "current_meaning": currentMeaning})},
		{Role: "assistant", Content: "["},
	}
	text, err := callAnthropic(model, rerollMeaningSystemPrompt, messages, 256)
	if err != nil {
		return nil, err
	}
	var result []string
	if err := json.Unmarshal([]byte("["+text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, nil
}

func rerollExamplesAnthropic(word, model string) ([]examplePair, error) {
	messages := []message{
		{Role: "user", Content: word},
		{Role: "assistant", Content: "["},
	}
	text, err := callAnthropic(model, rerollExamplesSystemPrompt, messages, 512)
	if err != nil {
		return nil, err
	}
	var result []examplePair
	if err := json.Unmarshal([]byte("["+text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, nil
}
