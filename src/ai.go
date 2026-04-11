package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// tokenUsage holds the input/output token counts from a single AI API call.
type tokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// message is a single chat turn shared by all AI provider helpers.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// kanjiAutoFillEntry holds AI-generated data for one kanji character within a word.
type kanjiAutoFillEntry struct {
	Character string   `json:"character"`
	Reading   string   `json:"reading"` // hiragana for kun'yomi, katakana for on'yomi
	Meanings  []string `json:"meanings"`
}

// wordAutoFill holds AI-generated fields for a Japanese word.
type wordAutoFill struct {
	Reading      string               `json:"reading"`
	PitchAccent  *int                 `json:"pitch_accent"`
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
- "pitch_accent": NHK-style pitch accent as an integer — 0 means heiban (rises after mora 1, never drops), 1 means atamadaka (drops after mora 1), N means the pitch drops after mora N; use null only if genuinely uncertain (e.g. 食べる → 2, 電話 → 0, 橋 → 0, 箸 → 1)
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
		word:   "食べる",
		result: `{"reading":"たべる","pitch_accent":2,"part_of_speech":"ichidan-verb","meaning":"to eat","example_jp":"朝ごはんを食べる。","example_en":"I eat breakfast.","kanji":[{"character":"食","reading":"た","meanings":["eat","food","meal"]}]}`,
	},
	{
		word:   "電話",
		result: `{"reading":"でんわ","pitch_accent":0,"part_of_speech":"noun","meaning":"telephone; phone call","example_jp":"電話をかけてもいいですか。","example_en":"May I make a phone call?","kanji":[{"character":"電","reading":"デン","meanings":["electricity","lightning","electric"]},{"character":"話","reading":"ワ","meanings":["talk","speech","story","conversation"]}]}`,
	},
	{
		word:   "きれい",
		result: `{"reading":"きれい","pitch_accent":0,"part_of_speech":"na-adjective","meaning":"beautiful; clean; pretty","example_jp":"この花はきれいですね。","example_en":"This flower is beautiful, isn't it?","kanji":[]}`,
	},
	{
		word:   "話す",
		result: `{"reading":"はなす","pitch_accent":2,"part_of_speech":"godan-verb","meaning":"to speak; to talk","example_jp":"友達と日本語で話す。","example_en":"I speak in Japanese with my friend.","kanji":[{"character":"話","reading":"はな","meanings":["talk","speech","story","conversation"]}]}`,
	},
	{
		word:   "早い",
		result: `{"reading":"はやい","pitch_accent":2,"part_of_speech":"i-adjective","meaning":"early; fast","example_jp":"今日は起きるのが早い。","example_en":"I wake up early today.","kanji":[{"character":"早","reading":"はや","meanings":["early","fast","quick"]}]}`,
	},
	{
		word:   "たぶん",
		result: `{"reading":"たぶん","pitch_accent":1,"part_of_speech":"adverb","meaning":"probably; perhaps","example_jp":"たぶん明日は雨です。","example_en":"It will probably rain tomorrow.","kanji":[]}`,
	},
	{
		word:   "こんにちは",
		result: `{"reading":"こんにちは","pitch_accent":0,"part_of_speech":"other","meaning":"hello; good afternoon","example_jp":"先生にこんにちはと言った。","example_en":"I said hello to the teacher.","kanji":[]}`,
	},
	{
		// Demonstrates that kanji readings must match the word's actual pronunciation,
		// not the kanji's most common standalone reading (日 → ニ here, not ニチ).
		word:   "日本語",
		result: `{"reading":"にほんご","pitch_accent":0,"part_of_speech":"noun","meaning":"Japanese language","example_jp":"日本語を毎日勉強しています。","example_en":"I study Japanese every day.","kanji":[{"character":"日","reading":"ニ","meanings":["sun","day","Japan"]},{"character":"本","reading":"ホン","meanings":["book","origin","Japan"]},{"character":"語","reading":"ゴ","meanings":["language","word","speech"]}]}`,
	},
}

