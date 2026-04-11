# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Japanese vocabulary drilling desktop app built with Go backend + web frontend. For now, assume a single user with no user login or need for security. The app has four top-level views:

- **Lexicon** � displays the user's full vocabulary set with word info (reading, part of speech, meaning, example sentences) and per-word correct/incorrect drill counts accumulated over all sessions.
- **Drill** � a round-based flashcard drill. Each round presents 10 words randomly drawn from the lexicon. The user marks each word as known or unknown; unknown words carry over to the next round alongside fresh picks. The current in-progress drill is persisted to SQLite so refreshing `/drill` or restarting the server restores the same round state, sidebar state, and last answered card.
- **Activity** � displays headline stats and a week-by-week calendar of recent drill activity. Each day cell shows how many words were drilled, added, and cleared. Clicking a day opens a detail modal listing the words involved.
- **Stories** - a top-level page at `/stories` that currently lists available stories by title and date. Each entry links to `/stories/{id}`, which renders the story text by reconstructing each sentence from the stored word-token display text. On the detail page, the header still includes the Stories link, but it is intentionally not marked as the active nav item.
  The stories index header also has a lexicon-style `＋` button that opens an add-story modal with title and content fields.
  The stories index also shows an `x` delete button on each card; clicking it opens a confirm modal before deleting that story.
  The story detail layout keeps the story title fixed directly beneath the global header; only the lower story action bar ("back", "generate translation", "generate audio") and the story content scroll.
  The detail page also has a collapsible left "Noted Words" sidebar inside the scroll area. Hovering a non-punctuation word token shows a tooltip hint; when story translation data exists, the tooltip shows the sentence translation plus the hovered word's gloss and reading on separate lines. Clicking the token saves that token's base word into the story's persisted noted-words list; the sidebar lists noted words with remove buttons and a placeholder "Add all to Lexicon" button that is intentionally non-functional for now.
  On desktop, the noted-words sidebar now scrolls independently from the story content so a long saved-word list does not move the story text position; on small screens the layout still collapses to one column and reverts to normal page flow.

Word data (lexicon) is stored in a SQLite database. Per-word drill counts (correct/incorrect) are persisted there, and the current in-progress drill session is also stored in SQLite as a serialized state snapshot on the active `drill_sessions` row.
Story content is also stored in SQLite. A story stores story-level metadata including a required title and an optional narration audio path. Each sentence row stores an ordered JSON sequence of word tokens rather than one raw Japanese string; each token includes display form, base form, an English gloss plus optional reading at API/read time, and an optional audio timestamp in milliseconds into the full-story narration. Sentence rows also carry an optional English sentence translation and a flag indicating whether that sentence starts a new paragraph.

Each word has two integers:

- "current drill count" (total number of times the word has been marked correct across all prior drills)
- "target drill count" (the number of times the word is intended to be drilled)

When words are randomly selected from the lexicon to drill, only active words are chosen.

Incorrect answers are not penalised beyond incrementing the word's lifetime incorrect count. There is no spaced-repetition penalty, cooldown, or demotion mechanic � a wrong answer simply carries the word forward to the next round as normal.

A word tracks three timestamps:

- "creation date": date and time when the word was added to the database
- "last drill date": date and time when the word was last drilled (updated not when drill starts but when the user gives answer for the word)
- "target last reached date": date and time when the word's current drill count matched or exceeded its target drill count (for new words, this starts out null)

## Terminology

- **Lexicon** � the user's full set of vocabulary words (stored in SQLite)
- **Pool** � the working set of words for the current drill session
- **Round** � one cycle of up to 10 words within a drill session

## Tech Stack

- **Backend:** Go 1.24, Chi router, Datastar (SSE)
- **Desktop shell:** Wails v3 (alpha) — wraps the web server in a native window. No Go↔JS bridge is used; Wails simply opens a webview pointed at the local server.
- **Frontend:** Vanilla JS, Vite 3.0.7
- **Database:** SQLite

## Commands

```bash
# Run with hot-reload (from src/)
cd src && air

# Run backend tests
cd src && go test ./...

# Run AI integration tests (makes real API calls � ask the user for permission first)
cd src && go test -tags integration ./...

# Go dependencies
go mod tidy

# Regenerate Windows icon resource (run after changing static/favicon.ico)
cd src && wails3 generate syso -icon static/favicon.ico -manifest wails.exe.manifest -out wails_windows.syso -arch amd64
```

