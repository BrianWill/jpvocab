package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// message is a single chat turn shared by all AI provider helpers.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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

// examplePair holds a Japanese/English example sentence pair.
type examplePair struct {
	JP string `json:"jp"`
	EN string `json:"en"`
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
- "reading": the word's reading in hiragana (use katakana only for loanwords); always include this even when the word has kanji — it is the full phonetic reading of the whole word
- "part_of_speech": must be exactly one of: ` + strings.Join(validPartsOfSpeech, ", ") + `. Always prefer the closest matching category; only use "other" if the word genuinely fits none of them.
- "meaning": concise English meaning (one short phrase or sentence)
- "example_jp": a short, natural example sentence in Japanese using the word
- "example_en": English translation of the example sentence
- "kanji": array of objects, one per kanji character in the word in order of appearance, each with:
  - "character": the kanji character
  - "reading": this kanji's reading in this specific word — use hiragana for kun'yomi, katakana for on'yomi; the readings of all kanji, taken in order, must concatenate to spell out the word's full reading exactly (e.g. for 日本語 read as にほんご, the readings must be ニ + ホン + ゴ, not ニチ + ホン + ゴ)
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
	{
		// Demonstrates that kanji readings must match the word's actual pronunciation,
		// not the kanji's most common standalone reading (日 → ニ here, not ニチ).
		word: "日本語",
		result: `{"reading":"にほんご","part_of_speech":"noun","meaning":"Japanese language","example_jp":"日本語を毎日勉強しています。","example_en":"I study Japanese every day.","kanji":[{"character":"日","reading":"ニ","meanings":["sun","day","Japan"]},{"character":"本","reading":"ホン","meanings":["book","origin","Japan"]},{"character":"語","reading":"ゴ","meanings":["language","word","speech"]}]}`,
	},
}

const rerollMeaningSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word and its current English meaning, return a JSON array of exactly 3 alternative concise English meanings (short phrases). Do not repeat the current meaning. Return only the JSON array with no markdown, no code fences, and no extra commentary.`

const rerollExamplesSystemPrompt = `You are a Japanese dictionary assistant. Given a Japanese word, return a JSON array of exactly 3 natural example sentences using that word. Each entry must have "jp" (the Japanese sentence) and "en" (its English translation). Return only the JSON array with no markdown, no code fences, and no extra commentary.`

const suggestImageSearchQuerySystemPrompt = `You are a helpful assistant. Given a Japanese word and its English meaning, return a JSON object with a single field "query" containing a concise English search query (2-5 words) suitable for finding a clear, representative photo on a stock photo site. Prefer concrete, visual terms. Return only the JSON object with no markdown, no code fences, and no extra commentary.`

const suggestImageSystemPrompt = `You are a helpful assistant. Given a Japanese word and its English meaning, return a JSON object with a single field "url" containing a URL to a freely licensed image on Wikimedia Commons using the Special:FilePath format: https://commons.wikimedia.org/wiki/Special:FilePath/<filename>. Choose a well-known, unambiguous photo that directly represents the concept. Return only a valid JSON object with no markdown, no code fences, and no extra commentary.`

// suggestImageSearchQuery asks the AI for a short English search query for the given word.
func suggestImageSearchQuery(word, meaning, providerModel string) (string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]
	userMsg := marshalUserMsg(map[string]string{"word": word, "meaning": meaning})

	var text, jsonPrefix string
	var err error
	if provider == "anthropic" {
		messages := []message{
			{Role: "user", Content: userMsg},
			{Role: "assistant", Content: "{"},
		}
		text, err = callAnthropic(model, suggestImageSearchQuerySystemPrompt, messages, 64)
		jsonPrefix = "{"
	} else {
		messages := []message{
			{Role: "system", Content: suggestImageSearchQuerySystemPrompt},
			{Role: "user", Content: userMsg},
		}
		switch provider {
		case "openai":
			text, err = callOpenAI(model, messages)
		case "google":
			text, err = callGoogle(model, "", messages)
		case "mistral":
			text, err = callMistral(model, messages)
		default: // glm
			text, err = callGLM(model, messages)
		}
	}
	if err != nil {
		return "", err
	}
	var result struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(jsonPrefix+text), &result); err != nil {
		return "", fmt.Errorf("parse image search query JSON: %w", err)
	}
	if result.Query == "" {
		return "", fmt.Errorf("empty query in image search response")
	}
	return result.Query, nil
}

