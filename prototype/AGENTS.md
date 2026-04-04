# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Japanese vocabulary drilling desktop app built with Go backend + web frontend. For now, assume a single user with no user login or need for security. The app has four top-level views:

- **Lexicon** � displays the user's full vocabulary set with word info (reading, part of speech, meaning, example sentences) and per-word correct/incorrect drill counts accumulated over all sessions.
- **Drill** � a round-based flashcard drill. Each round presents 10 words randomly drawn from the lexicon. The user marks each word as known or unknown; unknown words carry over to the next round alongside fresh picks. The current in-progress drill is persisted to SQLite so refreshing `/drill` or restarting the server restores the same round state, sidebar state, and last answered card.
- **Activity** � displays headline stats and a week-by-week calendar of recent drill activity. Each day cell shows how many words were drilled, added, and cleared. Clicking a day opens a detail modal listing the words involved.
- **Stories** - currently a stub top-level page at `/stories`; the shared page header navigation on the main app pages links to it alongside Drill, Activity, and Lexicon.

Word data (lexicon) is stored in a SQLite database. Per-word drill counts (correct/incorrect) are persisted there, and the current in-progress drill session is also stored in SQLite as a serialized state snapshot on the active `drill_sessions` row.

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
- **Frontend:** Vanilla JS, Vite 3.0.7
- **Database:** SQLite

## Commands

```bash
# Run with hot-reload (from backend/)
cd backend && air

# Run backend tests
cd backend && go test ./...

# Run AI integration tests (makes real API calls � ask the user for permission first)
cd backend && go test -tags integration ./...

# Go dependencies
go mod tidy
```

The Go backend has a test suite. The frontend has tests for pure JS business logic (no DOM) using the Node.js built-in test runner (`node:test`). Do not write tests for DOM operations, HTML, or browser-specific behaviour.

AI integration tests live in `backend/ai_integration_test.go` and are gated behind the `integration` build tag so they are excluded from normal runs. They make real API calls to OpenAI, Anthropic, Google, and/or Mistral. **Always ask the user for explicit permission before running them.**

- **Frontend test location:** `backend/static/tests/`
- **Run frontend tests:** `node --test "backend/static/tests/*.test.js"` does not expand reliably in PowerShell. Prefer an explicit file list such as `$files = Get-ChildItem 'backend/static/tests' -Filter '*.test.js' | ForEach-Object { $_.FullName }; node --test $files`
- **Pure utility functions** that have no DOM dependencies live in `lexicon-utils.js` and `drill-state.js` and are the primary target for frontend tests. Current exports under test include `isKanji`, `esc`, `renderReading`, `timeAgo`, `getSortedWords`, the detail item HTML builders (`detailItemPosSelect`, `detailItemKanjiReadings`, `detailItemInput`, `detailItemExInput`), and drill state helpers such as `createDrillState`, `matchesFilter`, `getFilteredWords`, `createSidebarItems`, `applySidebarAnswer`, `isSessionComplete`, `buildRoundState`, `getNextRevealState`, and `serializeSessionState`.

## Lexicon Features

- **Add words flow** � the user pastes Japanese words into an "add words" modal; the backend streams results back via SSE, adding words one by one and displaying them in the add-result modal. Implemented:
  - Words are normalised to their dictionary base form via `morphology.go` (e.g. conjugated verbs ? dictionary form) to prevent duplicates across inflections
  - Duplicates (same base form already in lexicon) are silently skipped with a reason badge
  - AI auto-generates reading (hiragana), meaning (English), and example sentence (JP + EN) � see `ai.go` and the `/api/words/{id}/autofill` endpoint
  - All generated fields are editable inline in the add-result modal; changes are auto-saved on blur via `PATCH /api/words/{id}`

- **Edit words** � clicking the ? button on any lexicon row opens the same add-result modal with just that word, allowing the user to edit reading, part of speech, meaning, and example sentences. Changes auto-save on blur.

- **Part of speech (POS)** � the current category set (`godan-verb`, `ichidan-verb`, `noun`, `i-adjective`, `na-adjective`, `adverb`, `other`) may need revisiting: check whether the categories cover all desired word types and that AI autofill is classifying words accurately. The canonical list lives in `typeLabels` in `lexicon.js`.