`src/wails_windows.syso` is a compiled Windows resource file that embeds the app icon at resource ID 3, which is where Wails v3 looks for it. `src/wails.exe.manifest` is its required companion (embeds DPI awareness settings). Both are checked into the repo. If `static/favicon.ico` changes, the syso must be regenerated and the app rebuilt for the new icon to appear.

The Go backend has a test suite. The frontend has tests for pure JS business logic (no DOM) using the Node.js built-in test runner (`node:test`). Do not write tests for DOM operations, HTML, or browser-specific behaviour.

AI integration tests live in `src/ai_integration_test.go` and are gated behind the `integration` build tag so they are excluded from normal runs. They make real API calls to OpenAI, Anthropic, Google, and/or Mistral. **Always ask the user for explicit permission before running them.**

- **Frontend test location:** `src/static/tests/`
- **Run frontend tests:** `node --test "src/static/tests/*.test.js"` does not expand reliably in PowerShell. Prefer an explicit file list such as `$files = Get-ChildItem 'src/static/tests' -Filter '*.test.js' | ForEach-Object { $_.FullName }; node --test $files`
- **Pure utility functions** that have no DOM dependencies live in `lexicon-utils.js` and `drill-state.js` and are the primary target for frontend tests. Current exports under test include `isKanji`, `esc`, `renderReading`, `timeAgo`, `getSortedWords`, the detail item HTML builders (`detailItemPosSelect`, `detailItemKanjiReadings`, `detailItemInput`, `detailItemExInput`), and drill state helpers such as `createDrillState`, `matchesFilter`, `getFilteredWords`, `createSidebarItems`, `applySidebarAnswer`, `isSessionComplete`, `buildRoundState`, `getNextRevealState`, and `serializeSessionState`.

## Lexicon Features

- **Add words flow** � the user pastes Japanese words into an "add words" modal; the backend streams results back via SSE, adding words one by one and displaying them in the add-result modal. Implemented:
  - Words are normalised to their dictionary base form via `morphology.go` (e.g. conjugated verbs ? dictionary form) to prevent duplicates across inflections
  - Duplicates (same base form already in lexicon) are silently skipped with a reason badge
  - Initial word info now comes from the bundled local dictionary (`dict/jdict.db`, built from JMdict + Kanjidic) when those fields are not already provided by a higher-priority source
  - Word-list adds use word-list metadata first for reading / part of speech / meaning, with dictionary lookup filling missing pieces and supplying kanji info
  - Story auto-inserted untracked words (`tracked=0`) also try to pick up dictionary reading / part of speech / meaning / kanji info at insert time
  - AI auto-generates reading (hiragana), meaning (English), and example sentence (JP + EN) � see `ai.go` and the `/api/words/{id}/autofill` endpoint
  - AI autofill remains an explicit user-requested action for richer or regenerated word info; it is not the default initial source for newly inserted words
  - All generated fields are editable inline in the add-result modal; changes are auto-saved on blur via `PATCH /api/words/{id}`

- **Edit words** � clicking the ? button on any lexicon row opens the same add-result modal with just that word, allowing the user to edit reading, part of speech, meaning, and example sentences. Changes auto-save on blur.

- **Part of speech (POS)** � the current category set (`godan-verb`, `ichidan-verb`, `noun`, `i-adjective`, `na-adjective`, `adverb`, `other`) may need revisiting: check whether the categories cover all desired word types and that AI autofill is classifying words accurately. The canonical list lives in `typeLabels` in `lexicon.js`.

- **Audio** — selecting "audio" in the generate-type dropdown and clicking generate calls `POST /api/words/{id}/generate-audio`, which synthesizes WAV files via the local VoiceVox engine (must be running at `http://localhost:50021`). Word audio is saved as `static/audio/{word}.wav`; sentence audio as `static/audio/{word}_sentence.wav`. The DB stores `has_word_audio` and `has_sentence_audio` boolean flags (not paths, which are derivable from the word text). When a word or sentence is played, the WAV file is used if the flag is set; otherwise falls back to Web Speech API TTS.

