# Japanese Trainer

- track and drill Japanese vocabulary
- read Japanese text with translation and vocabulary assisstance plus text-to-speech narration
- practice conversation and other language exercises with an AI tutor

The five main pages:

- `Lexicon`: your word list with readings, meanings, examples, images, and drill stats
- `Drill`: efficient, round-based vocab drills
- `Activity`: stats and calender tracking your activity
- `Stories`: import any Japanese text with translation (remote AI generated) and text-to-speech audio narration (locally generated) 
- `Tutor`: AI chat bots for conversation and other forms of language practice

## Setup

1. Install Go 1.25 or newer.
2. Clone this repository.
3. Start the app:

```bash
# from project root directory
cd src
go build
./jpvocab 
```

(On Windows, the executable will be named `jpvocab.exe`)

The application is a webapp served from `http://localhost:49200/`. Open that URL in your browser after starting the server.

## AI translations and tutor bots

To generate AI translations or use the tutor bots, you'll need an API key from an AI provider, such as OpenAI or Anthropic. As of early 2026, you generally should be fine using lower-tier models, and typical usage will not consume a lot of tokens. (In development, I tested with `gpt-4o-mini` and rarely used more than $0.03 of tokens per day.)

To use a key, set the appropriate environment variable with your key as value before starting the app:

- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`
- `GOOGLE_API_KEY`
- `MISTRAL_API_KEY`
- `GLM_API_KEY`

[How to set an environment variable for an API key.](https://help.openai.com/en/articles/5112595-best-practices-for-api-key-safety)

## Text-to-speech

The app can use the browser's built-in text-to-speech to read Japanese.

For higher quality output, install and run [VoiceVox](https://voicevox.hiroshiba.jp/). For the app to use VoiceVox, it must be running and listening at `http://localhost:50021` (the default).

You can customize voice options in the app's settings menu (the gear icon in the header).

## Speech-to-test

The tutor text input supports the browser's builtin speech-to-text. (This feature is currently supported in Chrome but not Firefox.)

## Devs

See notes in [DEVS.md](/Users/brianwill/code/projects/jpvocab/DEVS.md).