- **Audio** — selecting "audio" in the generate-type dropdown and clicking generate calls `POST /api/words/{id}/generate-audio`, which synthesizes WAV files via the local VoiceVox engine (must be running at `http://localhost:50021`). Word audio is saved as `static/audio/{word}.wav`; sentence audio as `static/audio/{word}_sentence.wav`. The DB stores `has_word_audio` and `has_sentence_audio` boolean flags (not paths, which are derivable from the word text). When a word or sentence is played, the WAV file is used if the flag is set; otherwise falls back to Web Speech API TTS.

- **Note:** `/api/words/{id}/reroll-meaning` and `/api/words/{id}/reroll-examples` may be dead code � the old edit modal that used them was removed (commit `f119e10`). Confirm before adding new callers.

## Frontend Pages

The HTML/CSS/JS frontend files live in `backend/static/` and are served by the backend.

- **drill.html** � the drill view
- **lexicon.html** � the lexicon/word management view
- **activity.html** � the activity/stats view
- **stories.html** - stub stories page with the shared app header/nav
- **tts-demo.html** � sandbox page for testing VoiceVox TTS audio generation (not a production view)

### Backend

`backend/` is a standalone Go module (separate `go.mod`) that runs a SQLite-backed HTTP server on port **1338**.

- **`main.go`** � entry point; opens the DB and starts the server
- **`db_schema.go`** � `initDB`, `migrate`, `resetDB`, `seedDB`, and schema-introspection helpers (`listTableInfos`, `queryTable`, etc.). No SQL appears outside the `db_*.go` files.
- **`db_words.go`** � all word and kanji database operations: insert, update, delete, list, upsert kanji.
- **`db_activity.go`** � drill session persistence and answer recording (`createDrillSession`, `getCurrentDrillSession`, `recordDrillAnswer`), plus activity stats and calendar queries.
- **`db_settings.go`** � user settings: `getDrillSettings` and `putDrillSettings` read/write the `user_settings` table using key/value pairs. `drillSettings` always returns fully-populated values with no null fields � `MaxWords` defaults to `100`, `RoundSize` to `10`, `WordTypes` to all four types. `MaxWords` is always = 1; `0` is not a valid value. The frontend should treat the `GET /api/settings/drill` response as always having concrete values and needs no null-handling.
- **`routes.go`** � `serverInit` (router setup), activity/drill/admin HTTP handlers, and template render helpers. No direct DB access; handlers call functions from the `db_*.go` files.
- **`routes_words.go`** — word and kanji API handlers: GET/PATCH/DELETE words, single and batch autofill, reroll meaning/examples, GET kanji.
- **`routes_handlers_test.go`** � HTTP handler tests for backend JSON endpoints, focused on request validation, status codes, and basic success-path responses for word, drill-session, and drill-settings APIs.
- **`ai.go`** - Shared AI types, prompts, few-shot examples, and provider-dispatch functions (`autoFillWord`, `autoFillWordsBatch`, `rerollMeaning`, `rerollExamples`). `autoFillWordsBatch` splits words into chunks of `autoFillBatchSize` (20) and runs chunks concurrently, each as a single AI call returning a JSON array. No direct DB access. Environment variables: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `MISTRAL_API_KEY`, `GLM_API_KEY`.
- **`ai_anthropic.go`** — Anthropic Messages API: `callAnthropic` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_openai.go`** — OpenAI Chat Completions API: `callOpenAI` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_google.go`** — Google Generative Language API: `callGoogle` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_mistral.go`** — Mistral Chat API (OpenAI-compatible): `callMistral` HTTP helper + single and batch autofill/reroll implementations.
- **`ai_glm.go`** — Zhipu GLM API (OpenAI-compatible): `callGLM` HTTP helper + single and batch autofill/reroll implementations. Environment variable: `GLM_API_KEY`.
- **`morphology.go`** � word normalisation to dictionary base form (used in the add-words flow).
- **`wordlists.go`** � loads JSON word-list files from the embedded `wordlists/` directory at startup (via `//go:embed wordlists`). Exposes `apiGetWordLists` and `apiGetWordListWords` handlers. Each file is `{slug}.json` with `name` (display name) and `words` (string array) fields. Current lists: `animals`, `colors`, `ichidan-verbs`.
- **`templates/`** - HTML templates parsed from disk on every request (live-editable without restart). `base.html` is the admin shell; `app_nav.html` is the shared top-nav partial used by the main app pages and includes both the page links and the `語` app-logo link back to `/welcome`.
- **`static/images/words/`** � word images served statically. Files are named after the word text (e.g. `?.jpg`, `??.jpg`). The `image_path` column stores the path relative to `static/`; the frontend constructs the full URL as `/static/<image_path>`. Images are optional � words without images have `NULL` in `image_path`. Images are displayed in the lexicon table as a fixed-width column on the far left; the image cell uses `rowspan="2"` to span both the main and example rows, with `object-fit: cover` and `vertical-align: middle`.
- **`static/`** � HTML pages, CSS, and JS, served from disk (live-editable without restart). The main app HTML pages (`welcome.html`, `activity.html`, `drill.html`, `lexicon.html`, `stories.html`) are rendered through Go's template engine so they can include shared partials such as the top-nav/app-logo cluster, while CSS/JS/assets are still served statically. Includes `admin.css` and `admin.js` for the admin UI (loaded by `templates/admin.html` and `templates/base.html`). JS files for the lexicon page are split across three files: `lexicon.js` (table state/rendering, sorting, delete modal, tooltip), `lexicon-add-edit.js` (add/edit result modal, autofill, status/footer, streaming), and `lexicon-utils.js` (pure helpers and detail item HTML builders). The drill page is now split across `drill.js` (bootstrap/event wiring), `drill-state.js` (drill state transitions, serialization, and drill API helpers), and `drill-view.js` (DOM/render helpers). Keep new drill changes aligned with that structure. Persisted drill session snapshots should contain durable progress/UI state only, not derived completion flags or settings-only values. `lexicon.js` must load first as `lexicon-add-edit.js` reads globals it defines (`words`, `defaultDrillTarget`, `typeLabels`, `reloadWords`, `renderTable`, `getSortedWords`). The lexicon table uses one `<tbody class="word-group">` per word (the `<table id="word-table">` has no static tbody in HTML); this allows `.word-group:hover` to treat both rows of a word � main row, example row, and the rowspan image cell � as a single hover target for background highlight and button visibility. The settings modal HTML (`#settings-modal-backdrop`) is injected into `<body>` at runtime by `injectSettingsModal()` in `common.js` � do not add it to any page's HTML.
- **`seed.json`** � fixture data loaded on first startup (or after a DB reset); contains `words` and `sessions` arrays