- **Note:** `/api/words/{id}/reroll-meaning` and `/api/words/{id}/reroll-examples` may be dead code � the old edit modal that used them was removed (commit `f119e10`). Confirm before adding new callers.


## Stories Features


- **Generate translation** — the story detail page has a "Generate translation" button that opens a confirmation modal with an AI provider selector (same provider/model list as the lexicon add-edit modal). On confirm, calls `POST /api/stories/{id}/generate-translation` with `{ai_model}`. The backend sends all sentences as a plain ordered string array plus unique base words not already in the lexicon with meanings; the AI returns a matching array of literal English sentence translations plus `{word, gloss, reading}` entries for the requested story words. Translations are stored in `story_sentences.english_text`; per-word gloss/reading metadata is merged into `stories.word_glosses`. Response is NDJSON: `{status, sentenceCount, wordCount}` immediately, then `{allDone:true}` on success. Literal translation over natural English is intentional — sentences are translated individually with no cross-sentence context, which is the desired behaviour for language learning.

- **Noted words** — story detail supports a per-story saved list of "noted words". The list is persisted on the `stories` row as JSON (`noted_words_json`) and exposed on `GET /api/stories/{id}` as `notedWords`. `POST /api/stories/{id}/noted-words` adds one token's base word/display word to that JSON list if the word exists in the story; `DELETE /api/stories/{id}/noted-words` removes by `baseWord`. The frontend uses clicking a hovered word token to add items and the sidebar `✕` buttons to remove them.
- **Add all to Lexicon** — the story detail noted-words sidebar button now batch-adds the current noted base words through the same `/admin/words/batch` flow used by the lexicon add-words modal, then opens a story-local edit-results modal so the user can immediately edit readings, POS, meanings, examples, targets, and per-word generate/remove actions for the added or skipped words.

- **Sentence play button** — on the story detail page, hovering a sentence shows a small floating play button (`.sentence-play-btn`, `position: fixed`, created dynamically in `story.js`) above the first word of that sentence. Clicking it seeks audio (or TTS) to the start of that sentence and begins playback if not already playing. The button fades out after ~2.5 s even with no further interaction; hovering the button itself pauses the fade timer. Moving to another sentence immediately repositions it. Existing sentence- and word-click behaviour is unchanged.

- **On-the-fly VoiceVox synthesis (synth mode)** — when VoiceVox is detected available at page load, story playback switches to *synth mode* (`initSynthPlayback` in `story-playback.js`). In synth mode, audio is synthesized on demand via `POST /api/voicevox/synthesize` rather than played from pre-generated `.ogg` files. Playback proceeds clause by clause: `splitByClause(sentence)` splits each sentence's word-token array into sub-arrays by breaking after any token whose `displayWord` contains a Japanese comma (`、`); sentences with no commas are treated as a single clause. As each clause begins playing, the next clause starts synthesizing in the background via `prefetchClause`. When the last clause of a sentence starts playing, the next sentence is split and its first clause is prefetched. Synthesized blob URLs are cached in `synth-cache.js` (LRU, max 100 entries) so replayed or prefetched clauses are served instantly. The `synthGeneration` counter in state prevents stale async synthesis from auto-playing after the user stops. `audioSentenceIdx` and `audioClauseIdx` in state track the current position; stopping preserves both so resuming restarts from the same clause. The plan is to apply the same on-the-fly VoiceVox strategy to other pages (lexicon, drill, etc.) in future work.

- **Playback speed control** — the story detail page playback speed stepper applies to both browser TTS and on-the-fly VoiceVox synthesis. The visible speed value is display-only; users adjust it with the `−` and `+` buttons rather than typing into a focused field, and holding either button repeats the change at a fixed interval. Generated audio updates speed immediately via `playbackRate`; browser TTS does not, so if TTS playback is already running when the speed changes, the page immediately restarts TTS at the current position so the new rate takes effect without needing a manual pause/resume.

- **Story length limit and auto-split (planned)** — when a story is added that exceeds a sentence-count threshold (exact limit TBD, roughly 50–75 sentences), the add flow should detect the overrun and ask the user to confirm auto-splitting. If confirmed, the story is split into roughly equal chunks and inserted as separate stories titled `"{Title} (1 / N)"`, `"{Title} (2 / N)"`, etc. This keeps translation quality high (shorter stories translate better) and keeps the UI manageable. Users can always split long source text manually before adding.