// autoFillBatchSize is the maximum number of words sent in a single AI batch request.
const autoFillBatchSize = 20

// autoFillBatchSystemPrompt instructs the AI to process an array of words at once.
var autoFillBatchSystemPrompt = `You are a Japanese dictionary assistant. Given a JSON array of Japanese words or phrases, return a JSON array of objects in the same order — one object per input word. Each object must have exactly these fields:
- "reading": the word's reading in hiragana (use katakana only for loanwords); always include this even when the word has kanji — it is the full phonetic reading of the whole word
- "pitch_accent": NHK-style pitch accent as an integer — 0 means heiban (rises after mora 1, never drops), 1 means atamadaka (drops after mora 1), N means the pitch drops after mora N; use null only if genuinely uncertain (e.g. 食べる → 2, 電話 → 0, 橋 → 0, 箸 → 1)
- "part_of_speech": must be exactly one of: ` + strings.Join(validPartsOfSpeech, ", ") + `. Always prefer the closest matching category; only use "other" if the word genuinely fits none of them.
- "meaning": concise English meaning (one short phrase or sentence)
- "example_jp": a short, natural example sentence in Japanese using the word
- "example_en": English translation of the example sentence
- "kanji": array of objects, one per kanji character in the word in order of appearance, each with:
  - "character": the kanji character
  - "reading": this kanji's reading in this specific word — use hiragana for kun'yomi, katakana for on'yomi; the readings of all kanji, taken in order, must concatenate to spell out the word's full reading exactly (e.g. for 日本語 read as にほんご, the readings must be ニ + ホン + ゴ, not ニチ + ホン + ゴ)
  - "meanings": array of concise English meanings for this kanji (2–4 entries)
  For words with no kanji (e.g. pure kana or katakana loanwords), use an empty array.
The output array must contain exactly as many objects as the input array, in the same order.
Return only a valid JSON array with no markdown, no code fences, and no extra commentary.`

const aiJSONRetryCount = 3

func autoFillBatchFewShot() (string, string) {
	count := min(4, len(autoFillExamples))
	inputWords := make([]string, 0, count)
	outputs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		inputWords = append(inputWords, autoFillExamples[i].word)
		outputs = append(outputs, autoFillExamples[i].result)
	}
	inputJSON, _ := json.Marshal(inputWords)
	return string(inputJSON), "[" + strings.Join(outputs, ",") + "]"
}

func extractJSONArray(text string) string {
	start := strings.IndexByte(text, '[')
	end := strings.LastIndexByte(text, ']')
	if start < 0 || end < start {
		return text
	}
	return text[start : end+1]
}

func extractJSONObject(text string) string {
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return text
	}
	return text[start : end+1]
}

func unmarshalJSONArrayWithSalvage(text string, v any) error {
	if err := json.Unmarshal([]byte(text), v); err == nil {
		return nil
	}
	salvaged := extractJSONArray(text)
	if salvaged == text {
		return fmt.Errorf("invalid JSON array")
	}
	return json.Unmarshal([]byte(salvaged), v)
}

func unmarshalJSONObjectWithSalvage(text string, v any) error {
	if err := json.Unmarshal([]byte(text), v); err == nil {
		return nil
	}
	salvaged := extractJSONObject(text)
	if salvaged == text {
		return fmt.Errorf("invalid JSON object")
	}
	return json.Unmarshal([]byte(salvaged), v)
}

func retryJSONRequest[T any](label string, fn func() (string, tokenUsage, error), parse func(string) (T, error)) (T, tokenUsage, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= aiJSONRetryCount; attempt++ {
		text, usage, err := fn()
		if err == nil {
			value, parseErr := parse(text)
			if parseErr == nil {
				return value, usage, nil
			}
			err = fmt.Errorf("%s parse: %w", label, parseErr)
		}
		lastErr = err
		if attempt < aiJSONRetryCount {
			time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
		}
	}
	return zero, tokenUsage{}, lastErr
}

