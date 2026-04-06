package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// callGoogle sends a request to the Google Generative Language API and returns the text of the first
// candidate's first part. The API key is read from GOOGLE_API_KEY.
// Messages with role "assistant" are converted to "model" (Google's convention).
func callGoogle(model, system string, messages []message) (string, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GOOGLE_API_KEY environment variable is not set")
	}

	type part struct {
		Text string `json:"text"`
	}

	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	type systemInstruction struct {
		Parts []part `json:"parts"`
	}

	// Convert messages: change "assistant" role to "model" for Google
	contents := make([]content, len(messages))
	for i, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		contents[i] = content{
			Role:  role,
			Parts: []part{{Text: msg.Content}},
		}
	}

	type reqBody struct {
		SystemInstruction systemInstruction `json:"system_instruction"`
		Contents          []content         `json:"contents"`
	}

	payload, err := json.Marshal(reqBody{
		SystemInstruction: systemInstruction{
			Parts: []part{{Text: system}},
		},
		Contents: contents,
	})
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
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
	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 || apiResp.Candidates[0].Content.Parts[0].Text == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return apiResp.Candidates[0].Content.Parts[0].Text, nil
}

func autoFillWordGoogle(word, model string) (*wordAutoFill, error) {
	messages := make([]message, 0, len(autoFillExamples)*2+2)
	messages = append(messages, message{Role: "system", Content: autoFillSystemPrompt})
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})

	text, err := callGoogle(model, "", messages)
	if err != nil {
		return nil, err
	}
	var e wordAutoFill
	if err := json.Unmarshal([]byte(text), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}

func autoFillWordsBatchGoogle(words []string, model string) ([]*wordAutoFill, error) {
	exInput, _ := json.Marshal([]string{autoFillExamples[0].word, autoFillExamples[1].word})
	exOutput := "[" + autoFillExamples[0].result + "," + autoFillExamples[1].result + "]"
	input, _ := json.Marshal(words)
	messages := []message{
		{Role: "user", Content: string(exInput)},
		{Role: "assistant", Content: exOutput},
		{Role: "user", Content: string(input)},
	}
	text, err := callGoogle(model, autoFillBatchSystemPrompt, messages)
	if err != nil {
		return nil, err
	}
	var fills []*wordAutoFill
	if err := json.Unmarshal([]byte(text), &fills); err != nil {
		return nil, fmt.Errorf("parse batch auto-fill JSON: %w", err)
	}
	return fills, nil
}

func rerollMeaningGoogle(word, currentMeaning, model string) ([]string, error) {
	messages := []message{
		{Role: "system", Content: rerollMeaningSystemPrompt},
		{Role: "user", Content: marshalUserMsg(map[string]string{"word": word, "current_meaning": currentMeaning})},
	}
	text, err := callGoogle(model, "", messages)
	if err != nil {
		return nil, err
	}
	var result []string
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, nil
}

func rerollExamplesGoogle(word, model string) ([]examplePair, error) {
	messages := []message{
		{Role: "system", Content: rerollExamplesSystemPrompt},
		{Role: "user", Content: word},
	}
	text, err := callGoogle(model, "", messages)
	if err != nil {
		return nil, err
	}
	var result []examplePair
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, nil
}