// suggestImageURL asks the AI to suggest a Wikimedia Commons image URL for the given word.
func suggestImageURL(word, meaning, providerModel string) (string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]
	userMsg := marshalUserMsg(map[string]string{"word": word, "meaning": meaning})

	var text, jsonPrefix string
	var err error
	if provider == "anthropic" {
		messages := []message{
			{Role: "user", Content: userMsg},
			{Role: "assistant", Content: "{"},
		}
		text, err = callAnthropic(model, suggestImageSystemPrompt, messages, 256)
		jsonPrefix = "{"
	} else {
		messages := []message{
			{Role: "system", Content: suggestImageSystemPrompt},
			{Role: "user", Content: userMsg},
		}
		switch provider {
		case "openai":
			text, err = callOpenAI(model, messages)
		case "google":
			text, err = callGoogle(model, "", messages)
		case "mistral":
			text, err = callMistral(model, messages)
		default: // glm
			text, err = callGLM(model, messages)
		}
	}
	if err != nil {
		return "", err
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(jsonPrefix+text), &result); err != nil {
		return "", fmt.Errorf("parse image URL JSON: %w", err)
	}
	if result.URL == "" {
		return "", fmt.Errorf("empty URL in image suggestion response")
	}
	return result.URL, nil
}

// aiProviders holds which AI providers have API keys configured.
type aiProviders struct {
	AnthropicAvail bool
	OpenAIAvail    bool
	GoogleAvail    bool
	MistralAvail   bool
	GLMAvail       bool
}

// checkAIProviders reports which providers have API keys set in the environment.
func checkAIProviders() aiProviders {
	return aiProviders{
		AnthropicAvail: os.Getenv("ANTHROPIC_API_KEY") != "",
		OpenAIAvail:    os.Getenv("OPENAI_API_KEY") != "",
		GoogleAvail:    os.Getenv("GOOGLE_API_KEY") != "",
		MistralAvail:   os.Getenv("MISTRAL_API_KEY") != "",
		GLMAvail:       os.Getenv("GLM_API_KEY") != "",
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
	case "google":
		return autoFillWordGoogle(word, parts[1])
	case "mistral":
		return autoFillWordMistral(word, parts[1])
	case "glm":
		return autoFillWordGLM(word, parts[1])
	default:
		return autoFillWordAnthropic(word, parts[1])
	}
}

// rerollMeaning asks the AI for 3 alternative English meanings for a word.
func rerollMeaning(word, currentMeaning, providerModel string) ([]string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	switch parts[0] {
	case "openai":
		return rerollMeaningOpenAI(word, currentMeaning, parts[1])
	case "google":
		return rerollMeaningGoogle(word, currentMeaning, parts[1])
	case "mistral":
		return rerollMeaningMistral(word, currentMeaning, parts[1])
	case "glm":
		return rerollMeaningGLM(word, currentMeaning, parts[1])
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
	case "google":
		return rerollExamplesGoogle(word, parts[1])
	case "mistral":
		return rerollExamplesMistral(word, parts[1])
	case "glm":
		return rerollExamplesGLM(word, parts[1])
	default:
		return rerollExamplesAnthropic(word, parts[1])
	}
}

// marshalUserMsg marshals a map to JSON, returning the string form.
// Used to build structured user messages for reroll requests.
func marshalUserMsg(v map[string]string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