## Frontend Pages

The HTML/CSS/JS frontend files live in `src/static/` and are served by the backend.

- **drill.html** � the drill view
- **lexicon.html** � the lexicon/word management view
- **activity.html** � the activity/stats view
- **stories.html** - stories index page with the shared app header/nav; currently lists story titles and dates
- **story.html** - story detail page for `/stories/{id}`; renders the title and reconstructed story text from sentence word tokens
- **tutor.html** - AI tutor chat page at `/tutor`. Header has a mode select (Free Conversation, Grammar Tutor, Vocabulary Quiz, Translation Practice, Reading Practice) and an AI provider/model select (same PROVIDER_MODELS pattern as other pages). Chat UI shows assistant messages on the left and user messages on the right; Enter sends, Shift+Enter inserts a newline. Uses `tutor.js` and `tutor.css`. Backend endpoint: `POST /api/tutor/chat`.
- **tts-demo.html** � sandbox page for testing VoiceVox TTS audio generation (not a production view)

### Backend

`src/` is a standalone Go module (separate `go.mod`) that runs a SQLite-backed HTTP server on port **49200** (dynamic/private range). `main.go` also launches a Wails v3 desktop window on the main goroutine; the web server runs in a background goroutine. The Wails window loads `http://localhost:49200/welcome` directly — no Wails JS runtime or Go↔JS bridge is used.