Key API endpoints (beyond CRUD on `/api/words` and `/api/kanji`):

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/providers` | Check which AI providers are configured/available |
| `GET` | `/api/wordlists` | List all word lists (slug, name, total count, in-lexicon count) |
| `GET` | `/api/wordlists/{slug}/words` | Words in the named list not yet in the lexicon |
| `PATCH` | `/api/words/{id}` | Update a word's reading, type, meaning, and example sentences |
| `PATCH` | `/api/words/{id}/target` | Update a word's target drill count |
| `POST` | `/api/words/autofill-batch` | AI-generate reading, meaning, and examples for multiple words at once; body: `{words:[{id,word}], ai_model}`; words are chunked (≤20 per AI call) and chunks run concurrently |
| `POST` | `/api/words/{id}/autofill` | AI-generate reading, meaning, and examples for a single word |
| `POST` | `/api/words/{id}/generate-audio` | Generate WAV audio via local VoiceVox engine; sets `has_word_audio`/`has_sentence_audio` flags |
| `POST` | `/api/words/{id}/reroll-meaning` | Regenerate just the meaning via AI *(may be unused � see Lexicon Features note)* |
| `POST` | `/api/words/{id}/reroll-examples` | Regenerate just the example sentences via AI *(may be unused � see Lexicon Features note)* |
| `GET` | `/api/drill/sessions/current` | Return the current in-progress drill session, if one exists |
| `POST` | `/api/drill/sessions` | Start a new drill session |
| `POST` | `/api/drill/sessions/{id}/answers` | Record an answer within a session |
| `GET` | `/api/settings/drill` | Retrieve saved drill defaults (maxWords, roundSize, wordTypes) |
| `PUT` | `/api/settings/drill` | Save drill defaults |

Run with hot-reload from the `backend/` directory:

```bash
cd backend && air
```

#### Database schema

> **No migration compatibility required.** During development it is fine to reset the database (`/admin` ? Reset DB) whenever the schema changes. Do not spend effort on backwards-compatible migrations or backfill logic at this stage.

Table definitions live in the `migrate()` function in `db.go`. Schema is versioned via `PRAGMA user_version` � each entry in the migrations slice runs exactly once. Current schema version: **12**. Current tables:

- **`words`** � the lexicon; one row per word with reading, part of speech, meaning, example sentences, audio flags (`has_word_audio`, `has_sentence_audio` INTEGER booleans — paths are derived as `static/audio/{word}.wav` / `static/audio/{word}_sentence.wav`, not stored), an optional `image_path` (relative to `static/`, e.g. `images/words/食べる.jpg`), drill counts, target, timestamps, and a `kanji_data` JSON column (array of `{id, reading}` linking to the `kanji` table). `word` column has a unique index. Because `word` is unique, image and audio files are named after the word text itself.
- **`kanji`** � one row per kanji character with `character` and `meanings` (JSON array of English meanings). Readings (on/kun) are stored per-word in the `kanji_data` column of `words`, not on the kanji row itself. Served via `/api/kanji`.
- **`drill_sessions`** � one row per drill session with a `started_at` timestamp, a persisted durable UI/session snapshot in `state_json` (round/pool/redo/remaining/sidebar/last-answered state, but not derived completion flags or copied settings defaults), and `completed_at` for distinguishing the single active in-progress drill from completed sessions.
- **`drill_answers`** � one row per answer within a session; references `words` and `drill_sessions`; stores `correct` (0/1) and `answered_at`.
- **`user_settings`** � key/value store for user preferences. Current keys: `drill_max_words` (int, always = 1; absent from the table means the default of `100` is used), `drill_round_size` (int), `drill_word_types` (JSON string array).

The admin UI at `http://localhost:1338/admin` shows live table schemas (column names, types, PK/UNIQUE/NOT NULL flags) and row counts, and links through to full table data views.