// autoFillWordsBatch sends words to the AI in chunks of autoFillBatchSize, processing
// chunks concurrently. The returned slice has exactly len(words) entries; entries are nil
// for any chunk that fails or returns fewer results than expected.
func autoFillWordsBatch(db *sql.DB, words []string, providerModel string) ([]*wordAutoFill, error) {
	fills, _, err := autoFillWordsBatchWithUsage(db, words, providerModel)
	return fills, err
}

// autoFillWordsBatchWithUsage is like autoFillWordsBatch but also returns the
// aggregated token usage across all AI calls made for the batch.
func autoFillWordsBatchWithUsage(db *sql.DB, words []string, providerModel string) ([]*wordAutoFill, tokenUsage, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, tokenUsage{}, fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]

	type chunk struct {
		start int
		words []string
	}
	var chunks []chunk
	for i := 0; i < len(words); i += autoFillBatchSize {
		end := min(i+autoFillBatchSize, len(words))
		chunks = append(chunks, chunk{start: i, words: words[i:end]})
	}

	results := make([]*wordAutoFill, len(words))
	var mu sync.Mutex
	var totalUsage tokenUsage
	var wg sync.WaitGroup
	for _, c := range chunks {
		wg.Add(1)
		go func(ch chunk) {
			defer wg.Done()
			var fills []*wordAutoFill
			var usage tokenUsage
			var err error
			switch provider {
			case "openai":
				fills, usage, err = autoFillWordsBatchOpenAI(ch.words, model)
			case "google":
				fills, usage, err = autoFillWordsBatchGoogle(ch.words, model)
			case "mistral":
				fills, usage, err = autoFillWordsBatchMistral(ch.words, model)
			case "glm":
				fills, usage, err = autoFillWordsBatchGLM(ch.words, model)
			default:
				fills, usage, err = autoFillWordsBatchAnthropic(ch.words, model)
			}
			if err != nil {
				return // entries for this chunk remain nil
			}
			mu.Lock()
			totalUsage.InputTokens += usage.InputTokens
			totalUsage.OutputTokens += usage.OutputTokens
			for i, f := range fills {
				if ch.start+i < len(results) {
					results[ch.start+i] = f
				}
			}
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	insertTokenUsage(db, provider, model, "autofill-batch", totalUsage.InputTokens, totalUsage.OutputTokens)
	return results, totalUsage, nil
}

const suggestImageSearchQuerySystemPrompt = `You are a helpful assistant. Given a Japanese word and its English meaning, return a JSON object with a single field "query" containing a concise English search query (2-5 words) suitable for finding a clear, representative photo on a stock photo site. Prefer concrete, visual terms. Return only the JSON object with no markdown, no code fences, and no extra commentary.`

const suggestImageSystemPrompt = `You are a helpful assistant. Given a Japanese word and its English meaning, return a JSON object with a single field "url" containing a URL to a freely licensed image on Wikimedia Commons using the Special:FilePath format: https://commons.wikimedia.org/wiki/Special:FilePath/<filename>. Choose a well-known, unambiguous photo that directly represents the concept. Return only a valid JSON object with no markdown, no code fences, and no extra commentary.`

// suggestImageSearchQuery asks the AI for a short English search query for the given word.
func suggestImageSearchQuery(db *sql.DB, word, meaning, providerModel string) (string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]
	userMsg := marshalUserMsg(map[string]string{"word": word, "meaning": meaning})

	result, usage, err := retryJSONRequest("suggest image query", func() (string, tokenUsage, error) {
		if provider == "anthropic" {
			messages := []message{
				{Role: "user", Content: userMsg},
				{Role: "assistant", Content: "{"},
			}
			text, usage, err := callAnthropic(model, suggestImageSearchQuerySystemPrompt, messages, 64)
			if err != nil {
				return "", tokenUsage{}, err
			}
			return "{" + text, usage, nil
		}
		messages := []message{
			{Role: "system", Content: suggestImageSearchQuerySystemPrompt},
			{Role: "user", Content: userMsg},
		}
		switch provider {
		case "openai":
			return callOpenAI(model, messages)
		case "google":
			return callGoogle(model, "", messages)
		case "mistral":
			return callMistral(model, messages)
		default:
			return callGLM(model, messages)
		}
	}, func(text string) (struct{ Query string `json:"query"` }, error) {
		var result struct {
			Query string `json:"query"`
		}
		if err := unmarshalJSONObjectWithSalvage(text, &result); err != nil {
			return result, fmt.Errorf("parse image search query JSON: %w", err)
		}
		return result, nil
	})
	if err != nil {
		return "", err
	}
	insertTokenUsage(db, provider, model, "suggest-image-query", usage.InputTokens, usage.OutputTokens)
	if result.Query == "" {
		return "", fmt.Errorf("empty query in image search response")
	}
	return result.Query, nil
}