- **`main.go`** \u{FFFD} entry point; opens the DB, starts the web server as a goroutine, then runs the Wails v3 desktop window on the main thread. Key window options: `ZoomControlEnabled` (lets Ctrl+scroll reach JS), `DevToolsEnabled`, and key bindings for Ctrl+R / Ctrl+Shift+R reload. Supports both a `--server-only` CLI flag and a `SERVER_ONLY=true` environment variable to skip launching the Wails window and run the web server only on the main goroutine; `.air.toml` uses the env var approach for hot-reload.
- **`db_schema.go`** � `initDB`, `migrate`, `resetDB`, `seedDB`, and schema-introspection helpers (`listTableInfos`, `queryTable`, etc.). No SQL appears outside the `db_*.go` files.
- **`db_words.go`** � all word and kanji database operations: insert, update, delete, list, upsert kanji.
- **`dict.go` / `dict_lookup.go`** � local bundled dictionary support. `dict.go` handles finding/opening the read-only SQLite dictionary and background decompression of `jdict.db.gz`; `dict_lookup.go` looks up word reading / part of speech / glosses from JMdict and kanji meanings / readings from Kanjidic.
- **`db_activity.go`** � drill session persistence and answer recording (`createDrillSession`, `getCurrentDrillSession`, `recordDrillAnswer`), plus activity stats and calendar queries.
- **`db_settings.go`** � user settings: `getDrillSettings` and `putDrillSettings` read/write the `user_settings` table using key/value pairs. `drillSettings` always returns fully-populated values with no null fields � `MaxWords` defaults to `100`, `RoundSize` to `10`, `WordTypes` to all four types. `MaxWords` is always = 1; `0` is not a valid value. The frontend should treat the `GET /api/settings/drill` response as always having concrete values and needs no null-handling.
- **`db_stories.go`** - story persistence helpers. `insertStory` creates a titled story plus its ordered sentence rows in one transaction; `listStories` returns stories with nested sentence data for the stories page. Sentence Japanese content is stored as `words_json` token data rather than duplicated raw sentence text. Also contains: `setSentenceEnglishText` (update one sentence translation), structured story word-gloss merge/read helpers for the story-level `word_glosses` JSON map (English gloss plus optional reading, with legacy plain-string gloss JSON still readable), and the Kagome-based helper used during seeding to turn raw Japanese story sentences into token arrays with display/base forms and provisional glosses.
- **`db_stories.go`** also contains `deleteStory`, which removes a story, its sentence rows, and any generated `static/audio/story_{id}` directory.
- **`db_stories.go`** also contains `buildStorySentencesFromText`, which splits pasted story content into paragraph-aware sentence rows for new story creation.
- **`routes.go`** � `serverInit` (router setup), activity/drill/admin HTTP handlers, and template render helpers. No direct DB access; handlers call functions from the `db_*.go` files.
- **`routes_tutor.go`** — tutor API handlers: `POST /api/tutor/chat` (parses `{ai_model, tutor_mode, messages}`, looks up system prompt via `tutorSystemPromptByID` in `db_tutor.go`, calls `tutorChat`, returns `{reply}`); `GET /api/tutor/prompts`; `POST /api/tutor/prompts`; `DELETE /api/tutor/prompts/{id}`.
- **`routes_stories.go`** — story API handlers: `GET /api/stories`, `GET /api/stories/{id}`, `POST /api/stories/{id}/noted-words`, `DELETE /api/stories/{id}/noted-words`, `POST /api/stories/{id}/generate-audio` (VoiceVox, NDJSON streaming), `POST /api/stories/{id}/generate-translation` (AI translation, NDJSON streaming).
- **`routes_words.go`** — word and kanji API handlers: GET/PATCH/DELETE words, single and batch autofill, reroll meaning/examples, GET kanji.
- **`routes_handlers_test.go`** � HTTP handler tests for backend JSON endpoints, focused on request validation, status codes, and basic success-path responses for word, drill-session, and drill-settings APIs.
- **`ai.go`** - Shared AI types, prompts, few-shot examples, and provider-dispatch functions (`autoFillWord`, `autoFillWordsBatch`, `rerollMeaning`, `rerollExamples`, `translateStory`). `autoFillWordsBatch` splits words into chunks of `autoFillBatchSize` (20) and runs chunks concurrently, each as a single AI call returning a JSON array. `translateStory` sends all story sentences and unlexiconed words in one call, returning ordered translation strings plus per-word gloss and reading data. No direct DB access. Environment variables: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `MISTRAL_API_KEY`, `GLM_API_KEY`.
- **`ai_anthropic.go`** — Anthropic Messages API: `callAnthropic` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_openai.go`** — OpenAI Chat Completions API: `callOpenAI` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_google.go`** — Google Generative Language API: `callGoogle` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_mistral.go`** — Mistral Chat API (OpenAI-compatible): `callMistral` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_glm.go`** — Zhipu GLM API (OpenAI-compatible): `callGLM` HTTP helper + single and batch autofill/reroll implementations. Environment variable: `GLM_API_KEY`.
- **`morphology.go`** � word normalisation to dictionary base form (used in the add-words flow).
- **`wordlists.go`** � loads JSON word-list files from the embedded `wordlists/` directory at startup (via `//go:embed wordlists`). Exposes `apiGetWordLists` and `apiGetWordListWords` handlers. Each file is `{slug}.json` with `name` (display name) and `words` (string array) fields. Current lists: `animals`, `colors`, `ichidan-verbs`.
- **Dictionary source precedence for initial word info** � explicit caller-provided values (for example word-list metadata) win first, then local dictionary lookup fills missing reading / part-of-speech / meaning / kanji info, and AI autofill only runs when explicitly requested by the user.
- **Bundled dictionary assets** � the repo includes `dict/jdict.db`, `dict/jdict.db.gz`, `dict/jmdict-eng.json`, `dict/kanjidic2-en.json`, and `dict/build-db/main.go`. Runtime lookups are local SQLite reads only; no external API is involved.
- **`templates/`** - HTML templates parsed from disk on every request (live-editable without restart). `base.html` is the admin shell; `app_nav.html` is the shared top-nav partial used by the main app pages and includes the page links, the `?` app-logo link back to `/welcome`, and an inline script that handles Ctrl+scroll and Ctrl++/-/0 zoom (persisted via localStorage) for the Wails desktop window.
- **`static/images/words/`** � word images served statically. Files are named after the word text (e.g. `?.jpg`, `??.jpg`). The `image_path` column stores the path relative to `static/`; the frontend constructs the full URL as `/static/<image_path>`. Images are optional � words without images have `NULL` in `image_path`. Images are displayed in the lexicon table as a fixed-width column on the far left; the image cell uses `rowspan="2"` to span both the main and example rows, with `object-fit: cover` and `vertical-align: middle`.
- **`static/`** � HTML pages, CSS, and JS, served from disk (live-editable without restart). The main app HTML pages (`welcome.html`, `activity.html`, `drill.html`, `lexicon.html`, `stories.html`, `story.html`) are rendered through Go's template engine so they can include shared partials such as the top-nav/app-logo cluster, while CSS/JS/assets are still served statically. Includes `admin.css` and `admin.js` for the admin UI (loaded by `templates/admin.html` and `templates/base.html`). JS files for the lexicon page are split across three files: `lexicon.js` (table state/rendering, sorting, delete modal, tooltip), `lexicon-add-edit.js` (add/edit result modal, autofill, status/footer, streaming), and `lexicon-utils.js` (pure helpers, shared `typeLabels`, and detail item HTML builders). `add-to-lexicon.js` now holds shared add-to-lexicon modal helpers used by both the lexicon and story add/edit flows, including row sorting, row detail/image HTML builders, shared editable-field event wiring, and common save/target-update helpers. The drill page is now split across `drill.js` (bootstrap/event wiring), `drill-state.js` (drill state transitions, serialization, and drill API helpers), and `drill-view.js` (DOM/render helpers). Answer network calls in `drill.js` are queued via a module-level `_answerQueue` promise chain so the UI is never blocked waiting for the server — the next card is shown immediately and answers are sent in order in the background. The stories index uses `stories.js` plus `stories.css` to render a title/date list from `GET /api/stories`; the detail page uses `story.js` plus `story.css` to fetch `GET /api/stories/{id}` and reconstruct sentence text from `word.displayWord`, and `story-add-to-lexicon.js` to inject and run the story-side add/edit result modal used by the noted-words "Add all to Lexicon" flow. `story.html` also loads `lexicon.css` so that story-side add/edit modal rendering matches the lexicon page's existing modal styling instead of maintaining a separate visual variant. Keep new drill changes aligned with that structure. Persisted drill session snapshots should contain durable progress/UI state only, not derived completion flags or settings-only values. `lexicon.js` must load first as `lexicon-add-edit.js` reads globals it defines (`words`, `defaultDrillTarget`, `typeLabels`, `reloadWords`, `renderTable`, `getSortedWords`). The lexicon table uses one `<tbody class="word-group">` per word (the `<table id="word-table">` has no static tbody in HTML); this allows `.word-group:hover` to treat both rows of a word � main row, example row, and the rowspan image cell � as a single hover target for background highlight and button visibility. The settings modal HTML (`#settings-modal-backdrop`) is injected into `<body>` at runtime by `injectSettingsModal()` in `common.js` � do not add it to any page's HTML. `common.js` also exports `playDing()`, a Web Audio API synthesized chime used to signal async completion (e.g. VoiceVox audio generation finishing); import and call it from any page that needs a completion sound.
- **`seed.json`** � fixture data loaded on first startup (or after a DB reset); contains `words`, `sessions`, and `stories`. Seed stories are authored as raw Japanese sentences plus paragraph-start flags; `seedDB` tokenizes those sentences with Kagome into the persisted `words_json` format during initialization.

