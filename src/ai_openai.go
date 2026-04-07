package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// callOpenAI sends a request to the OpenAI Chat Completions API and returns the content of the
// first choice plus token usage. The API key is read from OPENAI_API_KEY.
func callOpenAI(model string, messages []message) (string, tokenUsage, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", tokenUsage{}, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	type reqBody struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}
	payload, err := json.Marshal(reqBody{Model: model, Messages: messages})
	if err != nil {
		return "", tokenUsage{}, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", tokenUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", tokenUsage{}, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", tokenUsage{}, fmt.Errorf("decode API response: %w", err)
	}
	if apiResp.Error != nil {
		return "", tokenUsage{}, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return "", tokenUsage{}, fmt.Errorf("empty response from API")
	}
	usage := tokenUsage{InputTokens: apiResp.Usage.PromptTokens, OutputTokens: apiResp.Usage.CompletionTokens}
	return apiResp.Choices[0].Message.Content, usage, nil
}

func autoFillWordOpenAI(word, model string) (*wordAutoFill, tokenUsage, error) {
	messages := make([]message, 0, len(autoFillExamples)*2+2)
	messages = append(messages, message{Role: "system", Content: autoFillSystemPrompt})
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})

	text, usage, err := callOpenAI(model, messages)
	if err != nil {
		return nil, tokenUsage{}, err
	}
	var e wordAutoFill
	if err := json.Unmarshal([]byte(text), &e); err != nil {
		return nil, tokenUsage{}, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, usage, nil
}

func autoFillWordsBatchOpenAI(words []string, model string) ([]*wordAutoFill, tokenUsage, error) {
	exInput, _ := json.Marshal([]string{autoFillExamples[0].word, autoFillExamples[1].word})
	exOutput := "[" + autoFillExamples[0].result + "," + autoFillExamples[1].result + "]"
	input, _ := json.Marshal(words)
	messages := []message{
		{Role: "system", Content: autoFillBatchSystemPrompt},
		{Role: "user", Content: string(exInput)},
		{Role: "assistant", Content: exOutput},
		{Role: "user", Content: string(input)},
	}
	text, usage, err := callOpenAI(model, messages)
	if err != nil {
		return nil, tokenUsage{}, err
	}
	var fills []*wordAutoFill
	if err := json.Unmarshal([]byte(text), &fills); err != nil {
		return nil, tokenUsage{}, fmt.Errorf("parse batch auto-fill JSON: %w", err)
	}
	return fills, usage, nil
}

func rerollMeaningOpenAI(word, currentMeaning, model string) ([]string, tokenUsage, error) {
	messages := []message{
		{Role: "system", Content: rerollMeaningSystemPrompt},
		{Role: "user", Content: marshalUserMsg(map[string]string{"word": word, "current_meaning": currentMeaning})},
	}
	text, usage, err := callOpenAI(model, messages)
	if err != nil {
		return nil, tokenUsage{}, err
	}
	var result []string
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, tokenUsage{}, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, usage, nil
}

func rerollExamplesOpenAI(word, model string) ([]examplePair, tokenUsage, error) {
	messages := []message{
		{Role: "system", Content: rerollExamplesSystemPrompt},
		{Role: "user", Content: word},
	}
	text, usage, err := callOpenAI(model, messages)
	if err != nil {
		return nil, tokenUsage{}, err
	}
	var result []examplePair
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, tokenUsage{}, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, usage, nil
}