// suggestImageURL asks the AI to suggest a Wikimedia Commons image URL for the given word.
func suggestImageURL(db *sql.DB, word, meaning, providerModel string) (string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]
	userMsg := marshalUserMsg(map[string]string{"word": word, "meaning": meaning})

	result, usage, err := retryJSONRequest("suggest image URL", func() (string, tokenUsage, error) {
		if provider == "anthropic" {
			messages := []message{
				{Role: "user", Content: userMsg},
				{Role: "assistant", Content: "{"},
			}
			text, usage, err := callAnthropic(model, suggestImageSystemPrompt, messages, 256)
			if err != nil {
				return "", tokenUsage{}, err
			}
			return "{" + text, usage, nil
		}
		messages := []message{
			{Role: "system", Content: suggestImageSystemPrompt},
			{Role: "user", Content: userMsg},
		}
		switch provider {
		case "openai":
			return callOpenAI(model, messages)
		case "google":
			return callGoogle(model, "", messages)
		case "mistral":
			return callMistral(model, messages)
		default:
			return callGLM(model, messages)
		}
	}, func(text string) (struct{ URL string `json:"url"` }, error) {
		var result struct {
			URL string `json:"url"`
		}
		if err := unmarshalJSONObjectWithSalvage(text, &result); err != nil {
			return result, fmt.Errorf("parse image URL JSON: %w", err)
		}
		return result, nil
	})
	if err != nil {
		return "", err
	}
	insertTokenUsage(db, provider, model, "suggest-image", usage.InputTokens, usage.OutputTokens)
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

// autoFillWord dispatches to the appropriate AI provider and records token usage.
// providerModel must be in "provider/model" format, e.g. "anthropic/claude-haiku-4-5-20251001".
func autoFillWord(db *sql.DB, word, providerModel string) (*wordAutoFill, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ai_model value %q: expected provider/model", providerModel)
	}
	provider, model := parts[0], parts[1]
	var result *wordAutoFill
	var usage tokenUsage
	var err error
	switch provider {
	case "openai":
		result, usage, err = autoFillWordOpenAI(word, model)
	case "google":
		result, usage, err = autoFillWordGoogle(word, model)
	case "mistral":
		result, usage, err = autoFillWordMistral(word, model)
	case "glm":
		result, usage, err = autoFillWordGLM(word, model)
	default:
		result, usage, err = autoFillWordAnthropic(word, model)
	}
	if err != nil {
		return nil, err
	}
	insertTokenUsage(db, provider, model, "autofill", usage.InputTokens, usage.OutputTokens)
	return result, nil
}