Key API endpoints (beyond CRUD on `/api/words` and `/api/kanji`):

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/tutor/chat` | AI tutor chat turn; body: `{ai_model, tutor_mode, messages:[{role,content}]}`; returns `{reply}` |
| `GET` | `/api/providers` | Check which AI providers are configured/available |
| `GET` | `/api/wordlists` | List all word lists (slug, name, total count, in-lexicon count) |
| `GET` | `/api/wordlists/{slug}/words` | Words in the named list not yet in the lexicon |
| `GET` | `/api/stories` | Return all stories with title, date, and nested sentence/token data |
| `POST` | `/api/stories` | Create a story from a title plus pasted content; backend splits it into sentence rows and tokenizes them |
| `GET` | `/api/stories/{id}` | Return one story by id for the story detail page, including `notedWords` |
| `DELETE` | `/api/stories/{id}` | Delete a story, its sentence rows, and any generated story audio files |
| `POST` | `/api/stories/{id}/noted-words` | Add one story word to the story's persisted noted-words list; body: `{baseWord, displayWord}` |
| `DELETE` | `/api/stories/{id}/noted-words` | Remove one story word from the story's persisted noted-words list; body: `{baseWord}` |
| `PATCH` | `/api/words/{id}` | Update a word's reading, type, meaning, and example sentences |
| `PATCH` | `/api/words/{id}/target` | Update a word's target drill count |
| `POST` | `/api/words/{id}/upload-image` | Upload a local image file for a word; saves it under `static/images/words/` and replaces any previous word image |
| `POST` | `/api/words/autofill-batch` | AI-generate reading, meaning, and examples for multiple words at once; body: `{words:[{id,word}], ai_model}`; words are chunked (≤20 per AI call) and chunks run concurrently |
| `POST` | `/api/words/{id}/autofill` | AI-generate reading, meaning, and examples for a single word |
| `POST` | `/api/words/{id}/generate-audio` | Generate WAV audio via local VoiceVox engine; sets `has_word_audio`/`has_sentence_audio` flags |
| `POST` | `/api/words/{id}/reroll-meaning` | Regenerate just the meaning via AI *(may be unused � see Lexicon Features note)* |
| `POST` | `/api/words/{id}/reroll-examples` | Regenerate just the example sentences via AI *(may be unused � see Lexicon Features note)* |
| `GET` | `/api/drill/sessions/current` | Return the current in-progress drill session, if one exists |
| `POST` | `/api/stories/{id}/generate-translation` | AI-generate English sentence translations and word glosses for a story; body: `{ai_model}`; streams NDJSON |
| `POST` | `/api/drill/sessions` | Start a new drill session |
| `POST` | `/api/drill/sessions/{id}/answers` | Record an answer within a session |
| `GET` | `/api/settings/drill` | Retrieve saved drill defaults (maxWords, roundSize, wordTypes) |
| `PUT` | `/api/settings/drill` | Save drill defaults |

Run with hot-reload from the `src/` directory:

```bash
cd src && air
```

#### Database schema

> **No migration compatibility required.** During development it is fine to reset the database (`/admin` ? Reset DB) whenever the schema changes. Do not spend effort on backwards-compatible migrations or backfill logic at this stage.

Table definitions live in the `migrate()` function in `db_schema.go`. Schema is versioned via `PRAGMA user_version` � each entry in the migrations slice runs exactly once. Current tables:

- **`words`** — the lexicon; one row per word with reading, part of speech, meaning, example sentences, audio flags (`has_word_audio`, `has_sentence_audio` INTEGER booleans — paths are derived as `static/audio/{word}.wav` / `static/audio/{word}_sentence.wav`, not stored), an optional `image_path` (relative to `static/`, e.g. `images/words/食べる.jpg`), drill counts, target, timestamps, a `kanji_data` JSON column (array of `{id, reading}` linking to the `kanji` table), and `tracked INTEGER NOT NULL DEFAULT 1`. `word` column has a unique index. Because `word` is unique, image and audio files are named after the word text itself. `tracked=1` means the user has explicitly added the word; `tracked=0` means the word was auto-inserted from story content and hasn't been explicitly added yet.
- **`kanji`** � one row per kanji character with `character` and `meanings` (JSON array of English meanings). Readings (on/kun) are stored per-word in the `kanji_data` column of `words`, not on the kanji row itself. Served via `/api/kanji`.
- **`drill_sessions`** � one row per drill session with a `started_at` timestamp, a persisted durable UI/session snapshot in `state_json` (round/pool/redo/remaining/sidebar/last-answered state, but not derived completion flags or copied settings defaults), and `completed_at` for distinguishing the single active in-progress drill from completed sessions.
- **`drill_answers`** � one row per answer within a session; references `words` and `drill_sessions`; stores `correct` (0/1) and `answered_at`.
- **`user_settings`** � key/value store for user preferences. Current keys: `drill_max_words` (int, always = 1; absent from the table means the default of `100` is used), `drill_round_size` (int), `drill_word_types` (JSON string array).
- **`stories`** - one row per story with required `title`, optional narration `audio_path`, `story_words_json` (JSON array of unique base word strings for the story — used as the word-info lookup key against the `words` table at query time), `noted_words_json` JSON (an ordered array of `{displayWord, baseWord, english?, createdAt}` for the story's noted words), `has_audio`, and `created_at` timestamp.
- **`story_sentences`** - ordered sentence rows for each story; stores `words_json` (an ordered JSON array of `{displayWord, baseWord, english, reading, audioTimestampMs}` at API/read time; only display/base/timestamp are persisted per token), an optional English sentence translation, and `is_paragraph_start` for paragraph boundaries.
- **`tutor_prompts`** — one row per tutor mode; `label` (display name), `system_prompt` (full prompt text; built-in prompts have the shared JSON format prefix prepended at seed time; user-created prompts are stored verbatim), `greeting` (JSON string used as the opening message; special sentinels `__random_free__` and `__random_free_en__` trigger random topic greetings in the frontend), `lang_input` (`"ja"`, `"en"`, or `"mix"` — controls STT language for the mode), `can_remove` (0 for built-in seeded prompts, 1 for user-created custom prompts). Seeded by `seedTutorPrompts()` in `db_schema.go` whenever the table is empty.

The admin UI at `http://localhost:1338/admin` shows live table schemas (column names, types, PK/UNIQUE/NOT NULL flags) and row counts, and links through to full table data views.