### JavaScript conventions

- **No inline event handlers.** Do not use `onclick=`, `onmousedown=`, or other `on*` HTML attributes. Use `addEventListener` instead � either on the element directly (for static elements, added once at script load time) or immediately after setting `innerHTML` (for dynamically built elements).

### CSS organisation

Styles shared across pages belong in `common.css`, which is loaded first by all pages. Page-specific files only contain styles unique to that page. When adding new styles, prefer extending `common.css` over duplicating rules across page stylesheets. Current shared styles include: CSS reset, `body` base, page header, nav link, `.btn-header` (the header icon button), and the full modal system.

The edit modal uses two distinct field styles to signal interaction type:
- **Underline** (`border-bottom` only) � free-text editable fields (`.detail-input`)
- **Bordered button** (full border, rounded corners) � dropdown selects (`.detail-pos-select`)

When adding new fields to the word edit/add modal, follow whichever convention matches the field type. Do not use CSS `text-transform` or `letter-spacing` on `<select>` elements � browsers exclude these from intrinsic width calculations, causing text to overflow. Apply transforms in JS when generating `<option>` text instead.

## Working conventions

- **Scope changes to this project directory.** Do not read or write files outside `D:\code\jpvocab\prototype\` without explicit instruction.
- **Ask before touching unfamiliar files.** If a file has not been part of the current conversation and has not been recently discussed, confirm with the user before editing it. This applies especially to Go source files, config files, and anything outside `backend/`.
- **Keep AGENTS.md current.** After any non-trivial change � new files, new endpoints, renamed functions, changed conventions, new features, or shifted architecture � proactively propose specific updates to this file. Do not wait to be asked. If you added a file, added an endpoint, or changed how something works, edit AGENTS.md immediately (but be sure to tell me when you).
- **Use `git mv` for all file moves and renames.** Never copy-and-delete or use the Write tool to recreate a file at a new path. Always use `git mv <old> <new>` so history is preserved.
