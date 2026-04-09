package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// callGLM sends a request to the Zhipu GLM API (OpenAI-compatible format) and returns
// the content of the first choice plus token usage. The API key is read from GLM_API_KEY.
func callGLM(model string, messages []message) (string, tokenUsage, error) {
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		return "", tokenUsage{}, fmt.Errorf("GLM_API_KEY environment variable is not set")
	}

	type reqBody struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}
	payload, err := json.Marshal(reqBody{Model: model, Messages: messages})
	if err != nil {
		return "", tokenUsage{}, err
	}

	req, err := http.NewRequest("POST", "https://open.bigmodel.cn/api/paas/v4/chat/completions", bytes.NewReader(payload))
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

func autoFillWordGLM(word, model string) (*wordAutoFill, tokenUsage, error) {
	messages := make([]message, 0, len(autoFillExamples)*2+2)
	messages = append(messages, message{Role: "system", Content: autoFillSystemPrompt})
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})

	return retryJSONRequest("glm autofill", func() (string, tokenUsage, error) {
		return callGLM(model, messages)
	}, func(text string) (*wordAutoFill, error) {
		var e wordAutoFill
		if err := unmarshalJSONObjectWithSalvage(text, &e); err != nil {
			return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
		}
		return &e, nil
	})
}

func autoFillWordsBatchGLM(words []string, model string) ([]*wordAutoFill, tokenUsage, error) {
	exInput, exOutput := autoFillBatchFewShot()
	input, _ := json.Marshal(words)
	messages := []message{
		{Role: "system", Content: autoFillBatchSystemPrompt},
		{Role: "user", Content: string(exInput)},
		{Role: "assistant", Content: exOutput},
		{Role: "user", Content: string(input)},
	}
	return retryJSONRequest("glm batch autofill", func() (string, tokenUsage, error) {
		return callGLM(model, messages)
	}, func(text string) ([]*wordAutoFill, error) {
		var fills []*wordAutoFill
		if err := unmarshalJSONArrayWithSalvage(text, &fills); err != nil {
			return nil, fmt.Errorf("parse batch auto-fill JSON: %w", err)
		}
		return fills, nil
	})
}

func rerollMeaningGLM(word, currentMeaning, model string) ([]string, tokenUsage, error) {
	messages := []message{
		{Role: "system", Content: rerollMeaningSystemPrompt},
		{Role: "user", Content: marshalUserMsg(map[string]string{"word": word, "current_meaning": currentMeaning})},
	}
	return retryJSONRequest("glm reroll meaning", func() (string, tokenUsage, error) {
		return callGLM(model, messages)
	}, func(text string) ([]string, error) {
		var result []string
		if err := unmarshalJSONArrayWithSalvage(text, &result); err != nil {
			return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
		}
		return result, nil
	})
}

func rerollExamplesGLM(word, model string) ([]examplePair, tokenUsage, error) {
	messages := []message{
		{Role: "system", Content: rerollExamplesSystemPrompt},
		{Role: "user", Content: word},
	}
	return retryJSONRequest("glm reroll examples", func() (string, tokenUsage, error) {
		return callGLM(model, messages)
	}, func(text string) ([]examplePair, error) {
		var result []examplePair
		if err := unmarshalJSONArrayWithSalvage(text, &result); err != nil {
			return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
		}
		return result, nil
	})
}