### JavaScript conventions

- **No inline event handlers.** Do not use `onclick=`, `onmousedown=`, or other `on*` HTML attributes. Use `addEventListener` instead � either on the element directly (for static elements, added once at script load time) or immediately after setting `innerHTML` (for dynamically built elements).

- **Bundle element references into `els`, mutable state into `state`.** At the top of each page script, collect all DOM element references into a single `const els = { ... }` object (using `getElementById`/`querySelector`). Collect all mutable runtime state into a single `const state = { ... }` object with explicit initial values. Do not declare loose module-level `let` variables for state or element refs. Follow the pattern established in `drill.js` / `drill-view.js` / `drill-state.js`: `els` is built once at load time and never mutated structurally; `state` fields are updated in place as the page runs.

- **Prefer the shared hover tooltip system.** For simple hover help or timestamp/details overlays, use the shared `data-tooltip` / `data-tooltip-html` mechanism handled by `common.js` (`.lex-tooltip` in `common.css`) rather than native browser `title` tooltips, unless there is a specific accessibility or browser-behaviour reason to do otherwise.

### CSS organisation

Styles shared across pages belong in `common.css`, which is loaded first by all pages. Page-specific files only contain styles unique to that page. When adding new styles, prefer extending `common.css` over duplicating rules across page stylesheets. Current shared styles include: CSS reset, `body` base, page header, nav link, `.btn-header` (the header icon button), and the full modal system.