// marshalUserMsg marshals a map to JSON, returning the string form.
// Used to build structured user messages for AI requests.
func marshalUserMsg(v map[string]string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// storyTranslationResult holds the AI-generated sentence translations for a story.
type storyTranslationResult struct {
	Sentences []string `json:"sentences"`
}

// translateStory sends all sentences to the AI in a single call and returns ordered
// English translations plus the token usage for the underlying model call.
// providerModel must be "provider/model" format.
func translateStory(db *sql.DB, sentences []string, providerModel string) (*storyTranslationResult, tokenUsage, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return nil, tokenUsage{}, fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]
	prompt := `You are a Japanese language teacher helping English-speaking students read Japanese stories. Given a JSON array of Japanese sentences, return an equally ordered JSON array of English translations.

Instructions:
- Translate each sentence in isolation without using surrounding sentences for context.
- Favor literal, morpheme-by-morpheme accuracy over natural English fluency — the goal is to help learners understand Japanese grammatical structure, not to produce polished prose.

Example input:
["猫が窓の外を見ている。","彼女はゆっくりと立ち上がった。"]

Example output:
{"sentences":["The cat is looking at the outside of the window.","She slowly stood up."]}

Return only valid JSON with no markdown, no code fences, and no extra commentary.`

	userMsg, err := json.Marshal(sentences)
	if err != nil {
		return nil, tokenUsage{}, err
	}

	const maxTokens = 8192
	result, usage, err := retryJSONRequest("translate story", func() (string, tokenUsage, error) {
		if provider == "anthropic" {
			msgs := []message{
				{Role: "user", Content: string(userMsg)},
				{Role: "assistant", Content: "{"},
			}
			text, usage, err := callAnthropic(model, prompt, msgs, maxTokens)
			if err != nil {
				return "", tokenUsage{}, err
			}
			return "{" + text, usage, nil
		}
		msgs := []message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: string(userMsg)},
		}
		switch provider {
		case "openai":
			return callOpenAI(model, msgs)
		case "google":
			return callGoogle(model, "", msgs)
		case "mistral":
			return callMistral(model, msgs)
		default:
			return callGLM(model, msgs)
		}
	}, func(text string) (*storyTranslationResult, error) {
		var result storyTranslationResult
		if err := unmarshalJSONObjectWithSalvage(text, &result); err != nil {
			return nil, fmt.Errorf("parse translation JSON: %w", err)
		}
		return &result, nil
	})
	if err != nil {
		return nil, tokenUsage{}, err
	}
	insertTokenUsage(db, provider, model, "translate-story", usage.InputTokens, usage.OutputTokens)
	return result, usage, nil
}

// tutorChat sends the conversation history to the AI and returns its reply.
// providerModel must be "provider/model" format. systemPrompt primes the AI's behavior.
func tutorChat(db *sql.DB, msgs []message, systemPrompt, providerModel string) (string, error) {
	parts := strings.SplitN(providerModel, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ai_model value %q", providerModel)
	}
	provider, model := parts[0], parts[1]

	var reply string
	var usage tokenUsage
	var err error

	switch provider {
	case "openai":
		allMsgs := append([]message{{Role: "system", Content: systemPrompt}}, msgs...)
		reply, usage, err = callOpenAI(model, allMsgs)
	case "google":
		reply, usage, err = callGoogle(model, systemPrompt, msgs)
	case "mistral":
		allMsgs := append([]message{{Role: "system", Content: systemPrompt}}, msgs...)
		reply, usage, err = callMistral(model, allMsgs)
	case "glm":
		allMsgs := append([]message{{Role: "system", Content: systemPrompt}}, msgs...)
		reply, usage, err = callGLM(model, allMsgs)
	default: // anthropic
		reply, usage, err = callAnthropic(model, systemPrompt, msgs, 2048)
	}
	if err != nil {
		return "", err
	}
	insertTokenUsage(db, provider, model, "tutor", usage.InputTokens, usage.OutputTokens)
	return reply, nil
}
