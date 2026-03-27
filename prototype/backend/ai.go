package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// kanjiAutoFillEntry holds AI-generated data for one kanji character within a word.
type kanjiAutoFillEntry struct {
	Character string   `json:"character"`
	Reading   string   `json:"reading"`  // hiragana for kun'yomi, katakana for on'yomi
	Meanings  []string `json:"meanings"`
}

// wordAutoFill holds AI-generated fields for a Japanese word.
type wordAutoFill struct {
	Reading      string               `json:"reading"`
	PartOfSpeech string               `json:"part_of_speech"`
	Meaning      string               `json:"meaning"`
	ExampleJP    string               `json:"example_jp"`
	ExampleEN    string               `json:"example_en"`
	Kanji        []kanjiAutoFillEntry `json:"kanji"`
}

// validPartsOfSpeech is the canonical closed set of part-of-speech values used throughout the app.
// The AI is instructed to use only these; anything that doesn't fit maps to "other".
var validPartsOfSpeech = []string{
	"godan-verb",
	"ichidan-verb",
	"noun",
	"i-adjective",
	"na-adjective",
	"adverb",
	"other",
}

var autoFillSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word or phrase, return a JSON object with exactly these fields:
- "reading": the word's reading in hiragana (use katakana only for loanwords)
- "part_of_speech": must be exactly one of: ` + strings.Join(validPartsOfSpeech, ", ") + `. Always prefer the closest matching category; only use "other" if the word genuinely fits none of them.
- "meaning": concise English meaning (one short phrase or sentence)
- "example_jp": a short, natural example sentence in Japanese using the word
- "example_en": English translation of the example sentence
- "kanji": array of objects, one per kanji character in the word in order of appearance, each with:
  - "character": the kanji character
  - "reading": this kanji's reading in this specific word — use hiragana for kun'yomi, katakana for on'yomi
  - "meanings": array of concise English meanings for this kanji (2–4 entries)
  For words with no kanji (e.g. pure kana or katakana loanwords), use an empty array.
Return only a valid JSON object with no markdown, no code fences, and no extra commentary.`

// autoFillExample holds a single few-shot example: the input word and the expected JSON output.
type autoFillExample struct {
	word   string
	result string
}

// autoFillExamples are few-shot examples prepended to every request to improve output reliability.
// They cover: a verb with kun'yomi kanji, a noun with on'yomi kanji, and a pure-kana word.
var autoFillExamples = []autoFillExample{
	{
		word: "食べる",
		result: `{"reading":"たべる","part_of_speech":"ichidan-verb","meaning":"to eat","example_jp":"朝ごはんを食べる。","example_en":"I eat breakfast.","kanji":[{"character":"食","reading":"た","meanings":["eat","food","meal"]}]}`,
	},
	{
		word: "電話",
		result: `{"reading":"でんわ","part_of_speech":"noun","meaning":"telephone; phone call","example_jp":"電話をかけてもいいですか。","example_en":"May I make a phone call?","kanji":[{"character":"電","reading":"デン","meanings":["electricity","lightning","electric"]},{"character":"話","reading":"ワ","meanings":["talk","speech","story","conversation"]}]}`,
	},
	{
		word: "きれい",
		result: `{"reading":"きれい","part_of_speech":"na-adjective","meaning":"beautiful; clean; pretty","example_jp":"この花はきれいですね。","example_en":"This flower is beautiful, isn't it?","kanji":[]}`,
	},
}

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

	messages := make([]message, 0, len(autoFillExamples)*2+1)
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})
	messages = append(messages, message{Role: "assistant", Content: "{"})

	payload, err := json.Marshal(reqBody{
		Model:     model,
		MaxTokens: 512,
		System:    autoFillSystemPrompt,
		Messages:  messages,
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
	if err := json.Unmarshal([]byte("{"+apiResp.Content[0].Text), &e); err != nil {
		return nil, fmt.Errorf("parse auto-fill JSON: %w", err)
	}
	return &e, nil
}

// examplePair holds a Japanese/English example sentence pair.
type examplePair struct {
	JP string `json:"jp"`
	EN string `json:"en"`
}

const rerollMeaningSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word and its current English meaning, return a JSON array of exactly 3 alternative concise English meanings (short phrases). Do not repeat the current meaning. Return only the JSON array with no markdown, no code fences, and no extra commentary.`

const rerollExamplesSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word, return a JSON array of exactly 3 natural example sentences using that word. Each entry must have "jp" (the Japanese sentence) and "en" (its English translation). Return only the JSON array with no markdown, no code fences, and no extra commentary.`

// rerollMeaning asks the AI for 3 alternative English meanings for a word.
func rerollMeaning(word, currentMeaning, providerModel string) ([]string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	switch parts[0] {
	case "openai":
		return rerollMeaningOpenAI(word, currentMeaning, parts[1])
	default:
		return rerollMeaningAnthropic(word, currentMeaning, parts[1])
	}
}

// rerollExamples asks the AI for 3 alternative example sentence pairs.
func rerollExamples(word, providerModel string) ([]examplePair, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	switch parts[0] {
	case "openai":
		return rerollExamplesOpenAI(word, parts[1])
	default:
		return rerollExamplesAnthropic(word, parts[1])
	}
}

func rerollMeaningAnthropic(word, currentMeaning, model string) ([]string, error) {
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
	userMsgBytes, _ := json.Marshal(map[string]string{"word": word, "current_meaning": currentMeaning})
	payload, err := json.Marshal(reqBody{
		Model:     model,
		MaxTokens: 256,
		System:    rerollMeaningSystemPrompt,
		Messages: []message{
			{Role: "user", Content: string(userMsgBytes)},
			{Role: "assistant", Content: "["},
		},
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
		Error   *struct{ Message string `json:"message"` } `json:"error"`
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
	var result []string
	if err := json.Unmarshal([]byte("["+apiResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, nil
}

func rerollMeaningOpenAI(word, currentMeaning, model string) ([]string, error) {
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
	userMsgBytes, _ := json.Marshal(map[string]string{"word": word, "current_meaning": currentMeaning})
	payload, err := json.Marshal(reqBody{
		Model: model,
		Messages: []message{
			{Role: "system", Content: rerollMeaningSystemPrompt},
			{Role: "user", Content: string(userMsgBytes)},
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
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
		Error *struct{ Message string `json:"message"` } `json:"error"`
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
	var result []string
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-meaning JSON: %w", err)
	}
	return result, nil
}

func rerollExamplesAnthropic(word, model string) ([]examplePair, error) {
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
		System:    rerollExamplesSystemPrompt,
		Messages: []message{
			{Role: "user", Content: word},
			{Role: "assistant", Content: "["},
		},
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
		Error   *struct{ Message string `json:"message"` } `json:"error"`
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
	var result []examplePair
	if err := json.Unmarshal([]byte("["+apiResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, nil
}

func rerollExamplesOpenAI(word, model string) ([]examplePair, error) {
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
			{Role: "system", Content: rerollExamplesSystemPrompt},
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
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
		Error *struct{ Message string `json:"message"` } `json:"error"`
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
	var result []examplePair
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse reroll-examples JSON: %w", err)
	}
	return result, nil
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

	messages := make([]message, 0, len(autoFillExamples)*2+2)
	messages = append(messages, message{Role: "system", Content: autoFillSystemPrompt})
	for _, ex := range autoFillExamples {
		messages = append(messages, message{Role: "user", Content: ex.word})
		messages = append(messages, message{Role: "assistant", Content: ex.result})
	}
	messages = append(messages, message{Role: "user", Content: word})

	payload, err := json.Marshal(reqBody{
		Model:    model,
		Messages: messages,
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