The edit modal uses two distinct field styles to signal interaction type:
- **Underline** (`border-bottom` only) � free-text editable fields (`.detail-input`)
- **Bordered button** (full border, rounded corners) � dropdown selects (`.detail-pos-select`)

When adding new fields to the word edit/add modal, follow whichever convention matches the field type. Do not use CSS `text-transform` or `letter-spacing` on `<select>` elements � browsers exclude these from intrinsic width calculations, causing text to overflow. Apply transforms in JS when generating `<option>` text instead.

## Recent updates

- The shared word edit modal now supports direct local image uploads in both the lexicon page and the story-side add-to-lexicon flow.
- Clicking a word image or empty image placeholder in that modal opens a standard file picker.
- The selected file uploads through `POST /api/words/{id}/upload-image`, is stored under `static/images/words/`, and replaces any previous image for that word.
- `add-to-lexicon.js` owns the shared click-to-upload modal image behavior used by both modal variants.

## Working conventions

- **Scope changes to this project directory.** Do not read or write files outside the project directory without explicit instruction.
- **Ask before touching unfamiliar files.** If a file has not been part of the current conversation and has not been recently discussed, confirm with the user before editing it. This applies especially to Go source files, config files, and anything outside `src/`.
- **Keep AGENTS.md current.** After any non-trivial change � new files, new endpoints, renamed functions, changed conventions, new features, or shifted architecture � proactively propose specific updates to this file. Do not wait to be asked. If you added a file, added an endpoint, or changed how something works, edit AGENTS.md immediately (but be sure to tell me when you).
- **Use `git mv` for all file moves and renames.** Never copy-and-delete or use the Write tool to recreate a file at a new path. Always use `git mv <old> <new>` so history is preserved.
