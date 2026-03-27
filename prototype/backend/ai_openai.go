package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// callOpenAI sends a request to the OpenAI Chat Completions API and returns the content of the
// first choice. The API key is read from OPENAI_API_KEY.
func callOpenAI(model string, messages []message) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	type reqBody struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}
	payload, err := json.Marshal(reqBody{Model: model, Messages: messages})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("decode API response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return apiResp.Choices[0].Message.Content, nil
}

func autoFillWordOpenAI(word, model string) (*wordAutoFill, error) {
	messages := make([]message, 0, len(autoFillExamples)*2+2)
	messages = append(messages, message{Role: "system", Content: autoFillSystemPrompt})
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})

	text, err := callOpenAI(model, messages)
	if err != nil {
		return nil, err
	}
	var e wordAutoFill
	if err := json.Unmarshal([]byte(text), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}

func rerollMeaningOpenAI(word, currentMeaning, model string) ([]string, error) {
	messages := []message{
		{Role: "system", Content: rerollMeaningSystemPrompt},
		{Role: "user", Content: marshalUserMsg(map[string]string{"word": word, "current_meaning": currentMeaning})},
	}
	text, err := callOpenAI(model, messages)
	if err != nil {
		return nil, err
	}
	var result []string
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, nil
}

func rerollExamplesOpenAI(word, model string) ([]examplePair, error) {
	messages := []message{
		{Role: "system", Content: rerollExamplesSystemPrompt},
		{Role: "user", Content: word},
	}
	text, err := callOpenAI(model, messages)
	if err != nil {
		return nil, err
	}
	var result []examplePair
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, nil
}
